package agent

import (
	"testing"

	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

func TestSanitizeReasoningDropsCompletedReasoningForOrdinaryProvider(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleAssistant, Content: "done", ReasoningContent: "old chain"},
		{Role: openai.ChatMessageRoleUser, Content: "next"},
	}
	got := sanitizeReasoningForContinuation(messages, "openai", "gpt-4.1-mini")
	if got[0].ReasoningContent != "" {
		t.Fatalf("completed reasoning was replayed: %q", got[0].ReasoningContent)
	}
}

func TestSanitizeReasoningKeepsOnlyLatestRequiredToolContinuation(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleAssistant, ReasoningContent: "old", ToolCalls: []openai.ToolCall{{ID: "old"}}},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "old", Content: "old result"},
		{Role: openai.ChatMessageRoleAssistant, ReasoningContent: "current", ToolCalls: []openai.ToolCall{{ID: "current"}}},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "current", Content: "current result"},
	}
	got := sanitizeReasoningForContinuation(messages, "openrouter", "minimax/minimax-m2")
	if got[0].ReasoningContent != "" || got[2].ReasoningContent != "current" {
		t.Fatalf("unexpected continuation reasoning: %#v", got)
	}
}

func TestSanitizeReasoningKeepsNewestBlockForEligibleFallback(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleAssistant, ReasoningContent: "old"},
		{Role: openai.ChatMessageRoleAssistant, ReasoningContent: "required", ToolCalls: []openai.ToolCall{{ID: "call-1"}}},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call-1", Content: "done"},
	}
	routes := []RequestRouteBudget{{Limits: llm.ModelLimits{Route: llm.ModelRoute{
		ProviderType: "minimax", Model: "MiniMax-M2.1",
	}}}}

	got := sanitizeReasoningForRequestRoutes(messages, routes, "openai", "gpt-4.1-mini")
	if got[0].ReasoningContent != "" {
		t.Fatalf("older reasoning was not cleared: %q", got[0].ReasoningContent)
	}
	if got[1].ReasoningContent != "required" {
		t.Fatalf("fallback-required continuation reasoning was removed: %q", got[1].ReasoningContent)
	}
}
