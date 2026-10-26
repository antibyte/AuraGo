package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestRustFSDockerProtectionIgnoresHostAccessGrant(t *testing.T) {
	for _, hostAccess := range []bool{false, true} {
		cfg := &config.Config{}
		cfg.Docker.Enabled, cfg.Docker.AllowHostAccess = true, hostAccess
		cfg.Docker.Host = "tcp://127.0.0.1:1"
		for _, call := range []ToolCall{
			{Action: "docker", Operation: "create", Name: "aurago-rustfs", Image: "alpine"},
			{Action: "docker", Operation: "remove_volume", Name: "aurago_rustfs_data"},
			{Action: "docker", Operation: "run", Name: "reader", Image: "alpine", Volumes: []string{"aurago_rustfs_credentials:/keys"}},
		} {
			out, ok := dispatchServices(context.Background(), call, &DispatchContext{Cfg: cfg, Logger: testLogger})
			if !ok || !strings.Contains(out, "docker_managed_rustfs_resource") {
				t.Fatalf("host access=%v, op=%s: %s", hostAccess, call.Operation, out)
			}
		}
	}
}
