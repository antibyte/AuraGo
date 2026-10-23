package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// readDesktopLocaleValues parses one Desktop locale bundle.
func readDesktopLocaleValues(t *testing.T, lang string) map[string]string {
	t.Helper()

	path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
	var values map[string]string
	if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return values
}

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

	// The local webhost name fallback must not keep the English word.
	english := readDesktopLocaleValues(t, "en")
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		values := readDesktopLocaleValues(t, lang)
		for _, key := range []string{
			"homepage_studio.default_name",
			"desktop.chat_request_failed",
		} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
		if lang == "de" && values["desktop.chat_request_failed"] == "Request failed" {
			t.Fatalf("%s must not copy the English request-failed string", path)
		}
		switch lang {
		case "de", "fr", "it", "sv":
			if values["homepage_studio.default_name"] == english["homepage_studio.default_name"] {
				t.Fatalf("%s must not copy the English homepage default name", path)
			}
		}
	}
}

// TestDesktopHomepageStudioFallbackI18nLocaleKeysExist guards the Homepage
// Studio strings the studio renders: every locale carries them, and the
// placeholder based ones keep their placeholder.
func TestDesktopHomepageStudioFallbackI18nLocaleKeysExist(t *testing.T) {
	t.Parallel()

	keys := []string{
		"homepage_studio.agent_finished",
		"homepage_studio.copy_path",
		"homepage_studio.readonly_placeholder",
		"homepage_studio.resize_assistant",
		"homepage_studio.resize_inspector",
		"homepage_studio.target_unreachable",
		"homepage_studio.history_delete_entry",
		"homepage_studio.device_width_value",
	}
	placeholders := map[string][]string{
		"homepage_studio.history_delete_entry": {"{{date}}"},
		"homepage_studio.device_width_value":   {"{{width}}"},
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		values := readDesktopLocaleValues(t, lang)
		for _, key := range keys {
			value := values[key]
			if strings.TrimSpace(value) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
			for _, placeholder := range placeholders[key] {
				if !strings.Contains(value, placeholder) {
					t.Fatalf("%s drops %s from %s: %q", path, placeholder, key, value)
				}
			}
		}
	}
}
