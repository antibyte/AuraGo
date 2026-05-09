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
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("modern desktop calendar missing JS marker %q", marker)
		}
	}
}

func TestDesktopCalendarModernizationStyles(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-apps.css")
	for _, marker := range []string{
		".vd-calendar-shell",
		".vd-calendar-weekdays",
		".vd-calendar-event",
		".vd-calendar-sidebar",
		".vd-calendar-time-grid",
		".vd-calendar-recurring",
		"max-width: 760px",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("modern desktop calendar missing CSS marker %q", marker)
		}
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
