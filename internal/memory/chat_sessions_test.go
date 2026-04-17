package memory

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func newTestSTM(t *testing.T) *SQLiteMemory {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	stm, err := NewSQLiteMemory(dbPath, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

func TestCreateChatSession(t *testing.T) {
	stm := newTestSTM(t)

	sess, err := stm.CreateChatSession()
	if err != nil {
		t.Fatalf("CreateChatSession: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if sess.Preview != "" {
		t.Fatalf("expected empty preview, got %q", sess.Preview)
	}
	if sess.MessageCount != 0 {
		t.Fatalf("expected 0 messages, got %d", sess.MessageCount)
	}
}

func TestListChatSessions(t *testing.T) {
	stm := newTestSTM(t)

	// Should have at least the default session from EnsureDefaultSession
	sessions, err := stm.ListChatSessions()
	if err != nil {
		t.Fatalf("ListChatSessions: %v", err)
	}
	if len(sessions) < 1 {
		t.Fatal("expected at least 1 session (default)")
	}

	// Create additional sessions
	_, _ = stm.CreateChatSession()
	_, _ = stm.CreateChatSession()

	sessions, err = stm.ListChatSessions()
	if err != nil {
		t.Fatalf("ListChatSessions after create: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	// All sessions should have valid IDs
	for _, s := range sessions {
		if s.ID == "" {
			t.Fatal("found session with empty ID")
		}
	}
}

func TestGetChatSession(t *testing.T) {
	stm := newTestSTM(t)

	sess, err := stm.CreateChatSession()
	if err != nil {
		t.Fatalf("CreateChatSession: %v", err)
	}

	got, err := stm.GetChatSession(sess.ID)
	if err != nil {
		t.Fatalf("GetChatSession: %v", err)
	}
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if got.ID != sess.ID {
		t.Fatalf("expected ID %s, got %s", sess.ID, got.ID)
	}

	// Non-existent session
	missing, err := stm.GetChatSession("nonexistent")
	if err != nil {
		t.Fatalf("GetChatSession nonexistent: %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for nonexistent session")
	}
}

func TestUpdateChatSessionPreview(t *testing.T) {
	stm := newTestSTM(t)

	sess, _ := stm.CreateChatSession()

	// Insert a user message
	_, err := stm.InsertMessage(sess.ID, "user", "Hello, this is a test message", false, false)
	if err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	err = stm.UpdateChatSessionPreview(sess.ID)
	if err != nil {
		t.Fatalf("UpdateChatSessionPreview: %v", err)
	}

	got, _ := stm.GetChatSession(sess.ID)
	if got.Preview != "Hello, this is a test message" {
		t.Fatalf("expected preview 'Hello, this is a test message', got %q", got.Preview)
	}
	if got.MessageCount != 1 {
		t.Fatalf("expected 1 message, got %d", got.MessageCount)
	}
}

func TestUpdateChatSessionPreviewTruncation(t *testing.T) {
	stm := newTestSTM(t)

	sess, _ := stm.CreateChatSession()

	longMsg := ""
	for i := 0; i < 20; i++ {
		longMsg += "abcdefghij"
	}
	_, _ = stm.InsertMessage(sess.ID, "user", longMsg, false, false)
	_ = stm.UpdateChatSessionPreview(sess.ID)

	got, _ := stm.GetChatSession(sess.ID)
	if len(got.Preview) > 80 {
		t.Fatalf("expected preview <= 80 chars, got %d: %q", len(got.Preview), got.Preview)
	}
	if got.Preview[len(got.Preview)-3:] != "..." {
		t.Fatalf("expected truncated preview to end with '...', got %q", got.Preview)
	}
}

func TestDeleteChatSession(t *testing.T) {
	stm := newTestSTM(t)

	sess, _ := stm.CreateChatSession()
	_, _ = stm.InsertMessage(sess.ID, "user", "test", false, false)

	err := stm.DeleteChatSession(sess.ID)
	if err != nil {
		t.Fatalf("DeleteChatSession: %v", err)
	}

	got, _ := stm.GetChatSession(sess.ID)
	if got != nil {
		t.Fatal("expected nil after delete")
	}

	// Messages should also be gone
	msgs, err := stm.GetSessionMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetSessionMessages after delete: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after delete, got %d", len(msgs))
	}
}

func TestClearSession(t *testing.T) {
	stm := newTestSTM(t)

	sess, _ := stm.CreateChatSession()
	_, _ = stm.InsertMessage(sess.ID, "user", "test", false, false)
	_ = stm.UpdateChatSessionPreview(sess.ID)

	err := stm.ClearSession(sess.ID)
	if err != nil {
		t.Fatalf("ClearSession: %v", err)
	}

	got, _ := stm.GetChatSession(sess.ID)
	if got == nil {
		t.Fatal("session should still exist after clear")
	}
	if got.MessageCount != 0 {
		t.Fatalf("expected 0 messages after clear, got %d", got.MessageCount)
	}
	if got.Preview != "" {
		t.Fatalf("expected empty preview after clear, got %q", got.Preview)
	}
}

func TestRotateChatSessions(t *testing.T) {
	stm := newTestSTM(t)

	// Create MaxChatSessions + 1 sessions (including default)
	// Default already exists from EnsureDefaultSession
	for i := 0; i < MaxChatSessions; i++ {
		_, err := stm.CreateChatSession()
		if err != nil {
			t.Fatalf("CreateChatSession %d: %v", i, err)
		}
	}

	sessions, _ := stm.ListChatSessions()
	if len(sessions) > MaxChatSessions {
		t.Fatalf("expected at most %d sessions after rotation, got %d", MaxChatSessions, len(sessions))
	}
}

func TestGetSessionMessages(t *testing.T) {
	stm := newTestSTM(t)

	sess, _ := stm.CreateChatSession()
	_, _ = stm.InsertMessage(sess.ID, "user", "hello", false, false)
	_, _ = stm.InsertMessage(sess.ID, "assistant", "hi there", false, false)
	_, _ = stm.InsertMessage(sess.ID, "system", "internal note", false, true) // internal

	msgs, err := stm.GetSessionMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 visible messages (internal filtered by SQL), got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Fatalf("unexpected first message: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hi there" {
		t.Fatalf("unexpected second message: %+v", msgs[1])
	}
}

func TestEnsureDefaultSession(t *testing.T) {
	stm := newTestSTM(t)

	// Should already have default from init
	sess, err := stm.GetChatSession("default")
	if err != nil {
		t.Fatalf("GetChatSession default: %v", err)
	}
	if sess == nil {
		t.Fatal("expected default session to exist")
	}

	// Calling again should be idempotent
	err = stm.EnsureDefaultSession()
	if err != nil {
		t.Fatalf("EnsureDefaultSession idempotent: %v", err)
	}
}

func TestCountInternalToolResultMessages(t *testing.T) {
	stm := newTestSTM(t)

	sess, _ := stm.CreateChatSession()
	_, _ = stm.InsertMessage(sess.ID, "assistant", `{"action":"web_scraper"}`, false, true)
	_, _ = stm.InsertMessage(sess.ID, "tool", `{"status":"success"}`, false, true)
	_, _ = stm.InsertMessage(sess.ID, "user", `Tool Output: {"status":"success"}`, false, true)
	_, _ = stm.InsertMessage(sess.ID, "user", "visible user text", false, false)

	count, err := stm.CountInternalToolResultMessages(sess.ID)
	if err != nil {
		t.Fatalf("CountInternalToolResultMessages: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 internal tool result messages, got %d", count)
	}
}
