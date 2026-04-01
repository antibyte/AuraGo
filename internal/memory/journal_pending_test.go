package memory

import (
	"io"
	"log/slog"
	"testing"
)

func TestUpsertPendingEpisodicActionAndQuery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertPendingEpisodicAction("2026-04-01", "Help with Nextcloud", "User wants a Docker Compose setup for Nextcloud.", "nextcloud docker compose", "session-a", 3, []string{"doc-1"}); err != nil {
		t.Fatalf("UpsertPendingEpisodicAction: %v", err)
	}
	entries, err := stm.GetPendingEpisodicActionsForQuery("nextcloud", 5)
	if err != nil {
		t.Fatalf("GetPendingEpisodicActionsForQuery: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].ActionStatus != "pending" || entries[0].TriggerQuery == "" {
		t.Fatalf("unexpected pending action payload: %+v", entries[0])
	}
	if err := stm.ResolvePendingEpisodicAction(entries[0].ID); err != nil {
		t.Fatalf("ResolvePendingEpisodicAction: %v", err)
	}
	entries, err = stm.GetPendingEpisodicActionsForQuery("nextcloud", 5)
	if err != nil {
		t.Fatalf("GetPendingEpisodicActionsForQuery after resolve: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("len(entries) after resolve = %d, want 0", len(entries))
	}
}
