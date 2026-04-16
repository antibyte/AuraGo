package planner

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/dbutil"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_planner.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	return db
}

func TestInitDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "planner.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Expected database file to exist")
	}
}

func TestInitDBMigratesLegacySchemaAndNormalizesValues(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "planner_legacy.db")
	legacyDB, err := dbutil.Open(dbPath)
	if err != nil {
		t.Fatalf("open legacy planner db: %v", err)
	}
	defer legacyDB.Close()

	legacySchema := `
	CREATE TABLE appointments (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT DEFAULT '',
		date_time TEXT NOT NULL,
		notification_at TEXT DEFAULT '',
		wake_agent INTEGER DEFAULT 0,
		agent_instruction TEXT DEFAULT '',
		notified INTEGER DEFAULT 0,
		status TEXT DEFAULT 'upcoming',
		kg_node_id TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE TABLE todos (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT DEFAULT '',
		priority TEXT DEFAULT 'medium',
		status TEXT DEFAULT 'open',
		due_date TEXT DEFAULT '',
		kg_node_id TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);`
	if _, err := legacyDB.Exec(legacySchema); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if _, err := legacyDB.Exec(`INSERT INTO appointments (id, title, description, date_time, status, created_at, updated_at) VALUES ('a1', 'Legacy appointment', '', '2026-04-20T10:00:00Z', 'invalid', '2026-04-16T00:00:00Z', '2026-04-16T00:00:00Z')`); err != nil {
		t.Fatalf("insert legacy appointment: %v", err)
	}
	if _, err := legacyDB.Exec(`INSERT INTO todos (id, title, description, priority, status, due_date, created_at, updated_at) VALUES ('t1', 'Legacy todo', '', 'urgent', 'broken', '', '2026-04-16T00:00:00Z', '2026-04-16T00:00:00Z')`); err != nil {
		t.Fatalf("insert legacy todo: %v", err)
	}
	if err := dbutil.SetUserVersion(legacyDB, 1); err != nil {
		t.Fatalf("set legacy schema version: %v", err)
	}
	legacyDB.Close()

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB migration failed: %v", err)
	}
	defer db.Close()

	version, err := dbutil.GetUserVersion(db)
	if err != nil {
		t.Fatalf("GetUserVersion: %v", err)
	}
	if version != plannerSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, plannerSchemaVersion)
	}

	appointment, err := GetAppointment(db, "a1")
	if err != nil {
		t.Fatalf("GetAppointment migrated row: %v", err)
	}
	if appointment.Status != "upcoming" {
		t.Fatalf("appointment status = %q, want upcoming", appointment.Status)
	}

	todo, err := GetTodo(db, "t1")
	if err != nil {
		t.Fatalf("GetTodo migrated row: %v", err)
	}
	if todo.Priority != "medium" {
		t.Fatalf("todo priority = %q, want medium", todo.Priority)
	}
	if todo.Status != "open" {
		t.Fatalf("todo status = %q, want open", todo.Status)
	}
	if todo.RemindDaily {
		t.Fatal("migrated legacy todo should not enable daily reminder by default")
	}
	if todo.ItemCount != 0 || todo.DoneItemCount != 0 || todo.ProgressPercent != 0 {
		t.Fatalf("legacy todo counters = items:%d done:%d progress:%d, want 0/0/0", todo.ItemCount, todo.DoneItemCount, todo.ProgressPercent)
	}
}

func TestInitDBMigratesTodosToV3Fields(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "planner_v2.db")
	db, err := dbutil.Open(dbPath)
	if err != nil {
		t.Fatalf("open planner db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE todos (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			priority TEXT NOT NULL DEFAULT 'medium',
			status TEXT NOT NULL DEFAULT 'open',
			due_date TEXT NOT NULL DEFAULT '',
			kg_node_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
	`); err != nil {
		t.Fatalf("create v2 todos schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO todos (id, title, priority, status, created_at, updated_at) VALUES ('done1', 'Done todo', 'high', 'done', '2026-04-16T00:00:00Z', '2026-04-16T12:00:00Z')`); err != nil {
		t.Fatalf("insert v2 todo: %v", err)
	}
	if err := dbutil.SetUserVersion(db, 2); err != nil {
		t.Fatalf("set user version: %v", err)
	}
	db.Close()

	migrated, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB migration failed: %v", err)
	}
	defer migrated.Close()

	todo, err := GetTodo(migrated, "done1")
	if err != nil {
		t.Fatalf("GetTodo() error = %v", err)
	}
	if todo.CompletedAt != "2026-04-16T12:00:00Z" {
		t.Fatalf("CompletedAt = %q, want updated_at backfill", todo.CompletedAt)
	}
	if todo.ProgressPercent != 100 {
		t.Fatalf("ProgressPercent = %d, want 100", todo.ProgressPercent)
	}
}

func TestValidRFC3339(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"2025-03-15T14:00:00Z", true},
		{"2025-03-15T14:00:00+02:00", true},
		{"2025-12-31T23:59:59Z", true},
		{"", false},
		{"not-a-date", false},
		{"2025-13-01T00:00:00Z", false},
		{"2025-03-15", false},
		{"2025/03/15 14:00", false},
	}
	for _, tt := range tests {
		if got := validRFC3339(tt.input); got != tt.want {
			t.Errorf("validRFC3339(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestValidAppointmentStatus(t *testing.T) {
	for _, s := range []string{"upcoming", "completed", "cancelled"} {
		if !validAppointmentStatus(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	for _, s := range []string{"open", "done", "invalid", ""} {
		if validAppointmentStatus(s) {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestValidTodoStatus(t *testing.T) {
	for _, s := range []string{"open", "in_progress", "done"} {
		if !validTodoStatus(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	for _, s := range []string{"cancelled", "upcoming", "invalid", ""} {
		if validTodoStatus(s) {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestValidTodoPriority(t *testing.T) {
	for _, p := range []string{"low", "medium", "high"} {
		if !validTodoPriority(p) {
			t.Errorf("expected %q to be valid", p)
		}
	}
	for _, p := range []string{"critical", "urgent", "", "Medium"} {
		if validTodoPriority(p) {
			t.Errorf("expected %q to be invalid", p)
		}
	}
}

func TestCreateAppointment(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, err := CreateAppointment(db, Appointment{
		Title:    "Team Meeting",
		DateTime: "2025-06-15T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("CreateAppointment failed: %v", err)
	}
	if id == "" {
		t.Error("Expected non-empty ID")
	}
}

func TestCreateAppointmentValidation(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	tests := []struct {
		name string
		apt  Appointment
	}{
		{"empty title", Appointment{DateTime: "2025-06-15T10:00:00Z"}},
		{"empty date_time", Appointment{Title: "Test"}},
		{"invalid date_time format", Appointment{Title: "Test", DateTime: "2025-06-15"}},
		{"invalid notification_at", Appointment{Title: "Test", DateTime: "2025-06-15T10:00:00Z", NotificationAt: "not-a-date"}},
		{"invalid status", Appointment{Title: "Test", DateTime: "2025-06-15T10:00:00Z", Status: "unknown"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CreateAppointment(db, tt.apt)
			if err == nil {
				t.Error("Expected error but got nil")
			}
		})
	}
}

func TestGetAppointment(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateAppointment(db, Appointment{
		Title:       "Checkup",
		Description: "Annual checkup",
		DateTime:    "2025-08-20T09:00:00Z",
	})

	a, err := GetAppointment(db, id)
	if err != nil {
		t.Fatalf("GetAppointment failed: %v", err)
	}
	if a.Title != "Checkup" {
		t.Errorf("Expected title 'Checkup', got %q", a.Title)
	}
	if a.Description != "Annual checkup" {
		t.Errorf("Expected description 'Annual checkup', got %q", a.Description)
	}
	if a.Status != "upcoming" {
		t.Errorf("Expected status 'upcoming', got %q", a.Status)
	}
	if a.KGNodeID != "appointment_"+id {
		t.Errorf("Expected KGNodeID 'appointment_%s', got %q", id, a.KGNodeID)
	}
}

func TestGetAppointmentNotFound(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	_, err := GetAppointment(db, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent appointment")
	}
}

func TestUpdateAppointment(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateAppointment(db, Appointment{
		Title:    "Original",
		DateTime: "2025-07-01T10:00:00Z",
	})

	a, _ := GetAppointment(db, id)
	a.Title = "Updated"
	a.Status = "completed"
	err := UpdateAppointment(db, *a)
	if err != nil {
		t.Fatalf("UpdateAppointment failed: %v", err)
	}

	updated, _ := GetAppointment(db, id)
	if updated.Title != "Updated" {
		t.Errorf("Expected title 'Updated', got %q", updated.Title)
	}
	if updated.Status != "completed" {
		t.Errorf("Expected status 'completed', got %q", updated.Status)
	}
}

func TestUpdateAppointmentAllowsClearingDateTime(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateAppointment(db, Appointment{
		Title:    "Flexible",
		DateTime: "2025-07-01T10:00:00Z",
	})

	a, _ := GetAppointment(db, id)
	a.DateTime = ""
	if err := UpdateAppointment(db, *a); err != nil {
		t.Fatalf("UpdateAppointment failed: %v", err)
	}

	updated, _ := GetAppointment(db, id)
	if updated.DateTime != "" {
		t.Fatalf("date_time = %q, want empty string", updated.DateTime)
	}
}

func TestUpdateAppointmentInvalidStatus(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateAppointment(db, Appointment{
		Title:    "Test",
		DateTime: "2025-07-01T10:00:00Z",
	})

	a, _ := GetAppointment(db, id)
	a.Status = "invalid_status"
	err := UpdateAppointment(db, *a)
	if err == nil {
		t.Error("Expected error for invalid status")
	}
}

func TestDeleteAppointment(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateAppointment(db, Appointment{
		Title:    "To Delete",
		DateTime: "2025-07-01T10:00:00Z",
	})

	err := DeleteAppointment(db, id)
	if err != nil {
		t.Fatalf("DeleteAppointment failed: %v", err)
	}

	_, err = GetAppointment(db, id)
	if err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestDeleteAppointmentNotFound(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	err := DeleteAppointment(db, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent appointment")
	}
}

func TestListAppointments(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	CreateAppointment(db, Appointment{Title: "Alpha", DateTime: "2025-07-01T10:00:00Z"})
	CreateAppointment(db, Appointment{Title: "Beta Meeting", DateTime: "2025-07-02T14:00:00Z"})
	CreateAppointment(db, Appointment{Title: "Gamma", DateTime: "2025-07-03T09:00:00Z", Status: "completed"})

	all, err := ListAppointments(db, "", "")
	if err != nil {
		t.Fatalf("ListAppointments failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Expected 3 appointments, got %d", len(all))
	}

	filtered, _ := ListAppointments(db, "", "upcoming")
	if len(filtered) != 2 {
		t.Errorf("Expected 2 upcoming, got %d", len(filtered))
	}

	searched, _ := ListAppointments(db, "meeting", "")
	if len(searched) != 1 {
		t.Errorf("Expected 1 result for 'meeting', got %d", len(searched))
	}
	if searched[0].Title != "Beta Meeting" {
		t.Errorf("Expected 'Beta Meeting', got %q", searched[0].Title)
	}
}

func TestListAppointmentsEmpty(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	list, err := ListAppointments(db, "", "")
	if err != nil {
		t.Fatalf("ListAppointments failed: %v", err)
	}
	if list == nil {
		t.Error("Expected non-nil empty slice")
	}
	if len(list) != 0 {
		t.Errorf("Expected 0 appointments, got %d", len(list))
	}
}

func TestMarkNotified(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateAppointment(db, Appointment{
		Title:          "Notify Test",
		DateTime:       "2025-07-01T10:00:00Z",
		NotificationAt: "2025-06-30T10:00:00Z",
		WakeAgent:      true,
	})

	a, _ := GetAppointment(db, id)
	if a.Notified {
		t.Error("Should not be notified initially")
	}

	err := MarkNotified(db, id)
	if err != nil {
		t.Fatalf("MarkNotified failed: %v", err)
	}

	a, _ = GetAppointment(db, id)
	if !a.Notified {
		t.Error("Should be notified after MarkNotified")
	}
}

func TestCreateTodo(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, err := CreateTodo(db, Todo{
		Title:       "Buy groceries",
		Description: "Milk, eggs, bread",
		Priority:    "high",
		DueDate:     "2025-07-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("CreateTodo failed: %v", err)
	}
	if id == "" {
		t.Error("Expected non-empty ID")
	}
}

func TestCreateTodoDefaults(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateTodo(db, Todo{Title: "Minimal"})

	todo, _ := GetTodo(db, id)
	if todo.Status != "open" {
		t.Errorf("Expected default status 'open', got %q", todo.Status)
	}
	if todo.Priority != "medium" {
		t.Errorf("Expected default priority 'medium', got %q", todo.Priority)
	}
	if todo.ProgressPercent != 0 {
		t.Errorf("Expected default progress 0, got %d", todo.ProgressPercent)
	}
}

func TestCreateTodoValidation(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	tests := []struct {
		name string
		todo Todo
	}{
		{"empty title", Todo{}},
		{"invalid status", Todo{Title: "Test", Status: "cancelled"}},
		{"invalid priority", Todo{Title: "Test", Priority: "urgent"}},
		{"invalid due_date", Todo{Title: "Test", DueDate: "not-a-date"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CreateTodo(db, tt.todo)
			if err == nil {
				t.Error("Expected error but got nil")
			}
		})
	}
}

func TestGetTodo(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateTodo(db, Todo{
		Title:    "Test Todo",
		Priority: "low",
	})

	todo, err := GetTodo(db, id)
	if err != nil {
		t.Fatalf("GetTodo failed: %v", err)
	}
	if todo.Title != "Test Todo" {
		t.Errorf("Expected title 'Test Todo', got %q", todo.Title)
	}
	if todo.Priority != "low" {
		t.Errorf("Expected priority 'low', got %q", todo.Priority)
	}
	if todo.KGNodeID != "todo_"+id {
		t.Errorf("Expected KGNodeID 'todo_%s', got %q", id, todo.KGNodeID)
	}
}

func TestCreateTodoWithItemsComputesProgress(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, err := CreateTodo(db, Todo{
		Title:       "Launch checklist",
		RemindDaily: true,
		Items: []TodoItem{
			{Title: "Prepare assets", IsDone: true},
			{Title: "Deploy preview"},
			{Title: "Final QA"},
		},
	})
	if err != nil {
		t.Fatalf("CreateTodo() error = %v", err)
	}

	todo, err := GetTodo(db, id)
	if err != nil {
		t.Fatalf("GetTodo() error = %v", err)
	}
	if !todo.RemindDaily {
		t.Fatal("RemindDaily should persist")
	}
	if len(todo.Items) != 3 {
		t.Fatalf("len(todo.Items) = %d, want 3", len(todo.Items))
	}
	if todo.ItemCount != 3 || todo.DoneItemCount != 1 {
		t.Fatalf("counts = %d/%d, want 3/1", todo.ItemCount, todo.DoneItemCount)
	}
	if todo.ProgressPercent != 33 {
		t.Fatalf("ProgressPercent = %d, want 33", todo.ProgressPercent)
	}
	if todo.Status != "in_progress" {
		t.Fatalf("Status = %q, want in_progress", todo.Status)
	}
}

func TestGetTodoNotFound(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	_, err := GetTodo(db, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent todo")
	}
}

func TestUpdateTodo(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateTodo(db, Todo{Title: "Original"})

	todo, _ := GetTodo(db, id)
	todo.Title = "Updated"
	todo.Status = "in_progress"
	todo.Priority = "high"
	err := UpdateTodo(db, *todo)
	if err != nil {
		t.Fatalf("UpdateTodo failed: %v", err)
	}

	updated, _ := GetTodo(db, id)
	if updated.Title != "Updated" {
		t.Errorf("Expected 'Updated', got %q", updated.Title)
	}
	if updated.Status != "in_progress" {
		t.Errorf("Expected 'in_progress', got %q", updated.Status)
	}
	if updated.Priority != "high" {
		t.Errorf("Expected 'high', got %q", updated.Priority)
	}
}

func TestUpdateTodoDoneCompletesItems(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, err := CreateTodo(db, Todo{
		Title: "Ship feature",
		Items: []TodoItem{
			{Title: "Backend"},
			{Title: "Frontend", IsDone: true},
		},
	})
	if err != nil {
		t.Fatalf("CreateTodo() error = %v", err)
	}

	todo, err := GetTodo(db, id)
	if err != nil {
		t.Fatalf("GetTodo() error = %v", err)
	}
	todo.Status = "done"
	if err := UpdateTodo(db, *todo); err != nil {
		t.Fatalf("UpdateTodo() error = %v", err)
	}

	updated, err := GetTodo(db, id)
	if err != nil {
		t.Fatalf("GetTodo() error = %v", err)
	}
	if updated.CompletedAt == "" {
		t.Fatal("CompletedAt should be set when todo is done")
	}
	if updated.ProgressPercent != 100 {
		t.Fatalf("ProgressPercent = %d, want 100", updated.ProgressPercent)
	}
	for _, item := range updated.Items {
		if !item.IsDone {
			t.Fatalf("item %q should be done", item.Title)
		}
	}
}

func TestUpdateTodoInvalidStatus(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateTodo(db, Todo{Title: "Test"})

	todo, _ := GetTodo(db, id)
	todo.Status = "cancelled"
	err := UpdateTodo(db, *todo)
	if err == nil {
		t.Error("Expected error for cancelled status on todo")
	}
}

func TestDeleteTodo(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateTodo(db, Todo{Title: "Delete Me"})

	err := DeleteTodo(db, id)
	if err != nil {
		t.Fatalf("DeleteTodo failed: %v", err)
	}

	_, err = GetTodo(db, id)
	if err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestDeleteTodoNotFound(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	err := DeleteTodo(db, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent todo")
	}
}

func TestListTodos(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	CreateTodo(db, Todo{Title: "Low Priority", Priority: "low"})
	CreateTodo(db, Todo{Title: "High Priority Task", Priority: "high"})
	CreateTodo(db, Todo{Title: "Medium", Priority: "medium", Status: "done"})

	all, err := ListTodos(db, "", "")
	if err != nil {
		t.Fatalf("ListTodos failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Expected 3 todos, got %d", len(all))
	}

	if all[0].Priority != "high" {
		t.Errorf("Expected first todo to be 'high' priority, got %q", all[0].Priority)
	}

	filtered, _ := ListTodos(db, "", "done")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 done todo, got %d", len(filtered))
	}

	searched, _ := ListTodos(db, "task", "")
	if len(searched) != 1 {
		t.Errorf("Expected 1 result for 'task', got %d", len(searched))
	}
}

func TestAddUpdateDeleteTodoItem(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, err := CreateTodo(db, Todo{Title: "Checklist"})
	if err != nil {
		t.Fatalf("CreateTodo() error = %v", err)
	}

	itemID, err := AddTodoItem(db, id, TodoItem{Title: "First step"})
	if err != nil {
		t.Fatalf("AddTodoItem() error = %v", err)
	}
	todo, err := GetTodo(db, id)
	if err != nil {
		t.Fatalf("GetTodo() error = %v", err)
	}
	if len(todo.Items) != 1 {
		t.Fatalf("len(todo.Items) = %d, want 1", len(todo.Items))
	}

	item := todo.Items[0]
	item.ID = itemID
	item.TodoID = id
	item.IsDone = true
	item.Title = "Updated step"
	if err := UpdateTodoItem(db, item); err != nil {
		t.Fatalf("UpdateTodoItem() error = %v", err)
	}

	updated, err := GetTodo(db, id)
	if err != nil {
		t.Fatalf("GetTodo() error = %v", err)
	}
	if updated.DoneItemCount != 1 || updated.ProgressPercent != 100 || updated.Status != "done" {
		t.Fatalf("updated todo = status:%q done:%d progress:%d, want done/1/100", updated.Status, updated.DoneItemCount, updated.ProgressPercent)
	}

	if err := DeleteTodoItem(db, id, itemID); err != nil {
		t.Fatalf("DeleteTodoItem() error = %v", err)
	}
	withoutItem, err := GetTodo(db, id)
	if err != nil {
		t.Fatalf("GetTodo() error = %v", err)
	}
	if len(withoutItem.Items) != 0 {
		t.Fatalf("len(withoutItem.Items) = %d, want 0", len(withoutItem.Items))
	}
}

func TestCompleteTodoOptionallyCompletesItems(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, err := CreateTodo(db, Todo{
		Title: "Release",
		Items: []TodoItem{
			{Title: "Docs"},
			{Title: "Tag release"},
		},
	})
	if err != nil {
		t.Fatalf("CreateTodo() error = %v", err)
	}

	if err := CompleteTodo(db, id, true); err != nil {
		t.Fatalf("CompleteTodo() error = %v", err)
	}
	todo, err := GetTodo(db, id)
	if err != nil {
		t.Fatalf("GetTodo() error = %v", err)
	}
	if todo.Status != "done" || todo.ProgressPercent != 100 {
		t.Fatalf("todo status/progress = %q/%d, want done/100", todo.Status, todo.ProgressPercent)
	}
	for _, item := range todo.Items {
		if !item.IsDone {
			t.Fatalf("item %q should be done", item.Title)
		}
	}
}

func TestClaimDailyTodoReminderTodosOnlyOncePerDay(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	todoID, err := CreateTodo(db, Todo{
		Title:       "Morning routine",
		Priority:    "high",
		Status:      "open",
		RemindDaily: true,
		Description: "Daily check",
		Items: []TodoItem{
			{Title: "Check backups"},
			{Title: "Review logs", IsDone: true},
		},
	})
	if err != nil {
		t.Fatalf("CreateTodo() error = %v", err)
	}
	if _, err := CreateTodo(db, Todo{
		Title:       "Silent task",
		Priority:    "medium",
		Status:      "open",
		RemindDaily: false,
	}); err != nil {
		t.Fatalf("CreateTodo() second todo error = %v", err)
	}

	now := time.Date(2026, 4, 16, 9, 30, 0, 0, time.UTC)
	first, err := ClaimDailyTodoReminderTodos(db, now)
	if err != nil {
		t.Fatalf("ClaimDailyTodoReminderTodos() first error = %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("len(first) = %d, want 1", len(first))
	}
	if first[0].ID != todoID {
		t.Fatalf("first todo id = %q, want %q", first[0].ID, todoID)
	}
	if first[0].ItemCount != 2 || first[0].DoneItemCount != 1 {
		t.Fatalf("first counts = %d/%d, want 2/1", first[0].ItemCount, first[0].DoneItemCount)
	}

	second, err := ClaimDailyTodoReminderTodos(db, now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("ClaimDailyTodoReminderTodos() second error = %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("len(second) = %d, want 0", len(second))
	}

	stored, err := GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("GetTodo() error = %v", err)
	}
	if stored.LastDailyReminderAt == "" {
		t.Fatal("expected last_daily_reminder_at to be set")
	}

	third, err := ClaimDailyTodoReminderTodos(db, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("ClaimDailyTodoReminderTodos() third error = %v", err)
	}
	if len(third) != 1 {
		t.Fatalf("len(third) = %d, want 1", len(third))
	}
	if got := BuildDailyTodoReminderText(third); !strings.Contains(got, "Morning routine") || !strings.Contains(got, "Check backups") {
		t.Fatalf("BuildDailyTodoReminderText() = %q, want todo title and open item", got)
	}
}

func TestListTodosEmpty(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	list, err := ListTodos(db, "", "")
	if err != nil {
		t.Fatalf("ListTodos failed: %v", err)
	}
	if list == nil {
		t.Error("Expected non-nil empty slice")
	}
}

type mockKG struct {
	nodes   map[string]mockNode
	deleted []string
	addErr  error
}

type mockNode struct {
	id    string
	name  string
	props map[string]string
}

func newMockKG() *mockKG {
	return &mockKG{nodes: make(map[string]mockNode)}
}

func (m *mockKG) AddNode(id, name string, properties map[string]string) error {
	if m.addErr != nil {
		return m.addErr
	}
	m.nodes[id] = mockNode{id: id, name: name, props: properties}
	return nil
}

func (m *mockKG) DeleteNode(id string) error {
	m.deleted = append(m.deleted, id)
	delete(m.nodes, id)
	return nil
}

func TestSyncAppointmentToKG(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateAppointment(db, Appointment{
		Title:       "KG Test",
		Description: "Test description",
		DateTime:    "2025-07-01T10:00:00Z",
	})

	kg := newMockKG()
	err := SyncAppointmentToKG(kg, db, id)
	if err != nil {
		t.Fatalf("SyncAppointmentToKG failed: %v", err)
	}

	nodeID := "appointment_" + id
	node, ok := kg.nodes[nodeID]
	if !ok {
		t.Fatal("Expected node in KG")
	}
	if node.name != "KG Test" {
		t.Errorf("Expected name 'KG Test', got %q", node.name)
	}
	if node.props["type"] != "event" {
		t.Error("Expected type=event")
	}
	if node.props["source"] != "planner" {
		t.Error("Expected source=planner")
	}
	if node.props["description"] != "Test description" {
		t.Error("Expected description in props")
	}
}

func TestSyncAppointmentToKGNoDescription(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateAppointment(db, Appointment{
		Title:    "No Desc",
		DateTime: "2025-07-01T10:00:00Z",
	})

	kg := newMockKG()
	SyncAppointmentToKG(kg, db, id)

	nodeID := "appointment_" + id
	node := kg.nodes[nodeID]
	if _, has := node.props["description"]; has {
		t.Error("Expected no description in props when empty")
	}
}

func TestSyncTodoToKG(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateTodo(db, Todo{
		Title:       "KG Todo",
		Description: "Todo desc",
		Priority:    "high",
		DueDate:     "2025-08-01T00:00:00Z",
	})

	kg := newMockKG()
	err := SyncTodoToKG(kg, db, id)
	if err != nil {
		t.Fatalf("SyncTodoToKG failed: %v", err)
	}

	nodeID := "todo_" + id
	node, ok := kg.nodes[nodeID]
	if !ok {
		t.Fatal("Expected node in KG")
	}
	if node.props["type"] != "task" {
		t.Error("Expected type=task")
	}
	if node.props["priority"] != "high" {
		t.Error("Expected priority=high")
	}
	if node.props["due_date"] != "2025-08-01T00:00:00Z" {
		t.Error("Expected due_date in props")
	}
}

func TestSyncToKGNilKG(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateAppointment(db, Appointment{
		Title:    "Nil KG",
		DateTime: "2025-07-01T10:00:00Z",
	})

	err := SyncAppointmentToKG(nil, db, id)
	if err != nil {
		t.Errorf("Expected nil error with nil KG, got %v", err)
	}
}

func TestSyncToKGNilDB(t *testing.T) {
	kg := newMockKG()
	err := SyncAppointmentToKG(kg, nil, "some-id")
	if err != nil {
		t.Errorf("Expected nil error with nil DB, got %v", err)
	}
}

func TestToJSON(t *testing.T) {
	result := ToJSON(map[string]string{"status": "ok"})
	if result != `{"status":"ok"}` {
		t.Errorf("Unexpected JSON: %s", result)
	}
}

func TestToJSONWithHTML(t *testing.T) {
	result := ToJSON(map[string]string{"msg": "<script>alert('xss')</script>"})
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

// ── New tests for bug fixes and new features ──

func TestClaimNotification(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateAppointment(db, Appointment{
		Title:          "Claim Test",
		DateTime:       "2025-08-01T10:00:00Z",
		NotificationAt: "2025-07-31T10:00:00Z",
		WakeAgent:      true,
	})

	// First claim should succeed
	claimed, err := ClaimNotification(db, id)
	if err != nil {
		t.Fatalf("ClaimNotification failed: %v", err)
	}
	if !claimed {
		t.Error("Expected first claim to succeed")
	}

	// Second claim should fail (already notified)
	claimed2, err := ClaimNotification(db, id)
	if err != nil {
		t.Fatalf("Second ClaimNotification failed: %v", err)
	}
	if claimed2 {
		t.Error("Expected second claim to fail (already claimed)")
	}

	// Verify DB state
	a, _ := GetAppointment(db, id)
	if !a.Notified {
		t.Error("Appointment should be marked as notified after successful claim")
	}
}

func TestUpdateAppointmentResetsNotifiedOnReschedule(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateAppointment(db, Appointment{
		Title:          "Reschedule Test",
		DateTime:       "2025-08-01T10:00:00Z",
		NotificationAt: "2025-07-31T09:00:00Z",
		WakeAgent:      true,
	})

	// Mark as notified
	_, _ = ClaimNotification(db, id)
	a, _ := GetAppointment(db, id)
	if !a.Notified {
		t.Fatal("Setup: should be notified")
	}

	// Reschedule: change notification_at to a new time
	a.NotificationAt = "2025-08-15T09:00:00Z"
	if err := UpdateAppointment(db, *a); err != nil {
		t.Fatalf("UpdateAppointment failed: %v", err)
	}

	// notified should be reset to false
	updated, _ := GetAppointment(db, id)
	if updated.Notified {
		t.Error("BUG-3: notified should be reset to false after rescheduling notification_at")
	}
}

func TestUpdateAppointmentKeepsNotifiedWhenNotificationUnchanged(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateAppointment(db, Appointment{
		Title:          "No-Reschedule Test",
		DateTime:       "2025-08-01T10:00:00Z",
		NotificationAt: "2025-07-31T09:00:00Z",
		WakeAgent:      true,
	})
	_, _ = ClaimNotification(db, id)

	// Update title only, keep notification_at unchanged
	a, _ := GetAppointment(db, id)
	a.Title = "Updated Title"
	if err := UpdateAppointment(db, *a); err != nil {
		t.Fatalf("UpdateAppointment failed: %v", err)
	}

	updated, _ := GetAppointment(db, id)
	if !updated.Notified {
		t.Error("notified should remain true when notification_at is unchanged")
	}
}

func TestUpdateAppointmentValidatesDateFormat(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	id, _ := CreateAppointment(db, Appointment{
		Title:    "Validation Test",
		DateTime: "2025-08-01T10:00:00Z",
	})

	a, _ := GetAppointment(db, id)
	a.DateTime = "not-a-date"
	if err := UpdateAppointment(db, *a); err == nil {
		t.Error("ISSUE-5: UpdateAppointment should reject invalid date_time format")
	}

	a.DateTime = "2025-08-01T10:00:00Z"
	a.NotificationAt = "2025-13-01T00:00:00Z" // invalid month
	if err := UpdateAppointment(db, *a); err == nil {
		t.Error("ISSUE-5: UpdateAppointment should reject invalid notification_at format")
	}
}

func TestNormalizeDateInput(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2025-03-15T00:00:00Z", "2025-03-15T00:00:00Z"}, // already RFC3339, unchanged
		{"2025-03-15", "2025-03-15T00:00:00Z"},           // date-only → normalized
		{"", ""},                                         // empty → empty
		{"not-a-date", "not-a-date"},                     // invalid → unchanged (validation catches)
	}
	for _, tt := range tests {
		got := normalizeDateInput(tt.input)
		if got != tt.want {
			t.Errorf("normalizeDateInput(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCreateTodoDueDateNormalization(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// date-only format should be accepted and normalized
	id, err := CreateTodo(db, Todo{
		Title:   "Date normalization test",
		DueDate: "2025-03-15",
	})
	if err != nil {
		t.Fatalf("ISSUE-8: CreateTodo should accept date-only format, got error: %v", err)
	}

	todo, _ := GetTodo(db, id)
	if todo.DueDate != "2025-03-15T00:00:00Z" {
		t.Errorf("DueDate should be normalized to RFC3339, got %q", todo.DueDate)
	}
}
