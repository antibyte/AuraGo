package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopPixelOpenFilterI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/pixel-actions.js")
	if !strings.Contains(source, "label: this.t('desktop.file_dialog_images')") {
		t.Fatal("pixel open dialog must localize desktop.file_dialog_images")
	}
	if strings.Contains(source, "name: 'Images'") {
		t.Fatal("pixel open dialog still hardcodes Images")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.file_dialog_images"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.file_dialog_images", path)
		}
		if lang == "de" && got == "Images" {
			t.Fatalf("%s must not copy the English images filter label", path)
		}
		if lang == "zh" && got == "Images" {
			t.Fatalf("%s must not copy the English images filter label", path)
		}
	}
}
