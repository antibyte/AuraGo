package llm

import (
	"errors"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestNormalizeJSONContentStripsThinkingAndCodeFence(t *testing.T) {
	got, err := NormalizeJSONContent("<thinking>private reasoning</thinking>\n```json\n{\"ok\":true}\n```")
	if err != nil {
		t.Fatalf("NormalizeJSONContent: %v", err)
	}
	if got != `{"ok":true}` {
		t.Fatalf("normalized JSON = %q", got)
	}
}

func TestJSONContentFromResponseRejectsLengthFinishReason(t *testing.T) {
	resp := openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
		FinishReason: openai.FinishReasonLength,
		Message:      openai.ChatCompletionMessage{Content: `{"partial":`},
	}}}
	_, err := JSONContentFromResponse(resp)
	if !errors.Is(err, ErrJSONCompletionTruncated) {
		t.Fatalf("error = %v, want ErrJSONCompletionTruncated", err)
	}
}

func TestNormalizeJSONContentRejectsEmptyAndInvalid(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want error
	}{
		{name: "empty", raw: "```json\n\n```", want: ErrJSONCompletionEmpty},
		{name: "invalid", raw: `not {valid`, want: ErrJSONCompletionInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeJSONContent(tc.raw)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}
