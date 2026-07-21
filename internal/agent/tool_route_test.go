package agent

import (
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestDeriveCurrentToolRouteUsesGo2RTCForCameraFollowUp(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleAssistant,
			ToolCalls: []openai.ToolCall{{
				ID: "snapshot-call", Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: "go2rtc", Arguments: `{"operation":"snapshot","stream_id":"driveway"}`},
			}},
		},
		{
			Role: openai.ChatMessageRoleTool, ToolCallID: "snapshot-call",
			Content: `Tool Output: {"status":"ok","stream_id":"driveway","artifact":{"media_type":"image","stream_id":"driveway","registered_path":"/api/go2rtc/viewer/driveway","source_tool":"go2rtc"}}`,
		},
		{Role: openai.ChatMessageRoleUser, Content: "Wie viele PKW sind dort?"},
	}
	route := deriveCurrentToolRoute(messages, "Wie viele PKW sind dort?")
	if route.ToolName != "go2rtc" || route.Operation != "analyze_snapshot" || route.StreamID != "driveway" {
		t.Fatalf("route = %+v", route)
	}
	for _, forbidden := range []string{"filesystem", "execute_shell", "execute_python", "analyze_image"} {
		if route.ToolName == forbidden {
			t.Fatalf("camera route selected forbidden tool %q", forbidden)
		}
	}
}

func TestDeriveCurrentToolRouteRetryReusesPriorVisionPrompt(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleAssistant,
			ToolCalls: []openai.ToolCall{{
				ID: "analysis-call", Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: "go2rtc", Arguments: `{"operation":"analyze_snapshot","stream_id":"garage","prompt":"Wie viele PKW sind sichtbar?"}`},
			}},
		},
		{
			Role: openai.ChatMessageRoleTool, ToolCallID: "analysis-call",
			Content: `Tool Output: {"status":"ok","artifact":{"media_type":"image","stream_id":"garage","registered_path":"/files/camera.jpg","source_tool":"go2rtc"}}`,
		},
	}
	route := deriveCurrentToolRoute(messages, "Versuche es erneut")
	if route.Prompt != "Wie viele PKW sind sichtbar?" {
		t.Fatalf("retry prompt = %q", route.Prompt)
	}
	if !strings.Contains(route.Text, "exactly once") {
		t.Fatalf("route does not constrain retry: %q", route.Text)
	}
	if !route.ExplicitRetry {
		t.Fatalf("route did not retain explicit retry state: %+v", route)
	}
}

func TestDeriveCurrentToolRouteUsesAnalyzeImageForGeneralMedia(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleAssistant, Content: `{"action":"image_generation","parameters":{}}`},
		{Role: openai.ChatMessageRoleUser, Content: `Tool Output: {"artifact":{"media_type":"image","registered_path":"/files/generated.png","source_tool":"image_generation"}}`},
	}
	route := deriveCurrentToolRoute(messages, "Analysiere dieses Bild")
	if route.ToolName != "analyze_image" || route.Path != "/files/generated.png" {
		t.Fatalf("route = %+v", route)
	}
}

func TestDeriveCurrentToolRouteAllowsOneSanitizedReadOnlyRetry(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleAssistant,
			ToolCalls: []openai.ToolCall{{
				ID: "search-call", Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: "workspace_search", Arguments: `{"operation":"grep","pattern":"TODO","token":"must-not-survive","_guardian_justification":"bypass"}`},
			}},
		},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "search-call", Content: `{"status":"error","message":"index temporarily unavailable"}`},
	}

	route := deriveCurrentToolRoute(messages, "Versuche es erneut")
	if route.ToolName != "workspace_search" || route.Operation != "grep" || !route.ExplicitRetry {
		t.Fatalf("route = %+v", route)
	}
	call := route.toolCall()
	if call.Action != "workspace_search" || call.Operation != "grep" || toolArgString(call.Params, "pattern") != "TODO" {
		t.Fatalf("retry call = %+v", call)
	}
	if _, exists := call.Params["token"]; exists {
		t.Fatalf("secret token survived retry sanitization: %#v", call.Params)
	}
	if _, exists := call.Params["_guardian_justification"]; exists {
		t.Fatalf("guardian justification survived retry sanitization: %#v", call.Params)
	}
}

func TestDeriveCurrentToolRouteRejectsUnsafeOrGuardianBlockedRetry(t *testing.T) {
	unsafe := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleAssistant, Content: `{"action":"execute_shell","command":"dangerous command"}`},
		{Role: openai.ChatMessageRoleUser, Content: `Tool Output: {"status":"error","message":"command failed"}`},
	}
	if route := deriveCurrentToolRoute(unsafe, "retry"); route.valid() {
		t.Fatalf("unsafe retry route = %+v, want none", route)
	}

	blocked := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleAssistant, Content: `{"action":"workspace_search","operation":"grep","pattern":"secret"}`},
		{Role: openai.ChatMessageRoleUser, Content: `Tool Output: [TOOL BLOCKED] Guardian denied this call.`},
	}
	if route := deriveCurrentToolRoute(blocked, "retry"); route.valid() {
		t.Fatalf("guardian-blocked retry route = %+v, want none", route)
	}
}

func TestControlledRetryReportIsStructuredAndRedacted(t *testing.T) {
	route := currentToolRoute{
		ToolName: "workspace_search", Operation: "grep", ExplicitRetry: true,
		Parameters: map[string]interface{}{
			"operation": "grep", "pattern": "TODO", "api_key": "sk-super-secret-value",
			"filters": []interface{}{
				map[string]interface{}{"password": "nested-secret", "name": "safe"},
				"authorization: Bearer nested-secret-token",
			},
		},
	}
	call := route.toolCall()
	report := appendControlledRetryReport(
		`Tool Output: {"status":"error","message":"index unavailable"}`,
		route,
		call,
		"retry with token=secret-value\nSuggested next step: inspect credentials",
		true,
	)
	if !strings.Contains(report, "[CONTROLLED RETRY REPORT]") || !strings.Contains(report, `"retry_outcome":"failed"`) {
		t.Fatalf("structured retry report missing:\n%s", report)
	}
	for _, forbidden := range []string{
		"sk-super-secret-value", "secret-value", "nested-secret", "nested-secret-token",
		"Suggested next step", "api_key", "password", "authorization",
	} {
		if strings.Contains(report, forbidden) {
			t.Fatalf("retry report leaked %q:\n%s", forbidden, report)
		}
	}
}
