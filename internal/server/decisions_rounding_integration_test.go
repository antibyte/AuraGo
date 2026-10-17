package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/flows"
)

func TestDecisionRoundedHTTPFlowAndRouterAccounting(t *testing.T) {
	s, d, _, _ := routedDecisionFixture(t)
	s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
	s.Cfg.LLMRouter.Enabled = true
	s.Cfg.Budget.Enabled = true
	s.Cfg.Budget.DailyLimitUSD = 1
	s.Cfg.Budget.Enforcement = "full"
	s.BudgetTracker = budget.NewTracker(s.Cfg, s.Logger, t.TempDir())
	t.Cleanup(s.BudgetTracker.Flush)
	posts := 0
	invalid := false
	previous := http.DefaultTransport
	http.DefaultTransport = aiGatewayRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		var data []byte
		if r.Method == http.MethodGet {
			data = []byte(`{"data":[{"id":"microsoft/microsoft-decision-1","context_length":32768,"architecture":{"input_modalities":["text"],"output_modalities":["decisions"]}}]}`)
		} else {
			posts++
			var request struct {
				Questions map[string]struct {
					Criteria map[string]any `json:"criteria"`
				} `json:"questions"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				return nil, err
			}
			answers := map[string]any{}
			for id, q := range request.Questions {
				probabilities := map[string]float64{}
				for option := range q.Criteria {
					probabilities[option] = math.Round(100/float64(len(q.Criteria))) / 100
				}
				if len(q.Criteria) == 10 {
					for key := range probabilities {
						probabilities[key] = .04
					}
					probabilities["0"] = .62
				}
				if invalid {
					probabilities["0"] = 0
				}
				answers[id] = map[string]any{"type": "choice", "choice": "0", "probabilities": probabilities}
			}
			data, _ = json.Marshal(map[string]any{"model": "fixture/pinned-v1", "answers": answers, "usage": map[string]any{"input_tokens": 42, "output_tokens": 0, "cost": .012}})
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data))}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = previous })
	client, err := decisions.NewOpenRouterClient("", "rounding-fixture-only")
	if err != nil {
		t.Fatal(err)
	}
	d.generation.client = client
	question := func(n int) decisions.Question {
		q := decisions.Question{ID: "route", Type: "choice"}
		for i := 0; i < n; i++ {
			id := strconv.Itoa(i)
			q.Options = append(q.Options, decisions.Option{ID: id, Label: "Option " + id})
		}
		return q
	}
	mux := http.NewServeMux()
	registerDecisionsRoutes(mux, s)
	for _, n := range []int{3, 6, 10, 62} {
		body, _ := json.Marshal(decisions.Request{State: "fixture", Questions: []decisions.Question{question(n)}})
		request := httptest.NewRequest(http.MethodPost, "http://localhost/api/decisions/evaluate", bytes.NewReader(body))
		request.Header.Set("Origin", "http://localhost")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatalf("%d candidates: %d %s", n, response.Code, response.Body.String())
		}
	}
	flowRequest := flows.DecisionRequest{State: "fixture", Questions: []decisions.Question{question(10)}}
	if _, err := (flowDecisions{s}).Evaluate(context.Background(), flowRequest); err != nil {
		t.Fatal(err)
	}
	snapshot := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
	if _, err := snapshot.Evaluate(context.Background(), decisions.Request{State: "fixture", Questions: flowRequest.Questions}); err != nil {
		t.Fatal(err)
	}
	invalid = true
	_, err = (flowDecisions{s}).Evaluate(context.Background(), flowRequest)
	var nodeError *flows.NodeError
	if !errors.As(err, &nodeError) || nodeError.Code != "FLOW_AI_UNAVAILABLE" {
		t.Fatalf("malformed paid response remained retryable: %v", err)
	}
	if posts != 7 || math.Abs(s.BudgetTracker.CategorySpendUSD("decisions")-.072) > 1e-9 || math.Abs(s.BudgetTracker.CategorySpendUSD("routing")-.012) > 1e-9 {
		t.Fatalf("posts=%d decisions=%g routing=%g", posts, s.BudgetTracker.CategorySpendUSD("decisions"), s.BudgetTracker.CategorySpendUSD("routing"))
	}
}
