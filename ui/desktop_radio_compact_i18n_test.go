package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopRadioCompactI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/radio.js")
	for _, want := range []string{
		"function compactNumber(value, t)",
		"compactNumber(station.clickcount, t)",
		"'desktop.radio_compact_millions'",
		"'desktop.radio_compact_thousands'",
		"phrase.replaceAll('{{count}}', count)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("radio compact i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"+ 'M'",
		"+ 'K'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("radio still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"desktop.radio_compact_thousands", "desktop.radio_compact_millions"} {
			got := values[key]
			if strings.TrimSpace(got) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
			if !strings.Contains(got, "{{count}}") {
				t.Fatalf("%s %s must keep {{count}}", path, key)
			}
		}
		if lang == "de" {
			if values["desktop.radio_compact_thousands"] == "{{count}}K" {
				t.Fatalf("%s must not copy the English thousands compact unit", path)
			}
			if values["desktop.radio_compact_millions"] == "{{count}}M" {
				t.Fatalf("%s must not copy the English millions compact unit", path)
			}
		}
	}
}
