package rocketchat

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/ui"
)

// The shared room refuses operator commands unless
// rocketchat.allow_operator_commands opts in.
func TestRocketChatOperatorCommandsFollowAllowOperatorCommands(t *testing.T) {
	i18n.Load(ui.Content, slog.New(slog.NewTextHandler(io.Discard, nil)))

	var mu sync.Mutex
	var replies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Message struct {
				Msg string `json:"msg"`
			} `json:"message"`
		}
		if r.URL.Path == "/api/v1/chat.sendMessage" && json.NewDecoder(r.Body).Decode(&body) == nil {
			mu.Lock()
			replies = append(replies, body.Message.Msg)
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	promptsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(promptsDir, "personalities"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "personalities", "operatorcheck.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	refusal := i18n.T("de", "backend.cmd_operator_private_only", "/personality") // bot commands use the "de" default
	for _, tc := range []struct {
		allow         bool
		want, wantNot string
	}{
		{false, refusal, "operatorcheck"},
		{true, "operatorcheck", refusal},
	} {
		cfg := &config.Config{}
		cfg.Directories.PromptsDir = promptsDir
		cfg.RocketChat.Enabled = true
		cfg.RocketChat.URL = srv.URL
		cfg.RocketChat.AllowedUsers = []string{"id:owner"}
		cfg.RocketChat.AllowOperatorCommands = tc.allow
		msg := message{Msg: "/personality"}
		msg.User.ID = "owner"

		mu.Lock()
		replies = nil
		mu.Unlock()
		processMessage(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, nil, nil, nil, nil, nil, nil, "room-1", msg, nil, nil, nil)

		mu.Lock()
		got := strings.Join(replies, "\n")
		mu.Unlock()
		if !strings.Contains(got, tc.want) || strings.Contains(got, tc.wantNot) {
			t.Fatalf("allow_operator_commands=%v: reply %q, want %q and not %q", tc.allow, got, tc.want, tc.wantNot)
		}
	}
}
