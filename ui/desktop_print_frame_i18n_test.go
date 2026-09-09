package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopPrintFrameI18n(t *testing.T) {
	t.Parallel()

	viewer := readDesktopAssetText(t, "js/desktop/apps/viewer.js")
	if strings.Count(viewer, "throw new Error(t('desktop.print_failed'))") < 2 {
		t.Fatal("viewer print helpers must throw desktop.print_failed")
	}
	if strings.Contains(viewer, "print frame unavailable") {
		t.Fatal("viewer still hardcodes print frame unavailable")
	}
	if !strings.Contains(viewer, "notify(t('viewer.error') + ': ' + err.message)") {
		t.Fatal("viewer print catch must keep the viewer.error prefix")
	}

	writer := readDesktopAssetText(t, "js/desktop/apps/writer.js")
	if !strings.Contains(writer, "notice(ctx.t('desktop.print_failed'),true)") {
		t.Fatal("writer print must notify desktop.print_failed")
	}
	if strings.Contains(writer, "print frame unavailable") {
		t.Fatal("writer still hardcodes print frame unavailable")
	}

	sheets := readDesktopAssetText(t, "js/desktop/apps/sheets.js")
	if !strings.Contains(sheets, "setStatus(t('desktop.print_failed'))") {
		t.Fatal("sheets print must setStatus desktop.print_failed")
	}
	if !strings.Contains(sheets, "notify({ type: 'error', message: t('desktop.print_failed') })") {
		t.Fatal("sheets print must notify desktop.print_failed")
	}
	if strings.Contains(sheets, "print frame unavailable") {
		t.Fatal("sheets still hardcodes print frame unavailable")
	}

	english := "Could not open the print preview."
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.print_failed"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.print_failed", path)
		}
		if (lang == "de" || lang == "fr") && got == english {
			t.Fatalf("%s must not copy the English print failed string", path)
		}
	}
}
