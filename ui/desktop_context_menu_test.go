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
		for _, key := range []string{"desktop.context_empty_trash", "desktop.confirm_empty_trash", "desktop.confirm_empty_trash_msg", "desktop.trash_empty"} {
			if !strings.Contains(text, `"`+key+`"`) {
				t.Fatalf("%s desktop translations missing %q", lang, key)
			}
		}
	}
}

func TestDesktopBuiltInAppsDeclareContextMenuPolicy(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, check := range []struct {
		name      string
		signature string
		markers   []string
	}{
		{
			name:      "calculator",
			signature: "function renderCalculator(id)",
			markers: []string{
				"wireContextMenuBoundary(root",
				"showCalculatorContextMenu",
				"navigator.clipboard.writeText",
			},
		},
		{
			name:      "todo",
			signature: "async function renderTodo(id)",
			markers: []string{
				"showTodoContextMenu",
				"wireContextMenuBoundary(host",
			},
		},
		{
			name:      "calendar",
			signature: "async function renderCalendar(id)",
			markers: []string{
				"showCalendarContextMenu",
				"wireContextMenuBoundary(host",
			},
		},
		{
			name:      "gallery",
			signature: "async function renderGallery(id)",
			markers: []string{
				"showGalleryContextMenu",
				"wireContextMenuBoundary(host",
			},
		},
		{
			name:      "music player",
			signature: "async function renderMusicPlayer(id)",
			markers: []string{
				"showMusicPlayerContextMenu",
				"wireContextMenuBoundary(host",
			},
		},
		{
			name:      "quick connect",
			signature: "function renderQuickConnect(id)",
			markers: []string{
				"showDeviceContextMenu",
				"wireContextMenuBoundary(host",
			},
		},
		{
			name:      "launchpad",
			signature: "function renderLaunchpad(id)",
			markers: []string{
				"showLaunchpadContextMenu",
				"wireContextMenuBoundary(host",
			},
		},
	} {
		body := jsFunctionBodyInWindowMenuTest(t, mainText, check.signature)
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
	} {
		text := readDesktopAssetText(t, path)
		if !strings.Contains(text, "contextmenu") && !strings.Contains(text, "wireContextMenuBoundary") {
			t.Fatalf("%s missing explicit contextmenu handling", path)
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
