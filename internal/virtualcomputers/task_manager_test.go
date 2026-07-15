package virtualcomputers

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestTaskManagerRunsWebSocketJobAndPersistsEvents(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/machines/vm-1/shell-agent" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("goal") != "build a small app" {
			t.Fatalf("goal = %q", r.URL.Query().Get("goal"))
		}
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close()
		for _, event := range []string{
			`{"type":"say","text":"Starting now"}`,
			`{"type":"action","text":"$ node server.js"}`,
			`{"type":"preview","text":"8080"}`,
			`{"type":"done","text":"Done"}`,
		} {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(event)); err != nil {
				t.Fatalf("write event: %v", err)
			}
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{BaseURL: server.URL, Token: "secret-token", Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	path := t.TempDir() + "/virtual_computers.db"
	mgr, err := OpenTaskManager(path, slog.Default(), TaskManagerOptions{MaxConcurrent: 2, Timeout: 2 * time.Second, Retention: 30 * 24 * time.Hour})
	if err != nil {
		t.Fatalf("OpenTaskManager: %v", err)
	}
	task, err := mgr.Submit(client, "vm-1", AgentTaskKindShell, "build a small app")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	completed := waitForAgentTask(t, mgr, task.ID, AgentTaskStatusCompleted)
	if completed.PreviewPort != 8080 || len(completed.Events) != 4 {
		t.Fatalf("completed task = %+v", completed)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenTaskManager(path, slog.Default(), TaskManagerOptions{})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	persisted, ok := reopened.GetTask(task.ID)
	if !ok || persisted.Status != AgentTaskStatusCompleted || len(persisted.Events) != 4 {
		t.Fatalf("persisted task = %+v, ok=%v", persisted, ok)
	}
}

func TestTaskManagerMarksUnfinishedJobsInterruptedOnOpen(t *testing.T) {
	path := t.TempDir() + "/virtual_computers.db"
	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	now := time.Now().UTC()
	for _, status := range []string{AgentTaskStatusQueued, AgentTaskStatusRunning} {
		if err := ledger.InsertAgentTask(context.Background(), AgentTask{
			ID: "task-" + status, MachineID: "vm-1", Kind: AgentTaskKindShell,
			Instruction: "do work", Status: status, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertAgentTask(%s): %v", status, err)
		}
	}
	_ = ledger.Close()

	mgr, err := OpenTaskManager(path, slog.Default(), TaskManagerOptions{})
	if err != nil {
		t.Fatalf("OpenTaskManager: %v", err)
	}
	defer mgr.Close()
	for _, id := range []string{"task-queued", "task-running"} {
		task, ok := mgr.GetTask(id)
		if !ok || task.Status != AgentTaskStatusInterrupted || !strings.Contains(task.Error, "restart") {
			t.Fatalf("task %s = %+v, ok=%v", id, task, ok)
		}
	}
}

func TestTaskManagerCancelKeepsCanceledTerminalState(t *testing.T) {
	connected := make(chan struct{})
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		close(connected)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	client, _ := NewClient(ClientConfig{BaseURL: server.URL, Timeout: time.Second})
	mgr, err := OpenTaskManager(t.TempDir()+"/virtual_computers.db", slog.Default(), TaskManagerOptions{Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("OpenTaskManager: %v", err)
	}
	defer mgr.Close()
	task, err := mgr.Submit(client, "vm-1", AgentTaskKindDesktop, "open settings")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	select {
	case <-connected:
	case <-time.After(time.Second):
		t.Fatal("agent websocket did not connect")
	}
	if !mgr.CancelTask(task.ID) {
		t.Fatal("CancelTask returned false")
	}
	done := make(chan struct{})
	go func() {
		mgr.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cancel did not close the active websocket")
	}
	canceled, ok := mgr.GetTask(task.ID)
	if !ok || canceled.Status != AgentTaskStatusCanceled {
		t.Fatalf("canceled task = %+v, ok=%v", canceled, ok)
	}
}

func TestTaskManagerCancelsOrphanedActiveState(t *testing.T) {
	mgr, err := OpenTaskManager(t.TempDir()+"/virtual_computers.db", slog.Default(), TaskManagerOptions{})
	if err != nil {
		t.Fatalf("OpenTaskManager: %v", err)
	}
	defer mgr.Close()

	now := time.Now().UTC()
	for _, status := range []string{AgentTaskStatusQueued, AgentTaskStatusRunning} {
		id := "orphan-" + status
		if err := mgr.ledger.InsertAgentTask(context.Background(), AgentTask{
			ID: id, MachineID: "vm-gone", Kind: AgentTaskKindShell,
			Instruction: "do work", Status: status, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertAgentTask(%s): %v", status, err)
		}
		if !mgr.CancelTask(id) {
			t.Fatalf("CancelTask(%s) returned false", id)
		}
		task, ok := mgr.GetTask(id)
		if !ok || task.Status != AgentTaskStatusCanceled {
			t.Fatalf("canceled orphan %s = %+v, ok=%v", id, task, ok)
		}
	}

	terminal := AgentTask{
		ID: "already-complete", MachineID: "vm-gone", Kind: AgentTaskKindShell,
		Instruction: "done", Status: AgentTaskStatusCompleted, CreatedAt: now, UpdatedAt: now,
	}
	if err := mgr.ledger.InsertAgentTask(context.Background(), terminal); err != nil {
		t.Fatalf("InsertAgentTask(terminal): %v", err)
	}
	if mgr.CancelTask(terminal.ID) {
		t.Fatal("CancelTask accepted a terminal task")
	}
}

func TestTaskManagerPersistsErrorAndTimeoutTerminalStates(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		if r.URL.Query().Get("goal") == "fail" {
			_ = conn.WriteJSON(map[string]string{"type": "error", "text": "upstream failed"})
			return
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	client, _ := NewClient(ClientConfig{BaseURL: server.URL, Timeout: time.Second})

	failing, err := OpenTaskManager(t.TempDir()+"/failure.db", slog.Default(), TaskManagerOptions{Timeout: time.Second})
	if err != nil {
		t.Fatalf("OpenTaskManager failure: %v", err)
	}
	task, err := failing.Submit(client, "vm-1", AgentTaskKindShell, "fail")
	if err != nil {
		t.Fatalf("Submit failure: %v", err)
	}
	failed := waitForAgentTask(t, failing, task.ID, AgentTaskStatusFailed)
	if failed.Error != "upstream failed" {
		t.Fatalf("failed task = %+v", failed)
	}
	_ = failing.Close()

	timingOut, err := OpenTaskManager(t.TempDir()+"/timeout.db", slog.Default(), TaskManagerOptions{Timeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("OpenTaskManager timeout: %v", err)
	}
	defer timingOut.Close()
	task, err = timingOut.Submit(client, "vm-1", AgentTaskKindShell, "wait")
	if err != nil {
		t.Fatalf("Submit timeout: %v", err)
	}
	timedOut := waitForAgentTask(t, timingOut, task.ID, AgentTaskStatusFailed)
	if !strings.Contains(timedOut.Error, "timed out") {
		t.Fatalf("timed out task = %+v", timedOut)
	}
}

func TestAgentTaskEventLimitsAndRetention(t *testing.T) {
	path := t.TempDir() + "/virtual_computers.db"
	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	now := time.Now().UTC()
	task := AgentTask{ID: "limited", MachineID: "vm-1", Kind: AgentTaskKindShell, Instruction: "work", Status: AgentTaskStatusRunning, CreatedAt: now, UpdatedAt: now}
	if err := ledger.InsertAgentTask(context.Background(), task); err != nil {
		t.Fatalf("InsertAgentTask: %v", err)
	}
	for i := 0; i < maxAgentTaskEvents+1; i++ {
		if err := ledger.AppendAgentTaskEvent(context.Background(), task.ID, "say", "event", maxAgentTaskEvents, maxAgentTaskEventBytes); err != nil {
			t.Fatalf("AppendAgentTaskEvent %d: %v", i, err)
		}
	}
	limited, ok, err := ledger.GetAgentTask(context.Background(), task.ID)
	if err != nil || !ok || len(limited.Events) != maxAgentTaskEvents || !limited.EventsTruncated {
		t.Fatalf("limited task = %+v ok=%v err=%v", limited, ok, err)
	}
	old := now.Add(-31 * 24 * time.Hour)
	if _, err := ledger.db.Exec(`UPDATE agent_tasks SET status = ?, completed_at = ? WHERE id = ?`, AgentTaskStatusCompleted, timeText(old), task.ID); err != nil {
		t.Fatalf("age task: %v", err)
	}
	_ = ledger.Close()
	mgr, err := OpenTaskManager(path, slog.Default(), TaskManagerOptions{Retention: 30 * 24 * time.Hour})
	if err != nil {
		t.Fatalf("OpenTaskManager: %v", err)
	}
	defer mgr.Close()
	if _, ok := mgr.GetTask(task.ID); ok {
		t.Fatal("expired task history was not removed")
	}
}

func waitForAgentTask(t *testing.T, mgr *TaskManager, id, wantStatus string) AgentTask {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if task, ok := mgr.GetTask(id); ok && task.Status == wantStatus {
			return task
		}
		time.Sleep(10 * time.Millisecond)
	}
	task, _ := mgr.GetTask(id)
	t.Fatalf("task %s did not reach %s: %+v", id, wantStatus, task)
	return AgentTask{}
}
