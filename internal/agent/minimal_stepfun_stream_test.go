package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

func TestMinimalLoopStepFunStreamBudgetAndPrivateReasoning(t *testing.T) {
	for _, complete := range []bool{false, true} {
		t.Run(fmt.Sprintf("complete=%t", complete), func(t *testing.T) {
			const private = "synthetic private reasoning"
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req openai.ChatCompletionRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					return
				}
				if !req.Stream || req.MaxTokens != 12000 || len(req.Tools) != 0 {
					t.Errorf("source request lost stream, scope or provider cap: max_tokens=%d", req.MaxTokens)
				}
				for _, message := range req.Messages {
					if message.ReasoningContent != "" {
						t.Error("StepFun wire replayed private reasoning")
					}
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning\":%q}}]}\n\n", private)
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"export const game = 1;\"}}]}\n\n")
				if complete {
					fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
				}
			}))
			defer provider.Close()
			cfg := &config.Config{}
			cfg.Agent.ContextWindow = 65536
			cfg.LLM.Model, cfg.LLM.ProviderType, cfg.LLM.Provider = "step-5-preview", "stepfun", "fixture"
			cfg.Providers = []config.ProviderEntry{{ID: "fixture", Type: "stepfun", Model: cfg.LLM.Model, ContextWindow: 65536, MaxOutputTokens: 12000}}
			client := llm.NewClientFromProviderWithConfig(cfg, "stepfun", provider.URL+"/v1", "fixture", "")
			var checkpoint []openai.ChatCompletionMessage
			result, _, err := ExecuteMinimalLoop(context.Background(), client, cfg.LLM.Model, "Return source.", "Implement the game.", nil, &DispatchContext{Cfg: cfg}, nil, budgetTestLogger(), &MinimalLoopOptions{
				MaxToolRounds: 0, MaxOutputTokens: 16384, StreamText: true, PreserveReasoning: true,
				Checkpoint: func(messages []openai.ChatCompletionMessage) error {
					checkpoint = append([]openai.ChatCompletionMessage(nil), messages...)
					return nil
				},
			})
			if complete {
				if err != nil || result.Response != "export const game = 1;" {
					t.Fatalf("complete stream failed: %+v %v", result, err)
				}
			} else if !errors.Is(err, ErrIncompleteTextStream) || result.Response != "" || strings.Contains(err.Error(), private) {
				t.Fatalf("incomplete source or private reasoning escaped: %+v %v", result, err)
			}
			if len(checkpoint) == 0 || checkpoint[len(checkpoint)-1].ReasoningContent != private {
				t.Fatal("StepFun reasoning was not checkpointed privately")
			}
			if !complete && strings.Contains(checkpoint[len(checkpoint)-1].Content, "export const") {
				t.Fatal("interrupted source entered continuation")
			}
		})
	}
}
