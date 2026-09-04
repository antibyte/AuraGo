package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

var sysmonUptimeI18nKeys = []string{
	"desktop.system_info_uptime_days_hours",
	"desktop.system_info_uptime_hours_minutes",
	"desktop.system_info_uptime_minutes",
}

func TestDesktopSysmonUptimeI18n(t *testing.T) {
	t.Parallel()

	runtime := readDesktopAssetText(t, "js/desktop/core/widget-sysmon-runtime.js")
	for _, want := range []string{
		"t('desktop.system_info_uptime_days_hours'",
		"t('desktop.system_info_uptime_hours_minutes'",
		"t('desktop.system_info_uptime_minutes'",
	} {
		if !strings.Contains(runtime, want) {
			t.Fatalf("sysmon uptime i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"${days}d ${hours}h",
		"${hours}h ${minutes}m",
		"${minutes}m",
	} {
		if strings.Contains(runtime, forbidden) {
			t.Fatalf("sysmon uptime still hardcodes English units %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range sysmonUptimeI18nKeys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
		}
		if !strings.Contains(values["desktop.system_info_uptime_days_hours"], "{{days}}") ||
			!strings.Contains(values["desktop.system_info_uptime_days_hours"], "{{hours}}") {
			t.Fatalf("%s days/hours uptime must keep {{days}} and {{hours}}", path)
		}
		if !strings.Contains(values["desktop.system_info_uptime_hours_minutes"], "{{hours}}") ||
			!strings.Contains(values["desktop.system_info_uptime_hours_minutes"], "{{minutes}}") {
			t.Fatalf("%s hours/minutes uptime must keep {{hours}} and {{minutes}}", path)
		}
		if !strings.Contains(values["desktop.system_info_uptime_minutes"], "{{minutes}}") {
			t.Fatalf("%s minutes uptime must keep {{minutes}}", path)
		}
		if lang != "en" && values["desktop.system_info_uptime_days_hours"] == "{{days}}d {{hours}}h" {
			t.Fatalf("%s must not copy the English days/hours uptime string", path)
		}
	}
}
