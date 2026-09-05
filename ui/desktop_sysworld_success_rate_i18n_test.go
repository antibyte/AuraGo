package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopSysworldSuccessRateI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/sysworld.js")
	if !strings.Contains(source, "k: L('sysworld.panel.success_rate')") {
		t.Fatal("sysworld mission panel must use sysworld.panel.success_rate")
	}
	if strings.Contains(source, "k: 'OK'") {
		t.Fatal("sysworld mission panel still hardcodes OK")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["sysworld.panel.success_rate"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty sysworld.panel.success_rate", path)
		}
		if lang == "de" && got == "OK" {
			t.Fatalf("%s must not keep the English OK label", path)
		}
		if lang == "fr" && got == "Success rate" {
			t.Fatalf("%s must not copy the English success-rate label", path)
		}
	}
}
