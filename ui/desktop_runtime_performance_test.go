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

func TestDesktopIconsUseReconciliation(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/core/desktop-foundation.js")
	for _, marker := range []string{
		"function reconcileDesktopIcons(items, positions)",
		"function updateDesktopIconButton(btn, item, pos)",
		"function bindDesktopIconButton(btn)",
		"data-vd-icon-bound",
		"const seenIconIds = new Set()",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop icon reconciliation missing marker %q", marker)
		}
	}
	if strings.Contains(source, "icons.innerHTML = items.map(item =>") {
		t.Fatal("desktop icons must not fully rebuild via icons.innerHTML")
	}
}

func TestGalaxaDeluxeCachesCanvasResources(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/apps/galaxa-deluxe.js")
	for _, marker := range []string{
		"function ensureNebulaCanvas()",
		"nebulaCv.width = W",
		"const radialGradientCache = new Map()",
		"function cachedRadialGradient",
		"function drawPixelSprite",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("galaxa deluxe canvas optimization missing marker %q", marker)
		}
	}
	if strings.Contains(source, "nebulaCv = document.createElement('canvas'); nebulaCv.width = W; nebulaCv.height = H;") {
		t.Fatal("galaxa deluxe must reuse the nebula canvas instead of allocating a new one per stage")
	}
}

func TestPixelEditorUsesCanvasPoolAndBoundedHistory(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/apps/pixel.js")
	for _, marker := range []string{
		"const MAX_HISTORY = 5",
		"const canvasPool = []",
		"function acquireTempCanvas(width, height)",
		"function releaseTempCanvas(canvas)",
		"releaseTempCanvas(tmpCanvas)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("pixel editor runtime optimization missing marker %q", marker)
		}
	}
	if strings.Contains(source, "if (state.history.length > 20)") {
		t.Fatal("pixel editor history must not keep 20 full ImageData snapshots")
	}
}
