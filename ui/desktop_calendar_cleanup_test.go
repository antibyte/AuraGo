package ui

import (
	"strings"
	"testing"
)

func TestDesktopCalendarLoadsKeepTheNewestResponse(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/calendar.js")
	for _, marker := range []string{
		"session.loadGeneration = 0",
		"session.pendingDeletes = new Map()",
		"const generation = ++session.loadGeneration",
		"if (generation !== session.loadGeneration) return",
		"!session.pendingDeletes.has(String(item.id))",
		"loadCalendarAppointments(session, { silent: true })",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("calendar.js missing load marker %q", marker)
		}
	}
	if strings.Contains(source, "if (session.loading) return;") {
		t.Fatal("calendar.js still drops a load that starts while another is in flight")
	}
}

func TestDesktopCalendarDeleteWaitsForUndo(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/calendar.js")
	start := strings.Index(source, "async function deleteCalendarAppointment")
	end := strings.Index(source, "function hideCalendarSnack")
	if start < 0 || end < start {
		t.Fatal("calendar.js delete function bounds not found")
	}
	body := source[start:end]
	if strings.Contains(body, "DELETE") || strings.Contains(body, "plannerJSON('/api/desktop/integrations/appointments', 'POST'") {
		t.Fatal("delete still removes the row before the undo window ends")
	}
	for _, marker := range []string{
		"function commitCalendarDelete",
		"session.pendingDeletes.set(String(appointment.id)",
		"commitCalendarDelete(session, id)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("calendar.js missing deferred-delete marker %q", marker)
		}
	}
}

func TestDesktopCalendarWindowCleanupRemovesModals(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/calendar.js")
	if !strings.Contains(source, "registerWindowCleanup(id") || !strings.Contains(source, "session.dispose()") {
		t.Fatal("window cleanup must dispose the session")
	}
	if strings.Contains(source, "document.querySelectorAll('.vd-modal-backdrop.vd-calendar-editor-backdrop').forEach(el => el.remove());") {
		t.Fatal("window cleanup still removes every calendar editor")
	}
	start := strings.Index(source, "function disposeCalendarSession")
	end := strings.Index(source, "function calendarListen")
	if start < 0 || end < start || !strings.Contains(source[start:end], "closeCalendarEditor(session)") {
		t.Fatal("dispose must close this session's editor")
	}
}

func TestDesktopClockPopupUsesTheLocalDay(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/core/shell-chrome-runtime.js")
	if strings.Contains(source, "toISOString().slice(0, 10)") {
		t.Fatal("clock popup still treats the UTC date as today")
	}
	for _, marker := range []string{
		"function clockLocalDay",
		"clockLocalDay(now)",
		"clockLocalDay(item.date_time)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("shell-chrome-runtime.js missing %q", marker)
		}
	}
}

func TestDesktopCalendarCopyAvoidsTheKnownTraps(t *testing.T) {
	t.Parallel()
	shell := readDesktopAssetText(t, "js/desktop/apps/calendar.js")
	views := readDesktopAssetText(t, "js/desktop/apps/calendar-views.js")
	if strings.Contains(shell, "session.error.message") {
		t.Fatal("calendar load error still prints err.message")
	}
	if strings.Contains(views, "${last.getDate()}.") {
		t.Fatal("week title still appends a day-period dot")
	}
	for _, marker := range []string{"formatRange", "item.status === 'overdue'"} {
		if !strings.Contains(views, marker) {
			t.Fatalf("calendar-views.js missing %q", marker)
		}
	}
}
