package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopPeopleKgI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/people.js")
	for _, want := range []string{
		`">${esc(t(context, 'desktop.people_kg'))}</button>`,
		"const kgLabel = t(context, 'desktop.people_kg')",
		"kgLabel + ' ' + t(context, 'desktop.people_semantic_search')",
		"esc(t(inst.context, 'desktop.people_kg'))",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("people KG i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		`">KG</button>`,
		"'KG ' +",
		`: 'KG'`,
		`badge-kg">KG</span>`,
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("people still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.people_kg"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.people_kg", path)
		}
		if lang == "fr" && got == "KG" {
			t.Fatalf("%s must not copy the English KG badge", path)
		}
	}
}
