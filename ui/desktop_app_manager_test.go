package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopAppManagerAssets(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"function showAppManager()",
		"desktop.app_manager",
		"setAppVisibility(",
		"startMenuApps()",
		"dockApps()",
		`data-action="hide-dock"`,
		`data-action="show-start"`,
		"vd-app-manager",
		"vd-app-manager-backdrop",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop app manager implementation missing marker %q", want)
		}
	}

	css := readDesktopAssetText(t, "css/desktop.css")
	for _, want := range []string{
		".vd-app-manager",
		".vd-wm-badge-user",
		".vd-wm-badge-start",
		".vd-wm-badge-dock",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop app manager CSS missing marker %q", want)
		}
	}
}

func TestDesktopAppManagerTranslations(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.app_manager",
		"desktop.app_system",
		"desktop.app_generated",
		"desktop.app_in_dock",
		"desktop.app_hidden_from_dock",
		"desktop.app_in_start",
		"desktop.app_hidden_from_start",
		"desktop.app_add_to_dock",
		"desktop.app_remove_from_dock",
		"desktop.app_add_to_start",
		"desktop.app_remove_from_start",
		"desktop.app_delete_permanent",
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

func TestDesktopAppManagerAssetVersionsBustCache(t *testing.T) {
	t.Parallel()

	desktopHTML := readDesktopAssetText(t, "desktop.html")
	for _, want := range []string{
		`/css/desktop.css?v=28`,
		`/js/desktop/main.js?v=33`,
		`/js/desktop/apps/looper.js?v=3`,
	} {
		if !strings.Contains(desktopHTML, want) {
			t.Fatalf("desktop.html missing cache-busting asset version %q", want)
		}
	}

	mainBytes, err := Content.ReadFile("js/desktop/main.js")
	if err != nil {
		t.Fatalf("read main.js: %v", err)
	}
	mainJS := string(mainBytes)
	for _, want := range []string{
		`/js/desktop/core/desktop-foundation.js?v=9`,
		`/js/desktop/core/widget-autosize-runtime.js?v=2`,
		`/js/desktop/core/window-shell-runtime.js?v=5`,
		`/js/desktop/core/menus-and-routing.js?v=3`,
		`/js/desktop/core/shortcut-runtime.js?v=1`,
	} {
		if !strings.Contains(mainJS, want) {
			t.Fatalf("desktop main loader missing cache-busting part version %q", want)
		}
	}
}

func TestDesktopWindowRuntimeContainsFruityDockScrollHelper(t *testing.T) {
	t.Parallel()

	runtimeJS := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, want := range []string{
		"function wireFruityDockScroll(host)",
		"function updateFruityDockScrollControls(host)",
		"wireFruityDockScroll(host)",
	} {
		if !strings.Contains(runtimeJS, want) {
			t.Fatalf("window shell runtime missing fruity dock helper marker %q", want)
		}
	}
	mainJS := readDesktopAssetText(t, "js/desktop/main.js")
	if strings.Contains(mainJS, "/js/desktop/core/fruity-dock-scroll.js") {
		t.Fatal("fruity dock helper must not be loaded as a separate main chunk because desktop chunks are split across function bodies")
	}
}
