package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopSysworldPanelIdI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/sysworld.js")
	if strings.Count(source, "k: L('sysworld.panel.id')") < 3 {
		t.Fatal("sysworld panel must localize all three ID rows")
	}
	if strings.Contains(source, "k: 'ID'") {
		t.Fatal("sysworld panel still hardcodes ID")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["sysworld.panel.id"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty sysworld.panel.id", path)
		}
		if lang == "de" && got == "ID" {
			t.Fatalf("%s must not copy the English ID label", path)
		}
		if lang == "zh" && got == "ID" {
			t.Fatalf("%s must not copy the English ID label", path)
		}
	}
}
