package agent

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/memory"
)

func TestRecordToolAuditEventMapsExecutionOutcome(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	recordToolAuditEvent(stm, logger, ToolCall{
		Action:    "execute_shell",
		Operation: "run",
		Command:   "echo hello",
	}, toolExecutionResult{
		Content: "hello",
		Failed:  false,
		Outcome: ExecutionOutcomeSuccess,
	}, "sess-1", "web_chat", 37)

	recordToolAuditEvent(stm, logger, ToolCall{
		Action: "delete_file",
		Path:   "/tmp/x",
	}, toolExecutionResult{
		Content: "[TOOL BLOCKED]\nGuardian blocked this action",
		Failed:  true,
		Outcome: ExecutionOutcomeGuardianBlocked,
	}, "sess-1", "web_chat", 12)

	page, err := stm.SearchAuditEvents(memory.AuditFilter{Source: memory.AuditSourceAgentTool, Limit: 10})
	if err != nil {
		t.Fatalf("SearchAuditEvents: %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("total = %d, want 2", page.Total)
	}
	if page.Entries[0].Status != memory.AuditStatusBlocked {
		t.Fatalf("latest status = %q, want blocked", page.Entries[0].Status)
	}
	if page.Entries[1].Status != memory.AuditStatusSuccess {
		t.Fatalf("older status = %q, want success", page.Entries[1].Status)
	}
	if page.Entries[1].TargetName != "execute_shell" {
		t.Fatalf("target name = %q, want execute_shell", page.Entries[1].TargetName)
	}
}
