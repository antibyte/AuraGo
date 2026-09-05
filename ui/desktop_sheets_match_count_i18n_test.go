package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopSheetsMatchCountI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/sheets-search.js")
	for _, want := range []string{
		"t('desktop.sheets_match_count', { current: 1, total: state.matches.length })",
		"t('desktop.sheets_match_count', { current: state.current + 1, total: state.matches.length })",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("sheets match-count i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"'1 of '",
		"+ ' of '",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("sheets still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.sheets_match_count"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.sheets_match_count", path)
		}
		if !strings.Contains(got, "{{current}}") || !strings.Contains(got, "{{total}}") {
			t.Fatalf("%s desktop.sheets_match_count must keep {{current}} and {{total}}", path)
		}
		if lang == "de" && got == "{{current}} of {{total}}" {
			t.Fatalf("%s must not copy the English match-count string", path)
		}
	}
}
