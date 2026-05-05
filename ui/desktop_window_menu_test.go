package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopWindowMenuAssets(t *testing.T) {
	t.Parallel()

	mainBytes, err := Content.ReadFile("js/desktop/main.js")
	if err != nil {
		t.Fatalf("read desktop shell: %v", err)
	}
	mainText := string(mainBytes)
	for _, want := range []string{
		"windowMenus: new Map()",
		"function setWindowMenus(windowId, menus)",
		"function clearWindowMenus(windowId)",
		"function renderWindowMenus(windowId)",
		"function normalizeWindowMenuItems(",
		"function automaticWindowMenu(",
		"function handleWindowMenuShortcut(event)",
		"function shortcutMatches(event, shortcut)",
		"class=\"vd-window-menubar\"",
		"has-window-menu",
		"desktop:menu:set",
		"desktop:menu:clear",
		"postSDKMenuAction(",
		"setEditorMenus(id,",
		"setCalendarMenus(id,",
		"setGalleryMenus(id,",
		"setQuickConnectMenus(id,",
		"setMusicPlayerMenus(id,",
		"setLaunchpadMenus(id,",
		"setTodoMenus(",
		"setFallbackFileMenus(id,",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop shell missing menu marker %q", want)
		}
	}

	sdkBytes, err := Content.ReadFile("js/desktop/aura-desktop-sdk.js")
	if err != nil {
		t.Fatalf("read desktop SDK: %v", err)
	}
	sdkText := string(sdkBytes)
	for _, want := range []string{
		"const menuActionHandlers = new Map()",
		"ui.menu = {",
		"set(menus)",
		"clear()",
		"onAction(handler)",
		"desktop:menu:set",
		"desktop:menu:clear",
		"aurago.desktop.menu-action",
	} {
		if !strings.Contains(sdkText, want) {
			t.Fatalf("desktop SDK missing menu marker %q", want)
		}
	}

	cssBytes, err := Content.ReadFile("css/desktop.css")
	if err != nil {
		t.Fatalf("read desktop css: %v", err)
	}
	cssText := string(cssBytes)
	for _, want := range []string{
		".vd-window.has-window-menu",
		".vd-window-menubar",
		".vd-window-menu-button",
		".vd-window-menu-popover",
		".vd-window-menu-item.checked",
		".vd-window-menu-submenu",
		".desktop-body[data-theme=\"fruity\"] .vd-window-menubar",
		".desktop-body[data-theme=\"fruity\"] .vd-window.has-window-menu",
	} {
		if !strings.Contains(cssText, want) {
			t.Fatalf("desktop css missing menu marker %q", want)
		}
	}

	for _, path := range []string{
		filepath.Join("js", "desktop", "apps", "writer.js"),
		filepath.Join("js", "desktop", "apps", "sheets.js"),
		filepath.Join("js", "desktop", "apps", "code-studio.js"),
		filepath.Join("js", "desktop", "apps", "radio.js"),
		filepath.Join("js", "desktop", "file-manager.js"),
	} {
		data, err := Content.ReadFile(filepath.ToSlash(path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		if !strings.Contains(text, "setWindowMenus") {
			t.Fatalf("%s missing setWindowMenus integration", path)
		}
	}
}

func TestDesktopWindowMenuTranslations(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.menu_file",
		"desktop.menu_edit",
		"desktop.menu_view",
		"desktop.menu_insert",
		"desktop.menu_run",
		"desktop.menu_playback",
		"desktop.menu_window",
		"desktop.menu_help",
		"desktop.menu_minimize_window",
		"desktop.menu_maximize_window",
		"desktop.menu_restore_window",
		"desktop.menu_close_window",
		"desktop.menu_agent_panel",
		"desktop.menu_terminal",
		"desktop.menu_play_pause",
		"desktop.menu_mute",
		"desktop.menu_favorite",
		"desktop.menu_reopen_player",
		"desktop.menu_load_folder",
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}
}
