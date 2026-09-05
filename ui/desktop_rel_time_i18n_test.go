package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopRelTimeI18n(t *testing.T) {
	t.Parallel()

	sysworld := readDesktopAssetText(t, "js/desktop/apps/sysworld.js")
	for _, want := range []string{
		"function formatRelCount(inst, count, key)",
		"'desktop.rel_time_seconds'",
		"'desktop.rel_time_minutes'",
		"'desktop.rel_time_hours'",
		"'desktop.rel_time_days'",
		"inst.ctx.t(key, { count: n })",
	} {
		if !strings.Contains(sysworld, want) {
			t.Fatalf("sysworld relTime i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"+ 's'",
		"+ 'm'",
		"+ 'h'",
		"+ 'd'",
	} {
		if strings.Contains(sysworld, forbidden) {
			t.Fatalf("sysworld still hardcodes %q", forbidden)
		}
	}

	mission := readDesktopAssetText(t, "js/desktop/apps/mission-control.js")
	if !strings.Contains(mission, "t('desktop.rel_time_seconds', { count: cfg.min_interval_seconds })") {
		t.Fatal("mission control min-interval must use desktop.rel_time_seconds")
	}
	if strings.Contains(mission, "min_interval_seconds + 's'") {
		t.Fatal("mission control still hardcodes min_interval_seconds + 's'")
	}

	sip := readDesktopAssetText(t, "js/desktop/apps/sip-phone.js")
	if strings.Contains(sip, "desktop.rel_time_") {
		t.Fatal("SIP must not use desktop.rel_time_* keys")
	}
	noisemaker := readDesktopAssetText(t, "js/desktop/apps/noisemaker.js")
	if strings.Contains(noisemaker, "desktop.rel_time_") {
		t.Fatal("Noisemaker must not use desktop.rel_time_* keys")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{
			"desktop.rel_time_seconds",
			"desktop.rel_time_minutes",
			"desktop.rel_time_hours",
			"desktop.rel_time_days",
		} {
			got := values[key]
			if strings.TrimSpace(got) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
			if !strings.Contains(got, "{{count}}") {
				t.Fatalf("%s %s must keep {{count}}", path, key)
			}
		}
		if lang == "de" && values["desktop.rel_time_minutes"] == "{{count}} min" {
			t.Fatalf("%s must not copy the English relative-minute unit", path)
		}
		if lang == "ja" && values["desktop.rel_time_seconds"] == "{{count}} s" {
			t.Fatalf("%s must not copy the English relative-second unit", path)
		}
	}
}
