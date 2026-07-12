package agent

import (
	"log/slog"
	"strings"
	"testing"
)

func TestConsolidatedAnnouncementFeedbackRestrictsDoneToCompletion(t *testing.T) {
	handler := newConsolidatedRecoveryHandler(nil, NoopBroker{}, slog.Default())
	msg := handler.buildFeedbackMessage(ToolCallProblem{SubType: "announcement_only"}, ToolCall{}, true, false, false, nil)

	if !strings.Contains(msg, "native function-calling") {
		t.Fatalf("native announcement feedback should require native tool calls: %q", msg)
	}
	if strings.Contains(msg, "raw JSON tool call") || strings.Contains(msg, "Output the JSON") {
		t.Fatalf("native announcement feedback must not request legacy JSON tool calls: %q", msg)
	}
	if !strings.Contains(msg, "Append <done/> only if the requested work is fully complete") {
		t.Fatalf("announcement feedback should restrict <done/> to completed work: %q", msg)
	}
	if strings.Contains(msg, "If the task is genuinely complete") {
		t.Fatalf("announcement feedback should avoid soft completion wording: %q", msg)
	}
}

func TestConsolidatedToolInFenceFeedbackUsesNativeMode(t *testing.T) {
	handler := newConsolidatedRecoveryHandler(nil, NoopBroker{}, slog.Default())
	msg := handler.buildFeedbackMessage(ToolCallProblem{SubType: "tool_in_fence"}, ToolCall{}, true, true, false, nil)

	if !strings.Contains(msg, "native function-calling") {
		t.Fatalf("native fence feedback should require native tool calls: %q", msg)
	}
	if strings.Contains(msg, "raw JSON object") || strings.Contains(msg, "Output ONLY the JSON") {
		t.Fatalf("native fence feedback must not request legacy JSON tool calls: %q", msg)
	}
}
