package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopPixelSaveFilterI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/pixel-actions.js")
	for _, want := range []string{
		"label: this.t('desktop.file_dialog_png')",
		"label: this.t('desktop.file_dialog_jpeg')",
		"label: this.t('desktop.file_dialog_webp')",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("pixel save dialog missing %s", want)
		}
	}
	for _, forbidden := range []string{"PNG Image", "JPEG Image", "WebP Image"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("pixel save dialog still hardcodes %s", forbidden)
		}
	}

	keys := []string{"desktop.file_dialog_png", "desktop.file_dialog_jpeg", "desktop.file_dialog_webp"}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
		if lang == "de" && values["desktop.file_dialog_png"] == "PNG Image" {
			t.Fatalf("%s must not copy the English PNG filter label", path)
		}
		if lang == "zh" && values["desktop.file_dialog_jpeg"] == "JPEG Image" {
			t.Fatalf("%s must not copy the English JPEG filter label", path)
		}
	}
}
