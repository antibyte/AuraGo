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

func TestDesktopMissionControlTriggerModuleContract(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/mission-control-triggers.js")
	for _, marker := range []string{
		"window.MissionControlTriggers = {",
		"function summary(mission, t, ctx)",
		"function createPicker(deps)",
		"function createConfigPanel(deps)",
		"const REMOTE_ALLOWED = new Set(['system_startup', 'mqtt_message', 'home_assistant_state'])",
		"t('desktop.rel_time_seconds', { count: cfg.min_interval_seconds })",
		"'desktop.mc_trigger_group_missions'",
		"'desktop.mc_trigger_group_communication'",
		"'desktop.mc_trigger_group_devices'",
		"'desktop.mc_trigger_group_planner'",
		"'desktop.mc_trigger_group_invasion'",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("mission-control-triggers.js missing marker %q", marker)
		}
	}
	for _, key := range []string{
		"mission_completed", "email_received", "webhook", "egg_hatched", "nest_cleared", "mqtt_message",
		"system_startup", "home_assistant_state", "device_connected", "device_disconnected", "fritzbox_call",
		"budget_warning", "budget_exceeded", "planner_appointment_due", "planner_todo_overdue", "planner_operational_issue",
	} {
		if !strings.Contains(source, "key: '"+key+"'") {
			t.Fatalf("trigger catalog missing %q", key)
		}
	}
	if missionControlEmojiRe.MatchString(source) {
		t.Fatalf("triggers module must not embed emoji")
	}
}

func TestDesktopMissionControlMenusModuleContract(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/mission-control-menus.js")
	for _, marker := range []string{
		"window.MissionControlMenus = {",
		"id: 'file', labelKey: 'desktop.menu_file'",
		"id: 'view', labelKey: 'desktop.menu_view'",
		"function windowMenus(m)",
		"function missionContextItems(m, mission)",
		"function queueContextItems(m, missionId)",
		"function listContextItems(m)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("mission-control-menus.js missing marker %q", marker)
		}
	}
	for _, id := range []string{
		"'new-mission'", "'duplicate'", "'run'", "'cancel-run'", "'remove-from-queue'", "'pause-resume'", "'lock-toggle'", "'prepare'", "'invalidate-prep'", "'edit'", "'delete'",
		"'refresh'", "'filter-all'", "'filter-manual'", "'filter-scheduled'", "'filter-triggered'", "'filter-errors'",
		"'sort-name'", "'sort-last-run'", "'sort-next-run'", "'sort-priority'", "'tab-overview'", "'tab-history'", "'list-panel'",
	} {
		if !strings.Contains(source, "id: "+id) {
			t.Fatalf("menus module missing menu id %s", id)
		}
	}
	for _, iconName := range []string{"play", "stop", "plus", "copy", "edit", "trash", "lock", "unlock", "pause", "refresh", "search", "x", "chevronDown", "chevronUp", "chevronLeft", "more", "check", "alert", "clock", "calendar", "bolt", "hand", "sidebar", "history", "sparkles", "mail", "webhook", "phone", "radio", "home", "plug", "plugOff", "wallet", "walletOff", "power", "egg", "nest", "listCheck", "sliders", "globe", "queue", "info", "arrowUp"} {
		if !strings.Contains(source, iconName+": `<svg") {
			t.Fatalf("ICONS missing %q", iconName)
		}
	}
	if strings.Contains(source, "label: 'File'") || strings.Contains(source, "label: 'View'") {
		t.Fatalf("menus module must not hardcode English menu labels")
	}
}

func TestDesktopMissionControlListModuleContract(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/mission-control-list.js")
	for _, marker := range []string{
		"window.MissionControlList = { create }",
		"function create(deps)",
		"element.setAttribute('role', 'listbox')",
		"row.setAttribute('role', 'option')",
		"aria-selected",
		"'desktop.mc_section_running'",
		"'desktop.mc_section_waiting'",
		"'desktop.mc_section_missions'",
		"'desktop.mc_list_empty_title'",
		"'desktop.mc_list_no_match_title'",
		"case 'ArrowDown'",
		"case 'ArrowUp'",
		"case 'Home'",
		"case 'End'",
		"case 'Enter'",
		"case 'Delete'",
		"function rowSignature(",
		"rows.get(",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("mission-control-list.js missing marker %q", marker)
		}
	}
	for _, forbidden := range []string{"alert(", "confirm(", "innerHTML = ''; // full rerender"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("list module must not use %q", forbidden)
		}
	}
}

func TestDesktopMissionControlDetailModuleContract(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/mission-control-detail.js")
	for _, marker := range []string{
		"window.MissionControlDetail = { create, extractLastOutput }",
		"function create(deps)",
		"function extractLastOutput(raw)",
		"role=\"tablist\"",
		"role=\"tab\"",
		"role=\"tabpanel\"",
		"data-mc-panel=\"overview\"",
		"data-mc-panel=\"history\"",
		"'desktop.mc_tab_overview'",
		"'desktop.mc_tab_history'",
		"'desktop.mc_action_cancel'",
		"'desktop.mc_action_cancelling'",
		"'desktop.mc_overview_queued_position'",
		"'desktop.mc_overview_running_since'",
		"'desktop.mc_history_filter_cancelled'",
		"'desktop.mc_history_load_more'",
		"'desktop.mc_prep_title'",
		"'desktop.mc_empty_title'",
		"'desktop.mc_priority_' + priority",
		"Cancelled by user",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("mission-control-detail.js missing marker %q", marker)
		}
	}
	if strings.Contains(source, "alert(") || strings.Contains(source, "confirm(") {
		t.Fatalf("detail module must not use blocking dialogs")
	}
	if strings.Contains(source, "missions.form_priority_") {
		t.Fatalf("detail module must use the emoji-free desktop.mc_priority_* labels")
	}
}
