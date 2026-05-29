package ui

import (
	"strings"
	"testing"
)

func TestDesktopWebSocketReconnectCleansPreviousListeners(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/core/sdk-events-bootstrap.js")
	for _, marker := range []string{
		"let wsGeneration = 0",
		"function cleanupDesktopWS()",
		"state.wsCleanup",
		"ws.removeEventListener('open', onOpen)",
		"ws.removeEventListener('close', onClose)",
		"ws.removeEventListener('message', onMessage)",
		"const generation = ++wsGeneration",
		"if (generation !== wsGeneration || ws !== state.ws) return",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop websocket cleanup missing marker %q", marker)
		}
	}
}

func TestDesktopWidgetsBlankIframesBeforeRebuild(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/core/desktop-foundation.js")
	for _, marker := range []string{
		"function blankWidgetFrames(host)",
		"host.querySelectorAll('iframe')",
		"frame.src = 'about:blank'",
		"blankWidgetFrames(host);",
		"clearWidgetRuntime();",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop widget iframe cleanup missing marker %q", marker)
		}
	}
}

func TestDesktopTaskbarAndDockUseReconciliation(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/core/window-shell-runtime.js")
	for _, marker := range []string{
		"function reconcileStandardTaskbar()",
		"const seenWindowIds = new Set()",
		"data-taskbar-bound",
		"function updateTaskbarButton(btn, win, index)",
		"function reconcileFruityDock()",
		"const seenDockAppIds = new Set()",
		"data-dock-bound",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop taskbar reconciliation missing marker %q", marker)
		}
	}
	if strings.Contains(source, "function renderStandardTaskbar() {\n        const host = $('vd-taskbar-apps');\n        host.innerHTML =") {
		t.Fatal("standard taskbar must not fully rebuild via host.innerHTML")
	}
}
