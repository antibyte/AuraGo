package agent

import (
	"strings"
	"testing"
)

func TestPrecheckMessagingToolArgsSkipsEmptyMessageOnHeartbeat(t *testing.T) {
	tc := ToolCall{
		IsTool: true,
		Action: "send_telegram",
		Params: map[string]interface{}{"title": "Heartbeat"},
	}
	out, blocked := precheckMessagingToolArgs(tc, RunConfig{MessageSource: "heartbeat"}, "heartbeat") // result, blocked
	if !blocked {
		t.Fatal("expected empty heartbeat telegram call to be blocked")
	}
	if !strings.Contains(out, `"status":"skipped"`) {
		t.Fatalf("expected skipped status, got %q", out)
	}
	if strings.Contains(out, `"status":"error"`) {
		t.Fatalf("expected no error status for autonomous skip, got %q", out)
	}
}

func TestPrecheckMessagingToolArgsErrorsOnInteractiveChat(t *testing.T) {
	tc := ToolCall{
		IsTool: true,
		Action: "send_telegram",
		Params: map[string]interface{}{"title": "Update"},
	}
	out, blocked := precheckMessagingToolArgs(tc, RunConfig{MessageSource: "web_chat"}, "default")
	if !blocked {
		t.Fatal("expected empty interactive telegram call to be blocked")
	}
	if !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, "message is required") {
		t.Fatalf("expected validation error, got %q", out)
	}
}

func TestPrecheckMessagingToolArgsAllowsNonEmptyMessage(t *testing.T) {
	tc := ToolCall{
		IsTool: true,
		Action: "send_telegram",
		Params: map[string]interface{}{"message": "All clear"},
	}
	out, blocked := precheckMessagingToolArgs(tc, RunConfig{MessageSource: "heartbeat"}, "heartbeat") // result, blocked
	if blocked {
		t.Fatalf("expected call to proceed, got blocked output %q", out)
	}
}

func TestPrecheckMessagingToolArgsSkipsMissionMutationOnHeartbeat(t *testing.T) {
	tc := ToolCall{
		IsTool:    true,
		Action:    "manage_missions",
		Operation: "run",
		Params:    map[string]interface{}{"id": "mission_123"},
	}

	out, blocked := precheckMessagingToolArgs(tc, RunConfig{MessageSource: "heartbeat"}, "heartbeat")
	if !blocked {
		t.Fatal("expected heartbeat mission mutation to be blocked")
	}
	if !strings.Contains(out, `"status":"skipped"`) {
		t.Fatalf("expected skipped status, got %q", out)
	}
}

func TestPrecheckMessagingToolArgsAllowsMissionReadOnHeartbeat(t *testing.T) {
	tc := ToolCall{
		IsTool:    true,
		Action:    "manage_missions",
		Operation: "history",
	}

	out, blocked := precheckMessagingToolArgs(tc, RunConfig{MessageSource: "heartbeat"}, "heartbeat")
	if blocked {
		t.Fatalf("expected heartbeat mission read to proceed, got %q", out)
	}
}
