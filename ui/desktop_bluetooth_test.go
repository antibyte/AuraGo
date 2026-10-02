package ui

import (
	"strings"
	"testing"
)

func TestBluetoothTranslations(t *testing.T) {
	english := readMergedLangMap(t, "en")
	count := 0
	for key := range english {
		if strings.HasPrefix(key, "bluetooth.") {
			count++
		}
	}
	if count < 60 {
		t.Fatalf("english has %d bluetooth.* keys, want the full app set", count)
	}
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		t.Run(locale, func(t *testing.T) {
			words := readMergedLangMap(t, locale)
			for key := range english {
				if !strings.HasPrefix(key, "bluetooth.") && key != "desktop.app_bluetooth" {
					continue
				}
				if strings.TrimSpace(words[key]) == "" || strings.ContainsRune(words[key], '�') {
					t.Errorf("missing or damaged translation %s", key)
				}
				for _, placeholder := range []string{"{name}", "{count}", "{seconds}", "{time}", "{minutes}", "{percent}", "{service}"} {
					if strings.Contains(english[key], placeholder) && !strings.Contains(words[key], placeholder) {
						t.Errorf("%s lost placeholder %s", key, placeholder)
					}
				}
			}
		})
	}
}
