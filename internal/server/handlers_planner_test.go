package server

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/contacts"
	"aurago/internal/planner"
)

func testPlannerServer(t *testing.T) (*Server, *sql.DB) {
	t.Helper()
	db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatalf("planner.InitDB() error = %v", err)
	}
	return &Server{PlannerDB: db, Logger: slog.Default()}, db
}

func TestRunPlannerKGSyncAsyncReturnsImmediately(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})

	start := time.Now()
	runPlannerKGSyncAsync(slog.Default(), func() error {
		close(started)
		<-release
		close(done)
		return nil
	}, "test sync")
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Fatalf("runPlannerKGSyncAsync blocked for %s", elapsed)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("async sync did not start")
	}

	close(release)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("async sync did not complete")
	}
}

func TestHandleAppointmentsPersistsAndEnrichesContactParticipants(t *testing.T) {
	t.Parallel()

	server, plannerDB := testPlannerServer(t)
	defer plannerDB.Close()
	contactsDB, err := contacts.InitDB(filepath.Join(t.TempDir(), "contacts.db"))
	if err != nil {
		t.Fatalf("contacts.InitDB() error = %v", err)
	}
	defer contactsDB.Close()
	server.ContactsDB = contactsDB

	adaID, err := contacts.Create(contactsDB, contacts.Contact{Name: "Ada Lovelace", Email: "ada@example.com"})
	if err != nil {
		t.Fatalf("contacts.Create(Ada) error = %v", err)
	}
	graceID, err := contacts.Create(contactsDB, contacts.Contact{Name: "Grace Hopper", Email: "grace@example.com"})
	if err != nil {
		t.Fatalf("contacts.Create(Grace) error = %v", err)
	}

	createBody, err := json.Marshal(planner.Appointment{
		Title:      "Planning sync",
		DateTime:   "2099-05-01T10:00:00Z",
		ContactIDs: []string{adaID},
	})
	if err != nil {
		t.Fatalf("json.Marshal(create appointment) error = %v", err)
	}
	createReq := httptest.NewRequest(http.MethodPost, "/api/appointments", strings.NewReader(string(createBody)))
	createRec := httptest.NewRecorder()
	handleAppointments(server).ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	var created map[string]string
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create response) error = %v", err)
	}
	appointmentID := created["id"]
	if appointmentID == "" {
		t.Fatalf("create response missing id: %#v", created)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/appointments/"+appointmentID, nil)
	getRec := httptest.NewRecorder()
	handleAppointmentByID(server).ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	var appointment planner.Appointment
	if err := json.Unmarshal(getRec.Body.Bytes(), &appointment); err != nil {
		t.Fatalf("json.Unmarshal(get response) error = %v", err)
	}
	if len(appointment.ContactIDs) != 1 || appointment.ContactIDs[0] != adaID {
		t.Fatalf("get contact_ids = %#v, want [%s]", appointment.ContactIDs, adaID)
	}
	if len(appointment.Participants) != 1 || appointment.Participants[0].Name != "Ada Lovelace" {
		t.Fatalf("get participants = %#v, want Ada Lovelace", appointment.Participants)
	}

	updateBody, err := json.Marshal(map[string]interface{}{"contact_ids": []string{graceID}})
	if err != nil {
		t.Fatalf("json.Marshal(update appointment) error = %v", err)
	}
	updateReq := httptest.NewRequest(http.MethodPut, "/api/appointments/"+appointmentID, strings.NewReader(string(updateBody)))
	updateRec := httptest.NewRecorder()
	handleAppointmentByID(server).ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d; body=%s", updateRec.Code, http.StatusOK, updateRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/appointments", nil)
	listRec := httptest.NewRecorder()
	handleAppointments(server).ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	var appointments []planner.Appointment
	if err := json.Unmarshal(listRec.Body.Bytes(), &appointments); err != nil {
		t.Fatalf("json.Unmarshal(list response) error = %v", err)
	}
	if len(appointments) != 1 {
		t.Fatalf("list appointments = %#v, want one", appointments)
	}
	if len(appointments[0].ContactIDs) != 1 || appointments[0].ContactIDs[0] != graceID {
		t.Fatalf("list contact_ids = %#v, want [%s]", appointments[0].ContactIDs, graceID)
	}
	if len(appointments[0].Participants) != 1 || appointments[0].Participants[0].Name != "Grace Hopper" {
		t.Fatalf("list participants = %#v, want Grace Hopper", appointments[0].Participants)
	}
}

func TestHandleTodoByIDUpdatesRemindDailyAndItems(t *testing.T) {
	t.Parallel()

	server, db := testPlannerServer(t)
	defer db.Close()

	todoID, err := planner.CreateTodo(db, planner.Todo{
		Title:    "Launch checklist",
		Priority: "medium",
		Status:   "open",
	})
	if err != nil {
		t.Fatalf("planner.CreateTodo() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/todos/"+todoID, strings.NewReader(`{
		"remind_daily": true,
		"items": [
			{"title":"Draft release notes"},
			{"title":"Ship build","is_done":true}
		]
	}`))
	rec := httptest.NewRecorder()

	handleTodoByID(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	updated, err := planner.GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("planner.GetTodo() error = %v", err)
	}
	if !updated.RemindDaily {
		t.Fatal("RemindDaily = false, want true")
	}
	if updated.ItemCount != 2 || updated.DoneItemCount != 1 {
		t.Fatalf("items = %d/%d, want 2/1", updated.ItemCount, updated.DoneItemCount)
	}
	if updated.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", updated.Status)
	}
}

func TestHandleTodosCreatesTodoWithChecklist(t *testing.T) {
	t.Parallel()

	server, db := testPlannerServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/todos", strings.NewReader(`{
		"title": "Planning task",
		"description": "Prepare next milestone",
		"priority": "high",
		"status": "open",
		"remind_daily": true,
		"items": [
			{"title": "Define scope"},
			{"title": "Draft tasks", "is_done": true}
		]
	}`))
	rec := httptest.NewRecorder()

	handleTodos(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	todoID := payload["id"]
	if todoID == "" {
		t.Fatal("missing todo id in create response")
	}

	todo, err := planner.GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("planner.GetTodo() error = %v", err)
	}
	if !todo.RemindDaily {
		t.Fatal("RemindDaily = false, want true")
	}
	if todo.ItemCount != 2 || todo.DoneItemCount != 1 {
		t.Fatalf("items = %d/%d, want 2/1", todo.ItemCount, todo.DoneItemCount)
	}
	if todo.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", todo.Status)
	}
}

func TestHandleTodosListReturnsCreatedChecklistTodo(t *testing.T) {
	t.Parallel()

	server, db := testPlannerServer(t)
	defer db.Close()

	createReq := httptest.NewRequest(http.MethodPost, "/api/todos", strings.NewReader(`{
		"title": "Visible task",
		"priority": "medium",
		"status": "open",
		"items": [{"title":"First point"}]
	}`))
	createRec := httptest.NewRecorder()
	handleTodos(server).ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/todos", nil)
	listRec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handleTodos(server).ServeHTTP(listRec, listReq)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("GET /api/todos blocked after creating checklist todo")
	}

	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}

	var todos []planner.Todo
	if err := json.Unmarshal(listRec.Body.Bytes(), &todos); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("len(todos) = %d, want 1; body=%s", len(todos), listRec.Body.String())
	}
	if todos[0].Title != "Visible task" {
		t.Fatalf("title = %q, want %q", todos[0].Title, "Visible task")
	}
	if todos[0].ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1", todos[0].ItemCount)
	}
}

func TestHandleTodoItemsEndpointsManageChecklist(t *testing.T) {
	t.Parallel()

	server, db := testPlannerServer(t)
	defer db.Close()

	todoID, err := planner.CreateTodo(db, planner.Todo{
		Title:    "Server rollout",
		Priority: "high",
		Status:   "open",
	})
	if err != nil {
		t.Fatalf("planner.CreateTodo() error = %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/todos/"+todoID+"/items", strings.NewReader(`{"title":"Smoke test"}`))
	createRec := httptest.NewRecorder()
	handleTodoByID(server).ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	var created map[string]string
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	itemID := created["id"]
	if itemID == "" {
		t.Fatal("item id missing from response")
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/todos/"+todoID+"/items/"+itemID, strings.NewReader(`{"is_done":true}`))
	updateRec := httptest.NewRecorder()
	handleTodoByID(server).ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d; body=%s", updateRec.Code, http.StatusOK, updateRec.Body.String())
	}

	todo, err := planner.GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("planner.GetTodo() error = %v", err)
	}
	if todo.Status != "done" || todo.ProgressPercent != 100 {
		t.Fatalf("status/progress = %q/%d, want done/100", todo.Status, todo.ProgressPercent)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/todos/"+todoID+"/items/"+itemID, nil)
	deleteRec := httptest.NewRecorder()
	handleTodoByID(server).ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d; body=%s", deleteRec.Code, http.StatusOK, deleteRec.Body.String())
	}

	todo, err = planner.GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("planner.GetTodo() error = %v", err)
	}
	if todo.ItemCount != 0 {
		t.Fatalf("ItemCount = %d, want 0", todo.ItemCount)
	}
}

func TestHandleTodoItemCanBeUncheckedAfterTodoDone(t *testing.T) {
	t.Parallel()

	server, db := testPlannerServer(t)
	defer db.Close()

	todoID, err := planner.CreateTodo(db, planner.Todo{
		Title:    "Publish release",
		Priority: "medium",
		Status:   "open",
		Items: []planner.TodoItem{
			{Title: "Tag version"},
		},
	})
	if err != nil {
		t.Fatalf("planner.CreateTodo() error = %v", err)
	}
	todo, err := planner.GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("planner.GetTodo() error = %v", err)
	}
	itemID := todo.Items[0].ID

	completeReq := httptest.NewRequest(http.MethodPost, "/api/todos/"+todoID+"/complete", strings.NewReader(`{"complete_items_too":true}`))
	completeRec := httptest.NewRecorder()
	handleTodoByID(server).ServeHTTP(completeRec, completeReq)
	if completeRec.Code != http.StatusOK {
		t.Fatalf("complete status = %d, want %d; body=%s", completeRec.Code, http.StatusOK, completeRec.Body.String())
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/todos/"+todoID+"/items/"+itemID, strings.NewReader(`{"is_done":false}`))
	updateRec := httptest.NewRecorder()
	handleTodoByID(server).ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("uncheck status = %d, want %d; body=%s", updateRec.Code, http.StatusOK, updateRec.Body.String())
	}

	todo, err = planner.GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("planner.GetTodo() error = %v", err)
	}
	if todo.Status == "done" || todo.ProgressPercent == 100 || todo.Items[0].IsDone {
		t.Fatalf("todo remained complete after uncheck: status=%q progress=%d item_done=%v", todo.Status, todo.ProgressPercent, todo.Items[0].IsDone)
	}
}

func TestHandleTodoCompleteMarksItemsDone(t *testing.T) {
	t.Parallel()

	server, db := testPlannerServer(t)
	defer db.Close()

	todoID, err := planner.CreateTodo(db, planner.Todo{
		Title:    "Ops handover",
		Priority: "medium",
		Status:   "open",
		Items: []planner.TodoItem{
			{Title: "Update docs"},
			{Title: "Notify team"},
		},
	})
	if err != nil {
		t.Fatalf("planner.CreateTodo() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/todos/"+todoID+"/complete", strings.NewReader(`{"complete_items_too":true}`))
	rec := httptest.NewRecorder()
	handleTodoByID(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	todo, err := planner.GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("planner.GetTodo() error = %v", err)
	}
	if todo.Status != "done" || todo.DoneItemCount != 2 {
		t.Fatalf("status/items = %q/%d, want done/2", todo.Status, todo.DoneItemCount)
	}
}

func TestHandleTodoDeleteRemovesChecklistTodo(t *testing.T) {
	t.Parallel()

	server, db := testPlannerServer(t)
	defer db.Close()

	todoID, err := planner.CreateTodo(db, planner.Todo{
		Title:    "Delete me",
		Priority: "medium",
		Status:   "open",
		Items: []planner.TodoItem{
			{Title: "Subtask"},
		},
	})
	if err != nil {
		t.Fatalf("planner.CreateTodo() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/todos/"+todoID, nil)
	rec := httptest.NewRecorder()
	handleTodoByID(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if _, err := planner.GetTodo(db, todoID); err == nil {
		t.Fatal("planner.GetTodo() succeeded after delete, want not found")
	}
}
