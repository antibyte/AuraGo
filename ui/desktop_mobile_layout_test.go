package ui

import (
	"strings"
	"testing"
)

func TestDesktopMobileTaskbarStaysInVisualViewport(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	for _, want := range []string{
		"@supports (height: 100dvh)",
		"height: 100dvh;",
		"min-height: 0;",
		"grid-template-rows: minmax(0, 1fr) auto;",
		"env(safe-area-inset-bottom)",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop mobile stylesheet missing visible taskbar rule %q", want)
		}
	}
}

func TestVirtualDesktopHasMobileInteractionMarkers(t *testing.T) {
	t.Parallel()

	js := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"function isCompactViewport()",
		"function isTouchLikePointer(event)",
		"function wireLongPress(element, callback, options)",
		"function shouldOpenOnTap(event)",
		"function updateViewportMetrics()",
		"window.visualViewport",
		"function ensureFocusedControlVisible(event)",
		"function wireWindowTouchGestures(win, id)",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("desktop mobile interaction script missing marker %q", want)
		}
	}
}

func TestVirtualDesktopShortDesktopHeightDoesNotDisableWindowDragging(t *testing.T) {
	t.Parallel()

	sources := map[string]string{
		"source": rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js"),
		"bundle": readDesktopAssetText(t, "js/desktop/main.js"),
	}
	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			body := jsFunctionBodyInWindowMenuTest(t, source, "function isCompactViewport()")
			for _, want := range []string{
				"const widthMatch = window.matchMedia('(max-width: 820px)').matches;",
				"const heightMatch = window.matchMedia('(max-height: 720px)').matches;",
				"const coarsePointerMatch = window.matchMedia('(hover: none) and (pointer: coarse)').matches;",
				"return widthMatch || (heightMatch && coarsePointerMatch);",
			} {
				if !strings.Contains(body, want) {
					t.Fatalf("compact viewport detection must keep short desktop windows draggable, missing %q", want)
				}
			}
			if strings.Contains(body, "return widthMatch || heightMatch;") {
				t.Fatal("compact viewport detection must not treat short viewport height alone as mobile")
			}
		})
	}
}

func TestVirtualDesktopNarrowFinePointerDoesNotDisableWindowChrome(t *testing.T) {
	t.Parallel()

	foundationSources := map[string]string{
		"source": rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js"),
		"bundle": readDesktopAssetText(t, "js/desktop/main.js"),
	}
	for name, source := range foundationSources {
		t.Run(name+" mobile mode", func(t *testing.T) {
			body := jsFunctionBodyInWindowMenuTest(t, source, "function useMobileDesktopMode()")
			if !strings.Contains(body, "return isCompactViewport() && isTouchLikePointer();") {
				t.Fatalf("auto mobile desktop mode must require compact viewport and touch-like input, body:\n%s", body)
			}
			if strings.Contains(body, "return isCompactViewport() || isTouchLikePointer();") {
				t.Fatal("auto mobile desktop mode must not treat narrow desktop width alone as mobile")
			}
		})
	}

	interactionSources := map[string]string{
		"source": rawDesktopAssetText(t, "js/desktop/core/window-interactions-runtime.js"),
		"bundle": readDesktopAssetText(t, "js/desktop/main.js"),
	}
	for name, source := range interactionSources {
		t.Run(name+" pointer gates", func(t *testing.T) {
			wireBody := jsFunctionBodyInWindowMenuTest(t, source, "function wireWindow(win, id)")
			resizeBody := jsFunctionBodyInWindowMenuTest(t, source, "function wireWindowResize(win, id)")
			for label, body := range map[string]string{
				"window drag":   wireBody,
				"window resize": resizeBody,
			} {
				if !strings.Contains(body, "window.useMobileDesktopMode && window.useMobileDesktopMode()") {
					t.Fatalf("%s must use mobile desktop mode instead of compact viewport alone", label)
				}
				if strings.Contains(body, "if (isCompactViewport()) return;") {
					t.Fatalf("%s must not disable pointer chrome for narrow fine-pointer desktop viewports", label)
				}
			}
		})
	}

	cssSources := map[string]string{
		"source": rawDesktopAssetText(t, "css/desktop-windows.css"),
		"bundle": readDesktopAssetText(t, "css/desktop-shell.bundle.css"),
	}
	for name, css := range cssSources {
		t.Run(name+" css", func(t *testing.T) {
			want := "@media (max-width: 820px) and (hover: none), (max-width: 820px) and (pointer: coarse)"
			if !strings.Contains(css, want) {
				t.Fatalf("mobile fullscreen window CSS must be scoped to touch-like devices, missing %q", want)
			}
		})
	}
}

func TestVirtualDesktopHasMobileLayoutMarkers(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	for _, want := range []string{
		".vd-long-press-active",
		"overscroll-behavior: none",
		".vd-window-titlebar",
		"touch-action: none;",
		"max-height: min(70dvh",
		"@media (max-width: 560px)",
		".fm-sidebar-toggle",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop mobile stylesheet missing marker %q", want)
		}
	}
}

func TestVirtualDesktopMobileWorkspaceCanScrollHorizontally(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	for _, want := range []string{
		"--vd-mobile-workspace-width",
		"--vd-mobile-taskbar-reserve",
		"overflow-x: auto;",
		"-webkit-overflow-scrolling: touch;",
		"touch-action: pan-x pan-y;",
		"min-width: var(--vd-mobile-workspace-width);",
		".vd-window.maximized",
		"width: 100vw !important;",
		".vd-mobile-wide-window",
		"width: var(--vd-mobile-workspace-width) !important;",
		".vd-window.vd-mobile-wide-window .vd-window-titlebar",
		"position: sticky;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop mobile stylesheet missing horizontal scroll marker %q", want)
		}
	}

	js := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, want := range []string{
		"function shouldUseMobileWideWindow(appId)",
		"win.classList.toggle('vd-mobile-wide-window'",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("desktop mobile window runtime missing opt-in marker %q", want)
		}
	}
	if strings.Contains(js, "'agent-chat',") {
		t.Fatal("desktop agent chat should not opt into the wide mobile window layout")
	}
}

func TestVirtualDesktopMobileOverlaysAvoidTaskbar(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	for _, want := range []string{
		"bottom: var(--vd-mobile-taskbar-reserve",
		"max-height: min(70dvh",
		"calc(var(--vd-visual-height, 100dvh) - var(--vd-mobile-taskbar-reserve",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop mobile overlay stylesheet missing taskbar avoidance marker %q", want)
		}
	}

	js := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"function contextMenuUsableBottom()",
		"window.matchMedia('(max-width: 820px)')",
		"const usableBottom = contextMenuUsableBottom();",
		"menu.style.maxHeight = maxMenuHeight + 'px';",
		"menu.style.top = Math.max(8, Math.min(y, usableBottom - rect.height)) + 'px';",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("desktop mobile overlay script missing taskbar avoidance marker %q", want)
		}
	}
}

func TestDesktopAgentChatMobileKeepsInputReachable(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	for _, want := range []string{
		"height: calc(var(--vd-visual-height, 100dvh) - var(--vd-mobile-taskbar-reserve",
		".vd-chat-main",
		"overflow: hidden;",
		".vd-chat-input",
		"max-height: 96px;",
		"font-size: 16px;",
		"z-index: 16;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop chat mobile stylesheet missing reachable input marker %q", want)
		}
	}

	js := readDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	for _, want := range []string{
		"sidebarOpen = host.offsetWidth > 900;",
		"parseFloat(window.getComputedStyle(input).maxHeight || '')",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("desktop chat mobile script missing reachable input marker %q", want)
		}
	}
}

func TestDesktopHasPWAMetaTags(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "desktop.html")
	for _, want := range []string{
		`<link rel="manifest" href="/site.webmanifest" />`,
		`<link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon.png" />`,
		`<meta name="theme-color" content="#11151c" />`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("desktop.html missing PWA meta tag %q", want)
		}
	}
}

func TestDesktopInputsHaveMobileAttributes(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "desktop.html")
	if !strings.Contains(html, `inputmode="search"`) || !strings.Contains(html, `enterkeyhint="search"`) {
		t.Fatal("desktop.html start search input missing mobile input attributes")
	}

	jsFiles := []string{
		"js/desktop/file-manager/core-render.js",
		"js/desktop/apps/settings-calculator.js",
		"js/desktop/apps/quickconnect-launchpad-chat.js",
		"js/desktop/core/window-shell-runtime.js",
		"js/desktop/core/file-dialog-runtime.js",
		"js/desktop/apps/agent-chat.js",
		"js/desktop/apps/people.js",
		"js/desktop/apps/radio.js",
		"js/desktop/apps/teevee.js",
		"js/desktop/apps/mission-control.js",
		"js/desktop/apps/looper.js",
		"js/desktop/apps/code-studio/command-palette.js",
	}
	for _, file := range jsFiles {
		js := readDesktopAssetText(t, file)
		if !strings.Contains(js, "inputmode=") {
			t.Fatalf("%s missing inputmode attribute on inputs", file)
		}
		if !strings.Contains(js, "enterkeyhint=") {
			t.Fatalf("%s missing enterkeyhint attribute on inputs", file)
		}
	}
}

func TestDesktopHasMobileBackdropFilterReduction(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	if !strings.Contains(css, "@media (pointer: coarse)") {
		t.Fatal("desktop CSS missing mobile performance @media (pointer: coarse) query")
	}
	want := "blur(6px) saturate(120%)"
	if !strings.Contains(css, want) {
		t.Fatalf("desktop CSS missing mobile backdrop-filter reduction %q", want)
	}
}

func TestDesktopHasMobileInputFontSize(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	if !strings.Contains(css, "@media (pointer: coarse)") {
		t.Fatal("desktop CSS missing mobile @media (pointer: coarse) query")
	}
	if !strings.Contains(css, ".vd-shell input[type=\"text\"]") && !strings.Contains(css, `.vd-shell input[type="text"]`) {
		t.Fatal("desktop CSS missing mobile input font-size rule for text inputs")
	}
}

func TestFileManagerHasTouchSelection(t *testing.T) {
	t.Parallel()

	js := readDesktopAssetText(t, "js/desktop/file-manager/actions-input.js")
	for _, want := range []string{
		"function handleItemLongPress(e)",
		"function exitSelectionMode()",
		"fm.selectionMode = true",
		"fm.selectionMode = false",
		"wireLongPress(item, handleItemLongPress)",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("file manager actions-input.js missing touch selection marker %q", want)
		}
	}

	render := readDesktopAssetText(t, "js/desktop/file-manager/core-render.js")
	for _, want := range []string{
		"function renderSelectionToolbarHtml()",
		"fm-selection-toolbar",
		"selectionMode: false",
	} {
		if !strings.Contains(render, want) {
			t.Fatalf("file manager core-render.js missing touch selection marker %q", want)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-app-file-manager.css")
	for _, want := range []string{
		".fm-selection-toolbar",
		".fm-selection-toolbar.active",
		".fm-selection-btn",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("file manager CSS missing touch selection style %q", want)
		}
	}
}

func TestFileManagerSelectionToolbarAvoidsReadonlyShadowing(t *testing.T) {
	t.Parallel()

	for _, file := range []string{
		"js/desktop/file-manager/core-render.js",
		"js/desktop/bundles/file-manager.bundle.js",
	} {
		js := readDesktopAssetText(t, file)
		if strings.Contains(js, "const isReadonly = typeof isReadonly") {
			t.Fatalf("%s shadows isReadonly and can throw a temporal-dead-zone ReferenceError", file)
		}
		if !strings.Contains(js, "const readonly = (typeof isReadonly === 'function') ? isReadonly() : false;") {
			t.Fatalf("%s missing safe readonly selection toolbar marker", file)
		}
	}
}
