package agent

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/contacts"
	"aurago/internal/planner"
)

func newPlannerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatalf("planner.InitDB() error = %v", err)
	}
	return db
}

func decodeToolOutput(t *testing.T, output string) map[string]interface{} {
	t.Helper()
	trimmed := strings.TrimPrefix(output, "Tool Output: ")
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", output, err)
	}
	return payload
}

func TestDispatchManageTodosSupportsChecklistOperations(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()

	addResp := dispatchManageTodos(ToolCall{
		Operation: "add",
		Params: map[string]interface{}{
			"title":        "Launch release",
			"priority":     "high",
			"status":       "open",
			"remind_daily": true,
			"items": []interface{}{
				map[string]interface{}{"title": "Prepare changelog"},
			},
		},
	}, db, nil, slog.Default())
	addPayload := decodeToolOutput(t, addResp)
	todoID, _ := addPayload["id"].(string)
	if todoID == "" {
		t.Fatalf("add todo response missing id: %#v", addPayload)
	}

	addItemResp := dispatchManageTodos(ToolCall{
		Operation: "add_item",
		Params: map[string]interface{}{
			"id":           todoID,
			"item_title":   "Publish binaries",
			"item_is_done": false,
		},
	}, db, nil, slog.Default())
	itemPayload := decodeToolOutput(t, addItemResp)
	itemID, _ := itemPayload["item_id"].(string)
	if itemID == "" {
		t.Fatalf("add_item response missing item_id: %#v", itemPayload)
	}

	toggleResp := dispatchManageTodos(ToolCall{
		Operation: "toggle_item",
		Params: map[string]interface{}{
			"id":           todoID,
			"item_id":      itemID,
			"item_is_done": true,
		},
	}, db, nil, slog.Default())
	togglePayload := decodeToolOutput(t, toggleResp)
	if done, ok := togglePayload["item_is_done"].(bool); !ok || !done {
		t.Fatalf("toggle_item response = %#v, want item_is_done=true", togglePayload)
	}

	completeResp := dispatchManageTodos(ToolCall{
		Operation: "complete",
		Params: map[string]interface{}{
			"id":                 todoID,
			"complete_items_too": true,
		},
	}, db, nil, slog.Default())
	completePayload := decodeToolOutput(t, completeResp)
	if status, _ := completePayload["status"].(string); status != "success" {
		t.Fatalf("complete response = %#v, want success", completePayload)
	}

	todo, err := planner.GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("planner.GetTodo() error = %v", err)
	}
	if !todo.RemindDaily {
		t.Fatal("RemindDaily = false, want true")
	}
	if todo.Status != "done" || todo.ProgressPercent != 100 {
		t.Fatalf("todo status/progress = %q/%d, want done/100", todo.Status, todo.ProgressPercent)
	}
	if todo.DoneItemCount != todo.ItemCount {
		t.Fatalf("done/item count = %d/%d, want all done", todo.DoneItemCount, todo.ItemCount)
	}
}

func TestDailyTodoReminderTextOnlyOnEligibleFirstContact(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()

	if _, err := planner.CreateTodo(db, planner.Todo{
		Title:       "Daily review",
		Priority:    "medium",
		Status:      "open",
		RemindDaily: true,
		Items: []planner.TodoItem{
			{Title: "Inbox zero"},
		},
	}); err != nil {
		t.Fatalf("planner.CreateTodo() error = %v", err)
	}

	now := time.Date(2026, 4, 16, 8, 0, 0, 0, time.UTC)
	runCfg := RunConfig{PlannerDB: db, MessageSource: "web_chat"}

	first := dailyTodoReminderText(runCfg, "Guten Morgen", now, slog.Default())
	if !strings.Contains(first, "You currently have 1 open todos") || !strings.Contains(first, "Daily review") {
		t.Fatalf("first reminder = %q, want summary and todo title", first)
	}

	second := dailyTodoReminderText(runCfg, "Noch mal hallo", now.Add(time.Hour), slog.Default())
	if second != "" {
		t.Fatalf("second reminder = %q, want empty on same day", second)
	}

	blocked := dailyTodoReminderText(RunConfig{PlannerDB: db, MessageSource: "mission", IsMission: true}, "hello", now.Add(24*time.Hour), slog.Default())
	if blocked != "" {
		t.Fatalf("mission reminder = %q, want empty", blocked)
	}
}

func TestDispatchManageAppointmentsSupportsContactParticipants(t *testing.T) {
	plannerDB := newPlannerTestDB(t)
	defer plannerDB.Close()
	contactsDB, err := contacts.InitDB(filepath.Join(t.TempDir(), "contacts.db"))
	if err != nil {
		t.Fatalf("contacts.InitDB() error = %v", err)
	}
	defer contactsDB.Close()

	adaID, err := contacts.Create(contactsDB, contacts.Contact{
		Name:         "Ada Lovelace",
		Email:        "ada@example.com",
		Relationship: "friend",
	})
	if err != nil {
		t.Fatalf("contacts.Create(Ada) error = %v", err)
	}
	graceID, err := contacts.Create(contactsDB, contacts.Contact{
		Name:         "Grace Hopper",
		Email:        "grace@example.com",
		Relationship: "colleague",
	})
	if err != nil {
		t.Fatalf("contacts.Create(Grace) error = %v", err)
	}

	addResp := dispatchManageAppointments(ToolCall{
		Operation: "add",
		Params: map[string]interface{}{
			"title":       "Planning sync",
			"date_time":   "2099-05-01T10:00:00Z",
			"contact_ids": []interface{}{adaID},
		},
	}, plannerDB, contactsDB, nil, slog.Default())
	addPayload := decodeToolOutput(t, addResp)
	appointmentID, _ := addPayload["id"].(string)
	if appointmentID == "" {
		t.Fatalf("add appointment response missing id: %#v", addPayload)
	}
	ids, err := planner.GetAppointmentContactIDs(plannerDB, appointmentID)
	if err != nil {
		t.Fatalf("planner.GetAppointmentContactIDs() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != adaID {
		t.Fatalf("contact ids after add = %#v, want [%s]", ids, adaID)
	}

	updateResp := dispatchManageAppointments(ToolCall{
		Operation: "update",
		Params: map[string]interface{}{
			"id":          appointmentID,
			"contact_ids": []interface{}{graceID},
		},
	}, plannerDB, contactsDB, nil, slog.Default())
	updatePayload := decodeToolOutput(t, updateResp)
	if status, _ := updatePayload["status"].(string); status != "success" {
		t.Fatalf("update response = %#v, want success", updatePayload)
	}
	ids, err = planner.GetAppointmentContactIDs(plannerDB, appointmentID)
	if err != nil {
		t.Fatalf("planner.GetAppointmentContactIDs() after update error = %v", err)
	}
	if len(ids) != 1 || ids[0] != graceID {
		t.Fatalf("contact ids after update = %#v, want [%s]", ids, graceID)
	}

	getResp := dispatchManageAppointments(ToolCall{
		Operation: "get",
		Params:    map[string]interface{}{"id": appointmentID},
	}, plannerDB, contactsDB, nil, slog.Default())
	getPayload := decodeToolOutput(t, getResp)
	appointment, ok := getPayload["appointment"].(map[string]interface{})
	if !ok {
		t.Fatalf("get appointment payload = %#v, want appointment object", getPayload)
	}
	contactIDs, _ := appointment["contact_ids"].([]interface{})
	if len(contactIDs) != 1 || contactIDs[0] != graceID {
		t.Fatalf("get contact_ids = %#v, want [%s]", contactIDs, graceID)
	}
	participants, _ := appointment["participants"].([]interface{})
	if len(participants) != 1 {
		t.Fatalf("get participants = %#v, want one participant", participants)
	}
	participant, _ := participants[0].(map[string]interface{})
	if participant["name"] != "Grace Hopper" || participant["email"] != "grace@example.com" {
		t.Fatalf("get participant = %#v, want Grace Hopper", participant)
	}

	listResp := dispatchManageAppointments(ToolCall{
		Operation: "list",
		Params:    map[string]interface{}{"status": "upcoming"},
	}, plannerDB, contactsDB, nil, slog.Default())
	listPayload := decodeToolOutput(t, listResp)
	appointments, _ := listPayload["appointments"].([]interface{})
	if len(appointments) != 1 {
		t.Fatalf("list appointments = %#v, want one appointment", appointments)
	}
	listAppointment, _ := appointments[0].(map[string]interface{})
	listParticipants, _ := listAppointment["participants"].([]interface{})
	if len(listParticipants) != 1 {
		t.Fatalf("list participants = %#v, want one participant", listParticipants)
	}
	listParticipant, _ := listParticipants[0].(map[string]interface{})
	if listParticipant["name"] != "Grace Hopper" {
		t.Fatalf("list participant = %#v, want Grace Hopper", listParticipant)
	}
}

func TestOperationalIssueReminderTextOnlyOnDirectFirstContact(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()

	if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{
		Source:    "maintenance",
		Context:   "maintenance",
		Title:     "Maintenance agent loop failed",
		Detail:    "budget exceeded",
		Severity:  "error",
		Reference: "daily_maintenance",
	}); err != nil {
		t.Fatalf("planner.RecordOperationalIssue() error = %v", err)
	}

	runCfg := RunConfig{PlannerDB: db, MessageSource: "web_chat"}
	first := operationalIssueReminderText(runCfg, "Hallo", true, slog.Default())
	if !strings.Contains(first, "Maintenance agent loop failed") || !strings.Contains(first, "budget exceeded") {
		t.Fatalf("first operational reminder = %q, want issue title and detail", first)
	}

	repeated := operationalIssueReminderText(runCfg, "Noch mal", false, slog.Default())
	if repeated != "" {
		t.Fatalf("repeated operational reminder = %q, want empty after first turn", repeated)
	}

	newSession := operationalIssueReminderText(runCfg, "Neue Sitzung", true, slog.Default())
	if !strings.Contains(newSession, "Maintenance agent loop failed") {
		t.Fatalf("new session operational reminder = %q, want issue despite daily claim", newSession)
	}

	relevant := operationalIssueReminderText(runCfg, "debug maintenance failed", false, slog.Default())
	if !strings.Contains(relevant, "Maintenance agent loop failed") {
		t.Fatalf("relevant operational reminder = %q, want issue despite repeated contact", relevant)
	}

	blocked := operationalIssueReminderText(RunConfig{PlannerDB: db, MessageSource: "mission", IsMission: true}, "hello", true, slog.Default())
	if blocked != "" {
		t.Fatalf("mission operational reminder = %q, want empty", blocked)
	}
}

func TestOperationalIssueReminderTextLogsFirstTurnClaimFailure(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()
	db.SetMaxOpenConns(1)

	if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{
		Source:    "maintenance",
		Context:   "maintenance",
		Title:     "Maintenance agent loop failed",
		Detail:    "budget exceeded",
		Severity:  "error",
		Reference: "daily_maintenance",
	}); err != nil {
		t.Fatalf("planner.RecordOperationalIssue() error = %v", err)
	}
	if _, err := db.Exec("PRAGMA query_only = ON"); err != nil {
		t.Fatalf("enable query_only: %v", err)
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	reminder := operationalIssueReminderText(RunConfig{PlannerDB: db, MessageSource: "web_chat"}, "Hallo", true, logger)
	if !strings.Contains(reminder, "Maintenance agent loop failed") {
		t.Fatalf("operational reminder = %q, want issue despite claim failure", reminder)
	}
	if !strings.Contains(buf.String(), "Failed to claim operational issue reminder") {
		t.Fatalf("log output = %q, want claim failure warning", buf.String())
	}
}

func TestRecordToolFailureOperationalIssueOnlyForBackgroundRuns(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()

	recordToolFailureOperationalIssue(
		RunConfig{PlannerDB: db, MessageSource: "web_chat", SessionID: "default"},
		ToolCall{Action: "docker"},
		`Tool Output: {"status":"error","message":"container failed"}`,
		slog.Default(),
	)
	issues, err := planner.ListOperationalIssueTodos(db, 10)
	if err != nil {
		t.Fatalf("ListOperationalIssueTodos() error = %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("direct web chat created operational issues: %#v", issues)
	}

	recordToolFailureOperationalIssue(
		RunConfig{PlannerDB: db, IsMission: true, MissionID: "mission-42", MessageSource: "mission"},
		ToolCall{Action: "docker"},
		`Tool Output: {"status":"error","message":"container failed"}`,
		slog.Default(),
	)
	issues, err = planner.ListOperationalIssueTodos(db, 10)
	if err != nil {
		t.Fatalf("ListOperationalIssueTodos() second error = %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1", len(issues))
	}
	if !strings.Contains(issues[0].Title, "docker") || !strings.Contains(issues[0].Description, "container failed") {
		t.Fatalf("recorded issue = %#v, want tool and detail", issues[0])
	}
}
