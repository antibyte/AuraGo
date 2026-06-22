package ui

import (
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
		"workspaceRect.height - taskbarReserve - margin * 2",
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

	css := readAllDesktopCSS(t)
	for _, marker := range []string{
		"min-width: 0 !important;",
		"min-height: 0 !important;",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop mobile window sizing missing marker %q", marker)
		}
	}
}

func TestDesktopWindowCanOpenMetadataAppsMaximized(t *testing.T) {
	source := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"function shouldOpenMaximized(app)",
		"app.metadata.open_maximized === 'true'",
		"app.metadata.store_app_id === 'quakejs-rootless'",
		"if (shouldOpenMaximized(app)) toggleMaximizeWindow(id);",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop window runtime missing maximized metadata marker %q", marker)
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

	css := readAllDesktopCSS(t)
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

func TestDesktopWindowResizeHandlesStayOutsideScrollableContent(t *testing.T) {
	t.Parallel()

	css := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-windows.css"), "\r\n", "\n")
	blockFor := func(selector string) string {
		haystack := "\n" + css
		start := strings.LastIndex(haystack, "\n"+selector+" {")
		if start < 0 {
			t.Fatalf("desktop windows CSS missing selector %q", selector)
		}
		start++
		end := strings.Index(haystack[start:], "\n}")
		if end < 0 {
			t.Fatalf("desktop windows CSS selector %q is missing closing brace", selector)
		}
		return haystack[start : start+end]
	}

	for selector, markers := range map[string][]string{
		".vd-window":                             {"overflow: visible;"},
		".vd-window-content":                     {"overflow: hidden;"},
		".vd-resize-handle::after":               {"inset: 0;"},
		".vd-resize-e,\n.vd-resize-w":            {"width: 8px;"},
		".vd-resize-n,\n.vd-resize-s":            {"height: 8px;"},
		".vd-resize-e":                           {"right: -8px;"},
		".vd-resize-w":                           {"left: -8px;"},
		".vd-resize-n":                           {"top: -8px;"},
		".vd-resize-s":                           {"bottom: -8px;"},
		".vd-window.maximized .vd-resize-handle": {"display: none;"},
	} {
		block := blockFor(selector)
		for _, marker := range markers {
			if !strings.Contains(block, marker) {
				t.Fatalf("desktop resize handle selector %q missing marker %q in block: %s", selector, marker, block)
			}
		}
	}

	for selector, forbidden := range map[string][]string{
		".vd-resize-handle::after": {"inset: -3px;"},
		".vd-resize-e":             {"right: 0;"},
		".vd-resize-w":             {"left: 0;"},
		".vd-resize-n":             {"top: 0;"},
		".vd-resize-s":             {"bottom: 0;"},
	} {
		block := blockFor(selector)
		for _, marker := range forbidden {
			if strings.Contains(block, marker) {
				t.Fatalf("desktop resize handle selector %q must not cover scrollable content with %q in block: %s", selector, marker, block)
			}
		}
	}
}

func TestDesktopWindowScrollbarsStayWideAndUseArrowCursor(t *testing.T) {
	t.Parallel()

	css := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-windows.css"), "\r\n", "\n")
	for _, marker := range []string{
		".vd-window-content,\n.vd-window-content * {",
		"scrollbar-width: auto;",
		"scrollbar-color: var(--ds-color-control-hover) var(--ds-color-control-bg);",
		".vd-window-content ::-webkit-scrollbar {",
		"width: 14px;",
		"height: 14px;",
		"cursor: default;",
		".vd-window-content ::-webkit-scrollbar-thumb {",
		"min-height: 36px;",
		"border: 3px solid transparent;",
		"background-clip: padding-box;",
		".vd-window-content ::-webkit-scrollbar-track {",
		".vd-window-content ::-webkit-scrollbar-thumb:hover {",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop window scrollbar CSS missing marker %q", marker)
		}
	}
}
