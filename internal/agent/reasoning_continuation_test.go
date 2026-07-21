package agent

import (
	"testing"

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
