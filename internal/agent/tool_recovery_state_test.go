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
	if len(req.Messages) != 1 {
		t.Fatalf("expected one breaker message, got %d", len(req.Messages))
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
