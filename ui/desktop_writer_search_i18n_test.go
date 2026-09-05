package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopWriterSearchI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/writer.js")
	for _, want := range []string{
		"t('desktop.writer_match_count'",
		"current: searchState.currentMatch + 1",
		"total: searchState.matches.length",
		`title="${esc(t('desktop.close'))}"`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("writer search i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"+ '/' + searchState.matches.length",
		`title="Esc"`,
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("writer still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.writer_match_count"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.writer_match_count", path)
		}
		if !strings.Contains(got, "{{current}}") || !strings.Contains(got, "{{total}}") {
			t.Fatalf("%s desktop.writer_match_count must keep {{current}} and {{total}}", path)
		}
		if lang == "de" && got == "{{current}} of {{total}}" {
			t.Fatalf("%s must not copy the English writer match-count string", path)
		}
		if strings.TrimSpace(values["desktop.close"]) == "" {
			t.Fatalf("%s missing non-empty desktop.close", path)
		}
	}
}
