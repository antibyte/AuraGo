package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"

	openai "github.com/sashabaranov/go-openai"
)

func newTestChatClearServer(t *testing.T) (*Server, string) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { stm.Close() })

	history := memory.NewEphemeralHistoryManager()
	if err := history.AddMessage(openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "default history should stay",
	}, 1, false, false); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	sess, err := stm.CreateChatSession()
	if err != nil {
		t.Fatalf("CreateChatSession: %v", err)
	}
	if _, err := stm.InsertMessage(sess.ID, openai.ChatMessageRoleUser, "session history should clear", false, false); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	return &Server{
		Cfg:            &config.Config{},
		Logger:         logger,
		ShortTermMem:   stm,
		HistoryManager: history,
	}, sess.ID
}

func TestHandleClearChatSessionKeepsDefaultHistory(t *testing.T) {
	s, sessionID := newTestChatClearServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/clear?session_id="+sessionID, nil)
	rec := httptest.NewRecorder()
	handleClearChat(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := s.HistoryManager.GetForLLM(); len(got) != 1 || got[0].Content != "default history should stay" {
		t.Fatalf("default history = %#v, want preserved message", got)
	}
	messages, err := s.ShortTermMem.GetSessionMessages(sessionID)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("session messages = %d, want 0 after clear", len(messages))
	}
}
