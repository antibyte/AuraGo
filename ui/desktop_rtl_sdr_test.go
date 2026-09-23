package ui

import (
	"strings"
	"testing"
)

func TestRTLSDRTranslations(t *testing.T) {
	english := readMergedLangMap(t, "en")
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		t.Run(locale, func(t *testing.T) {
			words := readMergedLangMap(t, locale)
			for key := range english {
				if !strings.HasPrefix(key, "rtlSdr.") && key != "desktop.app_rtl_sdr" {
					continue
				}
				if strings.TrimSpace(words[key]) == "" || strings.ContainsRune(words[key], '\ufffd') {
					t.Errorf("missing or damaged translation %s", key)
				}
			}
		})
	}
}
