package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchTTSErrorWithQuoteStaysJSON(t *testing.T) {
	cfg := &config.Config{}
	cfg.TTS.Provider = "google"
	cfg.Directories.DataDir = filepath.Join(t.TempDir(), `bad"name`)
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
