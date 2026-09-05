package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopZipperStatusI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/zipper.js")
	for _, want := range []string{
		"t('desktop.bytes').replace('{{count}}'",
		"t('desktop.kib').replace('{{count}}'",
		"t('desktop.mib').replace('{{count}}'",
		"t('desktop.gib').replace('{{count}}'",
		"t('desktop.tib').replace('{{count}}'",
		"t('zipper.compressed_size').replace('{{size}}'",
		"t('zipper.selected').replace('{{count}}'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("zipper status i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"+ ' compressed'",
		" selected  ·  ",
		"+ ' KiB'",
		"+ ' MiB'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("zipper still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if !strings.Contains(values["zipper.compressed_size"], "{{size}}") {
			t.Fatalf("%s zipper.compressed_size must keep {{size}}", path)
		}
		if !strings.Contains(values["zipper.selected"], "{{count}}") {
			t.Fatalf("%s zipper.selected must keep {{count}}", path)
		}
		if lang == "de" {
			if values["zipper.compressed_size"] == "{{size}} compressed" {
				t.Fatalf("%s must not copy the English compressed size string", path)
			}
			if values["zipper.selected"] == "{{count}} selected" {
				t.Fatalf("%s must not copy the English selected string", path)
			}
		}
	}
}
