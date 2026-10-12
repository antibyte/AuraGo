package agent

import (
	"aurago/internal/config"
	"context"
	"strings"
	"testing"
)

func TestLayerlingDiscoveryAndPermissions(t *testing.T) {
	cfg := &config.Config{}
	if buildToolFlagsFromConfig(cfg).LayerlingEnabled {
		t.Fatal("enabled by default")
	}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.Layerling.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Tools.VirtualDesktop.Enabled = true
	for _, access := range []string{"off", "unknown", "read", "write"} {
		cfg.VirtualDesktop.Layerling.AgentAccess = access
		flags := buildToolFlagsFromConfig(cfg)
		want := access == "read" || access == "write"
		if flags.LayerlingEnabled != want {
			t.Fatalf("discovery %s", access)
		}
		found := false
		for _, name := range builtinToolNames(flags) {
			if name == "layerling" {
				found = true
			}
		}
		if found != want {
			t.Fatalf("registration %s", access)
		}
	}
	cfg.VirtualDesktop.Layerling.AgentAccess = "read"
	tc := ToolCall{Action: "layerling", Operation: "create_shape", Params: map[string]interface{}{"editor_id": "not-a-real-editor", "arguments": `{"kind":"box"}`}}
	if result := dispatchLayerling(context.Background(), tc, &DispatchContext{Cfg: cfg}); !strings.Contains(result, "access denied") {
		t.Fatalf("readonly dispatch: %s", result)
	}
	cfg.VirtualDesktop.Layerling.AgentAccess = "write"
	cfg.VirtualDesktop.ReadOnly = true
	if result := dispatchLayerling(context.Background(), tc, &DispatchContext{Cfg: cfg}); !strings.Contains(result, "access denied") {
		t.Fatalf("desktop readonly dispatch: %s", result)
	}
	cfg.VirtualDesktop.ReadOnly = false
	cfg.Tools.VirtualDesktop.Enabled = false
	if buildToolFlagsFromConfig(cfg).LayerlingEnabled {
		t.Fatal("desktop tool disabled")
	}
	if result := dispatchLayerling(context.Background(), tc, &DispatchContext{Cfg: cfg}); !strings.Contains(result, "access denied") {
		t.Fatalf("desktop tool gate: %s", result)
	}
}
