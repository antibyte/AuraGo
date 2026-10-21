package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/remote"
	"aurago/internal/security"
	"github.com/gorilla/websocket"
	"net/http/httptest"
)

func TestRemoteRejectedHandshakeCannotOwnExistingReader(t *testing.T) {
	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := remote.GenerateSharedKey()
	if err != nil {
		t.Fatal(err)
	}
	id, err := remote.CreateDevice(db, remote.DeviceRecord{Name: "fixture", Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("remote_shared_key_"+id, key); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := remote.NewRemoteHub(db, vault, logger)
	registered := make(chan struct{}, 1)
	hub.OnConnect = func(deviceID, _ string) {
		if deviceID == id {
			registered <- struct{}{}
		}
	}
	s := &Server{RemoteHub: hub, Logger: logger}
	server := httptest.NewTLSServer(handleRemoteWebSocket(s))
	defer server.Close()
	dial := func() *websocket.Conn {
		t.Helper()
		c, _, err := remoteTLSTestDialer(server).Dial("ws"+strings.TrimPrefix(server.URL, "http"), http.Header{})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		return c
	}
	good := dial()
	auth, _ := remote.NewMessage(remote.MsgAuth, id, key, 1, remote.AuthPayload{SecurityRevision: remote.ClientSecurityRevision, Capabilities: remote.ClassicCapabilities(), DeviceID: id})
	if err := good.WriteJSON(auth); err != nil {
		t.Fatal(err)
	}
	var response remote.RemoteMessage
	if err := good.ReadJSON(&response); err != nil {
		t.Fatal(err)
	}
	var accepted remote.AuthResponsePayload
	if err := json.Unmarshal(response.Payload, &accepted); err != nil {
		t.Fatal(err)
	}
	if valid, err := remote.VerifyMessage(response, key); err != nil || !valid || accepted.Status != "authenticated" || accepted.DeviceID != id || accepted.RequestNonce != auth.Nonce {
		t.Fatalf("accepted socket authentication failed: valid=%v status=%s err=%v", valid, accepted.Status, err)
	}
	// Authentication is written before Register publishes the connection so
	// other writers cannot race the authentication response on this socket.
	select {
	case <-registered:
	case <-time.After(3 * time.Second):
		t.Fatal("accepted socket registration did not complete")
	}
	original := hub.GetConnection(id)
	if original == nil {
		t.Fatal("accepted socket not registered")
	}
	bad := dial()
	invalid, _ := remote.NewMessage(remote.MsgAuth, id, strings.Repeat("b", 64), 1, remote.AuthPayload{SecurityRevision: remote.ClientSecurityRevision, Capabilities: remote.ClassicCapabilities(), DeviceID: id})
	if err := bad.WriteJSON(invalid); err != nil {
		t.Fatal(err)
	}
	if err := bad.ReadJSON(&response); err != nil {
		t.Fatal(err)
	}
	bad.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := bad.ReadMessage(); err == nil {
		t.Fatal("rejected socket remained open")
	}
	if hub.GetConnection(id) != original {
		t.Fatal("rejected handshake displaced accepted connection")
	}
	heartbeat, _ := remote.NewMessage(remote.MsgHeartbeat, id, key, 2, remote.HeartbeatPayload{Version: "still-live"})
	if err := good.WriteJSON(heartbeat); err != nil {
		t.Fatal(err)
	}
	good.SetReadDeadline(time.Now().Add(time.Second))
	// A protocol error must still be answered by the original reader.
	invalid.Type = remote.MsgHeartbeat
	if err := good.WriteJSON(invalid); err != nil {
		t.Fatal(err)
	}
	if err := good.ReadJSON(&response); err != nil {
		t.Fatalf("original reader stopped: %v", err)
	}
	if response.Type != remote.MsgError {
		t.Fatalf("unexpected response: %s", response.Type)
	}
}
