package agent

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		Title:    "Daily review",
		Priority: "medium",
		Status:   "open",
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

	blocked := operationalIssueReminderText(RunConfig{PlannerDB: db, MessageSource: "mission", IsMission: true}, "hello", true, slog.Default())
	if blocked != "" {
		t.Fatalf("mission operational reminder = %q, want empty", blocked)
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
