package ui

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var missionControlLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

// Spot-check keys; the full key set is derived from en.json at test time.
var missionControlSentinelKeys = []string{
	"desktop.mc_new_mission",
	"desktop.mc_search_placeholder",
	"desktop.mc_tab_overview",
	"desktop.mc_tab_history",
	"desktop.mc_action_run",
	"desktop.mc_action_cancel",
	"desktop.mc_section_running",
	"desktop.mc_section_waiting",
	"desktop.mc_section_missions",
	"desktop.mc_editor_section_task",
	"desktop.mc_editor_section_when",
	"desktop.mc_editor_section_advanced",
	"desktop.mc_schedule_daily_at",
	"desktop.mc_schedule_weekly_at",
	"desktop.mc_schedule_custom",
	"desktop.mc_trigger_group_missions",
	"desktop.mc_trigger_group_invasion",
	"desktop.mc_history_filter_cancelled",
	"desktop.mc_toast_cancel_requested",
	"desktop.mc_delete_message",
	"desktop.mc_editor_discard_title",
	"desktop.mc_readonly_banner",
	"desktop.mc_status_next",
	"desktop.mc_field_required",
}

var missionControlRemovedKeys = []string{"desktop.mc_view_grid", "desktop.mc_view_list"}

var missionControlPlaceholderRe = regexp.MustCompile(`\{\{[a-z_]+\}\}`)
var missionControlEmojiRe = regexp.MustCompile("[\U0001F300-\U0001FAFF\u2600-\u27BF]")

func loadDesktopLocale(t *testing.T, lang string) map[string]string {
	t.Helper()
	path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
	var values map[string]string
	if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return values
}

func missionControlKeys(values map[string]string) []string {
	keys := make([]string, 0, 256)
	for key := range values {
		if strings.HasPrefix(key, "desktop.mc_") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func TestDesktopMissionControlTranslationsCoverAllLocales(t *testing.T) {
	t.Parallel()

	en := loadDesktopLocale(t, "en")
	keys := missionControlKeys(en)
	if len(keys) < 180 {
		t.Fatalf("expected at least 180 desktop.mc_* keys in en.json, got %d", len(keys))
	}
	for _, key := range missionControlSentinelKeys {
		if strings.TrimSpace(en[key]) == "" {
			t.Fatalf("en.json missing sentinel key %q", key)
		}
	}
	for _, lang := range missionControlLocales {
		values := loadDesktopLocale(t, lang)
		for _, key := range missionControlRemovedKeys {
			if _, ok := values[key]; ok {
				t.Fatalf("%s still contains removed key %q", lang, key)
			}
		}
		for _, key := range keys {
			got := values[key]
			if strings.TrimSpace(got) == "" {
				t.Fatalf("%s missing non-empty %q", lang, key)
			}
			want := missionControlPlaceholderRe.FindAllString(en[key], -1)
			have := missionControlPlaceholderRe.FindAllString(got, -1)
			sort.Strings(want)
			sort.Strings(have)
			if strings.Join(want, ",") != strings.Join(have, ",") {
				t.Fatalf("%s %q placeholders %v differ from en %v", lang, key, have, want)
			}
			if missionControlEmojiRe.MatchString(got) {
				t.Fatalf("%s %q must not contain emoji: %q", lang, key, got)
			}
		}
		if lang == "de" {
			for _, key := range []string{"desktop.mc_tab_overview", "desktop.mc_action_run", "desktop.mc_section_running", "desktop.mc_editor_section_when"} {
				if values[key] == en[key] {
					t.Fatalf("de.json must translate %q instead of copying English", key)
				}
			}
			for key, value := range values {
				if strings.HasPrefix(key, "desktop.mc_") && (strings.Contains(value, " Sie ") || strings.HasPrefix(value, "Sie ")) {
					t.Fatalf("de.json %q must use the informal Du form: %q", key, value)
				}
			}
		}
	}
}

func TestDesktopMissionControlScheduleModuleContract(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/mission-control-schedule.js")
	for _, marker := range []string{
		"window.MissionControlSchedule = {",
		"function parse(expr)",
		"function build(state)",
		"function describe(expr, t, lang)",
		"function validate(expr)",
		"function isSupported(expr)",
		"'desktop.mc_schedule_every_minutes'",
		"'desktop.mc_schedule_weekly_at'",
		"'desktop.mc_schedule_descriptor_'",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("mission-control-schedule.js missing marker %q", marker)
		}
	}
	for _, forbidden := range []string{"document.", "localStorage", "fetch("} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("schedule module must stay pure; found %q", forbidden)
		}
	}
}
