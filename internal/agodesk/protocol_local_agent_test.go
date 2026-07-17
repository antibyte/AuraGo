package agodesk

import (
	"encoding/json"
	"testing"
)

func TestLocalAgentCapabilityIsNegotiatedOnlyWhenOffered(t *testing.T) {
	if !containsAgodeskTestString(DefaultCapabilities, CapabilityLocalAgent) {
		t.Fatalf("DefaultCapabilities missing %q", CapabilityLocalAgent)
	}
	withoutOffer := NegotiateCapabilities([]string{"chat.full_response"}, DefaultCapabilities)
	if containsAgodeskTestString(withoutOffer, CapabilityLocalAgent) {
		t.Fatalf("local.agent negotiated without client offer: %v", withoutOffer)
	}
	withOffer := NegotiateCapabilities([]string{CapabilityLocalAgent}, DefaultCapabilities)
	if !containsAgodeskTestString(withOffer, CapabilityLocalAgent) {
		t.Fatalf("local.agent not negotiated after client offer: %v", withOffer)
	}
}

func TestLocalAgentProtocolPayloadsRoundTrip(t *testing.T) {
	toolArguments := json.RawMessage(`{"path":"README.md"}`)
	parameters := json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)
	payload := LocalAgentLLMPayload{
		SessionID:       "agodesk:session-1",
		RequestID:       "{turn-1}:llm:2",
		ClientTimestamp: "2026-07-17T18:33:49Z",
		ProviderID:      "main",
		Model:           "test-model",
		Messages: []LocalAgentLLMMessage{
			{Role: "user", Content: "Read the file"},
			{
				Role: "assistant",
				ToolCalls: []LocalAgentLLMToolCall{{
					ID:        "call-1",
					Name:      "read_file",
					Arguments: toolArguments,
				}},
			},
			{Role: "tool", Content: "contents", Name: "read_file", ToolCallID: "call-1"},
		},
		Tools: []LocalAgentLLMTool{{
			Type: "function",
			Function: LocalAgentLLMFunction{
				Name:        "read_file",
				Description: "Read one file",
				Parameters:  parameters,
			},
		}},
	}
	env, err := NewEnvelope(TypeLocalAgentLLM, payload)
	if err != nil {
		t.Fatalf("NewEnvelope: %v", err)
	}
	var decoded LocalAgentLLMPayload
	if err := json.Unmarshal(env.Payload, &decoded); err != nil {
		t.Fatalf("unmarshal local.agent.llm: %v", err)
	}
	if decoded.RequestID != "{turn-1}:llm:2" ||
		decoded.ClientTimestamp != "2026-07-17T18:33:49Z" ||
		decoded.ProviderID != "main" ||
		decoded.Model != "test-model" {
		t.Fatalf("decoded request metadata = %+v", decoded)
	}
	if len(decoded.Messages) != 3 || decoded.Messages[1].ToolCalls[0].Name != "read_file" || decoded.Messages[2].ToolCallID != "call-1" {
		t.Fatalf("decoded messages = %+v", decoded.Messages)
	}
	if string(decoded.Tools[0].Function.Parameters) != string(parameters) {
		t.Fatalf("decoded parameters = %s, want %s", decoded.Tools[0].Function.Parameters, parameters)
	}

	resultEnv, err := NewEnvelope(TypeLocalAgentLLMResult, LocalAgentLLMResultPayload{
		SessionID: "agodesk:session-1",
		RequestID: "{turn-1}:llm:2",
		Success:   true,
		Message: &LocalAgentLLMMessage{
			Role: "assistant",
			ToolCalls: []LocalAgentLLMToolCall{{
				ID:        "call-2",
				Name:      "write_file",
				Arguments: json.RawMessage(`{"path":"out.txt","content":"ok"}`),
			}},
		},
		Usage: &LocalAgentLLMUsagePayload{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14},
	})
	if err != nil {
		t.Fatalf("NewEnvelope result: %v", err)
	}
	var result LocalAgentLLMResultPayload
	if err := json.Unmarshal(resultEnv.Payload, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.RequestID != "{turn-1}:llm:2" ||
		!result.Success ||
		result.Message == nil ||
		result.Message.ToolCalls[0].Name != "write_file" ||
		result.Usage.TotalTokens != 14 ||
		result.ErrorCode != nil ||
		result.ErrorMessage != nil {
		t.Fatalf("decoded result = %+v", result)
	}
	var resultShape map[string]interface{}
	if err := json.Unmarshal(resultEnv.Payload, &resultShape); err != nil {
		t.Fatalf("unmarshal result shape: %v", err)
	}
	if _, ok := resultShape["error_code"]; !ok || resultShape["error_code"] != nil {
		t.Fatalf("success result error_code = %#v", resultShape["error_code"])
	}
	if _, ok := resultShape["error_message"]; !ok || resultShape["error_message"] != nil {
		t.Fatalf("success result error_message = %#v", resultShape["error_message"])
	}
	messageShape, ok := resultShape["message"].(map[string]interface{})
	if !ok {
		t.Fatalf("success result message = %#v", resultShape["message"])
	}
	if content, ok := messageShape["content"]; !ok || content != "" {
		t.Fatalf("success result content = %#v", content)
	}
}

func TestLocalAgentResultErrorsCarryStableCodeAndRequestID(t *testing.T) {
	env, err := NewEnvelope(TypeLocalAgentRemoteToolResult, LocalAgentRemoteToolResultPayload{
		SessionID: "agodesk:session-1",
		RequestID: "req-error",
		Error: &LocalAgentErrorPayload{
			Code:    ErrorUnsupportedTool,
			Message: "Unsupported tool.",
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope: %v", err)
	}
	var result LocalAgentRemoteToolResultPayload
	if err := json.Unmarshal(env.Payload, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.RequestID != "req-error" || result.Error == nil || result.Error.Code != ErrorUnsupportedTool {
		t.Fatalf("decoded error result = %+v", result)
	}
}
