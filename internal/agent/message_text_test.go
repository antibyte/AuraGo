package agent

import (
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

