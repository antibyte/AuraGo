package agent

import (
	"strings"
	"testing"
)

func TestVirtualDesktopInstallRequiresHealthyDiagnosisBeforeOpen(t *testing.T) {
	t.Parallel()

	state := newToolRecoveryState()
	install := ToolCall{
		Action: "virtual_desktop_app_install",
		Params: map[string]interface{}{
			"manifest": map[string]interface{}{"id": "space-invaders"},
		},
	}
	recordVirtualDesktopAppVerification(install, `Tool Output: {"status":"ok"}`, false, &state)

	open := ToolCall{Action: "virtual_desktop_apps", Operation: "open_app", Params: map[string]interface{}{"app_id": "space-invaders"}}
	if result, blocked := precheckVirtualDesktopAppOpen(open, &state); !blocked || !strings.Contains(result, "diagnose_app") {
		t.Fatalf("open before diagnosis = (%q, %v), want diagnosis gate", result, blocked)
	}

	diagnose := ToolCall{Action: "virtual_desktop_apps", Operation: "diagnose_app", Params: map[string]interface{}{"app_id": "space-invaders"}}
	recordVirtualDesktopAppVerification(diagnose, `<external_data>Tool Output: {&#34;status&#34;:&#34;ok&#34;,&#34;data&#34;:{&#34;ok&#34;:false}}</external_data>`, false, &state)
	if _, blocked := precheckVirtualDesktopAppOpen(open, &state); !blocked {
		t.Fatal("unhealthy diagnosis must not unlock app opening")
	}

	recordVirtualDesktopAppVerification(diagnose, `<external_data>Tool Output: {&#34;status&#34;:&#34;ok&#34;,&#34;data&#34;:{&#34;ok&#34;:true}}</external_data>`, false, &state)
	if result, blocked := precheckVirtualDesktopAppOpen(open, &state); blocked {
		t.Fatalf("healthy diagnosis should unlock app opening, got %q", result)
	}
}

func TestVirtualDesktopOpenGateIsScopedToCurrentSuccessfulInstall(t *testing.T) {
	t.Parallel()

	state := newToolRecoveryState()
	existing := ToolCall{Action: "virtual_desktop_apps", Operation: "open_in_app", Params: map[string]interface{}{"path": "Apps/existing/index.html"}}
	if result, blocked := precheckVirtualDesktopAppOpen(existing, &state); blocked {
		t.Fatalf("existing app without a current install should remain openable, got %q", result)
	}

	failedInstall := ToolCall{Action: "virtual_desktop_apps", Operation: "install_app", Params: map[string]interface{}{
		"manifest": map[string]interface{}{"id": "broken-app"},
	}}
	recordVirtualDesktopAppVerification(failedInstall, `Tool Output: {"status":"error"}`, true, &state)
	brokenOpen := ToolCall{Action: "virtual_desktop_apps", Operation: "open_app", Params: map[string]interface{}{"app_id": "broken-app"}}
	if result, blocked := precheckVirtualDesktopAppOpen(brokenOpen, &state); blocked {
		t.Fatalf("failed install should not create a stale opening gate, got %q", result)
	}
}
