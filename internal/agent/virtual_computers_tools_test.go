package agent

import (
	"testing"

	"aurago/internal/config"
)

func TestVirtualComputersToolSchemaGated(t *testing.T) {
	if containsName(toolNames(builtinToolSchemas(ToolFeatureFlags{})), "virtual_computers") {
		t.Fatal("virtual_computers should be hidden when disabled")
	}
	names := toolNames(builtinToolSchemas(ToolFeatureFlags{VirtualComputersEnabled: true}))
	if !containsName(names, "virtual_computers") {
		t.Fatal("virtual_computers should be present when enabled")
	}
	props := nativeToolProperties(t, builtinToolSchemas(ToolFeatureFlags{VirtualComputersEnabled: true}), "virtual_computers")
	op, ok := props["operation"].(map[string]interface{})
	if !ok {
		t.Fatalf("operation property missing or wrong type: %#v", props["operation"])
	}
	enum, ok := op["enum"].([]string)
	if !ok {
		t.Fatalf("operation enum missing or wrong type: %#v", op["enum"])
	}
	for _, want := range []string{"status", "list_machines", "launch", "destroy", "exec", "screenshot", "run_desktop_task"} {
		if !containsString(enum, want) {
			t.Fatalf("operation enum missing %q: %#v", want, enum)
		}
	}
}

func TestVirtualComputersToolFlagsFromConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualComputers.Enabled = true
	cfg.Tools.VirtualComputers.Enabled = true
	flags := buildToolFlagsFromConfig(cfg)
	if !flags.VirtualComputersEnabled {
		t.Fatal("VirtualComputersEnabled should be true when integration and tool gate are enabled")
	}
	cfg.Tools.VirtualComputers.Enabled = false
	flags = buildToolFlagsFromConfig(cfg)
	if flags.VirtualComputersEnabled {
		t.Fatal("VirtualComputersEnabled should be false when tool gate is disabled")
	}
}
