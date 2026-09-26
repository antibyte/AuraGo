package agent

import (
	"context"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
)

func TestMinimalLoopEnforcesOutputBudget(t *testing.T) {
	for _, tc := range []struct {
		name, model                                  string
		requested, primaryLimit, fallbackLimit, want int
	}{
		{"default", "unknown-model", 0, 16000, 0, llm.ConservativeOutputTokens},
		{"reasoning", "step-5-preview", 0, 64000, 0, llm.ReasoningOutputTokens},
		{"source", "step-5-preview", 16384, 64000, 0, 16384},
		{"provider_cap", "step-5-preview", 16384, 6000, 0, 6000},
		{"fallback_cap", "step-5-preview", 16384, 64000, 3000, 3000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &minimalLoopRouteClient{routes: []llm.ModelRoute{{ProviderType: "openai", Model: tc.model, Primary: true, ContextWindowOverride: 65536, MaxOutputTokensOverride: tc.primaryLimit}}}
			if tc.fallbackLimit != 0 {
				client.routes = append(client.routes, llm.ModelRoute{ProviderType: "custom", Model: "fallback", ContextWindowOverride: 32768, MaxOutputTokensOverride: tc.fallbackLimit})
			}
			cfg := &config.Config{}
			cfg.Agent.ContextWindow = 65536
			profile, err := NewPreparedPromptProfile("test-output", "Return source.", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = ExecuteMinimalLoop(context.Background(), client, tc.model, "Return source.", "Implement the game.", nil, &DispatchContext{Cfg: cfg}, nil, budgetTestLogger(), &MinimalLoopOptions{MaxToolRounds: 0, MaxOutputTokens: tc.requested, PreparedPrompt: profile})
			if err != nil || len(client.requests) != 1 {
				t.Fatalf("request failed: calls=%d err=%v", len(client.requests), err)
			}
			req := client.requests[0]
			if req.MaxTokens != tc.want {
				t.Fatalf("sent output ceiling=%d, want %d", req.MaxTokens, tc.want)
			}
			budget, err := newRequestBudget(context.Background(), cfg, client, req, budgetTestLogger())
			if err != nil {
				t.Fatal(err)
			}
			usage, err := budget.validate(req.Messages, req.Tools, newTokenCountCache(32))
			if err != nil {
				t.Fatal(err)
			}
			for _, route := range usage {
				if !route.Fits || route.CompletionTokens != req.MaxTokens {
					t.Fatalf("wire budget differs from reserved budget: %+v", route)
				}
			}
		})
	}
}

func TestMinimalLoopOutputReserveCannotBypassContextLimit(t *testing.T) {
	client := &minimalLoopRouteClient{routes: []llm.ModelRoute{{Model: "fixture", Primary: true, ContextWindowOverride: 4096, MaxOutputTokensOverride: 32768}}}
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 4096
	_, _, err := ExecuteMinimalLoop(context.Background(), client, "fixture", "Return source.", "Implement the game.", nil, &DispatchContext{Cfg: cfg}, nil, budgetTestLogger(), &MinimalLoopOptions{MaxToolRounds: 0, MaxOutputTokens: 16384})
	if !IsContextBudgetExceeded(err) || len(client.requests) != 0 {
		t.Fatalf("oversized output reserve escaped: calls=%d err=%v", len(client.requests), err)
	}
}
