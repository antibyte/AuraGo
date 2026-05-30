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
		"if (top <= SNAP_EDGE && nearLeftEdge) zone = 'top-left';",
		"else if (top <= SNAP_EDGE && nearRightEdge) zone = 'top-right';",
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
