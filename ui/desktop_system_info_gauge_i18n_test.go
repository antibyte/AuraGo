package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopSystemInfoGaugeI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/system-info.js")
	for _, want := range []string{
		"function createGauge(canvas, color, context)",
		"t(context, 'desktop.system_info_used')",
		"t(context, 'desktop.system_info_free')",
		"createGauge(host.querySelector('[data-gauge=\"cpu\"]'), accent, context)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("system-info gauge i18n missing marker %q", want)
		}
	}
	if strings.Contains(source, "dashboard.gauge_") {
		t.Fatal("system-info must not use dashboard gauge keys")
	}
	if strings.Contains(source, "labels: ['Used', 'Free']") {
		t.Fatal("system-info still hardcodes Used/Free gauge labels")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"desktop.system_info_used", "desktop.system_info_free"} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
		if lang == "de" && values["desktop.system_info_used"] == "Used" {
			t.Fatalf("%s must not copy the English used label", path)
		}
		if lang == "fr" && values["desktop.system_info_free"] == "Free" {
			t.Fatalf("%s must not copy the English free label", path)
		}
	}
}
