package planner

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestOverdueClaimSurvivesNotifierRestartAndPanic(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	_, err := CreateTodo(db, Todo{Title: "claim test", Status: "open", Priority: "low", DueDate: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for i := 0; i < 2; i++ {
		notifier := NewNotifier(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
		notifier.SetTodoOverdueTrigger(func(Todo) { calls++; panic("simulated uncertain dispatch") })
		notifier.checkOverdueTodos()
	}
	if calls != 1 {
		t.Fatalf("dispatched %d times across restart", calls)
	}
	var status string
	if err := db.QueryRow("SELECT value FROM planner_meta WHERE key LIKE 'overdue_todo:%'").Scan(&status); err != nil || status != "claimed" {
		t.Fatalf("uncertain claim = %q %v", status, err)
	}
}
