package agent

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchOllama(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{}
	cfg.Ollama.Enabled = true

	for _, operation := range []string{"container_status", "container_start", "container_stop", "container_restart", "container_logs"} {
		got := dispatchOllama(cfg, ollamaArgs{Operation: operation}, logger)
		if !strings.Contains(got, "Unknown ollama operation") {
			t.Fatalf("%s = %s, want unknown operation", operation, got)
		}
		if strings.Contains(got, operation) {
			t.Fatalf("unknown-operation message still names %s: %s", operation, got)
		}
	}

	cfg.Ollama.ReadOnly = true
	readonly := dispatchOllama(cfg, ollamaArgs{Operation: "pull", Model: "llama3:latest"}, logger)
	if !strings.Contains(readonly, "read-only") {
		t.Fatalf("readonly pull = %s", readonly)
	}
	cfg.Ollama.ReadOnly = false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("list path = %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3:latest","size":1}]}`))
	}))
	t.Cleanup(server.Close)
	cfg.Ollama.URL = server.URL
	listed := dispatchOllama(cfg, ollamaArgs{Operation: "list"}, logger)
	if !strings.Contains(listed, `"status":"ok"`) || !strings.Contains(listed, "llama3:latest") {
		t.Fatalf("list = %s", listed)
	}
}
