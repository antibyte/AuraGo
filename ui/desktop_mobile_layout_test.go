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
		"max-height: 70dvh",
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
		"overflow-x: auto;",
		"-webkit-overflow-scrolling: touch;",
		"touch-action: pan-x pan-y;",
		"min-width: var(--vd-mobile-workspace-width);",
		".vd-window.maximized",
		"width: 100vw !important;",
		".vd-mobile-wide-window",
		"width: var(--vd-mobile-workspace-width) !important;",
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
