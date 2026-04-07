package agent

import (
	"errors"
	"strings"
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
				Content: "<thinking>I should use a tool</thinking>\n{\"action\":\"execute_shell\",\"command\":\"pwd\"}",
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

func TestParseToolResponseDetectsDoneTag(t *testing.T) {
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Content: "Die Demo läuft jetzt lokal auf http://192.168.6.238:8080 — viel Spaß! <done/>",
			},
		}},
	}

	parsed := parseToolResponse(resp, nil, AgentTelemetryScope{})

	if !parsed.IsFinished {
		t.Fatal("expected IsFinished=true when <done/> tag is present")
	}
	if parsed.ToolCall.IsTool {
		t.Fatal("did not expect a tool call from a completion message")
	}
	if strings.Contains(parsed.SanitizedContent, "<done/>") {
		t.Fatal("expected <done/> tag to be stripped from SanitizedContent")
	}
	if strings.Contains(parsed.SanitizedContent, "viel Spaß") == false {
		t.Fatal("expected actual message text to be preserved after stripping <done/>")
	}
}

func TestParseToolResponseNoTagIsNotFinished(t *testing.T) {
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Content: "Ich prüfe jetzt die Konfiguration.",
			},
		}},
	}

	parsed := parseToolResponse(resp, nil, AgentTelemetryScope{})

	if parsed.IsFinished {
		t.Fatal("expected IsFinished=false when <done/> tag is absent")
	}
}

func TestParseToolResponseDoneTagStrippedFromSanitized(t *testing.T) {
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Content: "Fertig! Die Dateien sind gespeichert. <done/>",
			},
		}},
	}

	parsed := parseToolResponse(resp, nil, AgentTelemetryScope{})

	if !parsed.IsFinished {
		t.Fatal("expected IsFinished=true")
	}
	if strings.Contains(parsed.SanitizedContent, "<done/>") {
		t.Fatalf("tag not stripped, SanitizedContent: %q", parsed.SanitizedContent)
	}
	if !strings.Contains(parsed.Content, "<done/>") {
		t.Fatal("expected raw Content to still contain the original tag (not stripped)")
	}
}

func TestParseToolCallMinimaxJSONFormat(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantAct string
		wantXML bool
	}{
		{
			name:    "minimax:tool_call JSON with action key",
			content: `Lass mich das prüfen:<minimax:tool_call>{"action":"execute_shell","command":"ls -la"}`,
			wantAct: "execute_shell",
			wantXML: true,
		},
		{
			name:    "minimax:tool_call JSON with tool_call key (MiniMax format)",
			content: `<minimax:tool_call>{"tool_call":"read_file","file_path":"/tmp/test.txt"}`,
			wantAct: "read_file",
			wantXML: true,
		},
		{
			name:    "minimax:tool_call JSON with name key",
			content: "<minimax:tool_call>\n{\"name\":\"execute_shell\",\"command\":\"echo hello\"}",
			wantAct: "execute_shell",
			wantXML: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseToolCall(tc.content)
			if !result.IsTool {
				t.Fatalf("IsTool=false, expected tool call to be parsed from: %q", tc.content)
			}
			if result.Action != tc.wantAct {
				t.Fatalf("Action=%q, want %q", result.Action, tc.wantAct)
			}
			if tc.wantXML && !result.XMLFallbackDetected {
				t.Fatal("XMLFallbackDetected=false, expected true for minimax:tool_call format")
			}
		})
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

func TestRecoverFromEmptyResponsePureThinkBlock(t *testing.T) {
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
	pureThinkContent := "<thinking>\nThe user wants me to update the file. Let me do that.\n</thinking>"
	emptyRetried := false

	recovered := recoverFromEmptyResponse(resp, pureThinkContent, &req, &emptyRetried, nil, nil, AgentTelemetryScope{})

	if !recovered {
		t.Fatal("expected pure think-block response to trigger empty-response recovery")
	}
	if !emptyRetried {
		t.Fatal("expected emptyRetried flag to be set")
	}
}

func TestRecoverFromEmptyResponseDoesNotTriggerForRealContent(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "sys"},
			{Role: openai.ChatMessageRoleUser, Content: "hello"},
		},
	}
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{},
		}},
	}
	contentWithThinkAndText := "<thinking>\nSome reasoning.\n</thinking>\n\nHere is my answer."
	emptyRetried := false

	recovered := recoverFromEmptyResponse(resp, contentWithThinkAndText, &req, &emptyRetried, nil, nil, AgentTelemetryScope{})

	if recovered {
		t.Fatal("should NOT trigger recovery when real content exists after think block")
	}
}

func TestParseToolCallBracketFormatSetsXMLFallbackDetected(t *testing.T) {
	content := `[TOOL_CALL]{tool => "homepage", args => {--url "http://example.com"}}[/TOOL_CALL]`
	result := ParseToolCall(content)
	if !result.IsTool {
		t.Fatal("expected IsTool=true for bracket format")
	}
	if result.Action != "homepage" {
		t.Fatalf("expected action=homepage, got %q", result.Action)
	}
	if !result.XMLFallbackDetected {
		t.Fatal("expected XMLFallbackDetected=true for bracket format tool call")
	}
}

func TestParseToolCallBracketFormatWithThinkingPreservesToolName(t *testing.T) {
	rawContent := ` Lass mich die Homepage aufrufen.
[TOOL_CALL]{tool => "homepage", args => {--url "http://example.com"}}
[/TOOL_CALL]`

	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Content: rawContent,
			},
		}},
	}

	parsed := parseToolResponse(resp, nil, AgentTelemetryScope{})

	if !parsed.ToolCall.IsTool {
		t.Fatal("expected IsTool=true for bracket format behind thinking tags")
	}
	if parsed.ToolCall.Action != "homepage" {
		t.Fatalf("expected action=homepage, got %q — tool name should not be overridden by secondary parser", parsed.ToolCall.Action)
	}
	if !parsed.ToolCall.XMLFallbackDetected {
		t.Fatal("expected XMLFallbackDetected=true for bracket format tool call")
	}
}

func TestParseToolCallBracketFormatInlineCloseTag(t *testing.T) {
	content := `[TOOL_CALL]{tool => "execute_shell", args => {--command "ls"}}[/TOOL_CALL]`
	result := ParseToolCall(content)
	if !result.IsTool {
		t.Fatal("expected IsTool=true for inline bracket format")
	}
	if result.Action != "execute_shell" {
		t.Fatalf("expected action=execute_shell, got %q", result.Action)
	}
}

func TestParseToolCallBracketFormatSingleQuotes(t *testing.T) {
	content := `[TOOL_CALL]{tool => 'homepage', args => {--operation "read_file" --path "phaser-demo/src/main.ts"}}[/TOOL_CALL]`
	result := ParseToolCall(content)
	if !result.IsTool {
		t.Fatal("expected IsTool=true for bracket format with single-quoted tool name")
	}
	if result.Action != "homepage" {
		t.Fatalf("expected action=homepage, got %q", result.Action)
	}
	if !result.XMLFallbackDetected {
		t.Fatal("expected XMLFallbackDetected=true for bracket format tool call")
	}
}

func TestParseToolCallBracketFormatSingleQuotesMultiline(t *testing.T) {
	content := "[[TOOL_CALL]\n{tool => 'homepage', args => {\n--operation \"read_file\"\n--path \"phaser-demo/src/main.ts\"\n}}\n[/TOOL_CALL]"
	result := ParseToolCall(content)
	if !result.IsTool {
		t.Fatal("expected IsTool=true for multiline bracket format with single quotes")
	}
	if result.Action != "homepage" {
		t.Fatalf("expected action=homepage, got %q", result.Action)
	}
}
