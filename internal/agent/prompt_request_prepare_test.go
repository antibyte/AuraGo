package agent

import (
	"encoding/json"
	"testing"

	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

func promptRequestFinalizationTestBudget() *RequestBudget {
	return &RequestBudget{
		Routes: []RequestRouteBudget{{
			Limits: llm.ModelLimits{
				Route:         llm.ModelRoute{ProviderType: "openai", Model: "agnes-2.0-flash"},
				ContextWindow: 8192,
			},
		}},
		CompletionReserve: 512,
		SafetyMargin:      requestProtocolSafetyTokens,
	}
}

func TestFinalizePromptRequestForSendOmitsToolOptionsWithoutSchemas(t *testing.T) {
	tests := []struct {
		name              string
		toolChoice        any
		parallelToolCalls any
	}{
		{name: "auto and true", toolChoice: "auto", parallelToolCalls: true},
		{name: "none and false", toolChoice: "none", parallelToolCalls: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := openai.ChatCompletionRequest{
				Model:             "agnes-2.0-flash",
				Messages:          []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "hello"}},
				ToolChoice:        test.toolChoice,
				ParallelToolCalls: test.parallelToolCalls,
			}

			if _, err := finalizePromptRequestForSend(&req, promptRequestFinalizationTestBudget(), newTokenCountCache(32), "openai", nil); err != nil {
				t.Fatalf("finalizePromptRequestForSend() error = %v", err)
			}
			if req.ToolChoice != nil || req.ParallelToolCalls != nil {
				t.Fatalf("tool options = (%#v, %#v), want both nil", req.ToolChoice, req.ParallelToolCalls)
			}

			payload, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			var wire map[string]json.RawMessage
			if err := json.Unmarshal(payload, &wire); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if _, exists := wire["tool_choice"]; exists {
				t.Fatalf("tool_choice was serialized: %s", payload)
			}
			if _, exists := wire["parallel_tool_calls"]; exists {
				t.Fatalf("parallel_tool_calls was serialized: %s", payload)
			}
		})
	}
}

func TestFinalizePromptRequestForSendKeepsToolOptionsWithSchemas(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Model:    "agnes-2.0-flash",
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "check status"}},
		Tools: []openai.Tool{{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:       "status",
				Parameters: map[string]any{"type": "object"},
			},
		}},
		ToolChoice:        "auto",
		ParallelToolCalls: true,
	}

	if _, err := finalizePromptRequestForSend(&req, promptRequestFinalizationTestBudget(), newTokenCountCache(32), "openai", nil); err != nil {
		t.Fatalf("finalizePromptRequestForSend() error = %v", err)
	}
	if req.ToolChoice != "auto" || req.ParallelToolCalls != true {
		t.Fatalf("tool options = (%#v, %#v), want (auto, true)", req.ToolChoice, req.ParallelToolCalls)
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(payload, &wire); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if _, exists := wire["tool_choice"]; !exists {
		t.Fatalf("tool_choice missing from serialized request: %s", payload)
	}
	if _, exists := wire["parallel_tool_calls"]; !exists {
		t.Fatalf("parallel_tool_calls missing from serialized request: %s", payload)
	}
}
