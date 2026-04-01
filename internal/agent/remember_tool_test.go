package agent

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/memory"
)

func TestNormalizeRememberCategorySupportsAliases(t *testing.T) {
	tests := map[string]string{
		"core_memory":     "fact",
		"journal_entry":   "event",
		"notes":           "task",
		"knowledge_graph": "relationship",
	}

	for input, want := range tests {
		if got := normalizeRememberCategory(input); got != want {
			t.Fatalf("normalizeRememberCategory(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestClassifyMemoryTargetPrefersStructuredRelationship(t *testing.T) {
	tc := ToolCall{Source: "server-a", Target: "portainer", Relation: "uses"}
	if got := classifyMemoryTarget(tc, "Portainer is used on server-a"); got != "relationship" {
		t.Fatalf("classifyMemoryTarget() = %q, want relationship", got)
	}
}

func TestClassifyMemoryTargetTreatsInstallationPreferencesAsFact(t *testing.T) {
	content := "Server installation preferences: use Docker Compose and keep volumes under /srv/data"
	if got := classifyMemoryTarget(ToolCall{}, content); got != "fact" {
		t.Fatalf("classifyMemoryTarget() = %q, want fact", got)
	}
}

func TestClassifyMemoryTargetTreatsDebuggingTipAsFact(t *testing.T) {
	content := "Python debugging tip: run pytest -k failing_test before touching the implementation"
	if got := classifyMemoryTarget(ToolCall{}, content); got != "fact" {
		t.Fatalf("classifyMemoryTarget() = %q, want fact", got)
	}
}

func TestClassifyMemoryTargetDetectsTasksConservatively(t *testing.T) {
	content := "Reminder: check the backup retention policy tomorrow"
	if got := classifyMemoryTarget(ToolCall{}, content); got != "task" {
		t.Fatalf("classifyMemoryTarget() = %q, want task", got)
	}
}

func TestClassifyMemoryTargetDetectsEventsWithTemporalSignals(t *testing.T) {
	content := "Completed Docker migration successfully yesterday"
	if got := classifyMemoryTarget(ToolCall{}, content); got != "event" {
		t.Fatalf("classifyMemoryTarget() = %q, want event", got)
	}
}

func TestRememberTaskUsesExplicitTitleForNote(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	out := handleRemember(
		ToolCall{Content: "Check the backup retention policy tomorrow", Title: "Review backup retention", Category: "task"},
		nil,
		logger,
		stm,
		nil,
		"session-test",
	)

	if !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("handleRemember output = %q, want success", out)
	}
	notes, err := stm.SearchNotes("Review backup retention", 5)
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}
	if len(notes) == 0 {
		t.Fatal("expected note to be created with explicit title")
	}
	if notes[0].Title != "Review backup retention" {
		t.Fatalf("note title = %q, want explicit title", notes[0].Title)
	}
}
