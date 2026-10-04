package virtualcomputers

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestTaskManagerOwnerRevocationClosesSocketAndRejectsLateEvents(t *testing.T) {
	connected := make(chan struct{})
	closed := make(chan struct{})
	upgrader := websocket.Upgrader{}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		close(connected)
		_, _, _ = conn.ReadMessage()
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"done","text":"late"}`))
		close(closed)
	}))
	defer origin.Close()
	m, err := OpenTaskManager(filepath.Join(t.TempDir(), "tasks.db"), slog.Default(), TaskManagerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	client, err := NewClient(ClientConfig{BaseURL: origin.URL, Token: "fixture", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	task, done, err := m.SubmitContext(ctx, client, "vm-1", AgentTaskKindDesktop, "test")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("no connection")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("task survived revocation")
	}
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("socket survived revocation")
	}
	result, ok := m.GetTask(task.ID)
	if !ok || result.Status == AgentTaskStatusCompleted || len(result.Events) != 0 {
		t.Fatalf("late success: %+v", result)
	}
}
