package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"

	openai "github.com/sashabaranov/go-openai"
)

// The web console and the desktop chat are admin surfaces, so operator slash
// commands keep running there (commands.Context.AllowOperator).
func TestAdminChatSurfacesRunOperatorCommands(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	promptsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(promptsDir, "personalities"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "personalities", "operatorcheck.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Server.UILanguage = "en"
	cfg.Directories.PromptsDir = promptsDir
	s := &Server{Cfg: cfg, Logger: logger, ShortTermMem: stm, HistoryManager: memory.NewEphemeralHistoryManager()}

	answer, handled, err := handleDesktopSlashCommand(s, "/personality")
	if err != nil || !handled || !strings.Contains(answer, "operatorcheck") {
		t.Fatalf("desktop /personality: answer=%q handled=%v err=%v, want the personality list", answer, handled, err)
	}

	payload, err := json.Marshal(openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{
		Role: openai.ChatMessageRoleUser, Content: "/personality",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handleChatCompletions(s, nil).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "operatorcheck") {
		t.Fatalf("web /personality: status=%d body=%s, want the personality list", rec.Code, rec.Body.String())
	}
}
