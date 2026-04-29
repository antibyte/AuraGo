package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchProxmoxBlocksDestructiveActionsWithoutToggle(t *testing.T) {
	cfg := &config.Config{}
	cfg.Proxmox.Enabled = true
	cfg.Proxmox.ReadOnly = false
	cfg.Proxmox.AllowDestructive = false

	for _, operation := range []string{"stop", "shutdown", "reboot", "suspend", "reset"} {
		t.Run(operation, func(t *testing.T) {
			output, ok := dispatchPlatform(context.Background(), ToolCall{
				Action:    "proxmox",
				Operation: operation,
				Args:      `{"vmid":"100","type":"qemu","node":"pve"}`,
			}, &DispatchContext{Cfg: cfg, Logger: testLogger})
			if !ok {
				t.Fatalf("expected proxmox operation to be handled")
			}
			if !strings.Contains(output, "allow_destructive") {
				t.Fatalf("expected allow_destructive denial for %s, got %s", operation, output)
			}
		})
	}
}
