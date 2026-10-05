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
	"aurago/internal/i18n"
	"aurago/internal/memory"
	"aurago/ui"

	openai "github.com/sashabaranov/go-openai"
)

func newOperatorCommandTestServer(t *testing.T) *Server {
	t.Helper()
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
	return &Server{
		Cfg: cfg, Logger: logger, ShortTermMem: stm, HistoryManager: memory.NewEphemeralHistoryManager(),
		internalToken: "operator-test-token",
	}
}

func postChatCommand(t *testing.T, s *Server, content string, headers map[string]string) string {
	t.Helper()
	payload, err := json.Marshal(openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{
		Role: openai.ChatMessageRoleUser, Content: content,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	req.RemoteAddr = "127.0.0.1:43210"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handleChatCompletions(s, nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST %q with %v: status=%d body=%s", content, headers, rec.Code, rec.Body.String())
	}
	var resp openai.ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || len(resp.Choices) != 1 {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	return resp.Choices[0].Message.Content
}

// The web console and the desktop chat are admin surfaces, so operator slash
// commands keep running there (commands.Context.AllowOperator).
func TestAdminChatSurfacesRunOperatorCommands(t *testing.T) {
	s := newOperatorCommandTestServer(t)

	answer, handled, err := handleDesktopSlashCommand(s, "/personality")
	if err != nil || !handled || !strings.Contains(answer, "operatorcheck") {
		t.Fatalf("desktop /personality: answer=%q handled=%v err=%v, want the personality list", answer, handled, err)
	}
	if got := postChatCommand(t, s, "/personality", nil); !strings.Contains(got, "operatorcheck") {
		t.Fatalf("web /personality = %q, want the personality list", got)
	}
}

// follow_up and wait_for_event deliver model-written prompts through the
// loopback with X-Internal-FollowUp; such turns must not run operator
// commands. Mission runs stay allowed.
func TestFollowUpLoopbackTurnsRefuseOperatorCommands(t *testing.T) {
	i18n.Load(ui.Content, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s := newOperatorCommandTestServer(t)
	followUp := map[string]string{"X-Internal-FollowUp": "true", "X-Internal-Token": s.internalToken}

	refusal := i18n.T("en", "backend.cmd_operator_private_only", "/personality")
	if got := postChatCommand(t, s, "/personality", followUp); got != refusal {
		t.Fatalf("follow-up /personality = %q, want refusal %q", got, refusal)
	}
	if got := postChatCommand(t, s, "/help", followUp); !strings.Contains(got, "/reset") {
		t.Fatalf("follow-up /help must still run, got %q", got)
	}

	mission := map[string]string{"X-Internal-FollowUp": "true", "X-Internal-Token": s.internalToken, "X-Mission-ID": "operator-check"}
	if got := postChatCommand(t, s, "/personality", mission); !strings.Contains(got, "operatorcheck") {
		t.Fatalf("mission /personality = %q, want the personality list", got)
	}
}
