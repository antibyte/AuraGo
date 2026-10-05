//go:build !remote_minimal

package remote

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/testutil"
	"github.com/gorilla/websocket"
)

// handleMessagesFixture runs HandleMessages for a registered "dev-1"
// connection and exposes the agent side of the socket plus the heartbeats that
// reached the handler.
type handleMessagesFixture struct {
	hub        *RemoteHub
	key        string
	agent      *websocket.Conn
	heartbeats chan HeartbeatPayload
	handled    chan struct{}
}

func startHandleMessagesFixture(t *testing.T) *handleMessagesFixture {
	t.Helper()
	f := &handleMessagesFixture{
		hub:        NewRemoteHub(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil))),
		key:        strings.Repeat("b", 64),
		heartbeats: make(chan HeartbeatPayload, 32),
		handled:    make(chan struct{}),
	}
	serverConn, clientConn, cleanup := newWebSocketPairForHubTest(t)
	f.agent = clientConn
	conn := &RemoteConnection{Conn: serverConn, DeviceID: "dev-1", SharedKey: f.key}
	f.hub.Register("dev-1", conn)
	f.hub.OnHeartbeat = func(_ string, hb HeartbeatPayload) { f.heartbeats <- hb }
	go func() {
		f.hub.HandleMessages(conn)
		close(f.handled)
	}()
	t.Cleanup(func() {
		cleanup()
		select {
		case <-f.handled:
		case <-time.After(5 * time.Second):
			t.Error("HandleMessages still running after the connection closed")
		}
	})
	return f
}

func (f *handleMessagesFixture) send(t *testing.T, msg *RemoteMessage) {
	t.Helper()
	if err := f.agent.WriteJSON(msg); err != nil {
		t.Fatal(err)
	}
}

// expectError reads the hub's next reply and checks it is an error frame.
func (f *handleMessagesFixture) expectError(t *testing.T, code, message string) {
	t.Helper()
	_ = f.agent.SetReadDeadline(time.Now().Add(5 * time.Second))
	var reply RemoteMessage
	if err := f.agent.ReadJSON(&reply); err != nil {
		t.Fatalf("expected an error frame (%s): %v", code, err)
	}
	var errPayload ErrorPayload
	if reply.Type != MsgError || json.Unmarshal(reply.Payload, &errPayload) != nil ||
		errPayload.Code != code || (message != "" && errPayload.Message != message) {
		t.Fatalf("unexpected reply: type=%s payload=%s", reply.Type, reply.Payload)
	}
}

func (f *handleMessagesFixture) expectHeartbeat(t *testing.T) {
	t.Helper()
	select {
	case <-f.heartbeats:
	case <-time.After(5 * time.Second):
		t.Fatal("fresh heartbeat was not handled")
	}
}

func (f *handleMessagesFixture) heartbeat(t *testing.T, key string) *RemoteMessage {
	t.Helper()
	msg, err := NewMessage(MsgHeartbeat, "dev-1", key, 1, HeartbeatPayload{Hostname: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// Under the old undelimited HMAC form a captured frame with seq=12 still
// verified as seq=1 with nonce "2"+nonce. The canonical form makes that copy
// fail authentication, and unversioned frames are refused outright.
func TestHandleMessagesRejectsSequenceShiftedNonce(t *testing.T) {
	f := startHandleMessagesFixture(t)
	key := f.key
	heartbeats := f.heartbeats

	frame, err := NewMessage(MsgHeartbeat, "dev-1", key, 12, HeartbeatPayload{Hostname: "original"})
	if err != nil {
		t.Fatal(err)
	}
	shifted := *frame
	shifted.Sequence = 1
	shifted.Nonce = "2" + frame.Nonce
	unversioned, err := NewMessage(MsgHeartbeat, "dev-1", key, 13, HeartbeatPayload{Hostname: "old-agent"})
	if err != nil {
		t.Fatal(err)
	}

	f.send(t, frame)
	f.expectHeartbeat(t)

	f.send(t, &shifted)
	f.expectError(t, "invalid_hmac", "")
	f.send(t, legacySigned(t, unversioned, key))
	f.expectError(t, "invalid_hmac", "")
	select {
	case hb := <-heartbeats:
		t.Fatalf("shifted or unversioned frame reached the heartbeat handler: %+v", hb)
	default:
	}
}

// Every rejected frame earns a freshly signed error reply, which an on-path
// attacker could relay to the agent. The hub hangs up after a run of
// maxConsecutiveBadFrames instead of answering them all.
func TestHandleMessagesClosesConnectionAfterRepeatedBadFrames(t *testing.T) {
	wrongKey := strings.Repeat("c", 64)

	t.Run("consecutive bad frames close the connection", func(t *testing.T) {
		f := startHandleMessagesFixture(t)
		for i := 0; i < maxConsecutiveBadFrames; i++ {
			f.send(t, f.heartbeat(t, wrongKey))
		}
		for i := 0; i < maxConsecutiveBadFrames-1; i++ {
			f.expectError(t, "invalid_hmac", "")
		}
		select {
		case <-f.handled:
		case <-time.After(5 * time.Second):
			t.Fatalf("HandleMessages must stop after %d consecutive bad frames", maxConsecutiveBadFrames)
		}
		_ = f.agent.SetReadDeadline(time.Now().Add(5 * time.Second))
		if _, _, err := f.agent.ReadMessage(); err == nil {
			t.Fatal("the hub must not answer the frame that exhausted the budget")
		}
		if f.hub.IsConnected("dev-1") {
			t.Fatal("the closed connection must be unregistered")
		}
	})

	t.Run("a good frame resets the run", func(t *testing.T) {
		f := startHandleMessagesFixture(t)
		for round := 0; round < 2; round++ {
			for i := 0; i < maxConsecutiveBadFrames-1; i++ {
				f.send(t, f.heartbeat(t, wrongKey))
				f.expectError(t, "invalid_hmac", "")
			}
			f.send(t, f.heartbeat(t, f.key))
			f.expectHeartbeat(t)
		}
		select {
		case <-f.handled:
			t.Fatal("bad frames separated by good ones must not close the connection")
		default:
		}
		if !f.hub.IsConnected("dev-1") {
			t.Fatal("the connection must stay registered")
		}
	})
}

func TestResultRequiresAuthenticatedDeviceAndConnection(t *testing.T) {
	hub := NewRemoteHub(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	key := strings.Repeat("b", 64)
	ready := make(chan struct{}, 3)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conn := &RemoteConnection{Conn: ws, DeviceID: r.URL.Query().Get("id"), SharedKey: key}
		hub.Register(conn.DeviceID, conn)
		ready <- struct{}{}
		hub.HandleMessages(conn)
	}))
	defer server.Close()
	defer hub.SetEnabled(false)
	dial := func(id string) *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?id="+id, nil)
		if err != nil {
			t.Fatal(err)
		}
		<-ready
		return conn
	}
	victim, attacker := dial("victim"), dial("attacker")
	defer victim.Close()
	defer attacker.Close()
	done := make(chan ResultPayload, 1)
	go func() {
		r, _ := hub.SendCommand("victim", CommandPayload{CommandID: "request", Operation: OpSysinfo}, time.Second)
		done <- r
	}()
	var command RemoteMessage
	if err := victim.ReadJSON(&command); err != nil {
		t.Fatal(err)
	}
	for _, deviceID := range []string{"victim", "attacker"} {
		msg, _ := NewMessage(MsgResult, deviceID, key, 1, ResultPayload{CommandID: "request", Status: "ok", Output: "forged"})
		if err := attacker.WriteJSON(msg); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case r := <-done:
		t.Fatalf("forged result accepted: %+v", r)
	case <-time.After(60 * time.Millisecond):
	}
	msg, _ := NewMessage(MsgResult, "victim", key, 1, ResultPayload{CommandID: "request", Status: "ok", Output: "authentic"})
	if err := victim.WriteJSON(msg); err != nil {
		t.Fatal(err)
	}
	if r := <-done; r.Output != "authentic" {
		t.Fatalf("result = %+v", r)
	}
	hub.StartHeartbeatMonitor(time.Millisecond, time.Hour)
	hub.SetEnabled(false)
	if hub.Enabled() || hub.IsConnected("victim") || hub.monitorCancel != nil {
		t.Fatal("disable left runtime active")
	}
	if _, err := hub.SendCommand("victim", CommandPayload{Operation: OpSysinfo}, time.Second); err == nil {
		t.Fatal("disabled command accepted")
	}
}
