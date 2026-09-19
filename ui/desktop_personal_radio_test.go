package ui

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPersonalRadioTranslations(t *testing.T) {
	var english map[string]string
	raw, err := Content.ReadFile("lang/desktop/en.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &english); err != nil {
		t.Fatal(err)
	}
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	for _, locale := range locales {
		t.Run(locale, func(t *testing.T) {
			raw, err := Content.ReadFile("lang/desktop/" + locale + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var words map[string]string
			if err = json.Unmarshal(raw, &words); err != nil {
				t.Fatal(err)
			}
			for key := range english {
				if strings.HasPrefix(key, "personalRadio.") || key == "desktop.app_personal_radio" {
					if strings.TrimSpace(words[key]) == "" || strings.ContainsRune(words[key], '\ufffd') {
						t.Errorf("missing/damaged translation %s", key)
					}
				}
			}
		})
	}
	literal := regexp.MustCompile(`\bt\('([a-z_]+)'\)`)
	for _, file := range []string{"personal-radio.js", "personal-radio-settings.js"} {
		raw, err := Content.ReadFile(filepath.ToSlash(filepath.Join("js/desktop/apps", file)))
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range literal.FindAllStringSubmatch(string(raw), -1) {
			if english["personalRadio."+match[1]] == "" {
				t.Errorf("unregistered string %s in %s", match[1], file)
			}
		}
	}
}
