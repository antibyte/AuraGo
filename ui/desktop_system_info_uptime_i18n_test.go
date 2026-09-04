package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopSystemInfoUptimeI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/system-info.js")
	for _, want := range []string{
		"function formatUptime(seconds, context)",
		"t(context, 'desktop.system_info_uptime_days_hours_minutes'",
		"t(context, 'desktop.system_info_uptime_hours_minutes'",
		"t(context, 'desktop.system_info_uptime_minutes'",
		"formatUptime(uptimeSeconds, instance.context)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("system-info uptime i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"${days}d ${hours}h ${minutes}m",
		"${hours}h ${minutes}m",
		"${minutes}m",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("system-info uptime still hardcodes English units %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		got := values["desktop.system_info_uptime_days_hours_minutes"]
		if strings.TrimSpace(got) == "" {
			t.Fatalf("%s missing non-empty desktop.system_info_uptime_days_hours_minutes", path)
		}
		for _, placeholder := range []string{"{{days}}", "{{hours}}", "{{minutes}}"} {
			if !strings.Contains(got, placeholder) {
				t.Fatalf("%s days/hours/minutes uptime must keep %s", path, placeholder)
			}
		}
		if lang != "en" && got == "{{days}}d {{hours}}h {{minutes}}m" {
			t.Fatalf("%s must not copy the English days/hours/minutes uptime string", path)
		}
	}
}
