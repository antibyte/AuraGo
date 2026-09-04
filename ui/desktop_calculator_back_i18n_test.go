package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopCalculatorBackI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/calculator.js")
	if strings.Count(source, "{ key: 'backspace', label: t('desktop.calc_back') }") != 3 {
		t.Fatal("calculator must use desktop.calc_back for all three backspace keys")
	}
	if strings.Contains(source, "label: 'Back'") {
		t.Fatal("calculator still hardcodes English Back")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.calc_back"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.calc_back", path)
		}
		if lang != "en" && got == "Back" {
			t.Fatalf("%s must not copy the English calculator back label", path)
		}
	}
}
