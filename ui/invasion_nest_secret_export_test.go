package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvasionNestSecretExportStringsAreTranslated(t *testing.T) {
	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{"invasion.export_nest_secret", "invasion.export_nest_secret_hint"}
	en, err := readJSONFileMap(filepath.Join("lang", "invasion", "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range langs {
		m, err := readJSONFileMap(filepath.Join("lang", "invasion", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range keys {
			v, _ := m[key].(string)
			if strings.TrimSpace(v) == "" {
				t.Fatalf("%s: %s missing", lang, key)
			}
			if lang != "en" && v == en[key] {
				t.Fatalf("%s: %s copies the English text", lang, key)
			}
		}
	}
	html, err := os.ReadFile("invasion_control.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{`id="nest-export-secret"`, `data-i18n="invasion.export_nest_secret"`, `data-i18n="invasion.export_nest_secret_hint"`} {
		if !strings.Contains(string(html), marker) {
			t.Fatalf("invasion_control.html lacks %s", marker)
		}
	}
}
