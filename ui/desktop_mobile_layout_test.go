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
