package ui

import (
	"strings"
	"testing"
)

func TestDesktopIconsSupportClickAndDragMultiSelection(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	foundation := rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	bootstrap := rawDesktopAssetText(t, "js/desktop/core/sdk-events-bootstrap.js")
	css := readAllDesktopCSS(t)

	for _, want := range []string{
		"selectedIconIds: new Set(),",
		"function selectDesktopIcon(btn, options)",
		"function selectedDesktopIcons()",
		"function syncDesktopIconSelection()",
		"function startDesktopSelectionDrag(event)",
		"function updateDesktopSelectionDrag(event)",
		"function finishDesktopSelectionDrag(event)",
		"function desktopSelectionRectFromPoints(",
		"function desktopIconIntersectsSelection(",
		"event.ctrlKey || event.metaKey",
		"desktopSelectionDrag.baseSelection",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop multi-select runtime missing marker %q", want)
		}
	}

	renderIconsBody := jsFunctionBodyInWindowMenuTest(t, foundation, "function renderIcons()")
	for _, want := range []string{
		"aria-selected=\"${state.selectedIconIds.has(item.id) ? 'true' : 'false'}\"",
		"selectDesktopIcon(btn, { extend: event.ctrlKey || event.metaKey, toggle: event.ctrlKey || event.metaKey })",
		"wireDraggableIcon(btn)",
	} {
		if !strings.Contains(renderIconsBody, want) {
			t.Fatalf("desktop icon rendering missing multi-select marker %q", want)
		}
	}

	wireChromeBody := jsFunctionBodyInWindowMenuTest(t, bootstrap, "function wireChrome()")
	if !strings.Contains(wireChromeBody, "$('vd-workspace').addEventListener('pointerdown', startDesktopSelectionDrag)") {
		t.Fatal("desktop chrome must start the drag selection rectangle from empty workspace pointerdown")
	}
	if !strings.Contains(wireChromeBody, "selectDesktopIcon(null)") {
		t.Fatal("desktop chrome must still clear icon selection when clicking empty desktop space")
	}

	for _, want := range []string{
		".vd-desktop-selection-marquee",
		"z-index: calc(var(--vd-z-desktop) + 2);",
		"pointer-events: none;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop multi-select CSS missing marker %q", want)
		}
	}
}
