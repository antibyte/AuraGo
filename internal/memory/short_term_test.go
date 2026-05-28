package memory

import (
	"log/slog"
	"os"
	"testing"
	"time"
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

func TestGetHoursSincePreviousUserMessageIgnoresCurrentTurn(t *testing.T) {
	stm := newInMemorySQLiteMemory(t)

	olderID, err := stm.InsertMessage("session-a", "user", "older message", false, false)
	if err != nil {
		t.Fatalf("InsertMessage older: %v", err)
	}
	if _, err := stm.InsertMessage("session-a", "assistant", "assistant reply", false, false); err != nil {
		t.Fatalf("InsertMessage assistant: %v", err)
	}
	latestID, err := stm.InsertMessage("session-a", "user", "current message", false, false)
	if err != nil {
		t.Fatalf("InsertMessage current: %v", err)
	}

	older := time.Now().UTC().Add(-8 * time.Hour).Format("2006-01-02 15:04:05")
	latest := time.Now().UTC().Format("2006-01-02 15:04:05")
	if _, err := stm.db.Exec(`UPDATE messages SET timestamp = ? WHERE id = ?`, older, olderID); err != nil {
		t.Fatalf("backdate older message: %v", err)
	}
	if _, err := stm.db.Exec(`UPDATE messages SET timestamp = ? WHERE id = ?`, latest, latestID); err != nil {
		t.Fatalf("set latest message timestamp: %v", err)
	}

	hours, err := stm.GetHoursSincePreviousUserMessage("session-a")
	if err != nil {
		t.Fatalf("GetHoursSincePreviousUserMessage: %v", err)
	}
	if hours < 7.9 || hours > 8.1 {
		t.Fatalf("hours since previous user message = %.2f, want about 8", hours)
	}
}

func TestGetHoursSincePreviousUserMessageSingleUserMessage(t *testing.T) {
	stm := newInMemorySQLiteMemory(t)
	if _, err := stm.InsertMessage("session-a", "user", "first message", false, false); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	hours, err := stm.GetHoursSincePreviousUserMessage("session-a")
	if err != nil {
		t.Fatalf("GetHoursSincePreviousUserMessage: %v", err)
	}
	if hours != 0 {
		t.Fatalf("hours since previous user message = %.2f, want 0 for single message", hours)
	}
}
