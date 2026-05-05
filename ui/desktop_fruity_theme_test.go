package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopFruityThemeSettingAssets(t *testing.T) {
	t.Parallel()

	shell, err := Content.ReadFile("js/desktop/main.js")
	if err != nil {
		t.Fatalf("desktop shell missing from embedded UI: %v", err)
	}
	shellText := string(shell)
	for _, want := range []string{
		"'appearance.theme': 'standard'",
		"body.dataset.theme = settingValue('appearance.theme')",
		"settingSelect('appearance.theme'",
		"desktop.settings_theme_standard",
		"desktop.settings_theme_fruity",
	} {
		if !strings.Contains(shellText, want) {
			t.Fatalf("desktop shell is missing Fruity theme setting marker %q", want)
		}
	}

	css, err := Content.ReadFile("css/desktop.css")
	if err != nil {
		t.Fatalf("desktop stylesheet missing from embedded UI: %v", err)
	}
	cssText := string(css)
	for _, want := range []string{
		".desktop-body[data-theme=\"fruity\"]",
		"@media (prefers-color-scheme: dark)",
		".desktop-body[data-theme=\"fruity\"] .vd-window",
		".desktop-body[data-theme=\"fruity\"] .vd-window-titlebar",
		".desktop-body[data-theme=\"fruity\"] .vd-window-actions",
		".desktop-body[data-theme=\"fruity\"] .vd-window-button[data-action=\"close\"]",
		".desktop-body[data-theme=\"fruity\"] .vd-window-button[data-action=\"minimize\"]",
		".desktop-body[data-theme=\"fruity\"] .vd-window-button[data-action=\"maximize\"]",
		".desktop-body[data-theme=\"fruity\"] .vd-window-button::after",
		"--fruity-window-close",
		".desktop-body[data-theme=\"fruity\"] .vd-taskbar",
		".desktop-body[data-theme=\"fruity\"] .vd-modal",
	} {
		if !strings.Contains(cssText, want) {
			t.Fatalf("desktop stylesheet is missing Fruity theme marker %q", want)
		}
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
			"desktop.settings_theme",
			"desktop.settings_theme_desc",
			"desktop.settings_theme_standard",
			"desktop.settings_theme_fruity",
		} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}
}
