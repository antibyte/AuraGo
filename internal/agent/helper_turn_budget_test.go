package agent

import (
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"context"
	"errors"
	"github.com/sashabaranov/go-openai"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func newHelperTurnBudget(t *testing.T) *budget.Tracker {
	t.Helper()
	cfg := &config.Config{}
	cfg.Budget.Enabled = true
	cfg.Budget.DailyLimitUSD = 1
	cfg.Budget.Enforcement = "full"
	tracker := budget.NewTracker(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	t.Cleanup(tracker.Flush)
	return tracker
}

func TestHelperTurnBudgetDenialAndPhysicalRetryAccounting(t *testing.T) {
	const valid = `{"memory_analysis":{"facts":[],"preferences":[],"corrections":[],"pending_actions":[]},"activity_digest":{},"personality_analysis":{}}`
	for _, kind := range []string{"truncated", "transport"} {
		t.Run(kind, func(t *testing.T) {
			tracker := newHelperTurnBudget(t)
			calls := 0
			client := maintenanceCompletionTestClient{complete: func(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
				calls++
				if req.MaxTokens <= 0 || req.MaxTokens > 400 {
					t.Fatalf("provider cap escaped: %d", req.MaxTokens)
				}
				response := helperRecoveryResponse(valid, openai.FinishReasonStop)
				response.Model = "helper-fixture"
				response.Usage = openai.Usage{PromptTokens: 100 * calls, CompletionTokens: 20 * calls, TotalTokens: 120 * calls}
				if calls == 1 {
					if kind == "transport" {
						return response, errors.New("503 temporary provider unavailable")
					}
					response.Choices[0].FinishReason = openai.FinishReasonLength
				}
				return response, nil
			}}
			manager := &helperLLMManager{client: client, model: "helper-fixture", route: llm.ModelRoute{Model: "helper-fixture", ContextWindowOverride: 32768, MaxOutputTokensOverride: 400}, responseCache: map[string]string{}}
			input := &helperTurnPersonalityInput{Language: "English"}
			if _, err := manager.AnalyzeTurn(t.Context(), "Human source", "Assistant source", nil, nil, input, nil); err == nil || calls != 0 {
				t.Fatal("untracked batch sent an LLM request")
			}
			if _, err := manager.AnalyzeTurn(t.Context(), "Human source", "Assistant source", nil, nil, input, tracker); err != nil {
				t.Fatal(err)
			}
			usage := tracker.GetStatus().Models["helper-fixture"]
			if calls != 2 || usage.Calls != 2 || usage.InputTokens != 300 || usage.OutputTokens != 60 {
				t.Fatalf("physical usage lost: calls=%d usage=%+v", calls, usage)
			}
			if _, err := manager.AnalyzeTurn(t.Context(), "Human source", "Assistant source", nil, nil, input, tracker); err != nil || calls != 2 {
				t.Fatal("cache charged a physical request")
			}
			tracker.RecordCostForCategory("helper_turn", 2)
			if _, err := manager.AnalyzeTurn(t.Context(), "Human source", "Assistant source", nil, nil, input, tracker); err == nil || calls != 2 {
				t.Fatal("exhausted budget sent a batch request")
			}
		})
	}
}

func TestHelperTurnBudgetOverflowDefersWithoutProviderCall(t *testing.T) {
	tracker := newHelperTurnBudget(t)
	client := &mockChatClient{response: `{}`}
	manager := &helperLLMManager{
		client: client,
		model:  "helper-overflow-fixture",
		route: llm.ModelRoute{
			Model: "helper-overflow-fixture", ContextWindowOverride: 2048, MaxOutputTokensOverride: 64,
		},
	}
	_, err := manager.AnalyzeTurn(t.Context(), strings.Repeat("<& complete source > ", 1000), "Assistant source", nil, nil,
		&helperTurnPersonalityInput{Language: "English", CurrentUserMessage: strings.Repeat("<& complete source > ", 1000)}, tracker)
	if !errors.Is(err, errHelperBudgetUnavailable) || !strings.Contains(err.Error(), "json_completion_context_budget") {
		t.Fatalf("overflow did not carry the no-fallback budget category: %v", err)
	}
	if client.calls != 0 || tracker.GetStatus().Models[manager.model].Calls != 0 {
		t.Fatal("oversize source dispatched or billed a provider call")
	}
}
