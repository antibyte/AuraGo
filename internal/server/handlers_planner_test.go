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
