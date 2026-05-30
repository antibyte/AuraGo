package desktop

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/inventory"
	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

func TestHandleVNCProxyMissingDeviceID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vault := &security.Vault{}
	handler := HandleVNCProxy(db, vault, testLogger)

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/vnc", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "missing device_id") {
		t.Fatalf("expected missing device_id error, got %q", rec.Body.String())
	}
}

func TestHandleVNCProxyProtocolMismatch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := inventory.AddDevice(db, inventory.DeviceRecord{ID: "test-ssh", Name: "test-ssh", Type: "server", Protocol: "ssh", IPAddress: "192.168.1.1", Port: 22, Username: "user", Description: "desc", Tags: []string{}})
	if err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	vault := &security.Vault{}
	handler := HandleVNCProxy(db, vault, testLogger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "?device_id=test-ssh"
	header := http.Header{"Origin": []string{server.URL}}
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}
	defer ws.Close()

	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("expected error message, got read error: %v", err)
	}
	if !strings.Contains(string(msg), "expected vnc") {
		t.Fatalf("expected protocol mismatch error, got %q", string(msg))
	}
}

func TestHandleVNCProxyDeviceNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vault := &security.Vault{}
	handler := HandleVNCProxy(db, vault, testLogger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http", "ws", 1) + "?device_id=nonexistent"
	header := http.Header{"Origin": []string{server.URL}}
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("unexpected dial error: %v", err)
	}
	defer ws.Close()

	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("expected error message, got read error: %v", err)
	}
	if !strings.Contains(string(msg), "not found") {
		t.Fatalf("expected device not found error, got %q", string(msg))
	}
}

func TestResolveVNCAccessDefaults(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	device := inventory.DeviceRecord{
		Name:      "vnc-test",
		Type:      "server",
		Protocol:  "vnc",
		IPAddress: "192.168.1.50",
		Port:      0,
	}

	// No credential, no vault
	host, port, password, err := resolveVNCAccess(device, db, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "192.168.1.50" {
		t.Fatalf("expected host 192.168.1.50, got %s", host)
	}
	if port != 5900 {
		t.Fatalf("expected port 5900, got %d", port)
	}
	if password != "" {
		t.Fatalf("expected empty password, got %q", password)
	}
}
