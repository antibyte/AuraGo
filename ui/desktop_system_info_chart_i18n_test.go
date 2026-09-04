package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopSystemInfoChartI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/system-info.js")
	for _, want := range []string{
		"function createHistoryChart(canvas, context)",
		"t(context, 'desktop.system_info_cpu')",
		"t(context, 'desktop.system_info_memory')",
		"t(context, 'desktop.system_info_disk')",
		"createHistoryChart(host.querySelector('[data-role=\"history\"]'), context)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("system-info chart i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"label: 'CPU'",
		"label: 'Memory'",
		"label: 'Disk'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("system-info chart still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{
			"desktop.system_info_cpu",
			"desktop.system_info_memory",
			"desktop.system_info_disk",
		} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
		}
	}
}
