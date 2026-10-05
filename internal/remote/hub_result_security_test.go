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

// hmacData joins Sequence and Nonce without a delimiter, so a captured frame
// with seq=12 still verifies as seq=1 with nonce "2"+nonce. The supervisor
// must reject that shifted copy instead of treating it as a fresh nonce.
func TestHandleMessagesRejectsSequenceShiftedNonce(t *testing.T) {
	hub := NewRemoteHub(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	key := strings.Repeat("b", 64)
	serverConn, clientConn, cleanup := newWebSocketPairForHubTest(t)
	conn := &RemoteConnection{Conn: serverConn, DeviceID: "dev-1", SharedKey: key}
	hub.Register("dev-1", conn)
	heartbeats := make(chan HeartbeatPayload, 4)
	hub.OnHeartbeat = func(_ string, hb HeartbeatPayload) { heartbeats <- hb }
	handled := make(chan struct{})
	go func() {
		hub.HandleMessages(conn)
		close(handled)
	}()
	defer func() {
		cleanup()
		select {
		case <-handled:
		case <-time.After(5 * time.Second):
			t.Error("HandleMessages still running after the connection closed")
		}
	}()

	frame, err := NewMessage(MsgHeartbeat, "dev-1", key, 12, HeartbeatPayload{Hostname: "original"})
	if err != nil {
		t.Fatal(err)
	}
	shifted := *frame
	shifted.Sequence = 1
	shifted.Nonce = "2" + frame.Nonce
	if ok, err := VerifyMessage(shifted, key); err != nil || !ok {
		t.Fatalf("shifted frame is expected to keep a valid HMAC (delimiter-free encoding): ok=%v err=%v", ok, err)
	}

	if err := clientConn.WriteJSON(frame); err != nil {
		t.Fatal(err)
	}
	select {
	case <-heartbeats:
	case <-time.After(5 * time.Second):
		t.Fatal("fresh heartbeat was not handled")
	}

	if err := clientConn.WriteJSON(&shifted); err != nil {
		t.Fatal(err)
	}
	_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var reply RemoteMessage
	if err := clientConn.ReadJSON(&reply); err != nil {
		t.Fatalf("expected an error frame for the shifted nonce: %v", err)
	}
	var errPayload ErrorPayload
	if reply.Type != MsgError || json.Unmarshal(reply.Payload, &errPayload) != nil ||
		errPayload.Code != "replay" || errPayload.Message != "invalid nonce format" {
		t.Fatalf("unexpected reply to shifted frame: type=%s payload=%s", reply.Type, reply.Payload)
	}
	select {
	case hb := <-heartbeats:
		t.Fatalf("shifted frame reached the heartbeat handler: %+v", hb)
	default:
	}
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
