package agent

import (
	"log/slog"
	"os"
	"testing"

	"aurago/internal/memory"
)

func newTestPersonalityRuntimeMemory(t *testing.T) *memory.SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitEmotionTables(); err != nil {
		t.Fatalf("InitEmotionTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	return stm
}

func TestLatestEmotionDescriptionFallsBackToPersistedHistory(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	if err := stm.InsertEmotionHistory("I feel calm and ready to help.", "focused", "user asked a clear question"); err != nil {
		t.Fatalf("InsertEmotionHistory: %v", err)
	}

	got := latestEmotionDescription(stm, nil)
	if got != "I feel calm and ready to help." {
		t.Fatalf("latestEmotionDescription() = %q, want persisted emotion", got)
	}
}
