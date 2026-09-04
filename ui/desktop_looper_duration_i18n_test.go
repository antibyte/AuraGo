package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopLooperDurationI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/looper.js")
	for _, want := range []string{
		"t('desktop.looper_duration_ms', { count: 0 })",
		"t('desktop.looper_duration_ms', { count: value })",
		"t('desktop.looper_duration_s', { count: (value / 1000).toFixed(1) })",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("looper duration i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"return '0ms'",
		"+ 'ms'",
		"+ 's'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("looper still hardcodes %q", forbidden)
		}
	}

	sip := readDesktopAssetText(t, "js/desktop/apps/sip-phone.js")
	if strings.Contains(sip, "desktop.looper_duration_") {
		t.Fatal("SIP must not use looper duration keys")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"desktop.looper_duration_ms", "desktop.looper_duration_s"} {
			got := values[key]
			if strings.TrimSpace(got) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
			if !strings.Contains(got, "{{count}}") {
				t.Fatalf("%s %s must keep {{count}}", path, key)
			}
		}
		if lang == "de" {
			if values["desktop.looper_duration_ms"] == "{{count}}ms" {
				t.Fatalf("%s must not copy a spaceless English millisecond unit", path)
			}
		}
	}
}
