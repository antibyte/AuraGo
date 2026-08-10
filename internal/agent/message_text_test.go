package agent

import (
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestMessageText_MultiContentImageOnly(t *testing.T) {
	msg := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{
			{Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: "data:image/png;base64,AA=="}},
		},
	}
	if got := messageText(msg); got != "[image]" {
		t.Fatalf("expected %q, got %q", "[image]", got)
	}
}

func TestMessageAccountingIncludesStructuredToolCallsAndImageReserve(t *testing.T) {
	msg := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant,
		ToolCalls: []openai.ToolCall{{
			ID:   "call-structured",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "execute_shell",
				Arguments: `{"command":"Get-Process"}`,
			},
		}},
		MultiContent: []openai.ChatMessagePart{{
			Type:     openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{URL: "data:image/png;base64,AA==", Detail: openai.ImageURLDetailHigh},
		}},
	}
	accounting := messageTextWithReasoningForAccounting(msg)
	for _, required := range []string{"execute_shell", "Get-Process", "call-structured", "image_detail=high"} {
		if !strings.Contains(accounting, required) {
			t.Fatalf("accounting payload missing %q", required)
		}
	}
	if got := strings.Count(accounting, " image"); got != multimodalImageTokenReserve {
		t.Fatalf("image reserve markers = %d, want %d", got, multimodalImageTokenReserve)
	}
}

func TestMessageText_MultiContentTextAndImage(t *testing.T) {
	msg := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{
			{Type: openai.ChatMessagePartTypeText, Text: "hello"},
			{Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: "data:image/png;base64,AA=="}},
		},
	}
	if got := messageText(msg); got != "hello\n[image]" {
		t.Fatalf("expected %q, got %q", "hello\n[image]", got)
	}
}

func TestMessageText_DoesNotMergeReasoningIntoContent(t *testing.T) {
	msg := openai.ChatCompletionMessage{
		Role:             openai.ChatMessageRoleAssistant,
		Content:          "visible answer",
		ReasoningContent: "hidden chain",
	}
	if got := messageText(msg); got != "visible answer" {
		t.Fatalf("messageText = %q, want content only", got)
	}
	if got := messageTextWithReasoningForAccounting(msg); got != "visible answer\nhidden chain" {
		t.Fatalf("messageTextWithReasoningForAccounting = %q, want content plus reasoning", got)
	}
}

func TestMessageText_ReasoningOnlyIsNotVisibleContent(t *testing.T) {
	msg := openai.ChatCompletionMessage{
		Role:             openai.ChatMessageRoleAssistant,
		ReasoningContent: "hidden chain",
	}
	if got := messageText(msg); got != "" {
		t.Fatalf("messageText = %q, want empty for reasoning-only message", got)
	}
	if got := messageTextWithReasoningForAccounting(msg); got != "hidden chain" {
		t.Fatalf("messageTextWithReasoningForAccounting = %q, want reasoning for accounting", got)
	}
}
