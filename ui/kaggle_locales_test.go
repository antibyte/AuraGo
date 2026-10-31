package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestKaggleLocalesComplete(t *testing.T) {
	read := func(locale string) map[string]string {
		t.Helper()
		raw, err := Content.ReadFile("lang/kaggle/" + locale + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err = json.Unmarshal(raw, &values); err != nil {
			t.Fatal(err)
		}
		return values
	}
	base := read("en")
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		t.Run(locale, func(t *testing.T) {
			values := read(locale)
			if len(values) != len(base) {
				t.Fatalf("locale keys: %d, expected %d", len(values), len(base))
			}
			for key := range base {
				if strings.TrimSpace(values[key]) == "" {
					t.Error("missing", key)
				}
			}
			if locale != "en" {
				for _, key := range []string{"config.kaggle.policy_help", "config.kaggle.local_help", "kaggle.state_lost", "kaggle.cancel_on_site"} {
					if values[key] == base[key] {
						t.Error("untranslated help", key)
					}
				}
			}
		})
	}
}
