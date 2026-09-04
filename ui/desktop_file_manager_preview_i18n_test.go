package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopFileManagerPreviewI18n(t *testing.T) {
	t.Parallel()

	preview := readDesktopAssetText(t, "js/desktop/file-manager/preview-panel.js")
	quickLook := readDesktopAssetText(t, "js/desktop/file-manager/actions-input.js")
	for _, want := range []string{
		"t('desktop.fm.preview_unavailable')",
	} {
		if !strings.Contains(preview, want) {
			t.Fatalf("file manager preview i18n missing marker %q", want)
		}
	}
	for _, want := range []string{
		"t('desktop.fm.preview_loading')",
		"t('desktop.fm.quick_look_close')",
		"t('desktop.fm.quick_look_error')",
	} {
		if !strings.Contains(quickLook, want) {
			t.Fatalf("file manager quick look i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"Cannot display preview",
		"Loading preview...",
		"Close (Space)",
		"Cannot load preview.",
	} {
		if strings.Contains(preview, forbidden) || strings.Contains(quickLook, forbidden) {
			t.Fatalf("file manager still hardcodes %q", forbidden)
		}
	}

	required := []string{
		"desktop.fm.preview_loading",
		"desktop.fm.preview_unavailable",
		"desktop.fm.quick_look_close",
		"desktop.fm.quick_look_error",
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
		}
		if lang == "de" {
			if values["desktop.fm.preview_unavailable"] == "Cannot display preview" {
				t.Fatalf("%s must not copy the English preview error", path)
			}
			if values["desktop.fm.quick_look_close"] == "Close (Space)" {
				t.Fatalf("%s must not copy the English quick look close label", path)
			}
		}
	}
}
