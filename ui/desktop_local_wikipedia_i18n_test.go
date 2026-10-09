package ui

import (
	"strings"
	"testing"
)

func TestLocalWikipediaTranslations(t *testing.T) {
	english := readMergedLangMap(t, "en")
	var keys []string
	for key := range english {
		if strings.HasPrefix(key, "desktop.local_wikipedia_") {
			keys = append(keys, key)
		}
	}
	if len(keys) != 51 {
		t.Fatalf("english has %d desktop.local_wikipedia_* keys, want 51", len(keys))
	}
	keys = append(keys, "desktop.app_local_wikipedia")
	placeholders := []string{"{query}", "{date}", "{percent}", "{title}"}
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		t.Run(locale, func(t *testing.T) {
			words := readMergedLangMap(t, locale)
			for _, key := range keys {
				value := words[key]
				if strings.TrimSpace(value) == "" || strings.ContainsRune(value, '\uFFFD') {
					t.Errorf("missing or damaged %s", key)
				}
				for _, placeholder := range placeholders {
					if strings.Contains(english[key], placeholder) != strings.Contains(value, placeholder) {
						t.Errorf("%s placeholder %s differs from English: %q", key, placeholder, value)
					}
				}
				if locale != "en" && key != "desktop.app_local_wikipedia" && value == english[key] {
					t.Errorf("%s is untranslated English: %q", key, value)
				}
			}
		})
	}
	german := readMergedLangMap(t, "de")
	for _, key := range keys {
		for _, formal := range []string{" Sie ", " Ihre ", " Ihnen "} {
			if strings.Contains(" "+german[key]+" ", formal) {
				t.Errorf("German %s must use Du: %q", key, german[key])
			}
		}
	}
}
