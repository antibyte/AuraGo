package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopZipperFilterI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/zipper.js")
	if !strings.Contains(source, "label: t('desktop.file_dialog_zip')") {
		t.Fatal("zipper open dialog must localize desktop.file_dialog_zip")
	}
	if strings.Contains(source, "ZIP Archives") {
		t.Fatal("zipper open dialog still hardcodes ZIP Archives")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.file_dialog_zip"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.file_dialog_zip", path)
		}
		if lang == "de" && got == "ZIP Archives" {
			t.Fatalf("%s must not copy the English ZIP filter label", path)
		}
		if lang == "fr" && got == "ZIP Archives" {
			t.Fatalf("%s must not copy the English ZIP filter label", path)
		}
	}
}
