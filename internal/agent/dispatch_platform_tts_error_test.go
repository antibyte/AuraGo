package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchTTSErrorWithQuoteStaysJSON(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `provider "fixture" unavailable`, http.StatusServiceUnavailable)
	}))
	defer provider.Close()
	cfg := &config.Config{}
	cfg.TTS.Provider = "supertonic"
	cfg.TTS.Supertonic.URL = provider.URL
	cfg.Directories.DataDir = t.TempDir()
	out, ok := dispatchPlatform(context.Background(), ToolCall{
		Action: "tts",
		Text:   "hallo",
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("tts was not handled")
	}
	const prefix = "Tool Output: "
	if !strings.HasPrefix(out, prefix) {
		t.Fatalf("output = %s", out)
	}
	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, prefix)), &payload); err != nil {
		t.Fatalf("unmarshal %s: %v", out, err)
	}
	if payload.Status != "error" || !strings.Contains(payload.Message, `"`) {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestDispatchTTSSuccessStaysJSON(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write([]byte("fixture audio"))
	}))
	defer provider.Close()
	cfg := &config.Config{}
	cfg.TTS.Provider = "supertonic"
	cfg.TTS.Supertonic.URL = provider.URL
	cfg.Directories.DataDir = t.TempDir()
	if runtime.GOOS != "windows" {
		cfg.Directories.DataDir = filepath.Join(cfg.Directories.DataDir, "bad\"name\t")
	}
	out, ok := dispatchPlatform(context.Background(), ToolCall{Action: "tts", Text: "hallo"}, &DispatchContext{
		Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("tts was not handled")
	}
	var payload struct {
		Status    string `json:"status"`
		File      string `json:"file"`
		URL       string `json:"url"`
		WebPath   string `json:"web_path"`
		LocalPath string `json:"local_path"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &payload); err != nil {
		t.Fatalf("unmarshal %s: %v", out, err)
	}
	if payload.Status != "success" || payload.File == "" || !strings.Contains(payload.URL, "/tts/"+payload.File) || !strings.HasPrefix(payload.WebPath, "/tts/"+payload.File) {
		t.Fatalf("payload = %#v", payload)
	}
	expectedPath, err := filepath.Abs(filepath.Join(cfg.Directories.DataDir, "tts", payload.File))
	if err != nil {
		t.Fatal(err)
	}
	if payload.LocalPath != filepath.ToSlash(expectedPath) {
		t.Fatalf("local_path = %q, want %q", payload.LocalPath, filepath.ToSlash(expectedPath))
	}
}
