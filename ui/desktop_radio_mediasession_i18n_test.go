package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopRadioMediaSessionI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/radio.js")
	for _, want := range []string{
		"updateMediaSession(station, t)",
		"function updateMediaSession(station, t)",
		"translate('desktop.app_radio')",
		"translate('desktop.radio_album')",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("radio mediaSession i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"|| 'Radio'",
		"'AuraGo Radio'",
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
		for _, key := range []string{"desktop.app_radio", "desktop.radio_album"} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
		if lang == "fr" && values["desktop.radio_album"] == "AuraGo Radio" {
			t.Fatalf("%s must not copy the English radio album string", path)
		}
		if lang == "ja" && values["desktop.app_radio"] == "Radio" {
			t.Fatalf("%s must not copy the English radio app name", path)
		}
	}
}
