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

func TestParseToolResponseStripsTTSBlocks(t *testing.T) {
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Content: "Ehrlich gesagt habe ich keine Wetterdaten.\n\n<tts>\n\n</tts>",
			},
		}},
	}

	parsed := parseToolResponse(resp, nil, AgentTelemetryScope{})

	if strings.Contains(strings.ToLower(parsed.SanitizedContent), "<tts") {
		t.Fatalf("expected TTS tags to be stripped, got %q", parsed.SanitizedContent)
	}
	if !strings.Contains(parsed.SanitizedContent, "keine Wetterdaten") {
		t.Fatalf("expected visible response to be preserved, got %q", parsed.SanitizedContent)
	}
}

func TestParseToolResponseParsesTTSTagToolCall(t *testing.T) {
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Content: "Klar, ich teste das.\n\n<tts>\n<parameter name=\"text\">Hallo! Ich teste gerade die Sprachausgabe.</parameter>\n<parameter name=\"language\">de</parameter>\n</tts>",
			},
		}},
	}

	parsed := parseToolResponse(resp, nil, AgentTelemetryScope{})

	if !parsed.ToolCall.IsTool {
		t.Fatal("expected TTS tag to be parsed as a tool call")
	}
	if parsed.ToolCall.Action != "tts" {
		t.Fatalf("Action = %q, want tts", parsed.ToolCall.Action)
	}
	if parsed.ToolCall.Text != "Hallo! Ich teste gerade die Sprachausgabe." {
		t.Fatalf("Text = %q", parsed.ToolCall.Text)
	}
	if parsed.ToolCall.Language != "de" {
		t.Fatalf("Language = %q, want de", parsed.ToolCall.Language)
	}
	if !parsed.ToolCall.XMLFallbackDetected {
		t.Fatal("expected XMLFallbackDetected=true")
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

func TestTrimMessagesForEmptyResponsePreservesLastUserIntentAndLatestToolResult(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "sys"},
		{Role: openai.ChatMessageRoleSystem, Content: "summary"},
		{Role: openai.ChatMessageRoleUser, Content: "older request"},
		{Role: openai.ChatMessageRoleAssistant, Content: "planning"},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{
			ID:       "call-1",
			Type:     openai.ToolTypeFunction,
			Function: openai.FunctionCall{Name: "execute_shell", Arguments: `{"command":"pwd"}`},
		}}},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call-1", Content: "tool output"},
		{Role: openai.ChatMessageRoleAssistant, Content: "post tool"},
		{Role: openai.ChatMessageRoleUser, Content: "latest user intent"},
		{Role: openai.ChatMessageRoleAssistant, Content: "tail a"},
		{Role: openai.ChatMessageRoleAssistant, Content: "tail b"},
		{Role: openai.ChatMessageRoleAssistant, Content: "tail c"},
	}

	trimmed, summary := trimMessagesForEmptyResponseWithSummary(msgs)

	if !summary.PreservedLastUserIntent {
		t.Fatal("expected summary to report preserved last user intent")
	}
	if !summary.PreservedLatestToolResult {
		t.Fatal("expected summary to report preserved latest tool result")
	}
	foundTool := false
	foundLatestUser := false
	for _, msg := range trimmed {
		if msg.Role == openai.ChatMessageRoleTool && msg.Content == "tool output" {
			foundTool = true
		}
		if msg.Role == openai.ChatMessageRoleUser && msg.Content == "latest user intent" {
			foundLatestUser = true
		}
	}
	if !foundTool {
		t.Fatal("expected trimmed context to keep latest tool result")
	}
	if !foundLatestUser {
		t.Fatal("expected trimmed context to keep latest user intent")
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

func TestParseToolCallBracketFormatClosingTagWithParen(t *testing.T) {
	// MiniMax sometimes emits ) instead of ] in the closing tag
	content := `[TOOL_CALL]{tool => "query_memory", args => {--query "sound" --type "all"}}[/TOOL_CALL)`
	result := ParseToolCall(content)
	if !result.IsTool {
		t.Fatalf("IsTool=false, expected tool call from bracket format with ) closing tag: %q", content)
	}
	if result.Action != "query_memory" {
		t.Fatalf("Action=%q, want %q", result.Action, "query_memory")
	}
}

func TestParseToolCallBracketFormatArrowDash(t *testing.T) {
	// Some GLM-family models use -> instead of => for the arrow operator
	content := `[TOOL_CALL]{tool -> "query_memory", args => {--query "sound" --type "all"}}[/TOOL_CALL]`
	result := ParseToolCall(content)
	if !result.IsTool {
		t.Fatalf("IsTool=false, expected tool call from bracket format with -> arrow: %q", content)
	}
	if result.Action != "query_memory" {
		t.Fatalf("Action=%q, want %q", result.Action, "query_memory")
	}
}

func TestParseToolCallBracketFormatMinimaxVariant(t *testing.T) {
	// Realistic MiniMax output: acknowledgment text + bracket format with ) closing
	content := `Ich schaue mir das kurz an.
[TOOL_CALL]{tool => "query_memory", args => {--query "Phaser sound effects" --type "all"}}[/TOOL_CALL)`
	result := ParseToolCall(content)
	if !result.IsTool {
		t.Fatalf("IsTool=false, expected tool call from MiniMax variant format: %q", content)
	}
	if result.Action != "query_memory" {
		t.Fatalf("Action=%q, want %q", result.Action, "query_memory")
	}
}

func TestParseToolCallBracketFormatAfterTagStripping(t *testing.T) {
	// Simulates what happens after StripThinkingTags removes [/TOOL_CALL] from its own line
	content := `[TOOL_CALL]{tool => "query_memory", args => {--query "sound"}}
[TOOL_CALL]`
	result := ParseToolCall(content)
	// This case can't be fixed by the bracket parser alone - the closing tag was stripped.
	// The announcement detector should NOT fire when the model outputs this format.
	// We verify the parser handles it gracefully (IsTool=false is expected here)
	if result.IsTool {
		t.Fatalf("expected IsTool=false when closing tag was stripped by tag removal")
	}
}

func TestParseToolCallAcceptsToolFieldJSON(t *testing.T) {
	content := `{"tool":"obsidian","operation":"create_note","path":"BugTest.md","content":"hello"}`

	result := ParseToolCall(content)

	if !result.IsTool {
		t.Fatal("expected IsTool=true for JSON using the tool field")
	}
	if result.Action != "obsidian" {
		t.Fatalf("Action=%q, want obsidian", result.Action)
	}
	if result.Operation != "create_note" {
		t.Fatalf("Operation=%q, want create_note", result.Operation)
	}
}

func TestParseToolCallAcceptsToolParametersWrapper(t *testing.T) {
	content := `{"tool":"invasion_control","parameters":{"operation":"egg_status","nest_id":"7680f451-bad4-4908-92da-e286eb5f7c2a"}}`

	result := ParseToolCall(content)

	if !result.IsTool {
		t.Fatal("expected IsTool=true for JSON using tool plus parameters wrapper")
	}
	if result.Action != "invasion_control" {
		t.Fatalf("Action=%q, want invasion_control", result.Action)
	}
	if result.Operation != "egg_status" {
		t.Fatalf("Operation=%q, want egg_status", result.Operation)
	}
	if result.NestID != "7680f451-bad4-4908-92da-e286eb5f7c2a" {
		t.Fatalf("NestID=%q, want flattened nest_id", result.NestID)
	}
	if result.Params == nil {
		t.Fatal("Params=nil, want flattened parameters")
	}
	if got, _ := result.Params["nest_id"].(string); got != "7680f451-bad4-4908-92da-e286eb5f7c2a" {
		t.Fatalf("Params[nest_id]=%q", got)
	}
}

func TestParseToolCallAcceptsDocumentCreatorSectionsArray(t *testing.T) {
	content := `{"action":"document_creator","operation":"create_pdf","title":"KI-News – Aktuelle Entwicklungen April 2026","filename":"ki-news-april-2026","sections":[{"type":"text","header":"Intro","body":"Hallo"},{"type":"text","header":"Trend","body":"Mehr KI"}]}`

	result := ParseToolCall(content)

	if !result.IsTool {
		t.Fatal("expected document_creator payload with native sections array to parse as tool call")
	}
	if result.Action != "document_creator" {
		t.Fatalf("Action=%q, want document_creator", result.Action)
	}
	if result.Operation != "create_pdf" {
		t.Fatalf("Operation=%q, want create_pdf", result.Operation)
	}
	if !strings.Contains(string(result.Sections), `"header":"Intro"`) {
		t.Fatalf("Sections=%q, want compact JSON array content", result.Sections)
	}
}

func TestParseToolCallAcceptsPDFOperationsSourceFilesArray(t *testing.T) {
	content := `{"action":"pdf_operations","operation":"merge","output_file":"combined.pdf","source_files":["one.pdf","two.pdf"]}`

	result := ParseToolCall(content)

	if !result.IsTool {
		t.Fatal("expected pdf_operations payload with native source_files array to parse as tool call")
	}
	if result.Action != "pdf_operations" {
		t.Fatalf("Action=%q, want pdf_operations", result.Action)
	}
	if result.Operation != "merge" {
		t.Fatalf("Operation=%q, want merge", result.Operation)
	}
	if got := string(result.SourceFiles); got != `["one.pdf","two.pdf"]` {
		t.Fatalf("SourceFiles=%q, want compact JSON array", got)
	}
}

func TestParseToolResponseParsesWrappedToolCallsJSON(t *testing.T) {
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Content: "<think>Need the obsidian tool</think>\n```json\n{\"tool_calls\":[{\"name\":\"obsidian\",\"arguments\":{\"operation\":\"create_note\",\"path\":\"BugTest.md\",\"content\":\"hello\"}}]}\n```",
			},
		}},
	}

	parsed := parseToolResponse(resp, nil, AgentTelemetryScope{})

	if !parsed.ToolCall.IsTool {
		t.Fatal("expected wrapped tool_calls JSON to be parsed as a tool call")
	}
	if parsed.ToolCall.Action != "obsidian" {
		t.Fatalf("Action=%q, want obsidian", parsed.ToolCall.Action)
	}
	if parsed.ToolCall.Path != "BugTest.md" {
		t.Fatalf("Path=%q, want BugTest.md", parsed.ToolCall.Path)
	}
	if parsed.ToolCall.Operation != "create_note" {
		t.Fatalf("Operation=%q, want create_note", parsed.ToolCall.Operation)
	}
	if parsed.ParseSource != ToolCallParseSourceReasoningCleanJSON {
		t.Fatalf("ParseSource=%q, want %q", parsed.ParseSource, ToolCallParseSourceReasoningCleanJSON)
	}
}

func TestParseToolResponseBareToolCallTag(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "bare <tool_call> after think block",
			content: "<think>\nI should check system status.\n</think>\n\nEinen Moment, ich check den Systemstatus...\n<tool_call>",
		},
		{
			name:    "bare <tool_call> alone",
			content: "Let me check that.\n<tool_call>",
		},
		{
			name:    "bare </tool_call> closing tag",
			content: "Checking system status...\n</tool_call>",
		},
		{
			name:    "bare minimax:tool_call marker",
			content: "<think>thinking</think>\nEinen Moment...\nminimax:tool_call",
		},
		{
			name:    "minimax:tool_call with junk log data after it",
			content: "Lass mich das reparieren:\nminimax:tool_call           log : app-page.runtime.dev.js:35:16871\n    at aw (node_modules/next/dist/compiled/next-server)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{{
					Message: openai.ChatCompletionMessage{Content: tt.content},
				}},
			}
			parsed := parseToolResponse(resp, nil, AgentTelemetryScope{})

			if !parsed.IncompleteToolCall {
				t.Fatal("expected IncompleteToolCall=true")
			}
			if parsed.ToolCall.IsTool {
				t.Fatal("bare tag should not parse as a valid tool call")
			}
			if strings.Contains(parsed.SanitizedContent, "<tool_call>") ||
				strings.Contains(parsed.SanitizedContent, "</tool_call>") ||
				strings.Contains(parsed.SanitizedContent, "minimax:tool_call") {
				t.Fatalf("bare tags should be stripped from SanitizedContent, got: %q", parsed.SanitizedContent)
			}
		})
	}
}

func TestParseToolResponseValidToolCallNotFlaggedAsIncomplete(t *testing.T) {
	// A real tool call with minimax:tool_call followed by JSON must NOT be flagged as incomplete
	content := `<think<thinking>
minimax:tool_call
{"action":"system_metrics"}`
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{Content: content},
		}},
	}
	parsed := parseToolResponse(resp, nil, AgentTelemetryScope{})

	if parsed.IncompleteToolCall {
		t.Fatal("valid tool call should not be flagged as incomplete")
	}
	if !parsed.ToolCall.IsTool {
		t.Fatal("expected valid tool call to be parsed")
	}
}

func TestParseToolResponsePromotesBareDiagnosticShellCommand(t *testing.T) {
	command := `docker stats --no-stream --format "table {{.Name}}\t{{.MemUsage}}\t{{.MemPerc}}" 2>/dev/null || echo "Docker nicht verfügbar oder keine Berechtigung"`
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{Content: command},
		}},
	}

	parsed := parseToolResponse(resp, nil, AgentTelemetryScope{})
	if !parsed.ToolCall.IsTool {
		t.Fatal("expected bare diagnostic command to be promoted to a tool call")
	}
	if parsed.ToolCall.Action != "execute_shell" {
		t.Fatalf("action = %q, want execute_shell", parsed.ToolCall.Action)
	}
	if parsed.ToolCall.Command != command {
		t.Fatalf("command = %q, want %q", parsed.ToolCall.Command, command)
	}
}

func TestParseToolCallDoesNotPromoteDestructiveBareShellCommand(t *testing.T) {
	parsed := ParseToolCall("docker rm -f aurago")
	if parsed.IsTool {
		t.Fatalf("destructive bare command should not become a tool call: %+v", parsed)
	}
}

func TestFormatAnnouncementFeedbackIncludesDone(t *testing.T) {
	msg := FormatAnnouncementFeedback(true, nil)
	if !strings.Contains(msg, "<done/>") {
		t.Fatal("FormatAnnouncementFeedback should tell model to append <done/>")
	}
	msg = FormatAnnouncementFeedback(false, nil)
	if !strings.Contains(msg, "<done/>") {
		t.Fatal("FormatAnnouncementFeedback (non-native) should also tell model to append <done/>")
	}
}

func TestFormatAnnouncementFeedbackRecentToolMentioned(t *testing.T) {
	msg := FormatAnnouncementFeedback(true, []string{"context_memory", "manage_missions"})
	if !strings.Contains(msg, "manage_missions") {
		t.Fatal("FormatAnnouncementFeedback should mention the most recent tool")
	}
	if !strings.Contains(msg, "do NOT call it again") {
		t.Fatal("FormatAnnouncementFeedback should tell model not to re-call the last tool")
	}
}

func TestFormatNonNativeToolCallFeedbackMentionsNativeAPI(t *testing.T) {
	msg := FormatNonNativeToolCallFeedback("execute_skill")
	if !strings.Contains(msg, "native function-calling API") {
		t.Fatal("expected feedback to require the native function-calling API")
	}
	if !strings.Contains(msg, "execute_skill") {
		t.Fatal("expected feedback to mention the affected tool name")
	}
}

func TestFormatDiscoverToolsFirstFeedbackMentionsDiscovery(t *testing.T) {
	msg := FormatDiscoverToolsFirstFeedback("pdf_operations")
	if !strings.Contains(msg, "discover_tools") {
		t.Fatal("expected feedback to require discover_tools first")
	}
	if !strings.Contains(msg, "pdf_operations") {
		t.Fatal("expected feedback to mention the affected tool name")
	}
}
