package remote

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

func TestRemoteHubSendCommandFallsBackToRegisteredTransportWithDefaults(t *testing.T) {
	hub := NewRemoteHub(nil, nil, slog.Default())
	transport := &recordingCommandTransport{
		connected: map[string]bool{"agodesk-1": true},
		result:    ResultPayload{Status: "ok", Output: `{"ok":true}`},
	}
	hub.RegisterCommandTransport("agodesk", transport)

	if !hub.IsConnected("agodesk-1") {
		t.Fatal("expected agodesk transport to make device connected")
	}

	result, err := hub.SendCommand("agodesk-1", CommandPayload{
		Operation: OpDesktopScreenshot,
		Args:      map[string]interface{}{"format": "png"},
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("result status = %q, want ok", result.Status)
	}
	if len(transport.calls) != 1 {
		t.Fatalf("transport calls = %d, want 1", len(transport.calls))
	}
	sent := transport.calls[0]
	if sent.CommandID == "" {
		t.Fatal("SendCommand did not assign a command id before dispatch")
	}
	if sent.TimeoutSec != 2 {
		t.Fatalf("TimeoutSec = %d, want 2", sent.TimeoutSec)
	}
	if result.CommandID != sent.CommandID {
		t.Fatalf("result CommandID = %q, want %q", result.CommandID, sent.CommandID)
	}
}

func TestRemoteHubExternalTransportRespectsReadOnlyPolicy(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	deviceID, err := CreateDevice(db, DeviceRecord{
		Name:     "agodesk",
		Status:   "approved",
		ReadOnly: true,
		Tags:     []string{"agodesk"},
	})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}

	hub := NewRemoteHub(db, nil, slog.Default())
	transport := &recordingCommandTransport{
		connected: map[string]bool{deviceID: true},
		result:    ResultPayload{Status: "ok"},
	}
	hub.RegisterCommandTransport("agodesk", transport)

	denied, err := hub.SendCommand(deviceID, CommandPayload{Operation: OpDesktopInput}, time.Second)
	if err != nil {
		t.Fatalf("SendCommand desktop input: %v", err)
	}
	if denied.Status != "denied" || !strings.Contains(denied.Error, "read-only") || denied.ErrorCode != "REMOTE_READ_ONLY" {
		t.Fatalf("desktop input result = %+v, want read-only denial", denied)
	}
	if len(transport.calls) != 0 {
		t.Fatalf("read-only denial should not dispatch to transport, got %d calls", len(transport.calls))
	}

	allowed, err := hub.SendCommand(deviceID, CommandPayload{Operation: OpDesktopScreenshot}, time.Second)
	if err != nil {
		t.Fatalf("SendCommand screenshot: %v", err)
	}
	if allowed.Status != "ok" {
		t.Fatalf("screenshot result = %+v, want ok", allowed)
	}
	if len(transport.calls) != 1 {
		t.Fatalf("screenshot should dispatch once, got %d calls", len(transport.calls))
	}
}

type recordingCommandTransport struct {
	connected map[string]bool
	result    ResultPayload
	calls     []CommandPayload
}

func (t *recordingCommandTransport) IsConnected(deviceID string) bool {
	return t.connected[deviceID]
}

func (t *recordingCommandTransport) SendCommand(deviceID string, cmd CommandPayload, timeout time.Duration) (ResultPayload, error) {
	t.calls = append(t.calls, cmd)
	result := t.result
	result.CommandID = cmd.CommandID
	return result, nil
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
