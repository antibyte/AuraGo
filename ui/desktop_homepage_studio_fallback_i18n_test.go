package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopHomepageStudioFallbackI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"item.name || t('homepage_studio.default_name')",
		"err || t('desktop.chat_request_failed')",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio fallback i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"|| 'Homepage'",
		"|| 'Request failed'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("homepage studio still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{
			"homepage_studio.default_name",
			"desktop.chat_request_failed",
		} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
		if lang == "fr" && values["homepage_studio.default_name"] == "Homepage" {
			t.Fatalf("%s must not copy the English homepage default name", path)
		}
		if lang == "de" && values["desktop.chat_request_failed"] == "Request failed" {
			t.Fatalf("%s must not copy the English request-failed string", path)
		}
	}
}
