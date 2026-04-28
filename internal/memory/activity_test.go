package memory

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestActivityOverviewBuildsFromTurnsAndNotes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}

	if _, err := stm.AddNote("todo", "Review deployment logs", "Check last run", 3, ""); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if _, err := stm.InsertActivityTurn(ActivityTurn{
		Date:            time.Now().Format("2006-01-02"),
		SessionID:       "default",
		Channel:         "web_chat",
		UserRelevant:    true,
		Intent:          "Deploy homepage update",
		UserRequest:     "Please deploy the homepage update",
		UserGoal:        "Deploy homepage update",
		ActionsTaken:    []string{"execute_shell: completed - docker compose up"},
		Outcomes:        []string{"Homepage update deployed successfully"},
		ImportantPoints: []string{"Deployment completed without downtime"},
		PendingItems:    []string{"Review deployment logs"},
		ToolNames:       []string{"execute_shell"},
		Source:          "runtime",
	}); err != nil {
		t.Fatalf("InsertActivityTurn: %v", err)
	}

	overview, err := stm.BuildRecentActivityOverview(7, true)
	if err != nil {
		t.Fatalf("BuildRecentActivityOverview: %v", err)
	}
	if overview == nil {
		t.Fatal("expected overview")
	}
	if !strings.Contains(overview.OverviewSummary, "Last 7 days overview") {
		t.Fatalf("overview summary = %q", overview.OverviewSummary)
	}
	if len(overview.Days) == 0 {
		t.Fatal("expected at least one day rollup")
	}
	if len(overview.PendingItems) == 0 || overview.PendingItems[0] != "Review deployment logs" {
		t.Fatalf("pending items = %#v", overview.PendingItems)
	}
	if len(overview.Entries) == 0 {
		t.Fatal("expected recent entries")
	}
}

func TestBuildRecentActivityPromptOverviewIncludesSummaryAndOpenItems(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}

	if _, err := stm.AddNote("todo", "Document rollback plan", "", 3, ""); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if _, err := stm.InsertActivityTurn(ActivityTurn{
		Date:            "2026-03-28",
		SessionID:       "default",
		UserRelevant:    true,
		Intent:          "Investigate backup issue",
		UserRequest:     "Please investigate the backup issue",
		UserGoal:        "Investigate backup issue",
		ActionsTaken:    []string{"query_memory"},
		Outcomes:        []string{"Found the root cause in yesterday's backup configuration"},
		ImportantPoints: []string{"The backup path changed unexpectedly"},
		Source:          "runtime",
	}); err != nil {
		t.Fatalf("InsertActivityTurn: %v", err)
	}

	promptView, err := stm.BuildRecentActivityPromptOverview(7)
	if err != nil {
		t.Fatalf("BuildRecentActivityPromptOverview: %v", err)
	}
	if !strings.Contains(promptView, "Summary:") {
		t.Fatalf("prompt overview = %q", promptView)
	}
	if !strings.Contains(promptView, "Open items:") {
		t.Fatalf("prompt overview = %q", promptView)
	}
}

func TestBuildRecentActivityPromptOverviewSkipsStaleOpenNotes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}

	id, err := stm.AddNote("todo", "Old WebGL follow-up", "", 2, "")
	if err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	old := time.Now().AddDate(0, 0, -14).UTC().Format(time.RFC3339)
	if _, err := stm.db.Exec(`UPDATE notes SET created_at=?, updated_at=? WHERE id=?`, old, old, id); err != nil {
		t.Fatalf("age note: %v", err)
	}
	if _, err := stm.InsertActivityTurn(ActivityTurn{
		Date:         time.Now().Format("2006-01-02"),
		SessionID:    "default",
		UserRelevant: true,
		Intent:       "Check current status",
		UserRequest:  "gibts was neues?",
		UserGoal:     "Get current status",
		Outcomes:     []string{"Current status checked"},
		Source:       "runtime",
	}); err != nil {
		t.Fatalf("InsertActivityTurn: %v", err)
	}

	promptView, err := stm.BuildRecentActivityPromptOverview(3)
	if err != nil {
		t.Fatalf("BuildRecentActivityPromptOverview: %v", err)
	}
	if strings.Contains(promptView, "Old WebGL follow-up") {
		t.Fatalf("stale note leaked into recent activity prompt: %q", promptView)
	}
}
