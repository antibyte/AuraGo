package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
)

func TestHelperJSONBudgetReservesReasoningAndHonorsCaps(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		output, context, want int
		reject                bool
	}{
		{"reasoning", 0, 0, llm.ReasoningOutputTokens, false},
		{"output_override", 2048, 0, 2048, false},
		{"context_cap", 0, 4096, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &mockChatClient{response: `{"ok":true}`}
			manager := &helperLLMManager{client: client, model: "step-3.7-flash", providerType: "openai", contextWindow: tc.context,
				route: llm.ModelRoute{ProviderType: "openai", Model: "step-3.7-flash", MaxOutputTokensOverride: tc.output}}
			_, err := manager.requestJSONResponse(context.Background(), "maintenance", "sample", "Return JSON.", "Synthetic sample.", 1500)
			if tc.reject {
				if err == nil || !strings.Contains(err.Error(), "json_completion_context_budget") || client.calls != 0 {
					t.Fatalf("oversized request: calls=%d err=%v", client.calls, err)
				}
				return
			}
			if err != nil || client.calls != 1 || client.lastReq.MaxTokens != tc.want {
				t.Fatalf("request: calls=%d output=%d err=%v; want %d", client.calls, client.lastReq.MaxTokens, err, tc.want)
			}
		})
	}
}

func TestHelperManagerRefreshesWhenProviderLimitsChange(t *testing.T) {
	ResetGlobalHelperLLMManager()
	t.Cleanup(ResetGlobalHelperLLMManager)
	cfg := &config.Config{}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = "helper"
	cfg.LLM.HelperProviderType = "openai"
	cfg.LLM.HelperBaseURL = "http://127.0.0.1:1/v1"
	cfg.LLM.HelperResolvedModel = "step-3.7-flash"
	cfg.Providers = []config.ProviderEntry{{ID: "helper", MaxOutputTokens: 2048, ContextWindow: 32768}}
	first := newHelperLLMManager(cfg, nil)
	if first == nil || first.route.MaxOutputTokensOverride != 2048 {
		t.Fatal("helper provider output override was not retained")
	}
	cfg.Providers[0].MaxOutputTokens = 4096
	second := newHelperLLMManager(cfg, nil)
	if second == first || second.route.MaxOutputTokensOverride != 4096 {
		t.Fatal("helper reused stale provider limits")
	}
	cfg.Agent.ContextWindow = 16384
	third := newHelperLLMManager(cfg, nil)
	if third == second || third.contextWindow != 16384 {
		t.Fatal("helper reused stale global context cap")
	}
}
