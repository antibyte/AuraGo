package agent

import (
	"log/slog"
	"strings"
	"testing"
)

func TestConsolidatedAnnouncementFeedbackRestrictsDoneToCompletion(t *testing.T) {
	handler := newConsolidatedRecoveryHandler(nil, NoopBroker{}, slog.Default())
	msg := handler.buildFeedbackMessage(ToolCallProblem{SubType: "announcement_only"}, ToolCall{}, true, false, false, nil)

	if !strings.Contains(msg, "Append <done/> only if the requested work is fully complete") {
		t.Fatalf("announcement feedback should restrict <done/> to completed work: %q", msg)
	}
	if strings.Contains(msg, "If the task is genuinely complete") {
		t.Fatalf("announcement feedback should avoid soft completion wording: %q", msg)
	}
}
