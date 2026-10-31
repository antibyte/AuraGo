package security

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestGuardianToolVerdictBudgetLeavesRoomForReasoning(t *testing.T) {
	for _, method := range []string{"evaluation", "clarification"} {
		t.Run(method, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request openai.ChatCompletionRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("decode request: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if request.MaxTokens < 4096 || request.MaxTokens > 8192 {
					t.Errorf("max_tokens = %d, want room for reasoning within the 8192 ceiling", request.MaxTokens)
				}
				// This provider needs more than the old 2048-token ceiling to
				// finish its reasoning and deliver a complete security verdict.
				choice := completionReply(openai.FinishReasonLength, "")
				if request.MaxTokens >= 4096 {
					choice = completionReply(openai.FinishReasonStop, "safe 3 routine fixture cleanup")
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{
					Choices: []openai.ChatCompletionChoice{choice},
					Usage:   openai.Usage{PromptTokens: 100, CompletionTokens: 4096, TotalTokens: 4196},
				})
			}))
			defer server.Close()
			guardian, _ := newTestGuardian(t, "block", server.URL)
			check := GuardianCheck{Operation: "filesystem", Parameters: map[string]string{"operation": "delete", "file_path": "synthetic/canary.txt"}}
			var result GuardianResult
			if method == "clarification" {
				result = guardian.EvaluateClarification(context.Background(), check)
			} else {
				result = guardian.Evaluate(context.Background(), check)
			}
			if result.Decision != DecisionAllow || result.TokensUsed != 4196 {
				t.Fatalf("complete verdict = %+v, want allow with measured usage", result)
			}
		})
	}
}
