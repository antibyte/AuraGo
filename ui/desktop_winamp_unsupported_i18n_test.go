package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopWinampUnsupportedI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/planning-gallery-music.js")
	if !strings.Contains(source, "t('desktop.winamp_unsupported')") {
		t.Fatal("webamp unsupported path must localize desktop.winamp_unsupported")
	}
	if strings.Contains(source, "throw new Error('Webamp is not supported in this browser.')") {
		t.Fatal("webamp still throws hardcoded English unsupported text")
	}
	if strings.Contains(source, "const message = err && err.message ? err.message : String(err);") {
		t.Fatal("webamp notifyError still dumps err.message")
	}
	if !strings.Contains(source, "Webamp is not supported in this browser.") {
		t.Fatal("webamp notifyError must still map the English unsupported sentinel")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.winamp_unsupported"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.winamp_unsupported", path)
		}
		if lang == "de" && got == "Webamp is not supported in this browser." {
			t.Fatalf("%s must not copy the English winamp unsupported string", path)
		}
		if lang == "fr" && got == "Webamp is not supported in this browser." {
			t.Fatalf("%s must not copy the English winamp unsupported string", path)
		}
	}
}
