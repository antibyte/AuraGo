package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestVirtualComputersTerminalAssetsLoadInDependencyOrder(t *testing.T) {
	t.Parallel()

	loader := normalizeAssetText(mustReadUIFile(t, "js/desktop/core/module-loader.js"))
	xtermCSS := strings.Index(loader, `'/css/xterm.css'`)
	appCSS := strings.Index(loader, `'/css/desktop-app-virtual-computers.css'`)
	xterm := strings.Index(loader, `'/js/vendor/xterm.min.js'`)
	fit := strings.Index(loader, `'/js/vendor/xterm-addon-fit.min.js'`)
	controller := strings.Index(loader, `'/js/desktop/apps/virtual-computers-terminal.js'`)
	app := strings.Index(loader, `'/js/desktop/apps/virtual-computers.js'`)
	if xtermCSS < 0 || appCSS < 0 || xterm < 0 || fit < 0 || controller < 0 || app < 0 {
		t.Fatalf("virtual computers loader missing terminal dependency: xtermCSS=%d appCSS=%d xterm=%d fit=%d controller=%d app=%d", xtermCSS, appCSS, xterm, fit, controller, app)
	}
	if !(xtermCSS < appCSS && xterm < fit && fit < controller && controller < app) {
		t.Fatalf("virtual computers terminal assets are not in dependency order")
	}
}

func TestVirtualComputersTerminalControllerUsesRawBinaryTTY(t *testing.T) {
	t.Parallel()

	controller := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers-terminal.js"))
	for _, want := range []string{
		`window.VirtualComputersTerminal = { mount }`,
		`new window.Terminal`,
		`new window.FitAddon.FitAddon`,
		`new window.WebSocket(settings.url)`,
		`binaryType = 'arraybuffer'`,
		`new TextEncoder()`,
		`terminal.onData`,
		`terminal.write`,
		`new ResizeObserver`,
		`data-terminal-action="reconnect"`,
		`data-terminal-action="disconnect"`,
		`clearTimeout`,
		`resizeObserver.disconnect()`,
		`terminal.dispose()`,
		`return { disconnect, reconnect, fit }`,
	} {
		if !strings.Contains(controller, want) {
			t.Errorf("virtual computers terminal controller missing %q", want)
		}
	}
	for _, forbidden := range []string{"wsProtocols", "hostKey", "credentials", "JSON.stringify", "/api/desktop/ssh"} {
		if strings.Contains(controller, forbidden) {
			t.Errorf("virtual computers terminal must not use Quick Connect protocol %q", forbidden)
		}
	}
}

func TestVirtualComputersTerminalLifecycleAndMachineGating(t *testing.T) {
	t.Parallel()

	app := normalizeAssetText(mustReadUIFile(t, "js/desktop/apps/virtual-computers.js"))
	for _, want := range []string{
		`function canUseTerminal(state, machine)`,
		`machine.display === false`,
		`window.VirtualComputersTerminal.mount`,
		`'/api/virtual-computers/machines/' + encodeURIComponent(machine.id) + '/tty'`,
		`function openTerminal(state, id)`,
		`function disconnectTerminal(state)`,
		`function reconcileTerminal(state)`,
		`state.terminalSession`,
		`state.terminalMachineId`,
		`state.detailMode === 'terminal'`,
		`data-role="terminal-mount"`,
		`canUseTerminal(state, machine) ?`,
		`disconnectRemoteSessions(state)`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("virtual computers app missing terminal lifecycle contract %q", want)
		}
	}
	if strings.Contains(app, "boringd") || strings.Contains(app, "token=") {
		t.Fatal("virtual computers browser code must not expose boringd credentials or tokens")
	}
	lifecycleContracts := []struct {
		name, start, end, want string
	}{
		{"draw", `function draw(state)`, `function toolbarPane`, `content.querySelector('[data-role="terminal-mount"]')`},
		{"select machine", `function selectMachine`, `function switchSection`, `disconnectRemoteSessions(state);`},
		{"switch section", `function switchSection`, `function applyResourceResult`, `disconnectRemoteSessions(state);`},
		{"destroy machine", `async function destroyMachine`, `function isCurrentScreenshotRequest`, `if (state.terminalMachineId === id) showOverview(state);`},
		{"take screenshot", `async function screenshot`, `function openModal`, `if (!id || !machine || machine.display !== true) return;`},
		{"dispose app window", `function dispose(windowId)`, `window.VirtualComputersApp`, `disconnectRemoteSessions(state);`},
	}
	for _, contract := range lifecycleContracts {
		start := strings.Index(app, contract.start)
		end := strings.Index(app, contract.end)
		if start < 0 || end <= start || !strings.Contains(app[start:end], contract.want) {
			t.Errorf("virtual computers terminal %s lifecycle contract missing", contract.name)
		}
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/desktop-app-virtual-computers.css"))
	if !strings.Contains(css, ".vc-terminal-toolbar") || !strings.Contains(css, ".vc-terminal-tool") || !strings.Contains(css, "min-height: 44px;") {
		t.Fatal("terminal toolbar must be responsive and expose at least 44-pixel touch targets")
	}
}

func TestVirtualComputersTerminalTranslations(t *testing.T) {
	t.Parallel()

	languages := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"virtual_computers_terminal",
		"virtual_computers_terminal_connecting",
		"virtual_computers_terminal_connected",
		"virtual_computers_terminal_disconnected",
		"virtual_computers_terminal_error",
		"virtual_computers_terminal_reconnect",
		"virtual_computers_terminal_disconnect",
		"virtual_computers_terminal_unavailable",
		"virtual_computers_terminal_copy_hint",
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
