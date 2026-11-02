package llm

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestAnthropicNonePreservesCachedToolCatalog(t *testing.T) {
	for _, choice := range []any{"none", map[string]any{"type": "none"}} {
		for _, tools := range [][]json.RawMessage{nil, {json.RawMessage(`{"type":"function","function":{"name":"ping","parameters":{"type":"object","properties":{}}}}`)}} {
			ant, err := translateOpenAIToAnthropic(openaiRequest{Model: "claude-sonnet-4-6", ToolChoice: choice, Tools: tools}, anthropicThinkingConfig{})
			if err != nil {
				t.Fatal(err)
			}
			if ant.ToolChoice == nil || ant.ToolChoice.Type != "none" || len(ant.Tools) != len(tools) {
				t.Fatalf("tool suppression changed catalog: choice=%+v tools=%d", ant.ToolChoice, len(ant.Tools))
			}
			if len(tools) > 0 && ant.Tools[0].CacheControl == nil {
				t.Fatal("tool suppression removed cache boundary")
			}
		}
	}
}

func TestAnthropicSpecialStopReasonsSurviveResponseTranslation(t *testing.T) {
	for _, test := range []struct{ native, want string }{
		{"refusal", "content_filter"}, {"pause_turn", "pause_turn"},
		{"model_context_window_exceeded", "length"}, {"future_reason", "future_reason"}, {"", ""},
	} {
		t.Run(test.native, func(t *testing.T) {
			mapped := mapAnthropicToOpenAI(anthropicResponse{StopReason: test.native})
			if got := *mapped.Choices[0].FinishReason; got != test.want {
				t.Fatalf("finish=%q, want %q", got, test.want)
			}
		})
	}
}

func TestAnthropicStreamUsageOnlyDeltaDoesNotFinish(t *testing.T) {
	sse := buildAnthropicSSE([]string{
		"event: message_start", `data: {"message":{"id":"msg_fixture","model":"claude-sonnet-4-6","usage":{"input_tokens":4,"output_tokens":0}}}`,
		"event: message_delta", `data: {"delta":{},"usage":{"output_tokens":1}}`,
		"event: content_block_delta", `data: {"index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		"event: message_delta", `data: {"delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
		"event: message_stop", `data: {}`,
	})
	var out bytes.Buffer
	if err := translateStreamEvents(strings.NewReader(sse), &out, "claude-sonnet-4-6", nil); err != nil {
		t.Fatal(err)
	}
	finishes, textSeen, usageOnly := 0, false, false
	for _, line := range strings.Split(out.String(), "\n") {
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok || data == "[DONE]" {
			continue
		}
		var chunk openaiStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			t.Fatal(err)
		}
		if chunk.Usage != nil && chunk.Usage.CompletionTokens == 1 {
			usageOnly = len(chunk.Choices) == 0 && chunk.Usage.PromptTokens == 4
		}
		for _, choice := range chunk.Choices {
			if choice.Delta != nil && choice.Delta.Content == "hello" {
				textSeen = true
			}
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				finishes++
				if !textSeen || *choice.FinishReason != "stop" {
					t.Fatalf("premature or incorrect finish: %+v", choice)
				}
			}
		}
	}
	if finishes != 1 || !usageOnly {
		t.Fatalf("finishes=%d usage-only preserved=%t", finishes, usageOnly)
	}
}
