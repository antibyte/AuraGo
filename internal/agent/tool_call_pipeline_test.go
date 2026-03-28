package agent

import (
	"errors"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestParseToolResponseUsesNativeToolCalls(t *testing.T) {
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				ToolCalls: []openai.ToolCall{
					{
						ID:   "call-1",
						Type: openai.ToolTypeFunction,
						Function: openai.FunctionCall{
							Name:      "execute_shell",
							Arguments: `{"command":"pwd"}`,
						},
					},
					{
						ID:   "call-2",
						Type: openai.ToolTypeFunction,
						Function: openai.FunctionCall{
							Name:      "filesystem",
							Arguments: `{"operation":"stat","file_path":"README.md"}`,
						},
					},
				},
			},
		}},
	}

	parsed := parseToolResponse(resp, nil, AgentTelemetryScope{})

	if !parsed.UseNativePath {
		t.Fatal("expected native tool path")
	}
	if parsed.ParseSource != ToolCallParseSourceNative {
		t.Fatalf("unexpected parse source: %s", parsed.ParseSource)
	}
	if parsed.ToolCall.Action != "execute_shell" {
		t.Fatalf("unexpected tool action: %s", parsed.ToolCall.Action)
	}
	if len(parsed.PendingToolCalls) != 1 {
		t.Fatalf("expected 1 queued native tool call, got %d", len(parsed.PendingToolCalls))
	}
}

func TestParseToolResponseStripsReasoningTagsBeforeFallbackParsing(t *testing.T) {
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Content: `<think>I should use a tool</think>
{"action":"execute_shell","command":"pwd"}`,
			},
		}},
	}

	parsed := parseToolResponse(resp, nil, AgentTelemetryScope{})

	if parsed.ToolCall.Action != "execute_shell" {
		t.Fatalf("unexpected tool action: %s", parsed.ToolCall.Action)
	}
	if parsed.ParseSource != ToolCallParseSourceReasoningCleanJSON {
		t.Fatalf("expected reasoning-clean parse source, got %s", parsed.ParseSource)
	}
	if parsed.SanitizedContent == parsed.Content {
		t.Fatal("expected sanitized content to differ after stripping reasoning tags")
	}
}

func TestRecoverFrom422TrimsMessagesAndRetries(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "sys"},
			{Role: openai.ChatMessageRoleUser, Content: "hello"},
			{Role: openai.ChatMessageRoleAssistant, Content: "working"},
			{Role: openai.ChatMessageRoleTool, Content: "tool result", ToolCallID: "call-1"},
		},
	}
	count := 0

	recovered, err := recoverFrom422(errors.New("422 Unprocessable Entity"), &count, &req, nil, nil, "Sync", AgentTelemetryScope{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !recovered {
		t.Fatal("expected 422 recovery to trigger")
	}
	if count != 1 {
		t.Fatalf("expected retry count to increment, got %d", count)
	}
	if got := req.Messages[len(req.Messages)-1].Content; got == "" {
		t.Fatal("expected recovery note to be appended")
	}
}

func TestRecoverFrom422WithPolicyHonorsCustomLimit(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "sys"},
			{Role: openai.ChatMessageRoleUser, Content: "hello"},
			{Role: openai.ChatMessageRoleAssistant, Content: "working"},
		},
	}
	count := 1

	recovered, err := recoverFrom422WithPolicy(RecoveryPolicy{MaxProvider422Recoveries: 1}, errors.New("422 Unprocessable Entity"), &count, &req, nil, nil, "Sync", AgentTelemetryScope{})

	if recovered {
		t.Fatal("did not expect recovery after custom retry budget was exhausted")
	}
	if err == nil {
		t.Fatal("expected custom retry budget to abort with error")
	}
}

func TestRecoverFrom422TreatsInvalidFunctionArgs400AsRecoverable(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "sys"},
			{Role: openai.ChatMessageRoleUser, Content: "hello"},
			{Role: openai.ChatMessageRoleAssistant, Content: "working"},
			{Role: openai.ChatMessageRoleTool, Content: "tool result", ToolCallID: "call-1"},
		},
	}
	count := 0

	recovered, err := recoverFrom422(errors.New("error, status code: 400, status: 400 Bad Request, message: invalid params, invalid function arguments json string, tool_call_id: call_function_123"), &count, &req, nil, nil, "Sync", AgentTelemetryScope{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !recovered {
		t.Fatal("expected invalid-function-args 400 to trigger recovery")
	}
	if count != 1 {
		t.Fatalf("expected retry count to increment, got %d", count)
	}
}

func TestRecoverFromEmptyResponseTrimsContextOnce(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "sys"},
			{Role: openai.ChatMessageRoleSystem, Content: "summary"},
			{Role: openai.ChatMessageRoleUser, Content: "u1"},
			{Role: openai.ChatMessageRoleAssistant, Content: "a1"},
			{Role: openai.ChatMessageRoleUser, Content: "u2"},
			{Role: openai.ChatMessageRoleAssistant, Content: "a2"},
			{Role: openai.ChatMessageRoleUser, Content: "u3"},
		},
	}
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{},
		}},
	}
	emptyRetried := false

	recovered := recoverFromEmptyResponse(resp, "", &req, &emptyRetried, nil, nil, AgentTelemetryScope{})

	if !recovered {
		t.Fatal("expected empty-response recovery to trigger")
	}
	if !emptyRetried {
		t.Fatal("expected emptyRetried flag to be set")
	}
	if len(req.Messages) != 6 {
		t.Fatalf("expected trimmed message count to be 6, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != openai.ChatMessageRoleSystem || req.Messages[1].Role != openai.ChatMessageRoleSystem {
		t.Fatal("expected system prompt and summary to be preserved")
	}
}

func TestRecoverFromEmptyResponseWithPolicyHonorsMinMessages(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "sys"},
			{Role: openai.ChatMessageRoleSystem, Content: "summary"},
			{Role: openai.ChatMessageRoleUser, Content: "u1"},
			{Role: openai.ChatMessageRoleAssistant, Content: "a1"},
			{Role: openai.ChatMessageRoleUser, Content: "u2"},
			{Role: openai.ChatMessageRoleAssistant, Content: "a2"},
			{Role: openai.ChatMessageRoleUser, Content: "u3"},
		},
	}
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{},
		}},
	}
	emptyRetried := false

	recovered := recoverFromEmptyResponseWithPolicy(RecoveryPolicy{MinMessagesForEmptyRetry: 8}, resp, "", &req, &emptyRetried, nil, nil, AgentTelemetryScope{})

	if recovered {
		t.Fatal("did not expect recovery below the custom minimum message threshold")
	}
	if emptyRetried {
		t.Fatal("did not expect emptyRetried flag to be set")
	}
}
