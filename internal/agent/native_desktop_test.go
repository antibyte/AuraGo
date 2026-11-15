package agent

import (
	"reflect"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/virtualcomputers"
)

func TestNativeDesktopToolSchemaAndRoles(t *testing.T) {
	if containsName(toolNames(builtinToolSchemas(ToolFeatureFlags{})), "virtual_computer_ui") {
		t.Fatal("native desktop enabled by default")
	}
	props := nativeToolProperties(t, builtinToolSchemas(ToolFeatureFlags{VirtualComputerUIEnabled: true}), "virtual_computer_ui")
	ops := props["operation"].(map[string]interface{})["enum"].([]string)
	if !reflect.DeepEqual(ops, virtualcomputers.DesktopOperations) {
		t.Fatal("tool and runtime operations differ")
	}
	for _, key := range []string{"args", "env", "force", "owner_session_id", "mission_id", "control_token", "credential_id"} {
		if _, ok := props[key]; ok {
			t.Fatalf("unsafe field exposed: %s", key)
		}
	}
	for _, op := range ops {
		tc := ToolCall{Action: "virtual_computer_ui", Operation: op}
		if roleToolRestriction(tc, &DispatchContext{IsCoAgent: true}) == "" {
			t.Fatalf("co-agent can call %s", op)
		}
		if roleToolRestriction(tc, &DispatchContext{MessageSource: "a2a"}) == "" {
			t.Fatalf("A2A can call %s", op)
		}
		if got := DescribeAction(tc, nil).Mutating; got != virtualcomputers.DesktopInput(op) {
			t.Fatalf("mutation classification %s=%v", op, got)
		}
		if !security.UntrustedContentTool(tc.Action, op) {
			t.Fatalf("untrusted provenance missing: %s", op)
		}
	}
}

func TestNativeDesktopFeatureFlagsAndVisionFallback(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualComputers.Enabled = true
	cfg.Tools.VirtualComputers.Enabled = true
	cfg.VirtualComputers.AgentControl.Enabled = true
	if buildToolFlagsFromConfig(cfg).VirtualComputerUIEnabled {
		t.Fatal("native toggle ignored")
	}
	cfg.VirtualComputers.AgentControl.NativeAppsEnabled = true
	if !buildToolFlagsFromConfig(cfg).VirtualComputerUIEnabled {
		t.Fatal("native toggle does not expose tool")
	}
	dc := &DispatchContext{Cfg: cfg}
	if nativeDesktopVisionGate(ToolCall{Operation: "snapshot"}, dc) != "" {
		t.Fatal("semantic path requires vision")
	}
	if nativeDesktopVisionGate(ToolCall{Operation: "click_xy"}, dc) == "" {
		t.Fatal("coordinate fallback accepted without vision")
	}
	cfg.Vision.APIKey = "fixture"
	cfg.Vision.BaseURL = "https://example.test"
	cfg.Vision.Model = "fixture"
	if nativeDesktopVisionGate(ToolCall{Operation: "screenshot"}, dc) != "" {
		t.Fatal("configured private vision rejected")
	}
	cfg.Vision.ProviderType = "agnes"
	if nativeDesktopVisionGate(ToolCall{Operation: "screenshot"}, dc) == "" {
		t.Fatal("private screenshot would require public hosting")
	}
}
