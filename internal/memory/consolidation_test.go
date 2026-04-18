package memory

import (
	"database/sql"
	"log/slog"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestConsolidationDB(t *testing.T) *SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

func TestDeleteOldMessagesArchives(t *testing.T) {
	stm := newTestConsolidationDB(t)

	// Insert 5 messages: system, user, assistant, user, assistant
	for _, m := range []struct{ role, content string }{
		{"system", "System prompt"},
		{"user", "Hello"},
		{"assistant", "Hi there"},
		{"user", "How are you?"},
		{"assistant", "I'm fine, thanks!"},
	} {
		if _, err := stm.InsertMessage("default", m.role, m.content, false, false); err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
	}

	// Keep only 2 messages — should archive the older user/assistant messages
	if err := stm.DeleteOldMessages("default", 2); err != nil {
		t.Fatalf("DeleteOldMessages: %v", err)
	}

	// Check that archived messages exist (only user+assistant, not system)
	archived, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages: %v", err)
	}
	if len(archived) < 1 {
		t.Fatalf("Expected archived messages, got %d", len(archived))
	}
	// Verify no system messages were archived
	for _, msg := range archived {
		if msg.Role == "system" {
			t.Errorf("System messages should not be archived, got role=%q", msg.Role)
		}
	}
}

func TestGetUnconsolidatedMessagesEmpty(t *testing.T) {
	stm := newTestConsolidationDB(t)

	msgs, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(msgs))
	}
}

func TestMarkConsolidated(t *testing.T) {
	stm := newTestConsolidationDB(t)

	// Insert messages then trigger archival
	for i := 0; i < 6; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		if _, err := stm.InsertMessage("default", role, "msg content", false, false); err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
	}
	if err := stm.DeleteOldMessages("default", 2); err != nil {
		t.Fatalf("DeleteOldMessages: %v", err)
	}

	// Get unconsolidated
	msgs, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("Expected unconsolidated messages")
	}

	// Mark them as consolidated
	var ids []int64
	for _, m := range msgs {
		ids = append(ids, m.ID)
	}
	if err := stm.MarkConsolidated(ids); err != nil {
		t.Fatalf("MarkConsolidated: %v", err)
	}

	// Should now return empty
	remaining, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages after mark: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("Expected 0 unconsolidated after marking, got %d", len(remaining))
	}
}

func TestMarkConsolidatedEmpty(t *testing.T) {
	stm := newTestConsolidationDB(t)
	// Should be a no-op, not an error
	if err := stm.MarkConsolidated(nil); err != nil {
		t.Fatalf("MarkConsolidated(nil): %v", err)
	}
	if err := stm.MarkConsolidated([]int64{}); err != nil {
		t.Fatalf("MarkConsolidated([]): %v", err)
	}
}

func TestCleanOldArchivedMessages(t *testing.T) {
	stm := newTestConsolidationDB(t)

	// Insert and archive some messages
	for i := 0; i < 4; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		if _, err := stm.InsertMessage("default", role, "content", false, false); err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
	}
	if err := stm.DeleteOldMessages("default", 1); err != nil {
		t.Fatalf("DeleteOldMessages: %v", err)
	}

	// With a large retain period, nothing is deleted — even if consolidated.
	cleaned, err := stm.CleanOldArchivedMessages(365)
	if err != nil {
		t.Fatalf("CleanOldArchivedMessages: %v", err)
	}
	if cleaned != 0 {
		t.Errorf("Expected 0 cleaned (messages are fresh), got %d", cleaned)
	}

	// Unconsolidated rows must NOT be cleaned even when old.
	_, err = stm.db.Exec("UPDATE archived_messages SET archived_at = datetime('now', '-60 days')")
	if err != nil {
		t.Fatalf("Backdate unconsolidated: %v", err)
	}
	cleaned, err = stm.CleanOldArchivedMessages(30)
	if err != nil {
		t.Fatalf("CleanOldArchivedMessages: %v", err)
	}
	if cleaned != 0 {
		t.Errorf("Expected 0 cleaned (not consolidated yet), got %d", cleaned)
	}

	// Mark all as successfully consolidated, then backdate again.
	unconsolidated, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages: %v", err)
	}
	var ids []int64
	for _, m := range unconsolidated {
		ids = append(ids, m.ID)
	}
	if err := stm.MarkConsolidationSuccess(ids); err != nil {
		t.Fatalf("MarkConsolidationSuccess: %v", err)
	}

	// Backdate so they become eligible for cleanup.
	_, err = stm.db.Exec("UPDATE archived_messages SET archived_at = datetime('now', '-60 days')")
	if err != nil {
		t.Fatalf("Backdate consolidated: %v", err)
	}

	cleaned, err = stm.CleanOldArchivedMessages(30)
	if err != nil {
		t.Fatalf("CleanOldArchivedMessages: %v", err)
	}
	if cleaned == 0 {
		t.Error("Expected some messages cleaned after consolidation + backdating, got 0")
	}

	// Verify they're gone
	remaining, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("Expected 0 remaining after cleanup, got %d", len(remaining))
	}
}

func TestGetUnconsolidatedMessagesLimit(t *testing.T) {
	stm := newTestConsolidationDB(t)

	// Insert many messages then archive
	for i := 0; i < 10; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		if _, err := stm.InsertMessage("default", role, "msg", false, false); err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
	}
	if err := stm.DeleteOldMessages("default", 1); err != nil {
		t.Fatalf("DeleteOldMessages: %v", err)
	}

	// Verify limit works
	msgs, err := stm.GetUnconsolidatedMessages(2)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages: %v", err)
	}
	if len(msgs) > 2 {
		t.Errorf("Expected at most 2 messages with limit=2, got %d", len(msgs))
	}
}

func TestArchivedMessageFields(t *testing.T) {
	stm := newTestConsolidationDB(t)

	if _, err := stm.InsertMessage("sess-1", "user", "Hello world", false, false); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}
	if _, err := stm.InsertMessage("sess-1", "assistant", "Hi!", false, false); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}
	if _, err := stm.InsertMessage("sess-1", "user", "Keep this", false, false); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	if err := stm.DeleteOldMessages("sess-1", 1); err != nil {
		t.Fatalf("DeleteOldMessages: %v", err)
	}

	msgs, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("Expected archived messages")
	}

	// Verify fields are populated
	for _, m := range msgs {
		if m.ID <= 0 {
			t.Errorf("Expected positive ID, got %d", m.ID)
		}
		if m.SessionID != "sess-1" {
			t.Errorf("Expected session_id 'sess-1', got %q", m.SessionID)
		}
		if m.Role == "" {
			t.Error("Role should not be empty")
		}
		if m.Content == "" {
			t.Error("Content should not be empty")
		}
		if m.Timestamp == "" {
			t.Error("Timestamp should not be empty")
		}
	}
}

func TestNewSQLiteMemoryNormalizesInvalidConsolidationStatuses(t *testing.T) {
	path := t.TempDir() + "/memory.db"
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE archived_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT,
			role TEXT,
			content TEXT,
			original_timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			archived_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			consolidated BOOLEAN DEFAULT 0,
			consolidation_status TEXT,
			consolidation_retries INTEGER DEFAULT 0,
			consolidation_last_error TEXT DEFAULT '',
			next_retry_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("create archived_messages: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO archived_messages(session_id, role, content, consolidated, consolidation_status)
		VALUES
			('s1', 'user', 'pending item', 0, 'retrying'),
			('s1', 'assistant', 'done item', 1, 'weird_done')
	`)
	if err != nil {
		t.Fatalf("insert archived_messages: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { stm.Close() })

	var pendingStatus, doneStatus string
	if err := stm.db.QueryRow("SELECT consolidation_status FROM archived_messages WHERE content = 'pending item'").Scan(&pendingStatus); err != nil {
		t.Fatalf("query pending status: %v", err)
	}
	if err := stm.db.QueryRow("SELECT consolidation_status FROM archived_messages WHERE content = 'done item'").Scan(&doneStatus); err != nil {
		t.Fatalf("query done status: %v", err)
	}
	if pendingStatus != "pending" {
		t.Fatalf("pending status = %q, want pending", pendingStatus)
	}
	if doneStatus != "done" {
		t.Fatalf("done status = %q, want done", doneStatus)
	}
}

func TestGetConsolidationCandidatesHonorsRetryWindow(t *testing.T) {
	stm := newTestConsolidationDB(t)
	_, err := stm.db.Exec(`
		INSERT INTO archived_messages(session_id, role, content, original_timestamp, consolidated, consolidation_status, consolidation_retries, next_retry_at)
		VALUES
			('s1', 'user', 'ready failed', datetime('now', '-2 minutes'), 0, 'failed', 1, datetime('now', '-1 minute')),
			('s1', 'assistant', 'not ready failed', datetime('now', '-2 minutes'), 0, 'failed', 1, datetime('now', '+10 minutes')),
			('s1', 'user', 'plain pending', datetime('now', '-2 minutes'), 0, 'pending', 0, datetime('now', '+10 minutes'))
	`)
	if err != nil {
		t.Fatalf("insert archived_messages: %v", err)
	}

	msgs, err := stm.GetConsolidationCandidates(10, 3)
	if err != nil {
		t.Fatalf("GetConsolidationCandidates: %v", err)
	}
	seen := make(map[string]bool, len(msgs))
	for _, msg := range msgs {
		seen[msg.Content] = true
	}
	if !seen["ready failed"] {
		t.Fatal("expected retry-ready failed message to be returned")
	}
	if !seen["plain pending"] {
		t.Fatal("expected pending message to be returned")
	}
	if seen["not ready failed"] {
		t.Fatal("did not expect future retry message to be returned")
	}
}
