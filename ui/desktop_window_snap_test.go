package ui

import (
	"strings"
	"testing"
)

func TestDesktopWindowTopSnapRequiresSideProximity(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"source": rawDesktopAssetText(t, "js/desktop/core/window-interactions-runtime.js"),
		"bundle": readDesktopAssetText(t, "js/desktop/main.js"),
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			assertDesktopWindowTopSnapRequiresSideProximity(t, source)
		})
	}
}

func assertDesktopWindowTopSnapRequiresSideProximity(t *testing.T, source string) {
	t.Helper()

	if !strings.Contains(source, "const SNAP_TOP_SIDE_EDGE = 96;") {
		t.Fatal("window snap runtime must define a top-edge side proximity threshold")
	}
	body := jsFunctionBodyInWindowMenuTest(t, source, "function updateSnapZone")
	for _, marker := range []string{
		"const nearLeftEdge = left <= SNAP_TOP_SIDE_EDGE;",
		"const nearRightEdge = left + width >= ww - SNAP_TOP_SIDE_EDGE;",
		"const nearBottomEdge = top + height >= wh - SNAP_EDGE;",
		"if (top <= SNAP_EDGE && nearLeftEdge) zone = 'top-left';",
		"else if (top <= SNAP_EDGE && nearRightEdge) zone = 'top-right';",
		"else if (nearBottomEdge && nearLeftEdge) zone = 'bottom-left';",
		"else if (nearBottomEdge && nearRightEdge) zone = 'bottom-right';",
		"else if (left <= SNAP_EDGE) zone = 'left-half';",
		"else if (left + width >= ww - SNAP_EDGE) zone = 'right-half';",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("window snap runtime missing marker %q", marker)
		}
	}
	for _, forbidden := range []string{
		"else if (top <= SNAP_EDGE) zone = 'top-half';",
		"top <= SNAP_EDGE && cx",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("window snap runtime still allows top-edge snap without side proximity via %q", forbidden)
		}
	}

	topCornerIndex := strings.Index(body, "if (top <= SNAP_EDGE && nearLeftEdge)")
	sideHalfIndex := strings.Index(body, "else if (left <= SNAP_EDGE)")
	if topCornerIndex < 0 || sideHalfIndex < 0 || topCornerIndex > sideHalfIndex {
		t.Fatal("top-corner snap checks must run before plain side-half snap checks")
	}
}

func TestDesktopWindowSnapKeyboardUsesMetaArrows(t *testing.T) {
	t.Parallel()

	source := rawDesktopAssetText(t, "js/desktop/core/window-interactions-runtime.js")
	for _, want := range []string{
		"function handleWindowSnapShortcut(",
		"function restoreWindowFromSnap(",
		"if (!event.metaKey || event.ctrlKey || event.altKey) return false",
		"const zone = desktopWindowKeyboardLayout(item.snapped, event.key)",
		"if (zone === 'restore') restoreWindowFromSnap(item.id)",
		"applyWindowSnap(item.element, zone)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("window snap keyboard handler missing %q", want)
		}
	}
}

func TestDesktopWindowSnapUsesSharedLayoutRectangles(t *testing.T) {
	t.Parallel()

	layouts := rawDesktopAssetText(t, "js/desktop/core/window-layouts-runtime.js")
	interactions := rawDesktopAssetText(t, "js/desktop/core/window-interactions-runtime.js")
	for _, zone := range []string{"left-half", "right-half", "top-half", "bottom-half", "top-left", "top-right", "bottom-left", "bottom-right", "maximize"} {
		if !strings.Contains(layouts, "'"+zone+"'") {
			t.Errorf("shared layout helper is missing %s", zone)
		}
	}
	for _, source := range []struct{ name, text string }{
		{"overlay", jsFunctionBodyInWindowMenuTest(t, interactions, "function showSnapOverlay")},
		{"placement", jsFunctionBodyInWindowMenuTest(t, interactions, "function applyWindowSnap")},
	} {
		if !strings.Contains(source.text, "desktopWindowLayoutRect(zone, ww, wh)") {
			t.Errorf("%s must calculate its rectangle through the shared layout helper", source.name)
		}
	}
	if !strings.Contains(interactions, "desktopWindowLayoutFits(p, limits)") {
		t.Fatal("window placement must reject layouts outside the app's minimum and maximum bounds")
	}
	if !strings.Contains(interactions, "desktopWindowLayoutMaxLimit(cssLimits && cssLimits.maxWidth, workspace.width)") {
		t.Fatal("window placement must account for the app's maximum width")
	}
}
