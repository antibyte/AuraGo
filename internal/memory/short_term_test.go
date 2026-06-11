package memory

import (
	"log/slog"
	"os"
	"strings"
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

func TestGetRecentMessagesGroupedBySessionPreservesSessionOrder(t *testing.T) {
	stm := newInMemorySQLiteMemory(t)

	olderID, err := stm.InsertMessage("session-older", "user", "older-first", false, false)
	if err != nil {
		t.Fatalf("InsertMessage older-first: %v", err)
	}
	olderSecondID, err := stm.InsertMessage("session-older", "assistant", "older-second", false, false)
	if err != nil {
		t.Fatalf("InsertMessage older-second: %v", err)
	}
	newerID, err := stm.InsertMessage("session-newer", "user", "newer-only", false, false)
	if err != nil {
		t.Fatalf("InsertMessage newer-only: %v", err)
	}

	olderTS := time.Now().UTC().Add(-48 * time.Hour).Format("2006-01-02 15:04:05")
	newerTS := time.Now().UTC().Format("2006-01-02 15:04:05")
	if _, err := stm.db.Exec(`UPDATE messages SET timestamp = ? WHERE id IN (?, ?)`, olderTS, olderID, olderSecondID); err != nil {
		t.Fatalf("backdate older session: %v", err)
	}
	if _, err := stm.db.Exec(`UPDATE messages SET timestamp = ? WHERE id = ?`, newerTS, newerID); err != nil {
		t.Fatalf("set newer session timestamp: %v", err)
	}

	messages, err := stm.GetRecentMessagesGroupedBySession(10)
	if err != nil {
		t.Fatalf("GetRecentMessagesGroupedBySession: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d (%#v)", len(messages), messages)
	}
	if messages[0].Content != "newer-only" {
		t.Fatalf("first message = %q, want newer session first", messages[0].Content)
	}
	if messages[1].Content != "older-first" || messages[2].Content != "older-second" {
		t.Fatalf("unexpected older session order: %#v", messages[1:])
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

func TestInsertMessageReturnsZeroOnInsertError(t *testing.T) {
	stm := newInMemorySQLiteMemory(t)
	if _, err := stm.db.Exec(`DROP TABLE messages`); err != nil {
		t.Fatalf("drop messages: %v", err)
	}

	id, err := stm.InsertMessage("session-a", "user", "will fail", false, false)
	if err == nil {
		t.Fatal("InsertMessage error = nil, want insert error")
	}
	if id != 0 {
		t.Fatalf("InsertMessage id = %d, want 0 on error", id)
	}
}

func TestLoadToolUsageAdaptiveReturnsScanError(t *testing.T) {
	stm := newInMemorySQLiteMemory(t)
	if _, err := stm.db.Exec(`
		INSERT INTO tool_usage_adaptive (tool_name, total_count, success_count, last_used)
		VALUES (NULL, 1, 1, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert corrupt tool usage: %v", err)
	}

	entries, err := stm.LoadToolUsageAdaptive()
	if err == nil {
		t.Fatal("LoadToolUsageAdaptive error = nil, want scan error")
	}
	if entries != nil {
		t.Fatalf("LoadToolUsageAdaptive entries = %+v, want nil on scan error", entries)
	}
	if !strings.Contains(err.Error(), "scan tool usage") {
		t.Fatalf("LoadToolUsageAdaptive error = %v, want scan context", err)
	}
}
