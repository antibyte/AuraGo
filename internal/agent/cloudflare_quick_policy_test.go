package agent

import (
	"aurago/internal/config"
	"aurago/internal/tools"
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloudflareQuickDispatcherRejectsLegacyPorts(t *testing.T) {
	cfg := &config.Config{}
	cfg.CloudflareTunnel.Enabled = true
	for _, operation := range []string{"start", "restart", "quick_tunnel"} {
		result, handled := dispatchNetwork(context.Background(), ToolCall{Action: "cloudflare_tunnel", Params: map[string]interface{}{"operation": operation, "port": float64(2375)}}, &DispatchContext{Cfg: cfg, Logger: slog.Default()})
		if !handled || !strings.Contains(result, "Port selection is disabled") {
			t.Fatalf("%s: %s", operation, result)
		}
	}
	req := decodeCloudflareTunnelArgs(ToolCall{Params: map[string]interface{}{"project_dir": "site"}})
	if req.ProjectDir != "site" {
		t.Fatal("project selection lost")
	}
}

func TestCloudflareQuickTunnelRequiresReasonBeforeStart(t *testing.T) {
	cfg := &config.Config{}
	cfg.CloudflareTunnel.Enabled = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	result, handled := dispatchNetwork(context.Background(), ToolCall{Action: "cloudflare_tunnel", Params: map[string]interface{}{"operation": "quick_tunnel", "project_dir": "site"}}, &DispatchContext{Cfg: cfg, Logger: slog.Default(), HomepageRegistryDB: db})
	if !handled || !strings.Contains(result, "reason is required") {
		t.Fatalf("quick_tunnel without reason must be refused: %s", result)
	}
}
