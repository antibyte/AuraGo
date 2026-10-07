package agent

import (
	"aurago/internal/config"
	"context"
	"strings"
	"testing"
)

func TestMeshCentralRequiresGlobalRemoteShellBeforeCredentials(t *testing.T) {
	cfg := &config.Config{}
	cfg.MeshCentral.Enabled = true
	cfg.MeshCentral.Username = "admin"
	output, ok := dispatchServices(context.Background(), ToolCall{Action: "meshcentral", Operation: "run_command", NodeID: "node//device1", Command: "id"}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok || !strings.Contains(output, "agent.allow_remote_shell") {
		t.Fatal(output)
	}
}

func TestHomepageVercelReadOnlyOverridesDeployGrant(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Vercel.Enabled = true
	cfg.Vercel.AllowDeploy = true
	cfg.Vercel.ReadOnly = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	output, ok := dispatchServices(context.Background(), ToolCall{Action: "homepage", Operation: "deploy_vercel", ProjectDir: "site"}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok || !strings.Contains(output, "vercel.readonly") {
		t.Fatal(output)
	}
}
