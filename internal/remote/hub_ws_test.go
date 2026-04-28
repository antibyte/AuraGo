package remote

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/security"
	"aurago/internal/testutil"

	"github.com/gorilla/websocket"
)

func TestHandleEnrollmentRejectsRevokedDeviceReconnectOverWebSocket(t *testing.T) {
	t.Parallel()

	db, err := InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}

	sharedKey, err := GenerateSharedKey()
	if err != nil {
		t.Fatalf("GenerateSharedKey: %v", err)
	}
	deviceID, err := CreateDevice(db, DeviceRecord{
		Name:          "revoked-device",
		Hostname:      "revoked-host",
		Status:        "revoked",
		ReadOnly:      true,
		SharedKeyHash: hashTokenSHA256(sharedKey),
	})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if err := vault.WriteSecret("remote_shared_key_"+deviceID, sharedKey); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	hub := NewRemoteHub(db, vault, slog.New(slog.NewTextHandler(io.Discard, nil)))
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("websocket upgrade: %v", err)
			return
		}
		defer conn.Close()

		var msg RemoteMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Errorf("read auth message: %v", err)
			return
		}
		if err := hub.HandleEnrollment(conn, msg); err != nil {
			t.Errorf("HandleEnrollment: %v", err)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer clientConn.Close()

	authMsg, err := NewMessage(MsgAuth, deviceID, sharedKey, 1, AuthPayload{
		DeviceID: deviceID,
		Version:  "test",
		Hostname: "revoked-host",
	})
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if err := clientConn.WriteJSON(authMsg); err != nil {
		t.Fatalf("write auth message: %v", err)
	}

	var response RemoteMessage
	if err := clientConn.ReadJSON(&response); err != nil {
		t.Fatalf("read auth response: %v", err)
	}
	if response.Type != MsgAuthResponse {
		t.Fatalf("response type = %q, want %q", response.Type, MsgAuthResponse)
	}

	var payload AuthResponsePayload
	if err := json.Unmarshal(response.Payload, &payload); err != nil {
		t.Fatalf("unmarshal auth response payload: %v", err)
	}
	if payload.Status != "rejected" {
		t.Fatalf("auth status = %q, want rejected", payload.Status)
	}
	if !strings.Contains(payload.Message, "revoked") {
		t.Fatalf("auth rejection message = %q, want revoked reason", payload.Message)
	}
	if hub.IsConnected(deviceID) {
		t.Fatal("revoked reconnect registered an active hub connection")
	}
	got, err := GetDevice(db, deviceID)
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if got.Status != "revoked" {
		t.Fatalf("device status = %q, want revoked", got.Status)
	}
}
