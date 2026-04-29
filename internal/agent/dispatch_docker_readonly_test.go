package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchDockerReadOnlyBlocksMutatingOperations(t *testing.T) {
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.ReadOnly = true

	tests := []struct {
		name      string
		operation string
		args      string
	}{
		{name: "exec", operation: "exec", args: `{"container":"app","cmd":"touch /tmp/pwned"}`},
		{name: "copy", operation: "copy", args: `{"container":"app","src":"/tmp/a","dst":"/tmp/b"}`},
		{name: "cp alias", operation: "cp", args: `{"container":"app","src":"/tmp/a","dst":"/tmp/b"}`},
		{name: "create network", operation: "create_network", args: `{"network":"audit-net"}`},
		{name: "remove network", operation: "remove_network", args: `{"network":"audit-net"}`},
		{name: "connect network", operation: "connect", args: `{"container":"app","network":"audit-net"}`},
		{name: "disconnect network", operation: "disconnect", args: `{"container":"app","network":"audit-net"}`},
		{name: "create volume", operation: "create_volume", args: `{"volume":"audit-vol"}`},
		{name: "remove volume", operation: "remove_volume", args: `{"volume":"audit-vol"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, ok := dispatchServices(context.Background(), ToolCall{
				Action:    "docker",
				Operation: tt.operation,
				Args:      tt.args,
			}, &DispatchContext{Cfg: cfg, Logger: testLogger})
			if !ok {
				t.Fatalf("expected docker operation to be handled")
			}
			if !strings.Contains(output, "read-only") {
				t.Fatalf("expected read-only denial for %s, got %s", tt.operation, output)
			}
		})
	}
}
