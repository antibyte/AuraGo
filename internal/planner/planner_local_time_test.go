package planner

import (
	"strings"
	"testing"
	"time"
)

func localTimeTestZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("time zone %s unavailable: %v", name, err)
	}
	return loc
}

func TestLocalizePlannerRecordsForDisplay(t *testing.T) {
	berlin := localTimeTestZone(t, "Europe/Berlin")

	appointments := []Appointment{{Title: "Dentist", DateTime: "2026-10-05T07:00:00Z", NotificationAt: "2026-10-05T06:30:00Z"}, {Title: "No reminder", DateTime: "2026-03-29T01:30:00Z"}}
	localized := LocalizeAppointments(appointments, berlin)
	if localized[0].DateTime != "2026-10-05T09:00:00+02:00" || localized[0].NotificationAt != "2026-10-05T08:30:00+02:00" {
		t.Fatalf("localized appointment = %q / %q", localized[0].DateTime, localized[0].NotificationAt)
	}
	if localized[1].DateTime != "2026-03-29T03:30:00+02:00" || localized[1].NotificationAt != "" {
		t.Fatalf("localized DST appointment = %q / %q", localized[1].DateTime, localized[1].NotificationAt)
	}
	if appointments[0].DateTime != "2026-10-05T07:00:00Z" {
		t.Fatalf("LocalizeAppointments modified its input: %q", appointments[0].DateTime)
	}
	// The displayed value must parse back to the stored instant.
	back, err := time.Parse(time.RFC3339, localized[0].DateTime)
	if err != nil || !back.Equal(time.Date(2026, 10, 5, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("localized value does not round-trip: %v %v", back, err)
	}

	// Date-only due dates (stored as UTC midnight) are civil days and must not shift.
	todos := []Todo{{Title: "Timed", DueDate: "2026-10-05T16:00:00Z"}, {Title: "Day", DueDate: "2026-10-05T00:00:00Z"}, {Title: "None"}}
	newYork := localTimeTestZone(t, "America/New_York")
	for _, loc := range []*time.Location{berlin, newYork} {
		got := LocalizeTodos(todos, loc)
		if got[1].DueDate != "2026-10-05" || got[2].DueDate != "" {
			t.Fatalf("civil/empty due in %s = %q / %q", loc, got[1].DueDate, got[2].DueDate)
		}
	}
	if got := LocalizeTodos(todos, berlin)[0].DueDate; got != "2026-10-05T18:00:00+02:00" {
		t.Fatalf("timed due in Berlin = %q", got)
	}
	if todos[0].DueDate != "2026-10-05T16:00:00Z" {
		t.Fatalf("LocalizeTodos modified its input: %q", todos[0].DueDate)
	}
}

func TestPromptContextShowsPlannerTimesInCallerZone(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	berlin := localTimeTestZone(t, "Europe/Berlin")
	now := time.Now().In(berlin).Truncate(time.Second)
	apptUTC := now.Add(3 * time.Hour).UTC().Format(time.RFC3339)
	dueUTC := now.Add(10 * time.Hour).UTC().Format(time.RFC3339)
	apptLocal := now.Add(3 * time.Hour).In(berlin).Format(time.RFC3339)
	dueLocal := now.Add(10 * time.Hour).In(berlin).Format(time.RFC3339)
	if _, err := CreateAppointment(db, Appointment{Title: "Dentist", DateTime: apptUTC}); err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}
	if _, err := CreateTodo(db, Todo{Title: "Pay invoice", DueDate: dueUTC}); err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}
	if _, err := CreateTodo(db, Todo{Title: "File taxes", DueDate: "2099-10-09"}); err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}

	snapshot, err := BuildPromptSnapshot(db, now, PromptSnapshotOptions{AppointmentWindow: 48 * time.Hour})
	if err != nil {
		t.Fatalf("BuildPromptSnapshot: %v", err)
	}
	text := BuildPromptContextText(snapshot)
	for _, want := range []string{apptLocal + ": Dentist", "Pay invoice (due: " + dueLocal + ")", "File taxes (due: 2099-10-09)"} {
		if !strings.Contains(text, want) {
			t.Fatalf("prompt context missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, apptUTC) || strings.Contains(text, dueUTC) {
		t.Fatalf("prompt context still shows UTC storage values:\n%s", text)
	}
}

func TestDailyReminderShowsPlannerTimesInCallerZone(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	berlin := localTimeTestZone(t, "Europe/Berlin")
	now := time.Now().In(berlin).Truncate(time.Second)
	apptLocal := now.Add(3 * time.Hour).In(berlin).Format(time.RFC3339)
	dueLocal := now.Add(10 * time.Hour).In(berlin).Format(time.RFC3339)
	if _, err := CreateAppointment(db, Appointment{Title: "Dentist", DateTime: now.Add(3 * time.Hour).UTC().Format(time.RFC3339)}); err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}
	if _, err := CreateTodo(db, Todo{Title: "Pay invoice", DueDate: now.Add(10 * time.Hour).UTC().Format(time.RFC3339), RemindDaily: true}); err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}

	snapshot, err := ClaimDailyReminderSnapshot(db, now)
	if err != nil {
		t.Fatalf("ClaimDailyReminderSnapshot: %v", err)
	}
	text := BuildDailyPlannerReminderText(snapshot)
	for _, want := range []string{"- " + apptLocal + ": Dentist", "Pay invoice (due: " + dueLocal + ")"} {
		if !strings.Contains(text, want) {
			t.Fatalf("daily reminder missing %q:\n%s", want, text)
		}
	}
}

func TestNotificationPromptShowsAppointmentInLocalZone(t *testing.T) {
	berlin := localTimeTestZone(t, "Europe/Berlin")
	prompt := buildNotificationPrompt(Appointment{Title: "Dentist", DateTime: "2026-10-05T07:00:00Z"}, berlin)
	if !strings.Contains(prompt, `"Dentist" scheduled for 2026-10-05T09:00:00+02:00.`) {
		t.Fatalf("notification prompt = %q, want Berlin wall-clock time", prompt)
	}
}
