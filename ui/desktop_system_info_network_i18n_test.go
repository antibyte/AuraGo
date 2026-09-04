package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopSystemInfoNetworkI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/system-info.js")
	if !strings.Contains(source, "t(instance.context, 'desktop.system_info_network_io'") {
		t.Fatal("system-info network totals must use desktop.system_info_network_io")
	}
	if strings.Contains(source, " up / ") || strings.Contains(source, "} down`") {
		t.Fatal("system-info still hardcodes English up/down")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.system_info_network_io"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.system_info_network_io", path)
		}
		if !strings.Contains(got, "{{sent}}") || !strings.Contains(got, "{{recv}}") {
			t.Fatalf("%s network io must keep {{sent}} and {{recv}}", path)
		}
		if lang != "en" && got == "{{sent}} up / {{recv}} down" {
			t.Fatalf("%s must not copy the English network io string", path)
		}
	}
}
