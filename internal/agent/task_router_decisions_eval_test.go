package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/decisions"
)

type decisionEvalCase struct{ ID, Language, Expected, Kind, Intent string }
type decisionEvalResult struct {
	ID        string   `json:"id"`
	Language  string   `json:"language"`
	Kind      string   `json:"kind"`
	Expected  string   `json:"expected"`
	Actual    string   `json:"actual"`
	Model     string   `json:"model"`
	Version   string   `json:"model_version"`
	LatencyMS float64  `json:"latency_ms"`
	CostUSD   *float64 `json:"cost_usd"`
	Error     string   `json:"error,omitempty"`
}
type decisionEvalReport struct {
	CorpusSHA256         map[string]string         `json:"corpus_sha256"`
	ProbabilityThreshold float64                   `json:"probability_threshold"`
	MarginThreshold      float64                   `json:"margin_threshold"`
	Results              []decisionEvalResult      `json:"results"`
	Confusion            map[string]map[string]int `json:"confusion_matrix"`
	Misroutes            int                       `json:"misroutes"`
	Abstentions          int                       `json:"abstentions"`
	Failures             int                       `json:"failures"`
	KnownCostUSD         float64                   `json:"reported_cost_usd"`
	UnknownCosts         int                       `json:"unknown_costs"`
	P50MS                float64                   `json:"latency_p50_ms"`
	P95MS                float64                   `json:"latency_p95_ms"`
}

func decisionEvaluationCorpus(t *testing.T) ([]decisionEvalCase, map[string]string) {
	t.Helper()
	var cases []decisionEvalCase
	versions := map[string]string{}
	for _, path := range []string{"../llm/testdata/task_router_eval.tsv", "testdata/task_router_decisions_edge_v1.tsv"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(data)
		versions[filepath.Base(path)] = hex.EncodeToString(hash[:])
		reader := csv.NewReader(bytes.NewReader(data))
		reader.Comma = '\t'
		reader.LazyQuotes = true
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		edge := strings.Contains(path, "edge_v1")
		if (!edge && len(rows) != 240) || (edge && len(rows) != 24) {
			t.Fatalf("unexpected corpus size: %s %d", path, len(rows))
		}
		for i, row := range rows {
			kind, intent := "baseline", row[len(row)-1]
			if edge {
				kind = row[2]
			}
			cases = append(cases, decisionEvalCase{ID: fmt.Sprintf("%s:%03d", filepath.Base(path), i+1), Language: row[0], Expected: row[1], Kind: kind, Intent: intent})
		}
	}
	return cases, versions
}

func runDecisionEvaluation(ctx context.Context, client *http.Client, origin, cookie string, cases []decisionEvalCase) decisionEvalReport {
	report := decisionEvalReport{ProbabilityThreshold: .60, MarginThreshold: .15, Confusion: map[string]map[string]int{}}
	var latencies []float64
	for _, sample := range cases {
		result := decisionEvalResult{ID: sample.ID, Language: sample.Language, Kind: sample.Kind, Expected: sample.Expected, Actual: "error"}
		payload, _ := json.Marshal(taskDecisionRequest(sample.Intent))
		callCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		request, err := http.NewRequestWithContext(callCtx, http.MethodPost, origin+"/api/decisions/evaluate", bytes.NewReader(payload))
		if err != nil {
			cancel()
			result.Error = "invalid_server"
			report.Results = append(report.Results, result)
			break
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", origin)
		if cookie != "" {
			request.Header.Set("Cookie", cookie)
		}
		started := time.Now()
		response, err := client.Do(request)
		if err != nil {
			result.Error = "transport_outcome_unknown"
		} else {
			data, readErr := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
			response.Body.Close()
			if readErr != nil || len(data) > 1<<20 {
				result.Error = "response_outcome_unknown"
			} else if response.StatusCode != http.StatusOK {
				result.Error = fmt.Sprintf("http_%d", response.StatusCode)
			} else {
				var decoded decisions.Response
				if json.Unmarshal(data, &decoded) != nil {
					result.Error = "response_outcome_unknown"
				} else {
					result.Model, result.Version, result.CostUSD = decoded.Model, decoded.ModelRevision, decoded.Usage.CostUSD
					classification, parseErr := parseTaskDecision(decoded)
					if errors.Is(parseErr, errTaskDecisionUncertain) {
						result.Actual = "abstain"
					} else if parseErr != nil {
						result.Error = "invalid_routing_response"
					} else {
						result.Actual = classification.Area()
						if result.Actual == "" {
							result.Actual = "abstain"
						}
					}
				}
			}
		}
		result.LatencyMS = float64(time.Since(started)) / float64(time.Millisecond)
		cancel()
		report.Results = append(report.Results, result)
		latencies = append(latencies, result.LatencyMS)
		if report.Confusion[result.Expected] == nil {
			report.Confusion[result.Expected] = map[string]int{}
		}
		report.Confusion[result.Expected][result.Actual]++
		if result.Actual == "abstain" {
			report.Abstentions++
		} else if result.Error == "" && result.Actual != result.Expected {
			report.Misroutes++
		}
		if result.CostUSD == nil {
			report.UnknownCosts++
		} else {
			report.KnownCostUSD += *result.CostUSD
		}
		if result.Error != "" {
			report.Failures++
			break
		} // Never replay an uncertain paid outcome, or continue an invalid setup.
	}
	if len(latencies) > 0 {
		sort.Float64s(latencies)
		report.P50MS = latencies[(len(latencies)-1)*50/100]
		report.P95MS = latencies[(len(latencies)-1)*95/100]
	}
	return report
}

func TestDecisionsRoutingEvaluationLive(t *testing.T) {
	if os.Getenv("AURAGO_DECISIONS_EVAL") != "1" {
		t.Skip("explicit AURAGO_DECISIONS_EVAL=1, server URL and case limit required; may incur provider cost")
	}
	origin := strings.TrimRight(os.Getenv("AURAGO_DECISIONS_EVAL_URL"), "/")
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" || (parsed.Scheme != "https" && !(parsed.Scheme == "http" && (parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "localhost" || parsed.Hostname() == "::1"))) {
		t.Fatal("provide a server origin with HTTPS, or HTTP loopback")
	}
	cases, versions := decisionEvaluationCorpus(t)
	limit, err := strconv.Atoi(os.Getenv("AURAGO_DECISIONS_EVAL_LIMIT"))
	if err != nil || limit < 1 || limit > len(cases) {
		t.Fatalf("explicit case limit must be 1..%d", len(cases))
	}
	selected := make([]decisionEvalCase, limit)
	for i := range selected {
		selected[i] = cases[i*len(cases)/limit]
	}
	client := &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	report := runDecisionEvaluation(context.Background(), client, origin, os.Getenv("AURAGO_DECISIONS_EVAL_COOKIE"), selected)
	report.CorpusSHA256 = versions
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join("..", "..", "reports", "decisions-evaluation")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(directory, "evaluation-*.json")
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal("write evaluation report failed")
	}
	t.Logf("report: %s; evaluated=%d misroutes=%d abstentions=%d unknown_costs=%d", file.Name(), len(report.Results), report.Misroutes, report.Abstentions, report.UnknownCosts)
	if report.Failures > 0 {
		t.Fatal("evaluation stopped on its first failed or uncertain request; inspect report before another run")
	}
}

func TestDecisionEvaluationUsesProductionRequestAndDoesNotRetry(t *testing.T) {
	cases, versions := decisionEvaluationCorpus(t)
	if len(cases) != 264 || len(versions) != 2 {
		t.Fatal("corpus changed")
	}
	for _, sample := range cases {
		if err := decisions.ValidateRequest(taskDecisionRequest(sample.Intent)); err != nil {
			t.Fatal(sample.ID, err)
		}
	}
	calls := 0
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request decisions.Request
		if json.NewDecoder(r.Body).Decode(&request) != nil || len(request.Questions) != 1 || request.Questions[0].ID != "task_area" || len(request.Questions[0].Options) != 10 || !strings.Contains(request.State.(string), "external_data") {
			t.Error("production request was bypassed")
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer fixture.Close()
	report := runDecisionEvaluation(context.Background(), fixture.Client(), fixture.URL, "", cases[:3])
	if calls != 1 || report.Failures != 1 || report.UnknownCosts != 1 || len(report.Results) != 1 {
		t.Fatalf("uncertain request retried or cost invented: %+v", report)
	}
}

func TestDecisionEvaluationRecordsMisroutesAbstentionsAndKnownCosts(t *testing.T) {
	calls := 0
	cost := .01
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		response := taskDecisionReply("coding", .8)
		if calls == 2 {
			response = taskDecisionReply("unknown", .8)
		}
		response.Model, response.ModelRevision = "fixture/alias", "fixture/version-1"
		response.Usage.CostUSD = &cost
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	defer fixture.Close()
	cases := []decisionEvalCase{{ID: "mismatch", Expected: "writing", Intent: "write a story"}, {ID: "uncertain", Expected: "coding", Intent: "unclear"}}
	report := runDecisionEvaluation(context.Background(), fixture.Client(), fixture.URL, "", cases)
	if calls != 2 || report.Failures != 0 || report.Misroutes != 1 || report.Abstentions != 1 || report.KnownCostUSD != .02 || report.UnknownCosts != 0 || report.Confusion["writing"]["coding"] != 1 || report.Confusion["coding"]["abstain"] != 1 || report.Results[0].Version != "fixture/version-1" || report.P95MS < report.P50MS {
		t.Fatalf("incorrect evaluation report: %+v", report)
	}
}
