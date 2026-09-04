package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopMissionControlMenuI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/mission-control.js")
	for _, want := range []string{
		"id: 'file', labelKey: 'desktop.menu_file'",
		"id: 'view', labelKey: 'desktop.menu_view'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("mission-control menu i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"label: 'File'",
		"label: 'View'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("mission-control still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"desktop.menu_file", "desktop.menu_view"} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
		}
		if lang == "de" {
			if values["desktop.menu_file"] == "File" {
				t.Fatalf("%s must not copy the English file menu label", path)
			}
			if values["desktop.menu_view"] == "View" {
				t.Fatalf("%s must not copy the English view menu label", path)
			}
		}
	}
}
