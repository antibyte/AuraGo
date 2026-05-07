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
