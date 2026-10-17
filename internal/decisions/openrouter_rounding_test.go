package decisions

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"testing"
)

func TestOpenRouterRoundedDistributions(t *testing.T) {
	for _, n := range []int{3, 6, 10, 62} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			options := make([]Option, n)
			probabilities := make(map[string]float64, n)
			for i := range options {
				id := strconv.Itoa(i)
				options[i] = Option{ID: id, Label: "Option " + id}
				probabilities[id] = math.Round(100/float64(n)) / 100
			}
			if n == 10 {
				for key := range probabilities {
					probabilities[key] = 0.04
				}
				probabilities["0"] = 0.62
			}
			selected := "0"
			question := Question{ID: "route", Type: "choice", Options: options}
			posts := 0
			client := openRouterTestClient(t, "rounding-fixture-key", func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodGet {
					return openRouterHTTPResponse(200, textDecisionCatalog, nil), nil
				}
				posts++
				body, _ := json.Marshal(map[string]any{
					"model": "typesafe/jev-1.13", "answers": map[string]any{"route": map[string]any{"type": "choice", "choice": selected, "probabilities": probabilities}},
					"usage": map[string]any{"input_tokens": 42, "output_tokens": 0, "cost": 0.012},
				})
				return openRouterHTTPResponse(200, string(body), nil), nil
			})
			response, err := client.Evaluate(context.Background(), "typesafe/jev-1.13", Request{State: "fixture", Questions: []Question{question}})
			if err != nil || posts != 1 || response.Usage.CostUSD == nil || *response.Usage.CostUSD != 0.012 {
				t.Fatalf("response=%+v posts=%d error=%v", response, posts, err)
			}
			answer := response.Answers["route"]
			total := 0.0
			for _, p := range answer.Probabilities {
				total += p
			}
			if math.Abs(total-1) > 1e-9 || answer.Decision != selected || answer.Confidence != answer.Probabilities[selected] {
				t.Fatalf("invalid normalized answer: %+v", answer)
			}
			if n != 10 {
				question.Type = "score"
				score := float64(n-1) / 2
				if _, err := mapOpenRouterAnswer(question, options, openRouterAnswer{Type: "score", Score: &score, Probabilities: probabilities}); err != nil {
					t.Fatalf("rounded score rejected: %v", err)
				}
				score = 0
				if _, err := mapOpenRouterAnswer(question, options, openRouterAnswer{Type: "score", Score: &score, Probabilities: probabilities}); err == nil {
					t.Fatal("accepted a score incompatible with rounded probabilities")
				}
			}
		})
	}
}

func TestOpenRouterProbabilityValidationRemainsStrict(t *testing.T) {
	keys := []string{"a", "b", "c"}
	for name, values := range map[string]map[string]float64{
		"zero":           {"a": 0, "b": 0, "c": 0},
		"missing":        {"a": 0.5, "b": 0.5},
		"extra":          {"a": 0.33, "b": 0.33, "other": 0.34},
		"negative":       {"a": -0.01, "b": 0.5, "c": 0.51},
		"nan":            {"a": math.NaN(), "b": 0.5, "c": 0.5},
		"infinite":       {"a": math.Inf(1), "b": 0, "c": 0},
		"too much mass":  {"a": 0.35, "b": 0.35, "c": 0.35},
		"high precision": {"a": 0.338, "b": 0.338, "c": 0.338},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizedProbabilityMap(values, keys); err == nil {
				t.Fatal("accepted invalid distribution")
			}
		})
	}
}
