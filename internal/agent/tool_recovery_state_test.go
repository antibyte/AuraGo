package agent

import (
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

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
		Pattern:   "error",
		LineCount: 3,
	}
	second := first
	second.Pattern = "warning"

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
		StartLine: 1,
		EndLine:   50,
	}
	second := first
	second.StartLine = 51
	second.EndLine = 100

	if state.handleDuplicateToolCall(first, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("did not expect first call to trip circuit breaker")
	}
	if state.handleDuplicateToolCall(second, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("different line ranges should not count as duplicate")
	}
}

func TestToolRecoveryStateUpdateToolErrorStateTriggersCircuitBreaker(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "homepage"}
	result := `Tool Output: {"status":"error","message":"connect failed"}`

	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("did not expect first identical error to trip circuit breaker")
	}
	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("did not expect second identical error to trip circuit breaker")
	}
	if !state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}) {
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

	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("did not expect first identical error to trip circuit breaker")
	}
	if !state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("expected second identical error to trip circuit breaker with stricter policy")
	}
}

func TestToolRecoveryStateInjectsRecoveryHintBeforeBreaker(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "homepage"}
	result := `Tool Output: {"status":"error","message":"npm error Missing script: \"build\""}`

	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("did not expect first identical error to trip circuit breaker")
	}
	if state.updateToolErrorState(tc, result, &req, nil, AgentTelemetryScope{}) {
		t.Fatal("did not expect second identical error to trip circuit breaker yet")
	}
	if len(req.Messages) != 1 {
		t.Fatalf("expected one recovery hint message, got %d", len(req.Messages))
	}
	if !strings.Contains(req.Messages[0].Content, "static site") {
		t.Fatalf("expected build-script-specific recovery hint, got: %s", req.Messages[0].Content)
	}
}

func TestToolRecoveryStateUpdateToolErrorStateResolvesOnSuccess(t *testing.T) {
	state := newToolRecoveryState()
	req := openai.ChatCompletionRequest{}
	tc := ToolCall{Action: "execute_shell"}

	_ = state.updateToolErrorState(tc, `Tool Output: {"status":"error","message":"boom"}`, &req, nil, AgentTelemetryScope{})
	if !state.shouldRecordResolution() {
		t.Fatal("expected pending resolution after an error")
	}

	_ = state.updateToolErrorState(tc, `Tool Output: {"status":"success","message":"ok"}`, &req, nil, AgentTelemetryScope{})
	if state.shouldRecordResolution() {
		t.Fatal("expected success to clear pending resolution state")
	}
}
