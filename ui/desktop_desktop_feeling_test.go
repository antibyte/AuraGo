package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopSessionRestoreSettingsAndRuntime(t *testing.T) {
	t.Parallel()

	foundRestore := false
	foundDockPins := false
	foundSessionWindows := false
	foundDefaultApps := false
	foundTerminal := false
	foundNotes := false
	for _, setting := range desktop.DesktopSettingDefinitions() {
		switch setting.Key {
		case "windows.restore_session":
			foundRestore = true
		case "appearance.dock_pins":
			foundDockPins = true
		case "session.windows":
			foundSessionWindows = true
		case "files.default_apps":
			foundDefaultApps = true
		}
	}
	if !foundRestore || !foundDockPins || !foundSessionWindows || !foundDefaultApps {
		t.Fatalf("desktop settings missing session restore keys")
	}
	for _, app := range desktop.BuiltinApps() {
		switch app.ID {
		case "terminal":
			foundTerminal = true
		case "notes":
			foundNotes = true
		}
	}
	if !foundTerminal || !foundNotes {
		t.Fatalf("builtin apps must include terminal and notes")
	}

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	if !strings.Contains(foundation, `'windows.restore_session'`) || !strings.Contains(foundation, "function dockApps()") {
		t.Fatalf("desktop foundation must define restore_session defaults and dockApps")
	}

	session := readDesktopAssetText(t, "js/desktop/core/session-runtime.js")
	for _, want := range []string{
		"function restoreDesktopSession()",
		"function scheduleSessionPersist()",
		"function dockPinIds()",
		"function recordRecentFile(",
		"SESSION_SKIP_APP_IDS",
		"version: 2",
		"activeSpaceId",
		"spaceId",
	} {
		if !strings.Contains(session, want) {
			t.Fatalf("session runtime missing %q", want)
		}
	}

	spaces := readDesktopAssetText(t, "js/desktop/core/spaces-runtime.js")
	for _, want := range []string{
		"function switchSpace(",
		"function applySpaceVisibility(",
		"SPACE_IDS = ['1', '2', '3']",
		"function renderSpacePager(",
		"function handleSpaceShortcut(",
	} {
		if !strings.Contains(spaces, want) {
			t.Fatalf("spaces runtime missing %q", want)
		}
	}

	thumbs := readDesktopAssetText(t, "js/desktop/core/taskbar-thumbnails-runtime.js")
	for _, want := range []string{
		"function wireTaskbarThumbnailHover(",
		"function hideTaskbarThumbnail(",
		"function taskbarThumbnailsEnabled(",
		"function windowPreviewMarkup(",
	} {
		if !strings.Contains(thumbs, want) {
			t.Fatalf("taskbar thumbnails runtime missing %q", want)
		}
	}

	mediaKeys := readDesktopAssetText(t, "js/desktop/core/media-keys-runtime.js")
	for _, want := range []string{
		"function dispatchWebampMediaAction(",
		"function handleDesktopMediaKeydown(",
		"notifyWebampMediaSessionChanged",
	} {
		if !strings.Contains(mediaKeys, want) {
			t.Fatalf("media keys runtime missing %q", want)
		}
	}

	overview := readDesktopAssetText(t, "js/desktop/core/spaces-overview-runtime.js")
	for _, want := range []string{
		"function openSpacesOverview(",
		"'vd-spaces-overview'",
		".vd-space-column",
		"function handleSpacesOverviewShortcut(",
		"function spacesOverviewEnabled(",
	} {
		if !strings.Contains(overview, want) {
			t.Fatalf("spaces overview runtime missing %q", want)
		}
	}

	interactions := readDesktopAssetText(t, "js/desktop/core/window-interactions-runtime.js")
	if !strings.Contains(interactions, "function toggleShowDesktop(") {
		t.Fatalf("window interactions must expose toggleShowDesktop")
	}

	bootstrap := readDesktopAssetText(t, "js/desktop/core/sdk-events-bootstrap.js")
	if !strings.Contains(bootstrap, "restoreDesktopSession()") {
		t.Fatalf("bootstrap must restore session after init")
	}
}

func TestDesktopShellChromeAndSpotlight(t *testing.T) {
	t.Parallel()

	shell := readDesktopAssetText(t, "js/desktop/core/shell-chrome-runtime.js")
	for _, want := range []string{
		"function pushNotificationRecord(",
		"function openClockPopup(",
		"function showShortcutsHelp(",
		"function beginWindowSwitcherHold(",
		"function handleWindowSwitcherKeydown(",
		"wireShellChromeControls()",
	} {
		if !strings.Contains(shell, want) {
			t.Fatalf("shell chrome runtime missing %q", want)
		}
	}

	spotlight := readDesktopAssetText(t, "js/desktop/core/spotlight-runtime.js")
	for _, want := range []string{
		"function openSpotlight()",
		"/api/desktop/search?query=",
		"spotlightRecentFileEntries",
	} {
		if !strings.Contains(spotlight, want) {
			t.Fatalf("spotlight runtime missing %q", want)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-chrome.css")
	for _, want := range []string{
		".vd-spotlight-backdrop",
		".vd-notification-center",
		".vd-window-switcher",
		".vd-space-pager",
		".vd-spaces-overview",
		".vd-space-column",
		"vd-space-hidden",
		".vd-taskbar-thumbnail",
		"--vd-theme-panel-bg-strong",
		".vd-terminal-toolbar",
		".vd-notes-toolbar",
		"background: var(--vd-theme-chrome-bg);",
		"background: #0f172a;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop chrome css missing %q", want)
		}
	}

	overrides := readDesktopAssetText(t, "css/desktop-shell-overrides.css")
	for _, want := range []string{
		".vd-space-pager-btn",
		".vd-notification-center",
	} {
		if !strings.Contains(overrides, want) {
			t.Fatalf("desktop shell overrides missing spaces/chrome bridge %q", want)
		}
	}

	html := readDesktopAssetText(t, "desktop.html")
	if !strings.Contains(html, `id="vd-notification-button"`) {
		t.Fatalf("desktop.html must expose notification center button")
	}
}

func TestDesktopFeelingBundlesIncludeChromeModules(t *testing.T) {
	t.Parallel()

	buildScriptBytes, err := os.ReadFile(filepath.Join("..", "scripts", "build-ui-bundles.js"))
	if err != nil {
		t.Fatalf("read build script: %v", err)
	}
	buildScript := string(buildScriptBytes)
	for _, want := range []string{
		"session-runtime.js",
		"spaces-runtime.js",
		"spaces-overview-runtime.js",
		"taskbar-thumbnails-runtime.js",
		"media-keys-runtime.js",
		"shell-chrome-runtime.js",
		"spotlight-runtime.js",
		"desktop-chrome.css",
	} {
		if !strings.Contains(buildScript, want) {
			t.Fatalf("build-ui-bundles.js must include %q", want)
		}
	}
}

func TestDesktopTerminalAndNotesApps(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	for _, want := range []string{
		"'terminal':",
		"'notes':",
		"/js/desktop/apps/terminal.js",
		"/js/desktop/apps/notes.js",
	} {
		if !strings.Contains(loader, want) {
			t.Fatalf("module loader missing %q", want)
		}
	}

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"appId === 'terminal'",
		"appId === 'notes'",
		"toggleDockPin",
		"setDefaultAppForExtension",
		"desktop.snap_left",
		"openSpacesOverviewForWindow",
	} {
		if !strings.Contains(routing, want) {
			t.Fatalf("menus-and-routing missing %q", want)
		}
	}

	writer := readDesktopAssetText(t, "js/desktop/apps/writer.js")
	sheets := readDesktopAssetText(t, "js/desktop/apps/sheets.js")
	if !strings.Contains(writer, "function printDocument()") {
		t.Fatalf("writer must support print")
	}
	if !strings.Contains(sheets, "function printWorkbook()") {
		t.Fatalf("sheets must support print")
	}
}
