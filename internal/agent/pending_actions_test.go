package agent

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/memory"
)

func TestResolveCompletedPendingActionsMarksResolvedWhenResponseAddressesTopic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertPendingEpisodicAction("2026-04-01", "Help with Nextcloud", "Prepare Docker Compose setup for Nextcloud reverse proxy", "nextcloud docker reverse proxy", "sess-1", 3, nil); err != nil {
		t.Fatalf("UpsertPendingEpisodicAction: %v", err)
	}
	pending, err := stm.GetPendingEpisodicActionsForQuery("nextcloud", 5)
	if err != nil {
		t.Fatalf("GetPendingEpisodicActionsForQuery: %v", err)
	}
	resolved := resolveCompletedPendingActions(stm, "please help with nextcloud docker", "Here is the Docker Compose setup for Nextcloud with reverse proxy and the matching container configuration.", pending)
	if len(resolved) != 1 {
		t.Fatalf("len(resolved) = %d, want 1", len(resolved))
	}
	remaining, err := stm.GetPendingEpisodicActionsForQuery("nextcloud", 5)
	if err != nil {
		t.Fatalf("GetPendingEpisodicActionsForQuery after resolve: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("len(remaining) = %d, want 0", len(remaining))
	}
}

func TestResolveCompletedPendingActionsKeepsDeferredResponsesOpen(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertPendingEpisodicAction("2026-04-01", "Review SSL renewal", "Follow up on SSL renewal before Friday", "ssl renewal friday", "sess-1", 3, nil); err != nil {
		t.Fatalf("UpsertPendingEpisodicAction: %v", err)
	}
	pending, err := stm.GetPendingEpisodicActionsForQuery("ssl", 5)
	if err != nil {
		t.Fatalf("GetPendingEpisodicActionsForQuery: %v", err)
	}
	resolved := resolveCompletedPendingActions(stm, "can you handle ssl renewal", "If you want, I can do that later once you send the certificate details.", pending)
	if len(resolved) != 0 {
		t.Fatalf("len(resolved) = %d, want 0", len(resolved))
	}
}

func TestShouldInjectRecentMemoryContextSkipsBareGreeting(t *testing.T) {
	if shouldInjectRecentMemoryContext("hi") {
		t.Fatal("bare greeting should not inject recent memory context")
	}
	if !shouldInjectRecentMemoryContext("gibts was neues?") {
		t.Fatal("status query should inject recent memory context")
	}
	if !shouldInjectRecentMemoryContext("prüfe tailscale status") {
		t.Fatal("specific operational query should inject recent memory context")
	}
}
