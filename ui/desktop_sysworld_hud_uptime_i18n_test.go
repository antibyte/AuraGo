package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopSysworldHudUptimeI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/sysworld-hud.js")
	for _, want := range []string{
		"function formatUptimePhrase(key, vars)",
		"inst.ctx.t(key, vars)",
		"'desktop.system_info_uptime_days_hours'",
		"'desktop.system_info_uptime_hours_minutes'",
		"'desktop.system_info_uptime_minutes'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("sysworld HUD uptime i18n missing marker %q", want)
		}
	}
	if strings.Contains(source, "desktop.system_info_uptime_days_hours_minutes") {
		t.Fatal("sysworld HUD must keep the compact Sysmon uptime form, not System Info minutes-in-days")
	}
	if strings.Contains(source, "desktop.rel_time_") {
		t.Fatal("sysworld HUD must not reuse desktop.rel_time_* for compact uptime")
	}
	for _, forbidden := range []string{
		"+ 'd '",
		"+ 'h '",
		"+ 'm'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("sysworld HUD still hardcodes %q", forbidden)
		}
	}

	sysworld := readDesktopAssetText(t, "js/desktop/apps/sysworld.js")
	if !strings.Contains(sysworld, "desktop.rel_time_") {
		t.Fatal("sysworld relTime must keep desktop.rel_time_*")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{
			"desktop.system_info_uptime_days_hours",
			"desktop.system_info_uptime_hours_minutes",
			"desktop.system_info_uptime_minutes",
		} {
			got := values[key]
			if strings.TrimSpace(got) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
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
		if lang == "de" && values["desktop.system_info_uptime_minutes"] == "{{minutes}}m" {
			t.Fatalf("%s must not copy the English minute uptime unit", path)
		}
		if lang == "ja" && values["desktop.system_info_uptime_days_hours"] == "{{days}}d {{hours}}h" {
			t.Fatalf("%s must not copy the English days/hours uptime string", path)
		}
	}
}
