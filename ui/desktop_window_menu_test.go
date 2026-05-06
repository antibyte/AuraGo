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

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
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
		text := readDesktopAssetText(t, path)
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
		"codeStudio.zoomIn",
		"codeStudio.zoomOut",
		"codeStudio.zoomReset",
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

func TestDesktopWindowMenuSelectiveMigration(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	fileManagerText := readDesktopAssetText(t, filepath.Join("js", "desktop", "file-manager.js"))
	sheetsText := readDesktopAssetText(t, filepath.Join("js", "desktop", "apps", "sheets.js"))

	fileToolbar := jsFunctionBodyInWindowMenuTest(t, fileManagerText, "function renderToolbarHtml()")
	for _, movedAction := range []string{
		`data-action="sort-menu"`,
		`data-action="refresh"`,
		`data-action="upload"`,
		`data-action="new-file"`,
		`data-action="new-folder"`,
	} {
		if strings.Contains(fileToolbar, movedAction) {
			t.Fatalf("file manager toolbar still contains menu-migrated action %s", movedAction)
		}
	}
	for _, retainedAction := range []string{
		`data-action="back"`,
		`data-action="forward"`,
		`data-action="up"`,
		`data-action="search-toggle"`,
		`data-action="view-grid"`,
		`data-action="view-list"`,
	} {
		if !strings.Contains(fileToolbar, retainedAction) {
			t.Fatalf("file manager toolbar lost direct UX action %s", retainedAction)
		}
	}

	sheetsMarkup := jsFunctionBodyInWindowMenuTest(t, sheetsText, "function render(host, windowId, context)")
	for _, movedAction := range []string{
		`data-action="save"`,
		`data-action="download"`,
		`data-action="export-csv"`,
		`data-action="add-row"`,
		`data-action="add-col"`,
	} {
		if strings.Contains(sheetsMarkup, movedAction) {
			t.Fatalf("sheets primary toolbar still contains menu-migrated action %s", movedAction)
		}
	}
	if !strings.Contains(sheetsMarkup, `data-action="apply-formula"`) {
		t.Fatalf("sheets lost direct formula apply action")
	}

	for _, check := range []struct {
		name     string
		body     string
		removed  []string
		retained []string
	}{
		{
			name:     "calendar",
			body:     jsFunctionBodyInWindowMenuTest(t, mainText, "async function renderCalendar(id)"),
			removed:  []string{`data-cal-new`},
			retained: []string{`data-cal-today`, `data-cal-nav`, `data-cal-view`},
		},
		{
			name:     "gallery",
			body:     jsFunctionBodyInWindowMenuTest(t, mainText, "async function renderGallery(id)"),
			removed:  []string{`data-gallery-refresh`},
			retained: []string{`data-gallery-tab`, `data-gallery-more`},
		},
		{
			name:     "quick connect",
			body:     jsFunctionBodyInWindowMenuTest(t, mainText, "function renderQuickConnect(id)"),
			removed:  []string{`data-action="add"`, `data-action="refresh"`},
			retained: []string{`data-device-list`, `data-terminal-area`},
		},
		{
			name:     "music player",
			body:     jsFunctionBodyInWindowMenuTest(t, mainText, "async function renderMusicPlayer(id)"),
			removed:  []string{`vd-webamp-launcher-actions`, `data-action="refresh-music"`, `data-action="load-folder"`, `data-action="reopen-webamp"`},
			retained: []string{`data-track-count`, `data-folder`},
		},
		{
			name:     "launchpad",
			body:     jsFunctionBodyInWindowMenuTest(t, mainText, "function renderLaunchpad(id)"),
			removed:  []string{`data-action="add"`},
			retained: []string{`vd-launchpad-search`, `vd-launchpad-category`},
		},
	} {
		for _, marker := range check.removed {
			if strings.Contains(check.body, marker) {
				t.Fatalf("%s still contains menu-migrated action %s", check.name, marker)
			}
		}
		for _, marker := range check.retained {
			if !strings.Contains(check.body, marker) {
				t.Fatalf("%s lost retained direct UX marker %s", check.name, marker)
			}
		}
	}
}

func TestCodeStudioWindowMenuEditorZoom(t *testing.T) {
	t.Parallel()

	codeStudioText := readDesktopAssetText(t, filepath.Join("js", "desktop", "apps", "code-studio.js"))
	codeStudioCSS := readDesktopAssetText(t, filepath.Join("css", "code-studio.css"))
	mainText := readDesktopAssetText(t, "js/desktop/main.js")

	menuBody := jsFunctionBodyInWindowMenuTest(t, codeStudioText, "function renderWindowMenus()")
	for _, want := range []string{
		`id: 'zoom-in'`,
		`id: 'zoom-out'`,
		`id: 'zoom-reset'`,
		`codeStudio.zoomIn`,
		`codeStudio.zoomOut`,
		`codeStudio.zoomReset`,
		`shortcut: 'Ctrl+='`,
		`shortcut: 'Ctrl+-'`,
		`shortcut: 'Ctrl+0'`,
	} {
		if !strings.Contains(menuBody, want) {
			t.Fatalf("Code Studio view menu missing editor zoom marker %q", want)
		}
	}

	for _, want := range []string{
		"editorFontSize:",
		"saved.editorFontSize",
		"editorFontSize: state.editorFontSize",
		"function adjustEditorZoom(",
		"function resetEditorZoom()",
		"--cs-editor-font-size",
	} {
		if !strings.Contains(codeStudioText, want) {
			t.Fatalf("Code Studio editor zoom implementation missing marker %q", want)
		}
	}
	if !strings.Contains(codeStudioCSS, "font-size: var(--cs-editor-font-size") {
		t.Fatalf("Code Studio CSS does not bind editor font size to --cs-editor-font-size")
	}
	for _, shortcut := range []string{"ctrl+=", "ctrl+-", "ctrl+0"} {
		if !strings.Contains(mainText, shortcut) {
			t.Fatalf("desktop menu shortcut router does not allow editor zoom shortcut %q", shortcut)
		}
	}
}

func readDesktopAssetText(t *testing.T, path string) string {
	t.Helper()
	data, err := Content.ReadFile(filepath.ToSlash(path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(data)
	if strings.Contains(text, "loadScriptParts(") {
		var combined strings.Builder
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "'/js/desktop/") {
				continue
			}
			part := strings.Trim(line, "',")
			if idx := strings.Index(part, "?"); idx >= 0 {
				part = part[:idx]
			}
			part = strings.TrimPrefix(part, "/")
			partData, err := Content.ReadFile(filepath.ToSlash(part))
			if err != nil {
				t.Fatalf("read %s referenced by %s: %v", part, path, err)
			}
			combined.Write(partData)
			combined.WriteByte('\n')
		}
		if combined.Len() > 0 {
			return combined.String()
		}
	}
	return text
}

func jsFunctionBodyInWindowMenuTest(t *testing.T, source, signature string) string {
	t.Helper()
	start := strings.Index(source, signature)
	if start < 0 {
		t.Fatalf("missing JS function %q", signature)
	}
	open := strings.Index(source[start:], "{")
	if open < 0 {
		t.Fatalf("missing opening brace for %q", signature)
	}
	pos := start + open
	depth := 0
	for i := pos; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[pos : i+1]
			}
		}
	}
	t.Fatalf("missing closing brace for %q", signature)
	return ""
}
