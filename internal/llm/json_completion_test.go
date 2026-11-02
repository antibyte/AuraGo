package llm

import (
	"errors"
	"reflect"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestJSONResponseFormatForModelBoundsKnownCapabilities(t *testing.T) {
	for _, test := range []struct {
		provider, model string
		want            bool
	}{
		{"anthropic", "claude-sonnet-4-6", false},
		{"opencode-go", "MiniMax-M3", false},
		{"minimax", "MiniMax-M3", true},
		{"opencode-go", "glm-5", true},
		{"openai", "gpt-4o", true},
		{"openai", "claude-sonnet-4-6", true},
		{"aurago-local", "gpt-4o", false},
		{"openai", "gpt-fixture-unknown-json-model", false},
	} {
		t.Run(test.provider+"/"+test.model, func(t *testing.T) {
			before, known := CapabilitiesFromRegistry(test.provider, test.model)
			format := JSONResponseFormatForModel(test.provider, test.model)
			if (format != nil) != test.want || format != nil && format.Type != openai.ChatCompletionResponseFormatTypeJSONObject {
				t.Fatalf("unexpected JSON response format: %+v", format)
			}
			after, afterKnown := CapabilitiesFromRegistry(test.provider, test.model)
			if known != afterKnown || !reflect.DeepEqual(before, after) {
				t.Fatal("format projection mutated raw model metadata")
			}
		})
	}
}

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
