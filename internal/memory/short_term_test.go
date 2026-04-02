package memory

import (
	"log/slog"
	"os"
	"testing"
)

func newInMemorySQLiteMemory(t *testing.T) *SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	return stm
}

func TestGetRecentMessagesAcrossSessions(t *testing.T) {
	stm := newInMemorySQLiteMemory(t)

	if _, err := stm.InsertMessage("session-a", "user", "alpha", false, false); err != nil {
		t.Fatalf("InsertMessage alpha: %v", err)
	}
	if _, err := stm.InsertMessage("session-b", "assistant", "beta", false, false); err != nil {
		t.Fatalf("InsertMessage beta: %v", err)
	}
	if _, err := stm.InsertMessage("session-a", "user", "gamma", false, false); err != nil {
		t.Fatalf("InsertMessage gamma: %v", err)
	}

	messages, err := stm.GetRecentMessagesAcrossSessions(2)
	if err != nil {
		t.Fatalf("GetRecentMessagesAcrossSessions: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Content != "beta" || messages[1].Content != "gamma" {
		t.Fatalf("unexpected message order/content: %#v", messages)
	}
}
