package bridge

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/testutil"

	"github.com/gorilla/websocket"
)

// testLogger returns a silent logger for tests.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// wsPair creates a connected pair of WebSocket connections (server + client)
// using httptest. Returns server-side conn, client-side conn, and a cleanup func.
func wsPair(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	serverConn := make(chan *websocket.Conn, 1)

	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		serverConn <- ws
	}))

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("dial: %v", err)
	}
	server := <-serverConn

	return server, client, func() {
		server.Close()
		client.Close()
		srv.Close()
	}
}

// ── Registration ────────────────────────────────────────────────────────────

func TestRegister_Success(t *testing.T) {
	hub := NewEggHub(testLogger())
	sConn, _, cleanup := wsPair(t)
	defer cleanup()

	conn := &EggConnection{
		Conn:      sConn,
		EggID:     "egg-1",
		NestID:    "nest-1",
		SharedKey: validKey(t),
	}

	if err := hub.Register("nest-1", conn); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !hub.IsConnected("nest-1") {
		t.Fatal("nest-1 should be connected")
	}
	if hub.ConnectionCount() != 1 {
		t.Errorf("count = %d, want 1", hub.ConnectionCount())
	}
}

func TestRegister_ReplacesExisting(t *testing.T) {
	hub := NewEggHub(testLogger())

	s1, _, cleanup1 := wsPair(t)
	defer cleanup1()
	s2, _, cleanup2 := wsPair(t)
	defer cleanup2()

	conn1 := &EggConnection{Conn: s1, EggID: "egg-1", NestID: "nest-1", SharedKey: validKey(t)}
	conn2 := &EggConnection{Conn: s2, EggID: "egg-1", NestID: "nest-1", SharedKey: validKey(t)}

	_ = hub.Register("nest-1", conn1)
	if err := hub.Register("nest-1", conn2); err != nil {
		t.Fatalf("Register replacement: %v", err)
	}

	if hub.ConnectionCount() != 1 {
		t.Errorf("count = %d, want 1 after replacement", hub.ConnectionCount())
	}

	got := hub.GetConnection("nest-1")
	if got != conn2 {
		t.Fatal("connection should be the replacement")
	}
}

func TestRegister_MaxConnections(t *testing.T) {
	hub := NewEggHub(testLogger())
	hub.MaxConnections = 1

	s1, _, cleanup1 := wsPair(t)
	defer cleanup1()
	s2, _, cleanup2 := wsPair(t)
	defer cleanup2()

	conn1 := &EggConnection{Conn: s1, EggID: "egg-1", NestID: "nest-1", SharedKey: validKey(t)}
	conn2 := &EggConnection{Conn: s2, EggID: "egg-2", NestID: "nest-2", SharedKey: validKey(t)}

	if err := hub.Register("nest-1", conn1); err != nil {
		t.Fatalf("Register first: %v", err)
	}

	if err := hub.Register("nest-2", conn2); err == nil {
		t.Fatal("expected error when max connections reached")
	}
}

func TestRegister_MaxConnections_AllowsReplacement(t *testing.T) {
	hub := NewEggHub(testLogger())
	hub.MaxConnections = 1

	s1, _, cleanup1 := wsPair(t)
	defer cleanup1()
	s2, _, cleanup2 := wsPair(t)
	defer cleanup2()

	conn1 := &EggConnection{Conn: s1, EggID: "egg-1", NestID: "nest-1", SharedKey: validKey(t)}
	conn2 := &EggConnection{Conn: s2, EggID: "egg-1", NestID: "nest-1", SharedKey: validKey(t)}

	_ = hub.Register("nest-1", conn1)
	// Replacing same nest should work even at max
	if err := hub.Register("nest-1", conn2); err != nil {
		t.Fatalf("replacement at max limit should succeed: %v", err)
	}
}

// ── Unregister ──────────────────────────────────────────────────────────────

func TestUnregister_Existing(t *testing.T) {
	hub := NewEggHub(testLogger())
	s, _, cleanup := wsPair(t)
	defer cleanup()

	var disconnected bool
	hub.OnDisconnect = func(nestID, eggID string) {
		disconnected = true
	}

	conn := &EggConnection{Conn: s, EggID: "egg-1", NestID: "nest-1", SharedKey: validKey(t)}
	_ = hub.Register("nest-1", conn)
	hub.Unregister("nest-1")

	if hub.IsConnected("nest-1") {
		t.Fatal("should not be connected after Unregister")
	}
	if !disconnected {
		t.Fatal("OnDisconnect should have been called")
	}
}

func TestUnregister_NonExistent(t *testing.T) {
	hub := NewEggHub(testLogger())
	// Should not panic
	hub.Unregister("ghost-nest")
}

// ── Queries ─────────────────────────────────────────────────────────────────

func TestIsConnected(t *testing.T) {
	hub := NewEggHub(testLogger())
	if hub.IsConnected("any") {
		t.Fatal("empty hub should not have connections")
	}
}

func TestConnectedNests(t *testing.T) {
	hub := NewEggHub(testLogger())
	s1, _, c1 := wsPair(t)
	defer c1()
	s2, _, c2 := wsPair(t)
	defer c2()

	_ = hub.Register("nest-a", &EggConnection{Conn: s1, EggID: "e1", NestID: "nest-a", SharedKey: validKey(t)})
	_ = hub.Register("nest-b", &EggConnection{Conn: s2, EggID: "e2", NestID: "nest-b", SharedKey: validKey(t)})

	nests := hub.ConnectedNests()
	if len(nests) != 2 {
		t.Fatalf("ConnectedNests = %d, want 2", len(nests))
	}
}

func TestConnectionCount(t *testing.T) {
	hub := NewEggHub(testLogger())
	if hub.ConnectionCount() != 0 {
		t.Fatalf("empty hub count = %d", hub.ConnectionCount())
	}
}

// ── SendTask ────────────────────────────────────────────────────────────────

func TestSendTask_Connected(t *testing.T) {
	hub := NewEggHub(testLogger())
	key := validKey(t)
	sConn, cConn, cleanup := wsPair(t)
	defer cleanup()

	_ = hub.Register("nest-1", &EggConnection{
		Conn: sConn, EggID: "egg-1", NestID: "nest-1", SharedKey: key,
	})

	err := hub.SendTask("nest-1", TaskPayload{TaskID: "t1", Description: "test"})
	if err != nil {
		t.Fatalf("SendTask: %v", err)
	}

	// Read from client side
	_, data, err := cConn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if msg.Type != MsgTask {
		t.Errorf("type = %q, want %q", msg.Type, MsgTask)
	}

	// Verify HMAC
	ok, err := VerifyMessage(msg, key)
	if err != nil || !ok {
		t.Fatal("HMAC verification failed on received task")
	}

	// Verify payload
	var task TaskPayload
	if err := json.Unmarshal(msg.Payload, &task); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if task.TaskID != "t1" {
		t.Errorf("task_id = %q, want %q", task.TaskID, "t1")
	}
}

func TestSendTask_NotConnected(t *testing.T) {
	hub := NewEggHub(testLogger())
	err := hub.SendTask("ghost", TaskPayload{TaskID: "t1"})
	if err == nil {
		t.Fatal("expected error for non-existent nest")
	}
}

func TestSendSecret_Connected(t *testing.T) {
	hub := NewEggHub(testLogger())
	key := validKey(t)
	sConn, cConn, cleanup := wsPair(t)
	defer cleanup()

	_ = hub.Register("nest-1", &EggConnection{
		Conn: sConn, EggID: "egg-1", NestID: "nest-1", SharedKey: key,
	})

	err := hub.SendSecret("nest-1", "api_key", "encrypted-value-hex")
	if err != nil {
		t.Fatalf("SendSecret: %v", err)
	}

	_, data, _ := cConn.ReadMessage()
	var msg Message
	_ = json.Unmarshal(data, &msg)
	if msg.Type != MsgSecret {
		t.Errorf("type = %q, want %q", msg.Type, MsgSecret)
	}
}

func TestSendStop_Connected(t *testing.T) {
	hub := NewEggHub(testLogger())
	key := validKey(t)
	sConn, cConn, cleanup := wsPair(t)
	defer cleanup()

	_ = hub.Register("nest-1", &EggConnection{
		Conn: sConn, EggID: "egg-1", NestID: "nest-1", SharedKey: key,
	})

	err := hub.SendStop("nest-1")
	if err != nil {
		t.Fatalf("SendStop: %v", err)
	}

	_, data, _ := cConn.ReadMessage()
	var msg Message
	_ = json.Unmarshal(data, &msg)
	if msg.Type != MsgStop {
		t.Errorf("type = %q, want %q", msg.Type, MsgStop)
	}

	// After stop, nest should be unregistered
	if hub.IsConnected("nest-1") {
		t.Fatal("nest should be unregistered after SendStop")
	}
}

// ── HandleMessages ──────────────────────────────────────────────────────────

func TestHandleMessages_Heartbeat(t *testing.T) {
	hub := NewEggHub(testLogger())
	key := validKey(t)
	sConn, cConn, cleanup := wsPair(t)
	defer cleanup()

	var heartbeatReceived bool
	var mu sync.Mutex
	hub.OnHeartbeat = func(nestID string, hb HeartbeatPayload) {
		mu.Lock()
		heartbeatReceived = true
		mu.Unlock()
	}

	conn := &EggConnection{
		Conn: sConn, EggID: "egg-1", NestID: "nest-1", SharedKey: key,
	}
	_ = hub.Register("nest-1", conn)

	// Send heartbeat from client side
	hbMsg, _ := NewMessage(MsgHeartbeat, "egg-1", "nest-1", key, HeartbeatPayload{
		CPUPercent: 25.0, MemPercent: 60.0, Status: "idle",
	})
	cConn.WriteJSON(hbMsg)

	// Start HandleMessages in background (it will read the heartbeat then block on next read)
	done := make(chan struct{})
	go func() {
		hub.HandleMessages(conn)
		close(done)
	}()

	// Give it a moment to process
	time.Sleep(100 * time.Millisecond)

	// Close client connection to unblock HandleMessages
	cConn.Close()
	<-done

	mu.Lock()
	if !heartbeatReceived {
		t.Fatal("OnHeartbeat should have been called")
	}
	mu.Unlock()
}

func TestHandleMessages_InvalidHMAC(t *testing.T) {
	hub := NewEggHub(testLogger())
	key := validKey(t)
	sConn, cConn, cleanup := wsPair(t)
	defer cleanup()

	conn := &EggConnection{
		Conn: sConn, EggID: "egg-1", NestID: "nest-1", SharedKey: key,
	}
	_ = hub.Register("nest-1", conn)

	// Send message with wrong key
	wrongKey := validKey(t)
	badMsg, _ := NewMessage(MsgHeartbeat, "egg-1", "nest-1", wrongKey, HeartbeatPayload{Status: "idle"})
	cConn.WriteJSON(badMsg)

	done := make(chan struct{})
	go func() {
		hub.HandleMessages(conn)
		close(done)
	}()

	// Read the error response the hub sends back
	_, data, err := cConn.ReadMessage()
	if err == nil {
		var msg Message
		if json.Unmarshal(data, &msg) == nil && msg.Type == MsgError {
			// Good — hub sent error
		}
	}

	cConn.Close()
	<-done
}

func TestHandleMessages_Result(t *testing.T) {
	hub := NewEggHub(testLogger())
	key := validKey(t)
	sConn, cConn, cleanup := wsPair(t)
	defer cleanup()

	var resultReceived string
	var mu sync.Mutex
	hub.OnResult = func(nestID string, result ResultPayload) {
		mu.Lock()
		resultReceived = result.TaskID
		mu.Unlock()
	}

	conn := &EggConnection{
		Conn: sConn, EggID: "egg-1", NestID: "nest-1", SharedKey: key,
	}
	_ = hub.Register("nest-1", conn)

	resultMsg, _ := NewMessage(MsgResult, "egg-1", "nest-1", key, ResultPayload{
		TaskID: "task-42", Status: "success", Output: "done",
	})
	cConn.WriteJSON(resultMsg)

	done := make(chan struct{})
	go func() {
		hub.HandleMessages(conn)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cConn.Close()
	<-done

	mu.Lock()
	if resultReceived != "task-42" {
		t.Errorf("resultReceived = %q, want %q", resultReceived, "task-42")
	}
	mu.Unlock()
}

func TestStartHeartbeatMonitorUnregistersStaleConnectionWithoutDisconnectCallback(t *testing.T) {
	hub := NewEggHub(testLogger())
	hub.mu.Lock()
	hub.connections["nest-stale"] = &EggConnection{
		EggID:         "egg-stale",
		NestID:        "nest-stale",
		LastHeartbeat: time.Now().Add(-time.Hour),
	}
	hub.mu.Unlock()

	disconnected := make(chan struct{}, 1)
	hub.OnDisconnect = func(nestID, eggID string) {
		disconnected <- struct{}{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stale := make(chan struct{}, 1)
	hub.StartHeartbeatMonitor(ctx, 5*time.Millisecond, 10*time.Millisecond, func(nestID, eggID string) {
		stale <- struct{}{}
	})

	select {
	case <-stale:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("stale heartbeat callback was not called")
	}
	time.Sleep(20 * time.Millisecond)

	if hub.IsConnected("nest-stale") {
		t.Fatal("stale connection should be unregistered")
	}
	select {
	case <-disconnected:
		t.Fatal("stale monitor should not invoke OnDisconnect after onStale handled failure state")
	default:
	}
}

// ── OnConnect callback ──────────────────────────────────────────────────────

func TestRegister_OnConnect(t *testing.T) {
	hub := NewEggHub(testLogger())
	s, _, cleanup := wsPair(t)
	defer cleanup()

	var connectedNest string
	hub.OnConnect = func(nestID, eggID string) {
		connectedNest = nestID
	}

	conn := &EggConnection{Conn: s, EggID: "egg-1", NestID: "nest-1", SharedKey: validKey(t)}
	_ = hub.Register("nest-1", conn)

	if connectedNest != "nest-1" {
		t.Errorf("OnConnect nest = %q, want %q", connectedNest, "nest-1")
	}
}

// ── GetTelemetry ────────────────────────────────────────────────────────────

func TestEggConnection_GetTelemetry(t *testing.T) {
	conn := &EggConnection{
		Telemetry: HeartbeatPayload{CPUPercent: 42.0, MemPercent: 78.0, Status: "busy"},
	}
	tel := conn.GetTelemetry()
	if tel.CPUPercent != 42.0 {
		t.Errorf("CPU = %f, want 42.0", tel.CPUPercent)
	}
	if tel.Status != "busy" {
		t.Errorf("status = %q, want %q", tel.Status, "busy")
	}
}
