package remote

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

func TestExecuteRemoteCommand_Timeout(t *testing.T) {
	// This test just ensures the function signature exists and compiles.
	// We use a short timeout and an unreachable address to trigger a quick error.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := ExecuteRemoteCommand(ctx, "127.0.0.1", 2222, "user", []byte("pass"), "ls")
	if err == nil {
		t.Error("Expected error for unreachable host, got nil")
	}
}

func TestRemoteHubAuditCallbackReceivesRemoteEvents(t *testing.T) {
	hub := NewRemoteHub(nil, nil, slog.Default())
	var events []RemoteAuditEvent
	hub.OnAudit = func(event RemoteAuditEvent) {
		events = append(events, event)
	}

	hub.emitAudit(RemoteAuditEvent{
		DeviceID:   "dev-1",
		DeviceName: "Kitchen Node",
		EventType:  "remote_heartbeat",
		Status:     "success",
		Summary:    "Remote heartbeat received",
	})
	hub.emitAudit(RemoteAuditEvent{
		DeviceID:   "dev-1",
		DeviceName: "Kitchen Node",
		EventType:  "remote_command",
		Status:     "error",
		Summary:    "Remote command failed",
		DurationMS: 150,
	})

	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].EventType != "remote_heartbeat" || events[0].Status != "success" {
		t.Fatalf("heartbeat event = %+v", events[0])
	}
	if events[1].EventType != "remote_command" || events[1].DurationMS != 150 {
		t.Fatalf("command event = %+v", events[1])
	}
}

func TestRemoteHubReplacementUnregisterDoesNotRemoveNewConnection(t *testing.T) {
	hub := NewRemoteHub(nil, nil, slog.Default())
	oldServerConn, _, cleanupOld := newWebSocketPairForHubTest(t)
	defer cleanupOld()
	newServerConn, _, cleanupNew := newWebSocketPairForHubTest(t)
	defer cleanupNew()

	oldRemote := &RemoteConnection{Conn: oldServerConn, DeviceID: "dev-1", Name: "old"}
	newRemote := &RemoteConnection{Conn: newServerConn, DeviceID: "dev-1", Name: "new"}

	hub.Register("dev-1", oldRemote)
	hub.Register("dev-1", newRemote)

	if id, conn := hub.FindByConn(newServerConn); id != "dev-1" || conn != newRemote {
		t.Fatalf("FindByConn(new) = (%q, %p), want (dev-1, %p)", id, conn, newRemote)
	}
	if id, conn := hub.FindByConn(oldServerConn); id != "" || conn != nil {
		t.Fatalf("FindByConn(old) = (%q, %p), want empty after replacement", id, conn)
	}

	hub.unregisterConnection("dev-1", oldRemote)
	if got := hub.GetConnection("dev-1"); got != newRemote {
		t.Fatalf("old connection unregister removed replacement: got %p, want %p", got, newRemote)
	}

	hub.unregisterConnection("dev-1", newRemote)
	if hub.IsConnected("dev-1") {
		t.Fatal("new connection unregister did not remove active connection")
	}
	if id, conn := hub.FindByConn(newServerConn); id != "" || conn != nil {
		t.Fatalf("FindByConn(new after unregister) = (%q, %p), want empty", id, conn)
	}
}

func newWebSocketPairForHubTest(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	t.Helper()

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	serverConnCh := make(chan *websocket.Conn, 1)
	serverErrCh := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverErrCh <- err
			return
		}
		serverConnCh <- conn
	}))

	clientConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial websocket pair: %v", err)
	}

	var serverConn *websocket.Conn
	select {
	case serverConn = <-serverConnCh:
	case err := <-serverErrCh:
		_ = clientConn.Close()
		server.Close()
		t.Fatalf("upgrade websocket pair: %v", err)
	case <-time.After(time.Second):
		_ = clientConn.Close()
		server.Close()
		t.Fatal("timed out waiting for websocket server connection")
	}

	cleanup := func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
		server.Close()
	}
	return serverConn, clientConn, cleanup
}
