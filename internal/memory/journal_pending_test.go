package memory

import (
	"fmt"
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
	greetingEntries, err := stm.GetPendingEpisodicActionsForQuery("hi", 5)
	if err != nil {
		t.Fatalf("GetPendingEpisodicActionsForQuery greeting: %v", err)
	}
	if len(greetingEntries) != 0 {
		t.Fatalf("greeting matched pending actions: %#v", greetingEntries)
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

func TestPendingEpisodicActionFindsRelevantEntryBelowFormerTopFifty(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	for i := 0; i < 55; i++ {
		if err := stm.UpsertPendingEpisodicAction(
			"2026-07-22",
			fmt.Sprintf("Unrelated backup task %02d", i),
			"Review the backup queue and retention policy.",
			fmt.Sprintf("backup retention queue %02d", i),
			"session-noise",
			4,
			nil,
		); err != nil {
			t.Fatalf("insert unrelated action %d: %v", i, err)
		}
	}
	if err := stm.UpsertPendingEpisodicAction(
		"2026-07-20",
		"Repair Tailscale route",
		"Inspect the Tailscale subnet route after the next login.",
		"tailscale subnet route",
		"session-target",
		2,
		nil,
	); err != nil {
		t.Fatalf("insert relevant action: %v", err)
	}

	entries, err := stm.GetPendingEpisodicActionsForQuery("Tailscale Status", 1)
	if err != nil {
		t.Fatalf("GetPendingEpisodicActionsForQuery: %v", err)
	}
	if len(entries) != 1 || entries[0].Title != "Repair Tailscale route" {
		t.Fatalf("relevant low-ranked pending action not found: %#v", entries)
	}
}

func TestPendingEpisodicActionRequiresMeaningfulTopicOverlap(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertPendingEpisodicAction(
		"2026-07-21",
		"MiniMax-Bild erzeugen",
		"Erzeuge das angefragte Bild später erneut mit MiniMax.",
		"versuche es erneut mit dem MiniMax Bild erzeugen",
		"session-image",
		3,
		nil,
	); err != nil {
		t.Fatalf("UpsertPendingEpisodicAction: %v", err)
	}

	generic, err := stm.GetPendingEpisodicActionsForQuery("versuche es erneut", 5)
	if err != nil {
		t.Fatalf("generic retry query: %v", err)
	}
	if len(generic) != 0 {
		t.Fatalf("generic retry activated stale image follow-up: %#v", generic)
	}

	specific, err := stm.GetPendingEpisodicActionsForQuery("MiniMax Bild erzeugen", 5)
	if err != nil {
		t.Fatalf("specific image query: %v", err)
	}
	if len(specific) != 1 {
		t.Fatalf("specific query matched %d actions, want 1", len(specific))
	}
}
