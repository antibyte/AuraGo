package memory

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

func newAuditTestMemory(t *testing.T) *SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	return stm
}

func TestAuditEventsRecordSearchUpdateAndDelete(t *testing.T) {
	stm := newAuditTestMemory(t)
	startedAt := time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339)

	id, err := stm.RecordAuditEvent(AuditEvent{
		Timestamp:     startedAt,
		Source:        AuditSourceMission,
		EventType:     "mission_run",
		Actor:         "agent",
		SessionID:     "mission",
		TargetID:      "mission_backup",
		TargetName:    "Nightly Backup",
		Status:        AuditStatusRunning,
		Summary:       "Mission Nightly Backup started",
		Detail:        "trigger=cron",
		CorrelationID: "run_123",
		MetadataJSON:  `{"trigger_type":"cron"}`,
	})
	if err != nil {
		t.Fatalf("RecordAuditEvent: %v", err)
	}
	if id <= 0 {
		t.Fatalf("audit id = %d, want positive", id)
	}

	if err := stm.UpsertAuditEventByCorrelation(AuditEvent{
		Source:        AuditSourceMission,
		EventType:     "mission_run",
		TargetID:      "mission_backup",
		TargetName:    "Nightly Backup",
		Status:        AuditStatusSuccess,
		Summary:       "Mission Nightly Backup completed",
		Detail:        "42 files backed up",
		DurationMS:    1250,
		CorrelationID: "run_123",
	}); err != nil {
		t.Fatalf("UpsertAuditEventByCorrelation: %v", err)
	}

	page, err := stm.SearchAuditEvents(AuditFilter{
		Q:      "backup",
		Source: AuditSourceMission,
		Status: AuditStatusSuccess,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("SearchAuditEvents: %v", err)
	}
	if page.Total != 1 || len(page.Entries) != 1 {
		t.Fatalf("page total=%d len=%d, want one entry", page.Total, len(page.Entries))
	}
	got := page.Entries[0]
	if got.ID != id {
		t.Fatalf("updated entry id=%d, want original id %d", got.ID, id)
	}
	if got.Status != AuditStatusSuccess {
		t.Fatalf("status = %q, want success", got.Status)
	}
	if got.DurationMS != 1250 {
		t.Fatalf("duration = %d, want 1250", got.DurationMS)
	}

	deleted, err := stm.DeleteAuditEvents(AuditFilter{Source: AuditSourceMission}, "DELETE_AUDIT_EVENTS")
	if err != nil {
		t.Fatalf("DeleteAuditEvents: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
}

func TestAuditEventsRequireBulkDeleteConfirmationAndScrubSecrets(t *testing.T) {
	stm := newAuditTestMemory(t)
	security.RegisterSensitive("top-secret-token")

	if _, err := stm.RecordAuditEvent(AuditEvent{
		Source:    AuditSourceAgentTool,
		EventType: "tool_call",
		Status:    AuditStatusSuccess,
		Summary:   "execute_shell used top-secret-token",
		Detail:    strings.Repeat("A", 2600) + " top-secret-token",
	}); err != nil {
		t.Fatalf("RecordAuditEvent: %v", err)
	}

	if _, err := stm.DeleteAuditEvents(AuditFilter{}, ""); err == nil {
		t.Fatal("expected confirmation error for bulk delete")
	}

	page, err := stm.SearchAuditEvents(AuditFilter{Limit: 1})
	if err != nil {
		t.Fatalf("SearchAuditEvents: %v", err)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(page.Entries))
	}
	if strings.Contains(page.Entries[0].Summary, "top-secret-token") || strings.Contains(page.Entries[0].Detail, "top-secret-token") {
		t.Fatalf("audit entry leaked sensitive value: %#v", page.Entries[0])
	}
	if len([]rune(page.Entries[0].Detail)) > 2000 {
		t.Fatalf("detail length = %d, want <= 2000 runes", len([]rune(page.Entries[0].Detail)))
	}
}
