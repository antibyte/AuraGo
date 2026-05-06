package ui

import (
	"strings"
	"testing"
)

func TestDesktopMobileTaskbarStaysInVisualViewport(t *testing.T) {
	t.Parallel()

	data, err := Content.ReadFile("css/desktop.css")
	if err != nil {
		t.Fatalf("desktop stylesheet missing from embedded UI: %v", err)
	}
	css := string(data)
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

	cssBytes, err := Content.ReadFile("css/desktop.css")
	if err != nil {
		t.Fatalf("desktop stylesheet missing from embedded UI: %v", err)
	}
	css := string(cssBytes)
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
