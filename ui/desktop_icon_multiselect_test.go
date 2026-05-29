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

	updateIconBody := jsFunctionBodyInWindowMenuTest(t, foundation, "function updateDesktopIconButton")
	bindIconBody := jsFunctionBodyInWindowMenuTest(t, foundation, "function bindDesktopIconButton")
	for _, want := range []string{
		"btn.setAttribute('aria-selected', state.selectedIconIds.has(item.id) ? 'true' : 'false')",
	} {
		if !strings.Contains(updateIconBody, want) {
			t.Fatalf("desktop icon rendering missing multi-select marker %q", want)
		}
	}
	for _, want := range []string{
		"selectDesktopIcon(btn, { extend: event.ctrlKey || event.metaKey, toggle: event.ctrlKey || event.metaKey })",
		"wireDraggableIcon(btn)",
	} {
		if !strings.Contains(bindIconBody, want) {
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

func TestDesktopIconGroupsMoveTogetherAndUseBatchContextActions(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	foundation := rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	menus := rawDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")

	for _, want := range []string{
		"function selectedDesktopCommandIcons(anchor)",
		"function desktopDragItemsForIcon(anchor)",
		"function clampDesktopDragDelta(items, dx, dy)",
		"function moveDesktopDragItems(items, dx, dy)",
		"function saveDesktopDragItems(items)",
		"function resetDesktopDragItems(items)",
		"function suppressDesktopIconClicks(items)",
		"function activateDesktopItems(icons)",
		"function desktopBatchPaths(anchor)",
		"function deleteDesktopPaths(paths)",
		"function removeDesktopShortcuts(ids)",
		"function deleteDesktopApps(appIds)",
		"function handleTrashDropForIcons(icons)",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop grouped icon command runtime missing marker %q", want)
		}
	}

	dragBody := jsFunctionBodyInWindowMenuTest(t, foundation, "function wireDraggableIcon(btn)")
	for _, want := range []string{
		"items: desktopDragItemsForIcon(btn)",
		"const delta = clampDesktopDragDelta(drag.items, dx, dy)",
		"moveDesktopDragItems(drag.items, delta.dx, delta.dy)",
		"saveDesktopDragItems(drag.items)",
		"resetDesktopDragItems(drag.items)",
		"handleTrashDropForIcons(drag.items.map(item => item.icon))",
	} {
		if !strings.Contains(dragBody, want) {
			t.Fatalf("desktop icon drag does not move selected groups, missing %q", want)
		}
	}

	contextBody := jsFunctionBodyInWindowMenuTest(t, menus, "function showIconContextMenu(event, btn)")
	for _, want := range []string{
		"const commandIcons = selectedDesktopCommandIcons(btn)",
		"activateDesktopItems(commandIcons)",
		"setDesktopFileClipboard('cut', desktopBatchPaths(btn))",
		"setDesktopFileClipboard('copy', desktopBatchPaths(btn))",
		"deleteDesktopPaths(desktopBatchPaths(btn))",
		"removeDesktopShortcuts(desktopBatchShortcutIds(btn))",
		"deleteDesktopApps(desktopBatchAppIds(btn))",
	} {
		if !strings.Contains(contextBody, want) {
			t.Fatalf("desktop icon context menu does not use selected batch actions, missing %q", want)
		}
	}
}
