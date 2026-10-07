package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
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

	if err := registerTestConnection(t, hub, "nest-1", conn); err != nil {
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

	_ = registerTestConnection(t, hub, "nest-1", conn1)
	if err := registerTestConnection(t, hub, "nest-1", conn2); err != nil {
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

	if err := registerTestConnection(t, hub, "nest-1", conn1); err != nil {
		t.Fatalf("Register first: %v", err)
	}

	if err := registerTestConnection(t, hub, "nest-2", conn2); err == nil {
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

	_ = registerTestConnection(t, hub, "nest-1", conn1)
	// Replacing same nest should work even at max
	if err := registerTestConnection(t, hub, "nest-1", conn2); err != nil {
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
	_ = registerTestConnection(t, hub, "nest-1", conn)
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

	_ = registerTestConnection(t, hub, "nest-a", &EggConnection{Conn: s1, EggID: "e1", NestID: "nest-a", SharedKey: validKey(t)})
	_ = registerTestConnection(t, hub, "nest-b", &EggConnection{Conn: s2, EggID: "e2", NestID: "nest-b", SharedKey: validKey(t)})

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

	_ = registerTestConnection(t, hub, "nest-1", &EggConnection{
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

	_ = registerTestConnection(t, hub, "nest-1", &EggConnection{
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

	_ = registerTestConnection(t, hub, "nest-1", &EggConnection{
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
	_ = registerTestConnection(t, hub, "nest-1", conn)

	// Send heartbeat from client side
	hbMsg, _ := NewMessage(MsgHeartbeat, "egg-1", "nest-1", key, HeartbeatPayload{
		CPUPercent: 25.0, MemPercent: 60.0, Status: "idle",
	})
	testSession(t, "egg-1", "nest-1", "egg").Prepare(hbMsg, key)
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
	_ = registerTestConnection(t, hub, "nest-1", conn)

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
	_ = registerTestConnection(t, hub, "nest-1", conn)

	resultMsg, _ := NewMessage(MsgResult, "egg-1", "nest-1", key, ResultPayload{
		TaskID: "task-42", Status: "success", Output: "done",
	})
	testSession(t, "egg-1", "nest-1", "egg").Prepare(resultMsg, key)
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
	_ = registerTestConnection(t, hub, "nest-1", conn)

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

// ── Key rotation ────────────────────────────────────────────────────────────

// rekeyFixture wires a registered hub connection to a live EggClient over a
// real socket pair, like TestHeartbeatAndRekeyRemainOrderedUnderConcurrentTraffic.
type rekeyFixture struct {
	hub    *EggHub
	conn   *EggConnection
	client *EggClient
	oldKey string
}

// newRekeyFixture runs configure before the read loops start so callbacks are
// installed without racing the readers.
func newRekeyFixture(t *testing.T, configure func(*EggHub, *EggClient)) rekeyFixture {
	t.Helper()
	hub := NewEggHub(testLogger())
	s, c, cleanup := wsPair(t)
	key := validKey(t)
	conn := &EggConnection{Conn: s, EggID: "egg", NestID: "nest", SharedKey: key}
	if err := registerTestConnection(t, hub, "nest", conn); err != nil {
		cleanup()
		t.Fatal(err)
	}
	client := NewEggClient("", "egg", "nest", key, "fixture", testLogger())
	client.conn, client.session = c, testSession(t, "egg", "nest", "egg")
	if configure != nil {
		configure(hub, client)
	}
	var readers sync.WaitGroup
	readers.Add(2)
	go func() { defer readers.Done(); hub.HandleMessages(conn) }()
	go func() { defer readers.Done(); client.readLoop() }()
	t.Cleanup(func() {
		client.Stop()
		cleanup()
		readers.Wait()
	})
	return rekeyFixture{hub: hub, conn: conn, client: client, oldKey: key}
}

func (f rekeyFixture) hubKeys() (current, previous string, version int) {
	f.conn.mu.Lock()
	defer f.conn.mu.Unlock()
	return f.conn.SharedKey, f.conn.PreviousKey, f.conn.KeyVersion
}

// sendAsEgg writes a frame on the egg's session signed with an arbitrary key.
func sendAsEgg(t *testing.T, c *EggClient, key, kind string, payload interface{}) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	msg, err := NewMessage(kind, c.EggID, c.NestID, key, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.session.Prepare(msg, key); err != nil {
		t.Fatal(err)
	}
	if err := c.conn.WriteJSON(msg); err != nil {
		t.Fatal(err)
	}
}

func waitForBridge(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestSendRekeyWaitsForAckAndPersistsOnEgg(t *testing.T) {
	persisted := make(chan string, 1)
	f := newRekeyFixture(t, func(_ *EggHub, c *EggClient) {
		oldKey := c.SharedKey
		c.OnRekey = func(newKey string, version int) error {
			if version != 1 {
				t.Errorf("version = %d, want 1", version)
			}
			// Persist runs before the switch and before any frame (the ack)
			// leaves the egg: the fixture's egg has sent nothing yet.
			c.mu.Lock()
			current, sent := c.SharedKey, c.session.sent
			c.mu.Unlock()
			if current != oldKey || sent != 0 {
				t.Errorf("OnRekey must run on the old key with no ack sent: key switched=%v, frames sent=%d", current != oldKey, sent)
			}
			persisted <- newKey
			return nil
		}
	})
	newKey := validKey(t)
	result, err := f.hub.SendRekey(context.Background(), "nest", newKey)
	if err != nil {
		t.Fatalf("SendRekey: %v", err)
	}
	if !result.Persisted || result.ReplacedKey != f.oldKey {
		t.Fatal("the egg's success ack must carry persisted:true and the hub must report the replaced key")
	}
	select {
	case got := <-persisted:
		if got != newKey {
			t.Fatalf("egg persisted %q, want the new key", got)
		}
	default:
		t.Fatal("egg must persist the key before acking")
	}
	current, previous, version := f.hubKeys()
	if f.client.SharedKeySnapshot() != newKey || current != newKey || version != 1 {
		t.Fatal("both sides must hold the new key after the ack")
	}
	if previous != "" {
		t.Fatal("the grace key must be cleared once the egg confirmed the new key")
	}
}

func TestSendRekeyRollsBackWhenEggCannotPersist(t *testing.T) {
	heartbeats := make(chan struct{}, 1)
	f := newRekeyFixture(t, func(h *EggHub, c *EggClient) {
		c.OnRekey = func(string, int) error { return errors.New("disk full at /secret/path") }
		h.OnHeartbeat = func(string, HeartbeatPayload) { heartbeats <- struct{}{} }
	})
	_, err := f.hub.SendRekey(context.Background(), "nest", validKey(t))
	if err == nil {
		t.Fatal("SendRekey must fail when the egg rejects")
	}
	if !IsAckRejected(err) {
		t.Fatalf("an explicit rejection must be reported as such: %v", err)
	}
	if strings.Contains(err.Error(), "/secret/path") {
		t.Fatalf("egg rejection leaked its local error: %v", err)
	}
	current, previous, version := f.hubKeys()
	if current != f.oldKey || previous != "" || version != 0 || f.client.SharedKeySnapshot() != f.oldKey {
		t.Fatal("rejected rotation must leave both sides on the old key")
	}
	if f.hub.rekeyUnresolved("nest") {
		t.Fatal("an explicit rejection resolves the rotation")
	}
	if err := f.client.send(MsgHeartbeat, HeartbeatPayload{Status: "idle"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-heartbeats:
	case <-time.After(2 * time.Second):
		t.Fatal("old-key traffic must keep flowing after a rejected rotation")
	}
}

func TestEggRejectsRekeyWithUnexpectedVersion(t *testing.T) {
	var called atomic.Bool
	f := newRekeyFixture(t, func(_ *EggHub, c *EggClient) {
		c.OnRekey = func(string, int) error { called.Store(true); return nil }
	})
	f.conn.mu.Lock()
	f.conn.KeyVersion = 5 // the egg expects version 1 on this session
	f.conn.mu.Unlock()
	if _, err := f.hub.SendRekey(context.Background(), "nest", validKey(t)); err == nil {
		t.Fatal("egg must reject a rekey that skips versions")
	}
	if called.Load() {
		t.Fatal("egg must not persist a rekey with an unexpected version")
	}
	current, _, version := f.hubKeys()
	if current != f.oldKey || version != 5 || f.client.SharedKeySnapshot() != f.oldKey {
		t.Fatal("version mismatch must leave both sides on the old key and version")
	}
}

func TestSendRekeyTimeoutRollsBackAndKeepsOldKeyUsable(t *testing.T) {
	release := make(chan error)
	heartbeats := make(chan struct{}, 4)
	f := newRekeyFixture(t, func(h *EggHub, c *EggClient) {
		c.OnRekey = func(string, int) error { return <-release }
		h.OnHeartbeat = func(string, HeartbeatPayload) { heartbeats <- struct{}{} }
	})
	newKey := validKey(t)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, err := f.hub.SendRekey(ctx, "nest", newKey); err == nil {
		close(release)
		t.Fatal("SendRekey must fail when the egg never acks")
	}
	current, previous, version := f.hubKeys()
	if current != f.oldKey || previous != "" || version != 0 {
		close(release)
		t.Fatal("unconfirmed rotation must roll the hub back to the old key")
	}
	// The egg may still adopt the key, so a second rotation must not start.
	if !f.hub.rekeyUnresolved("nest") {
		close(release)
		t.Fatal("a timed-out rotation stays unresolved until the egg answers")
	}
	if _, err := f.hub.BeginKeyRotation("nest"); err == nil {
		close(release)
		t.Fatal("a new rotation must wait for the unresolved one")
	}
	if _, err := f.hub.SendRekey(context.Background(), "nest", validKey(t)); err == nil {
		close(release)
		t.Fatal("SendRekey must refuse while an earlier rotation is unresolved")
	}

	release <- errors.New("late persist failure")
	waitForBridge(t, "late rejection to resolve the rotation", func() bool { return !f.hub.rekeyUnresolved("nest") })
	sendAsEgg(t, f.client, f.oldKey, MsgHeartbeat, HeartbeatPayload{Status: "idle"})
	select {
	case <-heartbeats:
	case <-time.After(2 * time.Second):
		t.Fatal("a frame signed with the old key must be accepted after the rollback")
	}
	sendAsEgg(t, f.client, newKey, MsgHeartbeat, HeartbeatPayload{Status: "idle"})
	waitForBridge(t, "new-key frame to drop the connection", func() bool { return !f.hub.IsConnected("nest") })
	select {
	case <-heartbeats:
		t.Fatal("a frame signed with the rolled-back key must be rejected")
	default:
	}
}

func TestSendRekeyLostAckLeavesEggOnPersistedKey(t *testing.T) {
	persisted := make(chan string, 1)
	f := newRekeyFixture(t, func(_ *EggHub, c *EggClient) {
		c.OnRekey = func(newKey string, _ int) error {
			persisted <- newKey
			// Cut the socket after the key is durable: the ack never arrives.
			c.mu.Lock()
			_ = c.conn.Close()
			c.mu.Unlock()
			return nil
		}
	})
	newKey := validKey(t)
	// No deadline: the wait must end when the socket closes, long before the
	// hub's 15 s ack timeout.
	started := time.Now()
	_, err := f.hub.SendRekey(context.Background(), "nest", newKey)
	if err == nil || IsAckRejected(err) {
		t.Fatalf("SendRekey must fail without an ack: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("SendRekey waited %v on a closed connection", elapsed)
	}
	if current, previous, _ := f.hubKeys(); current != f.oldKey || previous != "" {
		t.Fatal("hub must roll back to the old key when the ack is lost")
	}
	select {
	case got := <-persisted:
		if got != newKey {
			t.Fatal("egg persisted the wrong key")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("egg never persisted the key")
	}
	// The egg switches after persisting; its reconnect presents the new key,
	// which the master's handshake accepts from the staged _next candidate.
	waitForBridge(t, "egg to hold the persisted key", func() bool { return f.client.SharedKeySnapshot() == newKey })
}

// An egg predating the persisted flag switches in memory and acks without it;
// the hub still commits but reports the key as unconfirmed on disk.
func TestSendRekeyReportsLegacyAckWithoutPersistence(t *testing.T) {
	hub := NewEggHub(testLogger())
	s, c, cleanup := wsPair(t)
	defer cleanup()
	oldKey := validKey(t)
	conn := &EggConnection{Conn: s, EggID: "egg", NestID: "nest", SharedKey: oldKey}
	if err := registerTestConnection(t, hub, "nest", conn); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); hub.HandleMessages(conn) }()
	defer func() { c.Close(); <-done }()

	legacy := make(chan error, 1)
	go func() {
		egg := testSession(t, "egg", "nest", "egg")
		var msg Message
		if err := c.ReadJSON(&msg); err != nil {
			legacy <- err
			return
		}
		if err := egg.Accept(msg, oldKey, ""); err != nil {
			legacy <- err
			return
		}
		var rekey RekeyPayload
		if err := json.Unmarshal(msg.Payload, &rekey); err != nil {
			legacy <- err
			return
		}
		newKey, err := DecryptWithSharedKey(rekey.NewKeyEncrypted, oldKey)
		if err != nil {
			legacy <- err
			return
		}
		// The wire format of eggs that predate the persisted-ack flag: no
		// persisted field at all.
		ack, err := NewMessage(MsgAck, "egg", "nest", string(newKey), map[string]interface{}{"ref_id": msg.ID, "success": true, "detail": "key rotated to v1"})
		if err == nil {
			err = egg.Prepare(ack, string(newKey))
		}
		if err == nil {
			err = c.WriteJSON(ack)
		}
		legacy <- err
	}()

	newKey := validKey(t)
	result, err := hub.SendRekey(context.Background(), "nest", newKey)
	if legacyErr := <-legacy; legacyErr != nil {
		t.Fatal(legacyErr)
	}
	if err != nil {
		t.Fatalf("a legacy success ack still commits the rotation: %v", err)
	}
	if result.Persisted || result.ReplacedKey != oldKey {
		t.Fatal("an ack without the persisted field must not count as persisted")
	}
	if current, previous, _ := (rekeyFixture{conn: conn}).hubKeys(); current != newKey || previous != "" {
		t.Fatal("hub must commit the new key")
	}
}

// The egg's own ack carries the flag only when it stored the key; checked on
// the wire against a raw master socket.
func TestEggRekeyAckCarriesPersistedFlagOnlyOnSuccess(t *testing.T) {
	s, c, cleanup := wsPair(t)
	defer cleanup()
	oldKey, newKey := validKey(t), validKey(t)
	client := NewEggClient("", "egg", "nest", oldKey, "fixture", testLogger())
	client.conn, client.session = c, testSession(t, "egg", "nest", "egg")
	client.OnRekey = func(string, int) error { return nil }
	done := make(chan struct{})
	go func() { defer close(done); client.readLoop() }()
	defer func() { client.Stop(); <-done }()

	master := testSession(t, "egg", "nest", "master")
	exchange := func(version int, ackKey string) AckPayload {
		t.Helper()
		encrypted, err := EncryptWithSharedKey([]byte(newKey), oldKey)
		if err != nil {
			t.Fatal(err)
		}
		msg, err := NewMessage(MsgRekey, "egg", "nest", oldKey, RekeyPayload{NewKeyEncrypted: encrypted, KeyVersion: version})
		if err != nil {
			t.Fatal(err)
		}
		if err := master.Prepare(msg, oldKey); err != nil {
			t.Fatal(err)
		}
		if err := s.WriteJSON(msg); err != nil {
			t.Fatal(err)
		}
		_ = s.SetReadDeadline(time.Now().Add(2 * time.Second))
		var reply Message
		if err := s.ReadJSON(&reply); err != nil {
			t.Fatal(err)
		}
		if err := master.Accept(reply, ackKey, ""); err != nil {
			t.Fatalf("ack not signed with the expected key: %v", err)
		}
		var ack AckPayload
		if err := json.Unmarshal(reply.Payload, &ack); err != nil || ack.RefID != msg.ID {
			t.Fatalf("unexpected ack %s: %v", reply.Payload, err)
		}
		return ack
	}
	if ack := exchange(7, oldKey); ack.Success || ack.Persisted {
		t.Fatalf("a rejected rekey must carry success:false, persisted:false: %+v", ack)
	}
	if ack := exchange(1, newKey); !ack.Success || !ack.Persisted {
		t.Fatalf("a stored rekey must carry success:true, persisted:true: %+v", ack)
	}
}

func TestSendRekeyHonoursHubAckTimeout(t *testing.T) {
	release := make(chan error, 1)
	f := newRekeyFixture(t, func(h *EggHub, c *EggClient) {
		h.ackTimeout = 100 * time.Millisecond
		c.OnRekey = func(string, int) error { return <-release }
	})
	defer func() { release <- errors.New("test ended") }()
	started := time.Now()
	_, err := f.hub.SendRekey(context.Background(), "nest", validKey(t))
	if err == nil || !strings.Contains(err.Error(), "timed out waiting for ack") {
		t.Fatalf("SendRekey must give up at the hub's ack timeout: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("ack timeout ignored: waited %v", elapsed)
	}
	if current, _, _ := f.hubKeys(); current != f.oldKey || !f.hub.rekeyUnresolved("nest") {
		t.Fatal("a timed-out rotation rolls back and stays unresolved")
	}
}

// Documents a known loss: a command sent while the rekey is in flight is
// signed with the new key, so an egg that rejects the rotation cannot verify
// it and drops the socket. The command is not delivered; the egg reconnects.
func TestCommandSentDuringRejectedRekeyIsLost(t *testing.T) {
	started := make(chan struct{})
	release := make(chan error, 1)
	var tasks atomic.Int32
	f := newRekeyFixture(t, func(_ *EggHub, c *EggClient) {
		c.OnRekey = func(string, int) error { close(started); return <-release }
		c.OnTask = func(TaskPayload) { tasks.Add(1) }
	})
	newKey := validKey(t)
	result := make(chan error, 1)
	go func() {
		_, err := f.hub.SendRekey(context.Background(), "nest", newKey)
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		release <- nil
		t.Fatal("egg never received the rekey")
	}
	if err := f.hub.SendTask("nest", TaskPayload{TaskID: "in-flight"}); err != nil {
		release <- nil
		t.Fatal(err)
	}
	release <- errors.New("persist failed")
	if err := <-result; !IsAckRejected(err) {
		t.Fatalf("the rejection still reaches the master first: %v", err)
	}
	waitForBridge(t, "egg to drop the socket on the new-key frame", func() bool { return !f.hub.IsConnected("nest") })
	if tasks.Load() != 0 {
		t.Fatal("the in-flight command cannot have been delivered")
	}
}

func TestEggRejectsSecretsWithReservedNames(t *testing.T) {
	stored := make(chan string, 4)
	f := newRekeyFixture(t, func(_ *EggHub, c *EggClient) {
		c.OnSecret = func(secret SecretPayload) { stored <- secret.Key }
	})
	for _, key := range []string{"egg_shared_key", "EGG_SHARED_KEY", "egg_master_key_x"} {
		if err := f.hub.SendSecret("nest", key, "00"); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.hub.SendSecret("nest", "api_token", "00"); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-stored:
		if got != "api_token" {
			t.Fatalf("reserved secret %q reached OnSecret", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ordinary secret was not delivered")
	}
	if !IsReservedEggSecretName(" Egg_Shared_Key ") || IsReservedEggSecretName("egg_notes") {
		t.Fatal("reserved-name check must be case-insensitive and prefix-exact")
	}
}

func TestBeginKeyRotationSerializesRotationsPerNest(t *testing.T) {
	hub := NewEggHub(testLogger())
	done, err := hub.BeginKeyRotation("nest")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hub.BeginKeyRotation("nest"); err == nil {
		t.Fatal("a second rotation for the same nest must be refused")
	}
	other, err := hub.BeginKeyRotation("other")
	if err != nil {
		t.Fatalf("rotations for other nests stay independent: %v", err)
	}
	other()
	done()
	again, err := hub.BeginKeyRotation("nest")
	if err != nil {
		t.Fatalf("finished rotation must release the nest: %v", err)
	}
	again()
}

func registerTestConnection(t *testing.T, hub *EggHub, nestID string, conn *EggConnection) error {
	t.Helper()
	if conn.Session == nil {
		conn.Session = testSession(t, conn.EggID, nestID, "master")
	}
	return hub.Register(nestID, conn)
}

func testSession(t *testing.T, eggID, nestID, role string) *Session {
	t.Helper()
	s, err := NewSession(strings.Repeat("a", 64), eggID, nestID, role)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
