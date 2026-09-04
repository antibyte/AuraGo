package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopContextMenuAndClipboardAssets(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"function isNativeContextMenuTarget(",
		"function shouldAllowBrowserContextMenu(",
		"function suppressBrowserContextMenu(",
		"function wireContextMenuBoundary(",
		"function wireWindowContextMenu(",
		"desktop:context-menu:show",
		"desktop:context-menu:clear",
		"desktop:clipboard:read-text",
		"desktop:clipboard:write-text",
		"postSDKContextMenuAction(",
		`iframe.setAttribute('allow', 'clipboard-read; clipboard-write')`,
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop shell missing context menu/clipboard marker %q", want)
		}
	}
	if !strings.Contains(mainText, "allowSameOrigin") {
		t.Fatal("desktop shell must make same-origin iframe access an explicit store-app option")
	}
	if strings.Contains(mainText, "setAttribute('csp'") || strings.Contains(mainText, `setAttribute("csp"`) {
		t.Fatal("generated desktop iframes must rely on /files/desktop/ response CSP, not iframe csp attributes")
	}
	for _, want := range []string{
		"iframe.tabIndex = 0",
		"iframe.addEventListener('pointerdown', () => focusDesktopFrame(iframe))",
		"iframe.addEventListener('load', () => focusDesktopFrame(iframe))",
		"function focusDesktopFrame(iframe)",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("generated desktop iframes must actively support keyboard focus, missing %q", want)
		}
	}

	sdkText := readDesktopAssetText(t, "js/desktop/aura-desktop-sdk.js")
	for _, want := range []string{
		"const CONTEXT_MENU_ACTION_TYPE = 'aurago.desktop.context-menu-action'",
		"const contextMenuActionHandlers = new Map()",
		"const contextMenuDirectActions = new Map()",
		"function serializeContextMenuItems(",
		"function contextMenuPoint(",
		"contextMenu: ui.contextMenu",
		"clipboard: ui.clipboard",
		"desktop:context-menu:show",
		"desktop:context-menu:clear",
		"desktop:clipboard:read-text",
		"desktop:clipboard:write-text",
	} {
		if !strings.Contains(sdkText, want) {
			t.Fatalf("desktop SDK missing context menu/clipboard marker %q", want)
		}
	}
}

func TestDesktopTrashCanSupportsDropAndEmptyMenu(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"function isTrashIcon(",
		"function desktopTrashDropTargetAt(",
		"function handleTrashDrop(",
		"function movePathToTrash(",
		"function emptyTrash(",
		"vd-trash-drop-target",
		"desktop.context_empty_trash",
		"new_path: trashDestination",
		"body: JSON.stringify({ old_path: cleanPath, new_path: trashDestination })",
		"removeIconPosition('desktop-entry-' + cleanPath)",
		"await removeDesktopShortcut(btn.dataset.id || '')",
		"await api('/api/desktop/file?path=' + encodeURIComponent(entry.path), { method: 'DELETE' })",
		"function restorePathsFromTrash(",
		"function uniqueRestoreDestination(",
		"uniqueRestoreDestination('Desktop'",
		"restoreFromTrash: restorePathsFromTrash",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop trash can integration missing marker %q", want)
		}
	}

	css := readAllDesktopCSS(t)
	if !strings.Contains(css, ".vd-icon.vd-trash-drop-target") {
		t.Fatal("desktop trash drop target state is missing CSS styling")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		text := rawDesktopAssetText(t, filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json")))
		for _, key := range []string{"desktop.context_empty_trash", "desktop.confirm_empty_trash", "desktop.confirm_empty_trash_msg", "desktop.trash_empty", "desktop.fm.restore", "desktop.trash_restored", "desktop.trash_restored_items"} {
			if !strings.Contains(text, `"`+key+`"`) {
				t.Fatalf("%s desktop translations missing %q", lang, key)
			}
		}
	}
}

func TestDesktopWindowAlwaysOnTopMarkers(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"function toggleWindowAlwaysOnTop(",
		"function assignWindowZ(",
		"WINDOW_ALWAYS_ON_TOP_Z = 200000",
		"desktop.context_always_on_top",
		"alwaysOnTop: !!item.alwaysOnTop",
		"alwaysOnTop: !!(sessionRestore && sessionRestore.alwaysOnTop)",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop window always-on-top missing marker %q", want)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		text := rawDesktopAssetText(t, filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json")))
		if !strings.Contains(text, `"desktop.context_always_on_top"`) {
			t.Fatalf("%s desktop translations missing %q", lang, "desktop.context_always_on_top")
		}
	}
}

func TestDesktopIconGridContextMenuToggle(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"const ICON_GRID_KEY = 'aurago.desktop.iconGrid.v1'",
		"function desktopIconGridEnabled()",
		"function setDesktopIconGridEnabled(enabled)",
		"function toggleDesktopIconGrid()",
		"desktop.context_icon_grid",
		"icon: desktopIconGridEnabled() ? 'check-square' : 'square'",
		"action: toggleDesktopIconGrid",
		"setDesktopIconGridEnabled(enabled);",
		"if (enabled) arrangeDesktopIconsToGrid();",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop icon grid menu integration missing marker %q", want)
		}
	}
}

func TestDesktopIconGridSnapsDraggedIconsWhenEnabled(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"function desktopIconGridMetrics()",
		"function desktopIconGridPosition(index)",
		"function desktopIconGridNearestFreePosition(left, top, usedCells)",
		"function snapDesktopDragItemsToGrid(items)",
		"if (desktopIconGridEnabled()) {",
		"snapDesktopDragItemsToGrid(items);",
		"return;",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop icon grid drag snapping missing marker %q", want)
		}
	}
}

func TestDesktopIconGridSnapsIconsDuringDragMove(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	moveBody := jsFunctionBodyInWindowMenuTest(t, mainText, "function moveDesktopDragItems(items, dx, dy)")
	for _, want := range []string{
		"if (desktopIconGridEnabled()) {",
		"positionDesktopDragItemsOnGrid(items, item => ({ left: item.left + dx, top: item.top + dy }), false);",
		"return;",
	} {
		if !strings.Contains(moveBody, want) {
			t.Fatalf("desktop icon grid must snap icons during drag movement; missing %q", want)
		}
	}
	for _, want := range []string{
		"function positionDesktopDragItemsOnGrid(items, positionForItem, persist)",
		"if (persist) saveIconPosition(item.id, pos.x, pos.y);",
		"positionDesktopDragItemsOnGrid(items, item => desktopDragItemCurrentPosition(item), true);",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop icon grid must share live/final snap placement; missing %q", want)
		}
	}
}

func TestDesktopIconGridSnapsDesktopFileDropsWhenEnabled(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"function desktopFileDropIconPosition(left, top, usedCells)",
		"if (desktopIconGridEnabled()) return desktopIconGridNearestFreePosition(left, top, usedCells);",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop file drops must share grid-aware icon placement; missing %q", want)
		}
	}

	dropBody := jsFunctionBodyInWindowMenuTest(t, mainText, "async function moveDraggedFilesToDesktop(paths, clientX, clientY)")
	for _, want := range []string{
		"let usedCells = desktopIconGridEnabled() ? desktopIconGridUsedCells(desktopFileDropExcludedIconIds(cleanPaths)) : null;",
		"const iconPos = desktopFileDropIconPosition(basePos.x + offset, basePos.y + offset, usedCells);",
	} {
		if !strings.Contains(dropBody, want) {
			t.Fatalf("desktop file drag/drop must snap saved icon positions when grid is enabled; missing %q", want)
		}
	}

	pasteBody := jsFunctionBodyInWindowMenuTest(t, mainText, "async function pasteDesktopFileClipboard(destBase, options)")
	for _, want := range []string{
		"let usedCells = desktopIconGridEnabled() ? desktopIconGridUsedCells(desktopFileDropExcludedIconIds(clipboard.mode === 'cut' ? clipboard.paths : [])) : null;",
		"const iconPos = desktopFileDropIconPosition(basePos.x + offset, basePos.y + offset, usedCells);",
	} {
		if !strings.Contains(pasteBody, want) {
			t.Fatalf("desktop paste must snap saved icon positions when grid is enabled; missing %q", want)
		}
	}
}

func TestDesktopIconGridTranslations(t *testing.T) {
	t.Parallel()

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		text := rawDesktopAssetText(t, filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json")))
		if !strings.Contains(text, `"desktop.context_icon_grid"`) {
			t.Fatalf("%s desktop translations missing %q", lang, "desktop.context_icon_grid")
		}
	}
}

func TestDesktopBuiltInAppsDeclareContextMenuPolicy(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	calcText := readDesktopAssetText(t, "js/desktop/apps/calculator.js")
	for _, check := range []struct {
		name      string
		source    string
		signature string
		markers   []string
	}{
		{
			name:      "calculator",
			source:    calcText,
			signature: "function renderCalculator(id)",
			markers: []string{
				"wireContextMenuBoundary(root",
				"showCalculatorContextMenu",
				"navigator.clipboard.writeText",
			},
		},
		{
			name:      "todo",
			source:    mainText,
			signature: "async function renderTodo(id)",
			markers: []string{
				"showTodoContextMenu",
				"wireContextMenuBoundary(host",
			},
		},
		{
			name:      "calendar",
			source:    mainText,
			signature: "async function renderCalendar(id)",
			markers: []string{
				"showCalendarContextMenu",
				"wireContextMenuBoundary(host",
			},
		},
		{
			name:      "gallery",
			source:    mainText,
			signature: "async function renderGallery(id)",
			markers: []string{
				"showGalleryContextMenu",
				"wireContextMenuBoundary(host",
			},
		},
		{
			name:      "music player",
			source:    mainText,
			signature: "async function renderMusicPlayer(id)",
			markers: []string{
				"showMusicPlayerContextMenu",
				"wireContextMenuBoundary(host",
			},
		},
		{
			name:      "quick connect",
			source:    mainText,
			signature: "function renderQuickConnect(id)",
			markers: []string{
				"showDeviceContextMenu",
				"wireContextMenuBoundary(host",
			},
		},
		{
			name:      "launchpad",
			source:    mainText,
			signature: "function renderLaunchpad(id)",
			markers: []string{
				"showLaunchpadContextMenu",
				"wireContextMenuBoundary(host",
			},
		},
	} {
		body := jsFunctionBodyInWindowMenuTest(t, check.source, check.signature)
		for _, marker := range check.markers {
			if !strings.Contains(body, marker) {
				t.Fatalf("%s missing context menu policy marker %q", check.name, marker)
			}
		}
	}

	for _, path := range []string{
		filepath.Join("js", "desktop", "file-manager.js"),
		filepath.Join("js", "desktop", "apps", "writer.js"),
		filepath.Join("js", "desktop", "apps", "sheets.js"),
		filepath.Join("js", "desktop", "apps", "code-studio.js"),
		filepath.Join("js", "desktop", "apps", "radio.js"),
		filepath.Join("js", "desktop", "apps", "teevee.js"),
		filepath.Join("js", "desktop", "apps", "camera.js"),
	} {
		text := readDesktopAssetText(t, path)
		if !strings.Contains(text, "contextmenu") && !strings.Contains(text, "wireContextMenuBoundary") {
			t.Fatalf("%s missing explicit contextmenu handling", path)
		}
	}
}

func TestDesktopContextMenuParityMarkers(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	css := readAllDesktopCSS(t)

	for _, want := range []string{
		"contextMenuShortcutMarkup(",
		"<kbd class=\"vd-context-shortcut\">",
		"buildDesktopOpenWithSubmenu(",
		"desktop.fm.open_with",
		"vd-properties-modal",
		"vd-prop-row",
		"showProperties(title, pathOrEntry)",
		"desktopWallpaperMenuItems(",
		"saveDesktopWallpaper(",
		"selectAllDesktopIcons(",
		"sortDesktopIconsByName(",
		"openApp('settings', { category: 'appearance' })",
		"openApp(appId, { forceNew: true })",
		"setAppVisibility(btn.dataset.id, { dock_visible: false })",
		"setAppVisibility(btn.dataset.id, { dock_visible: true })",
		"desktop.app_add_to_dock",
		"desktop.app_remove_from_dock",
		"context.forceNew) ? null : findExistingAppWindow",
		"appId === 'settings' && context && context.category) renderAppContent",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop context menu parity missing marker %q", want)
		}
	}

	if !strings.Contains(css, ".vd-context-shortcut") {
		t.Fatal("desktop context menu shortcut styling is missing")
	}
	if !strings.Contains(css, ".vd-properties-modal") {
		t.Fatal("desktop properties modal styling is missing")
	}

	settingsText := readDesktopAssetText(t, "js/desktop/apps/settings.js")
	if !strings.Contains(settingsText, "host.dataset.activeSettings = category") {
		t.Fatal("settings app must honor context.category for active section")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		text := rawDesktopAssetText(t, filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json")))
		for _, key := range []string{
			"desktop.context_settings",
			"desktop.context_new_window",
			"desktop.context_sort_by_name",
			"desktop.fm.open_with",
		} {
			if !strings.Contains(text, `"`+key+`"`) {
				t.Fatalf("%s desktop translations missing %q", lang, key)
			}
		}
	}
}

func TestVirtualDesktopManualDocumentsContextMenuSDK(t *testing.T) {
	t.Parallel()

	manual, err := os.ReadFile(filepath.Join("..", "prompts", "tools_manuals", "virtual_desktop.md"))
	if err != nil {
		t.Fatalf("read virtual desktop manual: %v", err)
	}
	text := string(manual)
	for _, want := range []string{
		"AuraDesktop.contextMenu.set",
		"AuraDesktop.contextMenu.show",
		"AuraDesktop.contextMenu.clear",
		"AuraDesktop.contextMenu.onAction",
		"AuraDesktop.clipboard.readText",
		"AuraDesktop.clipboard.writeText",
		"Browser context menu",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("virtual desktop manual missing context menu SDK marker %q", want)
		}
	}
}
