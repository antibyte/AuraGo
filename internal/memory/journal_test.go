package memory

import (
	"log/slog"
	"os"
	"testing"
)

func newTestJournalDB(t *testing.T) *SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

func TestInsertAndGetJournalEntry(t *testing.T) {
	stm := newTestJournalDB(t)

	id, err := stm.InsertJournalEntry(JournalEntry{
		EntryType:  "milestone",
		Title:      "First Deploy",
		Content:    "Deployed successfully",
		Tags:       []string{"deploy", "docker"},
		Importance: 4,
		SessionID:  "session-1",
	})
	if err != nil {
		t.Fatalf("InsertJournalEntry: %v", err)
	}
	if id <= 0 {
		t.Fatalf("Expected positive ID, got %d", id)
	}

	entries, err := stm.GetJournalEntries("", "", nil, 10)
	if err != nil {
		t.Fatalf("GetJournalEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}
	if entries[0].Title != "First Deploy" {
		t.Errorf("Expected title 'First Deploy', got %q", entries[0].Title)
	}
	if entries[0].EntryType != "milestone" {
		t.Errorf("Expected type 'milestone', got %q", entries[0].EntryType)
	}
	if entries[0].Importance != 4 {
		t.Errorf("Expected importance 4, got %d", entries[0].Importance)
	}
	if len(entries[0].Tags) != 2 || entries[0].Tags[0] != "deploy" {
		t.Errorf("Expected tags [deploy docker], got %v", entries[0].Tags)
	}
}

func TestGetJournalEntriesTypeFilter(t *testing.T) {
	stm := newTestJournalDB(t)

	stm.InsertJournalEntry(JournalEntry{EntryType: "milestone", Title: "Milestone 1", Importance: 4})
	stm.InsertJournalEntry(JournalEntry{EntryType: "reflection", Title: "Reflection 1", Importance: 2})
	stm.InsertJournalEntry(JournalEntry{EntryType: "milestone", Title: "Milestone 2", Importance: 3})

	entries, err := stm.GetJournalEntries("", "", []string{"milestone"}, 10)
	if err != nil {
		t.Fatalf("GetJournalEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("Expected 2 milestone entries, got %d", len(entries))
	}
}

func TestSearchJournalEntries(t *testing.T) {
	stm := newTestJournalDB(t)

	stm.InsertJournalEntry(JournalEntry{EntryType: "task_completed", Title: "Docker Migration", Content: "Moved containers to new host", Tags: []string{"docker", "server"}, Importance: 4})
	stm.InsertJournalEntry(JournalEntry{EntryType: "reflection", Title: "Quiet Day", Content: "Not much happened", Tags: []string{"general"}, Importance: 2})

	entries, err := stm.SearchJournalEntries("docker", 10)
	if err != nil {
		t.Fatalf("SearchJournalEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 result for 'docker', got %d", len(entries))
	}
	if entries[0].Title != "Docker Migration" {
		t.Errorf("Expected 'Docker Migration', got %q", entries[0].Title)
	}
}

func TestDeleteJournalEntry(t *testing.T) {
	stm := newTestJournalDB(t)

	id, _ := stm.InsertJournalEntry(JournalEntry{EntryType: "reflection", Title: "To Delete", Importance: 1})
	err := stm.DeleteJournalEntry(id)
	if err != nil {
		t.Fatalf("DeleteJournalEntry: %v", err)
	}

	entries, _ := stm.GetJournalEntries("", "", nil, 10)
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after delete, got %d", len(entries))
	}
}

func TestDailySummary(t *testing.T) {
	stm := newTestJournalDB(t)

	err := stm.InsertDailySummary(DailySummary{
		Date:      "2025-01-15",
		Summary:   "Productive day with Docker work",
		KeyTopics: []string{"docker", "deployment"},
		ToolUsage: map[string]int{"shell": 3, "docker": 5},
		Sentiment: "positive",
	})
	if err != nil {
		t.Fatalf("InsertDailySummary: %v", err)
	}

	summary, err := stm.GetDailySummary("2025-01-15")
	if err != nil {
		t.Fatalf("GetDailySummary: %v", err)
	}
	if summary == nil {
		t.Fatal("Expected summary, got nil")
	}
	if summary.Summary != "Productive day with Docker work" {
		t.Errorf("Expected summary text, got %q", summary.Summary)
	}
	if summary.Sentiment != "positive" {
		t.Errorf("Expected 'positive' sentiment, got %q", summary.Sentiment)
	}

	// Test upsert
	err = stm.InsertDailySummary(DailySummary{
		Date:      "2025-01-15",
		Summary:   "Updated summary",
		KeyTopics: []string{"docker"},
		Sentiment: "neutral",
	})
	if err != nil {
		t.Fatalf("InsertDailySummary upsert: %v", err)
	}
	summary, err = stm.GetDailySummary("2025-01-15")
	if err != nil {
		t.Fatalf("GetDailySummary after upsert: %v", err)
	}
	if summary.Summary != "Updated summary" {
		t.Errorf("Expected updated summary, got %q", summary.Summary)
	}
}

func TestGetRecentDailySummaries(t *testing.T) {
	stm := newTestJournalDB(t)

	stm.InsertDailySummary(DailySummary{Date: "2025-01-13", Summary: "Day 1", KeyTopics: []string{"topics"}, Sentiment: "neutral"})
	stm.InsertDailySummary(DailySummary{Date: "2025-01-14", Summary: "Day 2", KeyTopics: []string{"topics"}, Sentiment: "positive"})
	stm.InsertDailySummary(DailySummary{Date: "2025-01-15", Summary: "Day 3", KeyTopics: []string{"topics"}, Sentiment: "positive"})

	summaries, err := stm.GetRecentDailySummaries(2)
	if err != nil {
		t.Fatalf("GetRecentDailySummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Errorf("Expected 2 summaries, got %d", len(summaries))
	}
	if summaries[0].Date != "2025-01-15" {
		t.Errorf("Expected most recent first, got %q", summaries[0].Date)
	}
}

func TestGetJournalStats(t *testing.T) {
	stm := newTestJournalDB(t)

	stm.InsertJournalEntry(JournalEntry{EntryType: "milestone", Title: "M1", Importance: 4})
	stm.InsertJournalEntry(JournalEntry{EntryType: "reflection", Title: "R1", Importance: 2})
	stm.InsertJournalEntry(JournalEntry{EntryType: "milestone", Title: "M2", Importance: 3, AutoGenerated: true})

	stats, err := stm.GetJournalStats("", "")
	if err != nil {
		t.Fatalf("GetJournalStats: %v", err)
	}
	if stats["milestone"] != 2 {
		t.Errorf("Expected 2 milestones, got %d", stats["milestone"])
	}
	if stats["reflection"] != 1 {
		t.Errorf("Expected 1 reflection, got %d", stats["reflection"])
	}
}

func TestFormatJournalEntriesJSON(t *testing.T) {
	entries := []JournalEntry{
		{ID: 1, EntryType: "milestone", Title: "Test", Importance: 4},
	}
	result := FormatJournalEntriesJSON(entries)
	if result == "[]" || result == "" {
		t.Error("Expected non-empty JSON output")
	}
}

func TestGetJournalEntryNotFound(t *testing.T) {
	stm := newTestJournalDB(t)

	summary, err := stm.GetDailySummary("2099-12-31")
	if err != nil {
		t.Fatalf("GetDailySummary: %v", err)
	}
	if summary != nil {
		t.Error("Expected nil for non-existent date")
	}
}

func TestJournalLimitEnforced(t *testing.T) {
	stm := newTestJournalDB(t)

	for i := 0; i < 10; i++ {
		stm.InsertJournalEntry(JournalEntry{EntryType: "reflection", Title: "Entry", Importance: 2})
	}

	entries, err := stm.GetJournalEntries("", "", nil, 3)
	if err != nil {
		t.Fatalf("GetJournalEntries: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries with limit, got %d", len(entries))
	}
}
