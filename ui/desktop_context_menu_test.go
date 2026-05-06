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
	if strings.Contains(mainText, "allow-same-origin") {
		t.Fatal("generated desktop iframes must not enable allow-same-origin")
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
