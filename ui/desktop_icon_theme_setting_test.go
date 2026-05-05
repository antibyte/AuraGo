package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopIconThemeSettingAssets(t *testing.T) {
	t.Parallel()

	shell, err := Content.ReadFile("js/desktop/main.js")
	if err != nil {
		t.Fatalf("desktop shell missing from embedded UI: %v", err)
	}
	shellText := string(shell)
	for _, want := range []string{
		"'appearance.icon_theme': 'papirus'",
		"body.dataset.iconTheme = settingValue('appearance.icon_theme')",
		"settingSelect('appearance.icon_theme'",
		"desktop.settings_icon_theme_papirus",
		"desktop.settings_icon_theme_whitesur",
		"/img/whitesur/manifest.json",
		"iconThemeManifests",
		"function renderStartButtonIcon()",
		"renderStartButtonIcon();",
		"settingIconCatalog(",
		"function renderIconCatalogSetting(",
		"desktop.settings_icon_catalog_aliases",
	} {
		if !strings.Contains(shellText, want) {
			t.Fatalf("desktop shell is missing icon theme setting marker %q", want)
		}
	}
	if strings.Contains(shellText, "desktop.settings_icon_theme_aurago") || strings.Contains(shellText, "['aurago'") {
		t.Fatal("desktop shell must not expose the removed AuraGo Classic icon theme")
	}

	css, err := Content.ReadFile("css/desktop.css")
	if err != nil {
		t.Fatalf("desktop stylesheet missing from embedded UI: %v", err)
	}
	if !strings.Contains(string(css), ".vd-icon-catalog") {
		t.Fatalf("desktop stylesheet is missing icon catalog settings styles")
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
		for _, key := range []string{
			"desktop.settings_icon_theme",
			"desktop.settings_icon_theme_desc",
			"desktop.settings_icon_theme_papirus",
			"desktop.settings_icon_theme_whitesur",
			"desktop.settings_icon_catalog",
			"desktop.settings_icon_catalog_desc",
			"desktop.settings_icon_catalog_aliases",
			"desktop.settings_icon_catalog_empty",
		} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
		if _, ok := values["desktop.settings_icon_theme_aurago"]; ok {
			t.Fatalf("%s still exposes removed AuraGo Classic icon theme translation", path)
		}
	}
}
