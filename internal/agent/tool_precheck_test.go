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