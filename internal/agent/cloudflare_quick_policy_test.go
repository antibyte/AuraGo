package agent

import (
	"aurago/internal/config"
	"context"
	"log/slog"
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
