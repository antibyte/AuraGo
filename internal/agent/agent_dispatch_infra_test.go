package agent

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchInfraManageWebhooksUsesActionAlias(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("mkdir temp dir: %v", err)
	}

	cfg := &config.Config{}
	cfg.Webhooks.Enabled = true
	cfg.Directories.DataDir = tmpDir

	out, ok := dispatchInfra(context.Background(), ToolCall{
		Action: "manage_webhooks",
		Params: map[string]interface{}{
			"action": "create",
			"name":   "Inbox Hook",
			"slug":   "inbox-hook",
		},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchInfra to handle manage_webhooks")
	}
	if !strings.Contains(out, `"status": "ok"`) && !strings.Contains(out, `"status":"ok"`) {
		t.Fatalf("expected success output, got %s", out)
	}

	webhookFile := filepath.Join(tmpDir, "webhooks.json")
	data, err := os.ReadFile(webhookFile)
	if err != nil {
		t.Fatalf("expected webhook file to be written: %v", err)
	}
	if !strings.Contains(string(data), "inbox-hook") {
		t.Fatalf("expected webhook file to contain slug, got %s", string(data))
	}
}
