package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopQcAuragoHostI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/planning-gallery-music.js")
	for _, want := range []string{
		"t('desktop.qc_aurago_host')",
		"t('desktop.qc_aurago_host_description')",
		"id === '__aurago-host__'",
		"name === 'aurago host'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("quick connect host i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"name: 'AuraGo Host'",
		"description: 'Current AuraGo web host'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("quick connect host still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"desktop.qc_aurago_host", "desktop.qc_aurago_host_description"} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
		if lang == "de" && values["desktop.qc_aurago_host"] == "AuraGo Host" {
			t.Fatalf("%s must not copy the English AuraGo host name", path)
		}
		if lang == "fr" && values["desktop.qc_aurago_host_description"] == "Current AuraGo web host" {
			t.Fatalf("%s must not copy the English AuraGo host description", path)
		}
	}
}
