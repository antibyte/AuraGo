//go:build !remote_minimal

package remote

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/testutil"
	"github.com/gorilla/websocket"
)

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
