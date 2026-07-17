package agent

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestLocalAgentMemoryAdaptersReuseNativeMemoryResults(t *testing.T) {
	vectorDB := &fakeVectorDB{
		documents: map[string]string{"memory-1": "remembered content"},
	}
	search, err := QueryMemoryForLocalAgent("Vincenzo", 5, nil, vectorDB, nil, nil, nil)
	if err != nil {
		t.Fatalf("QueryMemoryForLocalAgent: %v", err)
	}
	if search["status"] != "success" || !vectorDB.searchSimilarCalled {
		t.Fatalf("search result = %#v, search called = %v", search, vectorDB.searchSimilarCalled)
	}

	recall, err := RecallMemoryForLocalAgent("memory-1", vectorDB)
	if err != nil {
		t.Fatalf("RecallMemoryForLocalAgent: %v", err)
	}
	results, ok := recall["results"].([]interface{})
	if !ok || len(results) != 1 {
		t.Fatalf("recall results = %#v", recall["results"])
	}
	item, ok := results[0].(map[string]interface{})
	if !ok || item["id"] != "memory-1" || item["content"] != "remembered content" {
		t.Fatalf("recall item = %#v", results[0])
	}
}

func TestSyncLocalAgentTurnStoresStatusSourceProviderAndJournal(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	cfg := &config.Config{}
	cfg.Tools.Journal.Enabled = true
	cfg.Journal.AutoEntries = true
	timestamp := time.Date(2026, 7, 17, 9, 30, 0, 0, time.UTC)

	if err := SyncLocalAgentTurn(context.Background(), cfg, slog.Default(), stm, nil, nil, LocalAgentTurnSync{
		SessionID:        "sess-local",
		UserMessage:      "Inspect the project status",
		AssistantMessage: "The project is healthy.",
		Status:           "completed",
		Provider:         "main/test-model",
		ClientTimestamp:  timestamp,
		ToolNames:        []string{"workspace_search"},
		ToolSummaries:    []string{"workspace_search: completed (README.md)"},
	}); err != nil {
		t.Fatalf("SyncLocalAgentTurn completed: %v", err)
	}
	turns, err := stm.SearchActivityTurnsInRange("Inspect", "", "", 10)
	if err != nil {
		t.Fatalf("SearchActivityTurnsInRange: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("activity turns = %d, want 1", len(turns))
	}
	if turns[0].Status != "completed" || turns[0].Source != "agodesk_local_agent" || turns[0].Channel != "agodesk_local_agent" {
		t.Fatalf("activity turn = %+v", turns[0])
	}
	if turns[0].Timestamp != timestamp.Format(time.RFC3339) {
		t.Fatalf("timestamp = %q, want %q", turns[0].Timestamp, timestamp.Format(time.RFC3339))
	}
	if len(turns[0].ImportantPoints) == 0 || turns[0].ImportantPoints[len(turns[0].ImportantPoints)-1] != "Provider: main/test-model" {
		t.Fatalf("important points = %#v", turns[0].ImportantPoints)
	}
	journal, err := stm.GetJournalEntries("", "", nil, 10)
	if err != nil {
		t.Fatalf("GetJournalEntries: %v", err)
	}
	if len(journal) == 0 || journal[0].EntryType != "activity" {
		t.Fatalf("journal = %+v", journal)
	}
}

func TestSyncLocalAgentFailedTurnUsesStatusJournal(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	cfg := &config.Config{}
	cfg.Tools.Journal.Enabled = true
	cfg.Journal.AutoEntries = true

	if err := SyncLocalAgentTurn(context.Background(), cfg, slog.Default(), stm, nil, nil, LocalAgentTurnSync{
		SessionID:       "sess-failed",
		UserMessage:     "Run the check",
		Status:          "failed",
		ClientTimestamp: time.Now().UTC(),
		ToolNames:       []string{"execute_shell"},
	}); err != nil {
		t.Fatalf("SyncLocalAgentTurn failed: %v", err)
	}
	turns, err := stm.SearchActivityTurnsInRange("Run the check", "", "", 10)
	if err != nil || len(turns) != 1 {
		t.Fatalf("activity turns = %+v, err = %v", turns, err)
	}
	if turns[0].Status != "failed" {
		t.Fatalf("status = %q, want failed", turns[0].Status)
	}
	journal, err := stm.GetJournalEntries("", "", nil, 10)
	if err != nil || len(journal) != 1 {
		t.Fatalf("journal = %+v, err = %v", journal, err)
	}
	if journal[0].Title != "Local agent turn failed" {
		t.Fatalf("journal title = %q", journal[0].Title)
	}
}
