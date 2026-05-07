package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopWindowPlacementIsClamped(t *testing.T) {
	source := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"function clampWindowSize",
		"function nextWindowPosition",
		"workspaceRect.width",
		"workspaceRect.height",
		"workspaceRect.width - margin * 2",
		"workspaceRect.height - margin * 2",
		"Math.min(maxLeft",
		"Math.min(maxTop",
		"const requestedSize = appWindowSize(appId)",
		"const size = clampWindowSize(requestedSize)",
		"win.style.width = size.width + 'px'",
		"win.style.height = size.height + 'px'",
		"win.style.minWidth = Math.min(WINDOW_MIN_W, size.width) + 'px'",
		"win.style.minHeight = Math.min(WINDOW_MIN_H, size.height) + 'px'",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("window manager placement missing marker %q", marker)
		}
	}

	cssBytes, err := os.ReadFile(filepath.Join("css", "desktop.css"))
	if err != nil {
		t.Fatalf("read desktop stylesheet: %v", err)
	}
	css := string(cssBytes)
	for _, marker := range []string{
		"min-width: 0 !important;",
		"min-height: 0 !important;",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop mobile window sizing missing marker %q", marker)
		}
	}
}

func TestDesktopWindowChromeUsesCssGlyphsInsteadOfTextFallbacks(t *testing.T) {
	source := readDesktopAssetText(t, "js/desktop/main.js")
	openAppBody := jsFunctionBodyInWindowMenuTest(t, source, "function openApp(appId, context)")
	for _, bad := range []string{"â", ">_</button>", ">x</button>"} {
		if strings.Contains(openAppBody, bad) {
			t.Fatalf("desktop window chrome should not render visible text fallback %q in control buttons", bad)
		}
	}
	for _, want := range []string{
		`data-action="minimize" title="${esc(t('desktop.minimize'))}" aria-label="${esc(t('desktop.minimize'))}"></button>`,
		`data-action="maximize" title="${esc(t('desktop.maximize'))}" aria-label="${esc(t('desktop.maximize'))}"></button>`,
		`data-action="close" title="${esc(t('desktop.close'))}" aria-label="${esc(t('desktop.close'))}"></button>`,
	} {
		if !strings.Contains(openAppBody, want) {
			t.Fatalf("desktop window chrome missing icon-only button markup %q", want)
		}
	}

	cssBytes, err := os.ReadFile(filepath.Join("css", "desktop.css"))
	if err != nil {
		t.Fatalf("read desktop stylesheet: %v", err)
	}
	css := string(cssBytes)
	for _, want := range []string{
		`.vd-window-button[data-action="maximize"]::before`,
		"border: 2px solid currentColor;",
		`.desktop-body[data-theme="fruity"] .vd-window-button::before`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop window chrome CSS missing marker %q", want)
		}
	}
}
