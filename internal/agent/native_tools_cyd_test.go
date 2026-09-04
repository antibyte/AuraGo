package agent

import "testing"

func TestBuiltinToolSchemasIncludeCYDWhenEnabled(t *testing.T) {
	names := toolNames(builtinToolSchemas(ToolFeatureFlags{CydEnabled: true, NotificationEnabled: true}))
	if !containsName(names, "cyd_display") {
		t.Fatal("expected cyd_display when CydEnabled")
	}
	if !containsName(names, "send_notification") {
		t.Fatal("expected send_notification when NotificationEnabled")
	}
	disabled := toolNames(builtinToolSchemas(ToolFeatureFlags{}))
	if containsName(disabled, "cyd_display") {
		t.Fatal("cyd_display must be gated")
	}
}
