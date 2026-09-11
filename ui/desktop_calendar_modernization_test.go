package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopCalendarModernizationAssets(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"vd-calendar-shell",
		"data-cal-create",
		"data-cal-sidebar",
		"data-cal-drop-date",
		"updateAppointmentDateTime",
		"createRecurringAppointments",
		"notification_at",
		"agent_instruction",
		"data-cal-status-action",
		// Redesign: dedicated views/editor continuations, snackbar with undo, peek and keyboard navigation.
		"function calendarTimeGridHTML",
		"function calendarAgendaHTML",
		"function calendarMiniMonthHTML",
		"function calendarSkeletonHTML",
		"function openAppointmentPeek",
		"function showCalendarSnack",
		"function calendarHandleKeydown",
		"data-cal-snackbar",
		"data-cal-search",
		"data-cal-view",
		"data-cal-now",
		"aurago.desktop.calendar.prefs",
		"desktop.cal_undo",
		"desktop.load_failed",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("modern desktop calendar missing JS marker %q", marker)
		}
	}
	for _, forbidden := range []string{
		"alert(",
	} {
		for _, part := range []string{"js/desktop/apps/calendar.js", "js/desktop/apps/calendar-views.js", "js/desktop/apps/calendar-editor.js"} {
			if strings.Contains(readDesktopAssetText(t, part), forbidden) {
				t.Fatalf("%s must not use %q", part, forbidden)
			}
		}
	}
}

func TestDesktopCalendarModernizationStyles(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-calendar.css")
	for _, marker := range []string{
		".vd-calendar-shell",
		".vd-calendar-command",
		".vd-calendar-weekdays",
		".vd-calendar-event",
		".vd-calendar-sidebar",
		".vd-calendar-time-grid",
		".vd-calendar-now-line",
		".vd-calendar-snackbar",
		".vd-calendar-peek",
		".vd-calendar-skeleton",
		".vd-calendar-recurring",
		".vd-calendar-editor",
		"color-scheme: light",
		"prefers-reduced-motion",
		"hover: none",
		"max-width: 760px",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("modern desktop calendar missing CSS marker %q", marker)
		}
	}
	// The calendar owns its stylesheet; planning.css keeps only todo and music player rules.
	if planning := readDesktopAssetText(t, "css/desktop-app-planning.css"); strings.Contains(planning, ".vd-calendar-") {
		t.Fatalf("desktop-app-planning.css must not contain calendar rules anymore")
	}
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(loader, "'calendar': {\n            styles: appStyles('/css/desktop-app-calendar.css')") {
		t.Fatalf("module loader must register desktop-app-calendar.css for the calendar app")
	}
}

func TestDesktopCalendarModernizationTranslations(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.cal_new",
		"desktop.cal_previous",
		"desktop.cal_next",
		"desktop.cal_today_panel",
		"desktop.cal_upcoming",
		"desktop.cal_overdue",
		"desktop.cal_no_events",
		"desktop.cal_more_events",
		"desktop.cal_reminder",
		"desktop.cal_agent_instruction",
		"desktop.cal_recurring",
		"desktop.cal_repeat_none",
		"desktop.cal_repeat_daily",
		"desktop.cal_repeat_weekly",
		"desktop.cal_repeat_monthly",
		"desktop.cal_repeat_count",
		"desktop.cal_participants",
		"desktop.cal_mark_complete",
		"desktop.cal_cancel_appointment",
		"desktop.cal_drag_hint",
		"desktop.cal_agenda",
		"desktop.cal_next_up",
		"desktop.cal_search_placeholder",
		"desktop.cal_reminder_15m",
		"desktop.cal_reminder_custom",
		"desktop.cal_wake_agent",
		"desktop.cal_rescheduled",
		"desktop.cal_undo",
		"desktop.cal_restored",
		"desktop.cal_show_completed",
		"desktop.cal_show_cancelled",
		"desktop.cal_empty_hint",
		"desktop.cal_week_short",
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}
}
