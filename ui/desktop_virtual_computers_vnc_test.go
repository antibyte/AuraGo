package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestVirtualComputersVNCAssetsLoadInDependencyOrder(t *testing.T) {
	t.Parallel()

	loader := normalizeAssetText(mustReadUIFile(t, "js/desktop/core/module-loader.js"))
	novnc := strings.Index(loader, `'/js/vendor/novnc.min.js'`)
	controller := strings.Index(loader, `'/js/desktop/apps/virtual-computers-vnc.js'`)
	app := strings.Index(loader, `'/js/desktop/apps/virtual-computers.js'`)
	if novnc < 0 || controller < 0 || app < 0 {
		t.Fatalf("virtual computers loader missing VNC dependency: noVNC=%d controller=%d app=%d", novnc, controller, app)
	}
	if !(novnc < controller && controller < app) {
		t.Fatalf("virtual computers VNC scripts must load noVNC, controller, then app: noVNC=%d controller=%d app=%d", novnc, controller, app)
	}

	routing := normalizeAssetText(mustReadUIFile(t, "js/desktop/core/menus-and-routing.js"))
	branchStart := strings.Index(routing, `if (appId === 'virtual-computers')`)
	if branchStart < 0 {
		t.Fatal("desktop routing missing virtual-computers branch")
	}
	branch := routing[branchStart:]
	if next := strings.Index(branch[len(`if (appId === 'virtual-computers')`):], "\n        if (appId === '"); next >= 0 {
		branch = branch[:len(`if (appId === 'virtual-computers')`)+next]
	}
	if !strings.Contains(branch, `toggleMaximize: () => toggleMaximizeWindow(id)`) {
		t.Fatal("virtual-computers render context must receive the app-window maximize callback")
	}
}

func TestVirtualComputersVNCControllerProvidesSafeInteractiveControls(t *testing.T) {
	t.Parallel()

	controller := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers-vnc.js"))
	for _, want := range []string{
		`window.VirtualComputersVNC = { mount }`,
		`new window.RFB`,
		`data-vnc-scale="fit"`,
		`data-vnc-scale="one-to-one"`,
		`data-vnc-action="view-only"`,
		`data-vnc-action="ctrl-alt-del"`,
		`data-vnc-action="reconnect"`,
		`data-vnc-action="disconnect"`,
		`data-vnc-action="expand"`,
		`data-vnc-action="fullscreen"`,
		`credentialsrequired`,
		`securityfailure`,
		`sendCtrlAltDel()`,
		`function applyVNCPreferences(rfb, preferences)`,
		`preferences.scaleMode === 'fit'`,
		`preferences.viewOnly === true`,
		`viewOnly =`,
		`requestFullscreen`,
		`document.fullscreenElement`,
		`onExpandedChange`,
		`disconnect`,
		`reconnect`,
	} {
		if !strings.Contains(controller, want) {
			t.Errorf("virtual computers VNC controller missing %q", want)
		}
	}
	if strings.Contains(controller, "RFB.credentials(") || strings.Contains(controller, "prompt(") {
		t.Fatal("virtual computers VNC must never collect credentials in the browser")
	}
	if strings.Contains(controller, "wsProtocols") {
		t.Fatal("virtual computers VNC must use boringd's native WebSocket handshake without Quick Connect subprotocols")
	}
	for _, forbidden := range []string{`toggleMaximize`, `data-vnc-action="maximize"`, `.closest('.vd-window')`, `.classList.contains('maximized')`} {
		if strings.Contains(controller, forbidden) {
			t.Errorf("virtual computers VNC controller must not resize the outer desktop window via %q", forbidden)
		}
	}
	quickConnect := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/quickconnect-launchpad-chat.js"))
	if !strings.Contains(quickConnect, `wsProtocols: ['binary']`) {
		t.Fatal("Quick Connect must retain its binary WebSocket subprotocol")
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/desktop-app-virtual-computers.css"))
	if !strings.Contains(css, ".vc-vnc-toolbar") || !strings.Contains(css, "min-height: 44px;") {
		t.Fatal("VNC toolbar must be responsive and expose at least 44-pixel touch targets")
	}
}

func TestVirtualComputersVNCLifecycleAndPermissionGating(t *testing.T) {
	t.Parallel()

	app := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers.js"))
	for _, want := range []string{
		`function canUseVNC(state, machine)`,
		`machine.display === true`,
		`state.status.readonly !== true`,
		`state.context.readonly !== true`,
		`window.VirtualComputersVNC.mount`,
		`window.location.protocol === 'https:' ? 'wss:' : 'ws:'`,
		`'/api/virtual-computers/machines/' + encodeURIComponent(machine.id) + '/vnc'`,
		`function disconnectVNC(state`,
		`function reconcileVNC(state)`,
		`disconnectVNC(state);`,
		`if (state.vncMachineId === id) showOverview(state);`,
		`if (state.vncSession && canUseVNC(state, machine)) return;`,
		`if (state.clickHandler) state.host.removeEventListener('click', state.clickHandler);`,
		`state.vncMachineId`,
		`state.detailMode === 'vnc'`,
		`function setVNCExpanded(state, expanded)`,
		`onExpandedChange: expanded => setVNCExpanded(state, expanded)`,
		`classList.toggle('is-vnc-expanded', state.vncExpanded)`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("virtual computers app missing VNC lifecycle contract %q", want)
		}
	}
	if strings.Contains(app, "boringd") || strings.Contains(app, "token=") {
		t.Fatal("virtual computers browser code must not expose boringd credentials or tokens")
	}
	if strings.Contains(app, `toggleMaximize: state.context.toggleMaximize`) {
		t.Fatal("live VNC must not receive the outer app-window maximize callback")
	}
}

func TestVirtualComputersVNCTranslations(t *testing.T) {
	t.Parallel()

	languages := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"virtual_computers_vnc_live",
		"virtual_computers_vnc_connecting",
		"virtual_computers_vnc_connected",
		"virtual_computers_vnc_disconnected",
		"virtual_computers_vnc_fit",
		"virtual_computers_vnc_one_to_one",
		"virtual_computers_vnc_view_only",
		"virtual_computers_vnc_ctrl_alt_del",
		"virtual_computers_vnc_reconnect",
		"virtual_computers_vnc_disconnect",
		"virtual_computers_vnc_expand",
		"virtual_computers_vnc_collapse",
		"virtual_computers_vnc_fullscreen",
		"virtual_computers_vnc_exit_fullscreen",
		"virtual_computers_vnc_credentials_error",
		"virtual_computers_vnc_security_error",
		"virtual_computers_vnc_unavailable",
	}
	for _, language := range languages {
		language := language
		t.Run(language, func(t *testing.T) {
			t.Parallel()
			var messages map[string]string
			if err := json.Unmarshal(mustReadUIFile(t, fmt.Sprintf("lang/desktop/%s.json", language)), &messages); err != nil {
				t.Fatalf("parse desktop locale: %v", err)
			}
			for _, key := range keys {
				value, ok := messages["desktop."+key]
				if !ok || strings.TrimSpace(value) == "" {
					t.Errorf("desktop locale missing non-empty %q", key)
				}
			}
		})
	}
}
