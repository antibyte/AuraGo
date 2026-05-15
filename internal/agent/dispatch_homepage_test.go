package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchHomepageDestroyRequiresForce(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowContainerManagement = true
	cfg.Homepage.WorkspacePath = t.TempDir()

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "destroy",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, "requires force=true") {
		t.Fatalf("expected destroy without force to be rejected, got %s", output)
	}
}
