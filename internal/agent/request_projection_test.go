package agent

import (
	"strings"
	"testing"

	"aurago/internal/llm"
	openai "github.com/sashabaranov/go-openai"
)

func TestRequestBudgetStepFunProjectionPreservesOtherRoutes(t *testing.T) {
	reasoning := strings.Repeat("retained private reasoning ", 10000)
	messages := []openai.ChatCompletionMessage{
		{Role: "user", Content: "Build the forest"},
		{Role: "assistant", ReasoningContent: reasoning},
		{Role: "assistant", ReasoningContent: reasoning, ToolCalls: []openai.ToolCall{{ID: "call", Function: openai.FunctionCall{Name: "game_maker_file", Arguments: `{"operation":"read","path":"src/main.ts"}`}}}},
		{Role: "tool", ToolCallID: "call", Content: "source", Name: "game_maker_file"},
	}
	step := llm.ModelRoute{ProviderType: "stepfun", Model: "step-5-preview"}
	other := llm.ModelRoute{ProviderType: "deepseek", Model: "deepseek-reasoner"}
	budget := &RequestBudget{CompletionReserve: 1000, SafetyMargin: 256, MinimumSystem: 500, Routes: []RequestRouteBudget{{Limits: llm.ModelLimits{Route: step, ContextWindow: 8192}}, {Limits: llm.ModelLimits{Route: other, ContextWindow: 8192}}}}
	cache := newTokenCountCache(64)
	usage, err := budget.validate(messages, nil, cache)
	if err == nil || !usage[0].Fits || usage[1].Fits || usage[0].HistoryTokens >= usage[1].HistoryTokens {
		t.Fatalf("route accounting: %+v %v", usage, err)
	}
	if messages[1].ReasoningContent != reasoning || messages[2].ReasoningContent != reasoning {
		t.Fatal("accounting mutated private history")
	}
	budget.Routes = budget.Routes[:1]
	if !budget.historyWorkingSetFits(messages, 0, "system", nil, cache) {
		t.Fatal("working set charged unsent reasoning")
	}
	if budget.maxMessagesTokens(messages, cache) != usage[0].HistoryTokens {
		t.Fatal("token accounting paths disagree")
	}
	for _, route := range []llm.ModelRoute{{ProviderType: "openrouter", Model: "stepfun/step-5-preview"}, {ProviderType: "openai", BaseURL: "https://api.stepfun.ai/v1", Model: "step-5-preview"}} {
		if routeMessageTokens(messages[1], route, cache) != 0 {
			t.Error("reasoning-only message counted on StepFun route")
		}
	}
}
