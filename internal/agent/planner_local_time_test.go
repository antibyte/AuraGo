package agent

import (
	"log/slog"
	"testing"
	"time"

	"aurago/internal/planner"
)

// localPlannerTime renders a stored UTC planner value the way agent-facing
// output shows it: in the server zone used by the prompt's "Current time".
func localPlannerTime(t *testing.T, stored string) string {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, stored)
	if err != nil {
		t.Fatalf("parse %q: %v", stored, err)
	}
	return ts.In(time.Local).Format(time.RFC3339)
}

func TestManagePlannerToolsReturnTimesInServerZone(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()

	addAppointment := decodeToolOutput(t, dispatchManageAppointments(ToolCall{
		Operation: "add",
		Params: map[string]interface{}{
			"title":           "Dentist",
			"date_time":       "2026-10-05T09:00:00+02:00",
			"notification_at": "2026-10-05T08:30:00+02:00",
		},
	}, db, nil, nil, slog.Default()))
	appointmentID, _ := addAppointment["id"].(string)
	if appointmentID == "" {
		t.Fatalf("add appointment response = %#v", addAppointment)
	}
	wantStart := localPlannerTime(t, "2026-10-05T07:00:00Z")
	wantReminder := localPlannerTime(t, "2026-10-05T06:30:00Z")

	got := decodeToolOutput(t, dispatchManageAppointments(ToolCall{Operation: "get", Params: map[string]interface{}{"id": appointmentID}}, db, nil, nil, slog.Default()))
	appointment, _ := got["appointment"].(map[string]interface{})
	if appointment["date_time"] != wantStart || appointment["notification_at"] != wantReminder {
		t.Fatalf("get appointment times = %v / %v, want %s / %s", appointment["date_time"], appointment["notification_at"], wantStart, wantReminder)
	}
	listed := decodeToolOutput(t, dispatchManageAppointments(ToolCall{Operation: "list", Params: map[string]interface{}{"status": "all"}}, db, nil, nil, slog.Default()))
	appointments, _ := listed["appointments"].([]interface{})
	if len(appointments) != 1 {
		t.Fatalf("list appointments = %#v", listed)
	}
	if first, _ := appointments[0].(map[string]interface{}); first["date_time"] != wantStart {
		t.Fatalf("list appointment date_time = %v, want %s", first["date_time"], wantStart)
	}

	addTodo := decodeToolOutput(t, dispatchManageTodos(ToolCall{
		Operation: "add",
		Params:    map[string]interface{}{"title": "Pay invoice", "due_date": "2026-10-05T18:00:00+02:00"},
	}, db, nil, slog.Default()))
	todoID, _ := addTodo["id"].(string)
	if todoID == "" {
		t.Fatalf("add todo response = %#v", addTodo)
	}
	wantDue := localPlannerTime(t, "2026-10-05T16:00:00Z")
	gotTodo := decodeToolOutput(t, dispatchManageTodos(ToolCall{Operation: "get", Params: map[string]interface{}{"id": todoID}}, db, nil, slog.Default()))
	if todo, _ := gotTodo["todo"].(map[string]interface{}); todo["due_date"] != wantDue {
		t.Fatalf("get todo due_date = %v, want %s", todo["due_date"], wantDue)
	}
	listedTodos := decodeToolOutput(t, dispatchManageTodos(ToolCall{Operation: "list", Params: map[string]interface{}{"status": "all"}}, db, nil, slog.Default()))
	todos, _ := listedTodos["todos"].([]interface{})
	if len(todos) != 1 {
		t.Fatalf("list todos = %#v", listedTodos)
	}
	if first, _ := todos[0].(map[string]interface{}); first["due_date"] != wantDue {
		t.Fatalf("list todo due_date = %v, want %s", first["due_date"], wantDue)
	}

	// Display localization must not leak into storage.
	stored, err := planner.GetAppointment(db, appointmentID)
	if err != nil {
		t.Fatalf("planner.GetAppointment: %v", err)
	}
	if stored.DateTime != "2026-10-05T07:00:00Z" {
		t.Fatalf("stored date_time = %q, want UTC", stored.DateTime)
	}
}

func TestSearchPlannerMemoryReturnsTimesInCallerZone(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()

	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("Europe/Berlin unavailable: %v", err)
	}
	now := time.Now().In(berlin).Truncate(time.Second)
	if _, err := planner.CreateAppointment(db, planner.Appointment{Title: "Dentist", DateTime: now.Add(3 * time.Hour).UTC().Format(time.RFC3339)}); err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}
	if _, err := planner.CreateTodo(db, planner.Todo{Title: "Pay invoice", DueDate: now.Add(10 * time.Hour).UTC().Format(time.RFC3339)}); err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}

	result, err := searchPlannerMemory(db, "", "", now, 5, 5)
	if err != nil {
		t.Fatalf("searchPlannerMemory: %v", err)
	}
	if len(result.Appointments) != 1 || result.Appointments[0].DateTime != now.Add(3*time.Hour).Format(time.RFC3339) {
		t.Fatalf("appointments = %#v, want Berlin wall-clock time", result.Appointments)
	}
	if len(result.Todos) != 1 || result.Todos[0].DueDate != now.Add(10*time.Hour).Format(time.RFC3339) {
		t.Fatalf("todos = %#v, want Berlin wall-clock due date", result.Todos)
	}
}
