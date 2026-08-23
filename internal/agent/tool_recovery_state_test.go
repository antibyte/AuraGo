package agent

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestRecoveryHintForReadToolOutputMissingRef(t *testing.T) {
	hint := recoveryHintForToolFailure(
		ToolCall{Action: "read_tool_output"},
		`Tool Output: {"status":"error","message":"ref is required"}`,
	)
	if !strings.Contains(hint, "output_ref") {
		t.Fatalf("hint = %q, want output_ref guidance", hint)
	}
}

func TestToolRecoveryStateHandleDuplicateToolCallTriggersCircuitBreaker(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{
		Action:    "execute_shell",
		Command:   "pwd",
		Operation: "run",
	}

	if state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("did not expect first call to trip circuit breaker")
	}
	if state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("did not expect second identical call to trip circuit breaker yet")
	}
	if !state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("expected third identical call to trip circuit breaker")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected one breaker message, got %d", len(req.Messages))
	}
	if !strings.Contains(req.Messages[0].Content, "CIRCUIT BREAKER") {
		t.Fatal("expected circuit breaker guidance in injected message")
	}

	// After the circuit breaker fires, the frequency counter is preserved.
	// Any subsequent identical call must immediately re-trigger until a different
	// successful tool step proves that recovery work happened.
	if !state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("expected 4th identical call to immediately re-trigger circuit breaker (persistent block)")
	}

	recoveryTC := ToolCall{Action: "homepage", Operation: "rebuild"}
	_ = state.updateToolErrorState(recoveryTC, `Tool Output: {"status":"success","message":"rebuilt"}`, &req, nil, AgentTelemetryScope{}, "v1", 100)
	if state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("expected successful different tool step to reset duplicate breaker for the blocked signature")
	}
}

func TestToolRecoveryStateHandleDuplicateToolCallHonorsCustomThreshold(t *testing.T) {
	state := newToolRecoveryStateWithPolicy(RecoveryPolicy{
		DuplicateConsecutiveHits: 1,
		DuplicateFrequencyHits:   2,
	})
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{
		Action:    "execute_shell",
		Command:   "pwd",
		Operation: "run",
	}

	if state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("did not expect first call to trip circuit breaker")
	}
	if !state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("expected second identical call to trip circuit breaker with stricter policy")
	}
}

func TestToolRecoveryStateHandleDuplicateToolCallAllowsDifferentSearchPatterns(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}

	first := ToolCall{
		Action:    "file_reader_advanced",
		Operation: "search_context",
		FilePath:  "server.log",
		Params: map[string]interface{}{
			"pattern":    "error",
			"line_count": float64(3),
		},
	}
	second := ToolCall{
		Action:    first.Action,
		Operation: first.Operation,
		FilePath:  first.FilePath,
		Params: map[string]interface{}{
			"pattern":    "warning",
			"line_count": float64(3),
		},
	}

	if state.handleDuplicateToolCall(first, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("did not expect first call to trip circuit breaker")
	}
	if state.handleDuplicateToolCall(second, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("different pattern should not count as duplicate")
	}
}

func TestToolRecoveryStateHandleDuplicateToolCallAllowsDifferentLineRanges(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}

	first := ToolCall{
		Action:    "file_reader_advanced",
		Operation: "read_lines",
		FilePath:  "main.go",
		Params: map[string]interface{}{
			"start_line": float64(1),
			"end_line":   float64(50),
		},
	}
	second := ToolCall{
		Action:    first.Action,
		Operation: first.Operation,
		FilePath:  first.FilePath,
		Params: map[string]interface{}{
			"start_line": float64(51),
			"end_line":   float64(100),
		},
	}

	if state.handleDuplicateToolCall(first, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("did not expect first call to trip circuit breaker")
	}
	if state.handleDuplicateToolCall(second, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("different line ranges should not count as duplicate")
	}
}

func TestToolRecoveryStateAllowsRepeatedCoAgentMonitoringCalls(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}

	for _, tc := range []ToolCall{
		{Action: "co_agent", Operation: "list"},
		{Action: "co_agent", Operation: "status"},
		{Action: "co_agent", Operation: "get_result", CoAgentID: "specialist-writer-1"},
		{Action: "co_agents", Operation: "result", CoAgentID: "specialist-writer-1"},
	} {
		t.Run(tc.Action+"_"+tc.Operation, func(t *testing.T) {
			for i := 0; i < 5; i++ {
				if state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
					t.Fatalf("co-agent monitoring call %s/%s should remain pollable at attempt %d", tc.Action, tc.Operation, i+1)
				}
			}
		})
	}
	if len(req.Messages) != 0 {
		t.Fatalf("expected no circuit-breaker messages for co-agent monitoring, got %d", len(req.Messages))
	}
}

func TestToolRecoveryStateStillBlocksRepeatedCoAgentSpawn(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{
		Action:    "co_agent",
		Operation: "spawn_specialist",
		Task:      "Write a short story",
		Params: map[string]interface{}{
			"specialist": "writer",
		},
	}

	if state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("did not expect first co-agent spawn to trip circuit breaker")
	}
	if state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("did not expect second co-agent spawn to trip circuit breaker yet")
	}
	if !state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("expected repeated co-agent spawn to stay protected by circuit breaker")
	}
}

func TestToolRecoveryStateUpdateToolErrorStateTriggersCircuitBreaker(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "homepage"}
	result := `Tool Output: {"status":"error","message":"connect failed"}`

	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect first identical error to trip circuit breaker")
	}
	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect second identical error to trip circuit breaker")
	}
	if !state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("expected third identical error to trip circuit breaker")
	}
	if len(req.Messages) < 1 {
		t.Fatalf("expected at least one injected message, got %d", len(req.Messages))
	}
	if !strings.Contains(req.Messages[len(req.Messages)-1].Content, "CIRCUIT BREAKER") {
		t.Fatalf("expected final injected message to be the circuit breaker, got: %s", req.Messages[len(req.Messages)-1].Content)
	}
	if state.ConsecutiveErrorCount != 0 || state.LastToolError != "" {
		t.Fatal("expected error state to reset after circuit breaker triggers")
	}
}

func TestToolRecoveryStateUpdateToolErrorStateHonorsCustomThreshold(t *testing.T) {
	state := newToolRecoveryStateWithPolicy(RecoveryPolicy{
		IdenticalToolErrorHits: 2,
	})
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "homepage"}
	result := `Tool Output: {"status":"error","message":"connect failed"}`

	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect first identical error to trip circuit breaker")
	}
	if !state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("expected second identical error to trip circuit breaker with stricter policy")
	}
}

func TestToolRecoveryStateInjectsRecoveryHintBeforeBreaker(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "homepage"}
	result := `Tool Output: {"status":"error","message":"npm error Missing script: \"build\""}`

	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect first identical error to trip circuit breaker")
	}
	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect second identical error to trip circuit breaker yet")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected one recovery hint message, got %d", len(req.Messages))
	}
	if !strings.Contains(req.Messages[0].Content, "static site") {
		t.Fatalf("expected build-script-specific recovery hint, got: %s", req.Messages[0].Content)
	}
}

func TestRecoveryHintStopsAfterRepeatedIdenticalHints(t *testing.T) {
	state := newToolRecoveryState()
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.shouldSendRecoveryHintLocked("inspect the last error", 2) {
		t.Fatal("first hint should be sent")
	}
	if !state.shouldSendRecoveryHintLocked("inspect the last error", 2) {
		t.Fatal("second hint should be sent")
	}
	if state.shouldSendRecoveryHintLocked("inspect the last error", 2) {
		t.Fatal("third identical hint should be suppressed")
	}
}

func TestToolRecoveryStateHintsHomepagePathRequiredWithExample(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "homepage", Operation: "write_file"}
	result := `Tool Output: {"status":"error","message":"path is required"}`

	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect first identical error to trip circuit breaker")
	}
	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect second identical error to trip circuit breaker yet")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected one recovery hint message, got %d", len(req.Messages))
	}
	if !strings.Contains(req.Messages[0].Content, `path`) || !strings.Contains(req.Messages[0].Content, `my-site/index.html`) {
		t.Fatalf("expected homepage path-required recovery hint with example, got: %s", req.Messages[0].Content)
	}
}

func TestToolRecoveryStateUpdateToolErrorStateResolvesOnSuccess(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "execute_shell"}

	_ = state.updateToolErrorState(tc, `Tool Output: {"status":"error","message":"boom"}`, &req, nil, AgentTelemetryScope{}, "v1", 100)
	if !state.shouldRecordResolution() {
		t.Fatal("expected pending resolution after an error")
	}

	_ = state.updateToolErrorState(tc, `Tool Output: {"status":"success","message":"ok"}`, &req, nil, AgentTelemetryScope{}, "v1", 100)
	if state.shouldRecordResolution() {
		t.Fatal("expected success to clear pending resolution state")
	}
}

func TestToolRecoveryStateSuccessBreaksIdenticalErrorChain(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	errorResult := `Tool Output: {"status":"error","message":"shell failed"}`

	if state.updateToolErrorState(ToolCall{Action: "execute_shell", NativeCallID: "call-1"}, errorResult, &req, nil, AgentTelemetryScope{}, "v1", 10) {
		t.Fatal("first error triggered circuit breaker")
	}
	if state.updateToolErrorState(ToolCall{Action: "execute_shell", NativeCallID: "call-2"}, `Tool Output: {"status":"success","message":"ok"}`, &req, nil, AgentTelemetryScope{}, "v1", 10) {
		t.Fatal("success triggered circuit breaker")
	}
	if state.updateToolErrorState(ToolCall{Action: "execute_shell", NativeCallID: "call-3"}, errorResult, &req, nil, AgentTelemetryScope{}, "v1", 10) {
		t.Fatal("post-success error incorrectly continued the old chain")
	}
	if state.ConsecutiveErrorCount != 1 {
		t.Fatalf("consecutive errors = %d, want 1", state.ConsecutiveErrorCount)
	}
}

func TestToolRecoveryStateProcessesToolCallIDOnce(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "execute_shell", NativeCallID: "same-call"}
	result := `Tool Output: {"status":"error","message":"boom"}`

	_ = state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 10)
	_ = state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 10)
	if state.TotalErrorCount != 1 || state.ConsecutiveErrorCount != 1 {
		t.Fatalf("duplicate call counted as total=%d consecutive=%d, want 1/1", state.TotalErrorCount, state.ConsecutiveErrorCount)
	}
}

func TestToolRecoveryStateParallelResultsCountEachCallOnce(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	result := `Tool Output: {"status":"error","message":"parallel failure"}`
	callIDs := []string{"parallel-a", "parallel-b", "parallel-a", "parallel-c"}

	var wg sync.WaitGroup
	for _, callID := range callIDs {
		callID := callID
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = state.updateToolErrorState(ToolCall{Action: "execute_shell", NativeCallID: callID}, result, &req, nil, AgentTelemetryScope{}, "v1", 10)
		}()
	}
	wg.Wait()
	if state.TotalErrorCount != 3 {
		t.Fatalf("parallel unique errors counted %d, want 3", state.TotalErrorCount)
	}
	breakerMessages := 0
	for _, message := range req.Messages {
		if strings.Contains(message.Content, "returned the same error 3 times") {
			breakerMessages++
		}
	}
	if breakerMessages != 1 {
		t.Fatalf("breaker messages = %d, want exactly 1; messages=%#v", breakerMessages, req.Messages)
	}
}

func TestToolRecoveryStateHintsVirtualDesktopAppNotFound(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "virtual_desktop", Operation: "get_app"}
	result := `Tool Output: {"status":"error","message":"desktop app \"missing\" not found","data":{"code":"desktop_app_not_found","app_id":"missing"}}`

	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect first identical error to trip circuit breaker")
	}
	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect second identical error to trip circuit breaker yet")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected one recovery hint message, got %d", len(req.Messages))
	}
	if !strings.Contains(req.Messages[0].Content, "list_apps") {
		t.Fatalf("expected virtual_desktop app-not-found recovery hint with list_apps guidance, got: %s", req.Messages[0].Content)
	}
}

func TestToolRecoveryStateHintsVirtualDesktopWidgetNotFound(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "virtual_desktop", Operation: "get_widget"}
	result := `Tool Output: {"status":"error","message":"desktop widget \"missing\" not found","data":{"code":"desktop_widget_not_found","widget_id":"missing"}}`

	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect first identical error to trip circuit breaker")
	}
	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect second identical error to trip circuit breaker yet")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected one recovery hint message, got %d", len(req.Messages))
	}
	if !strings.Contains(req.Messages[0].Content, "list_widgets") {
		t.Fatalf("expected virtual_desktop widget-not-found recovery hint with list_widgets guidance, got: %s", req.Messages[0].Content)
	}
}

func TestToolRecoveryStateHintsVirtualDesktopPathEscape(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "virtual_desktop", Operation: "write_file"}
	result := `Tool Output: {"status":"error","message":"desktop path escapes workspace: /workspace/Apps/foo.html"}`

	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect first identical error to trip circuit breaker")
	}
	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect second identical error to trip circuit breaker yet")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected one recovery hint message, got %d", len(req.Messages))
	}
	if !strings.Contains(req.Messages[0].Content, "Apps/my-app/index.html") || !strings.Contains(req.Messages[0].Content, "/workspace/") {
		t.Fatalf("expected virtual_desktop path-escape recovery hint with workspace-relative guidance, got: %s", req.Messages[0].Content)
	}
}

func TestToolRecoveryStateHintsVirtualDesktopInvalidIcon(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "virtual_desktop", Operation: "create_app"}
	result := `Tool Output: {"status":"error","message":"desktop app icon must use icon_catalog.preferred, icon_catalog.aliases, or sprite:<name>"}`

	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect first identical error to trip circuit breaker")
	}
	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 100) {
		t.Fatal("did not expect second identical error to trip circuit breaker yet")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected one recovery hint message, got %d", len(req.Messages))
	}
	if !strings.Contains(req.Messages[0].Content, "icon_catalog") || !strings.Contains(req.Messages[0].Content, "emoji") {
		t.Fatalf("expected virtual_desktop invalid-icon recovery hint with icon_catalog guidance, got: %s", req.Messages[0].Content)
	}
}

func TestToolRecoveryStateHintsFocusedVirtualDesktopInstallErrors(t *testing.T) {
	tests := []struct {
		name   string
		error  string
		needle string
	}{
		{name: "files required", error: `Tool Output: {"status":"error","message":"files are required"}`, needle: "atomic"},
		{name: "files wrong type", error: `Tool Output: {"status":"error","message":"files must be an object of path to content"}`, needle: "non-empty files object"},
		{name: "entry missing", error: `Tool Output: {"status":"error","message":"desktop app entry file is missing"}`, needle: "exactly equal to manifest.entry"},
		{name: "app id missing", error: `Tool Output: {"status":"error","message":"app_id is required"}`, needle: "requires app_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := recoveryHintForToolFailure(ToolCall{Action: "virtual_desktop_app_install"}, tt.error)
			if !strings.Contains(hint, tt.needle) {
				t.Fatalf("hint = %q, want %q", hint, tt.needle)
			}
		})
	}
}

func TestToolRecoveryStateBlocksFourthFailedInstallAcrossDifferentErrors(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	install := func(callID, marker string) ToolCall {
		return ToolCall{
			Action:       "virtual_desktop_app_install",
			NativeCallID: callID,
			Params: map[string]interface{}{
				"manifest": map[string]interface{}{"id": "space-invaders", "name": "Space Invaders", "entry": "index.html"},
				"files":    map[string]interface{}{marker: "content"},
			},
		}
	}
	errors := []string{
		`Tool Output: {"status":"error","message":"files are required"}`,
		`Tool Output: {"status":"error","message":"desktop app icon must use icon_catalog"}`,
		`Tool Output: {"status":"error","message":"desktop app entry file is missing"}`,
	}
	for i, result := range errors {
		tc := install(fmt.Sprintf("call-%d", i), fmt.Sprintf("file-%d.html", i))
		if state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
			t.Fatalf("install attempt %d blocked before three failures", i+1)
		}
		_ = state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}, "v1", 10)
		list := ToolCall{Action: "virtual_desktop_apps", NativeCallID: fmt.Sprintf("list-%d", i), Params: map[string]interface{}{"operation": "list_apps"}}
		_ = state.updateToolErrorState(list, `Tool Output: {"status":"success"}`, &req, nil, AgentTelemetryScope{}, "v1", 10)
	}
	if !state.handleDuplicateToolCall(install("call-four", "index.html"), &req, nil, AgentTelemetryScope{}) {
		t.Fatal("fourth install attempt was not blocked after three varied failures")
	}
	if len(req.Messages) == 0 || !strings.Contains(req.Messages[len(req.Messages)-1].Content, "manifest.entry") {
		t.Fatalf("install breaker lost the latest concrete recovery guidance: %#v", req.Messages)
	}
}

func TestToolRecoveryStateSuccessfulInstallResetsInstallBudget(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "virtual_desktop_apps", Operation: "install_app", Params: map[string]interface{}{"app_id": "demo"}}
	for i := 0; i < 2; i++ {
		tc.NativeCallID = fmt.Sprintf("failure-%d", i)
		_ = state.updateToolErrorState(tc, `Tool Output: {"status":"error","message":"desktop app entry file is missing"}`, &req, nil, AgentTelemetryScope{}, "v1", 10)
	}
	tc.NativeCallID = "success"
	_ = state.updateToolErrorState(tc, `Tool Output: {"status":"success"}`, &req, nil, AgentTelemetryScope{}, "v1", 10)
	if state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("successful install did not reset the app-specific failure budget")
	}
}

func TestToolRecoveryStateHandleDuplicateToolCallBoundsFrequencyMap(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}

	for i := 0; i < maxTrackedToolCallSignatures+64; i++ {
		tc := ToolCall{
			Action:    "execute_shell",
			Command:   fmt.Sprintf("echo %d", i),
			Operation: "run",
		}
		if state.handleDuplicateToolCall(tc, &req, nil, AgentTelemetryScope{}) {
			t.Fatalf("did not expect unique tool call %d to trip circuit breaker", i)
		}
	}

	if len(state.ToolCallFrequency) > maxTrackedToolCallSignatures {
		t.Fatalf("tool call frequency size = %d, want <= %d", len(state.ToolCallFrequency), maxTrackedToolCallSignatures)
	}
	if got := state.ToolCallFrequency[buildToolSignature(ToolCall{Action: "execute_shell", Command: fmt.Sprintf("echo %d", maxTrackedToolCallSignatures+63), Operation: "run"})]; got != 1 {
		t.Fatalf("expected latest tool signature to remain tracked, got count %d", got)
	}
}

func TestBuildToolSignatureStableAcrossMapOrder(t *testing.T) {
	first := ToolCall{
		Action:    "api_request",
		Operation: "request",
		Params: map[string]interface{}{
			"zeta": 1,
			"alpha": map[string]interface{}{
				"b": true,
				"a": "x",
			},
		},
		Headers: map[string]string{
			"X-Z": "1",
			"X-A": "2",
		},
		SkillArgs: map[string]interface{}{
			"items": []interface{}{"one", 2.0},
		},
		Items: []map[string]interface{}{{
			"second": 2,
			"first":  1,
		}},
	}
	second := ToolCall{
		Action:    "api_request",
		Operation: "request",
		Params: map[string]interface{}{
			"alpha": map[string]interface{}{
				"a": "x",
				"b": true,
			},
			"zeta": 1,
		},
		Headers: map[string]string{
			"X-A": "2",
			"X-Z": "1",
		},
		SkillArgs: map[string]interface{}{
			"items": []interface{}{"one", 2.0},
		},
		Items: []map[string]interface{}{{
			"first":  1,
			"second": 2,
		}},
	}

	if got, want := buildToolSignature(first), buildToolSignature(second); got != want {
		t.Fatalf("signature mismatch: %q != %q", got, want)
	}
}
