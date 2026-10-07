package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/gorilla/websocket"
)

func TestDesktopHTTPAndWebSocketShareScopeProjection(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	auth, readToken, _ := testDesktopPermissionServer(t)
	s.TokenManager, s.Vault = auth.TokenManager, auth.Vault
	s.Cfg.Auth = auth.Cfg.Auth
	r := httptest.NewRequest(http.MethodGet, "/api/desktop/bootstrap", nil)
	r.Header.Set("Authorization", "Bearer "+readToken)
	w := httptest.NewRecorder()
	handleDesktopBootstrap(s)(w, r)
	if w.Code != 200 {
		t.Fatalf("bootstrap: %d %s", w.Code, w.Body.String())
	}
	var boot desktop.BootstrapPayload
	if err := json.Unmarshal(w.Body.Bytes(), &boot); err != nil {
		t.Fatal(err)
	}
	if boot.Settings != nil || boot.AllWidgets != nil || boot.Providers != nil || !boot.ReadOnly {
		t.Fatal("HTTP leaked administrative fields")
	}
	server := httptest.NewServer(handleDesktopWS(s))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/desktop/ws", r.Header)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var welcome struct {
		Type    string                   `json:"type"`
		Payload desktop.BootstrapPayload `json:"payload"`
	}
	if err := conn.ReadJSON(&welcome); err != nil {
		t.Fatal(err)
	}
	if welcome.Type != "welcome" || welcome.Payload.Settings != nil || welcome.Payload.AllWidgets != nil || !welcome.Payload.ReadOnly {
		t.Fatal("WebSocket leaked administrative fields")
	}
	s.DesktopHub.Broadcast(desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "set_settings", "settings": map[string]string{"private": "fixture-private-setting"}}})
	s.DesktopHub.Broadcast(desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{"operation": "write_file", "path": "Desktop/file.txt", "private": "fixture-private-setting"}})
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private") || !strings.Contains(string(data), "write_file") {
		t.Fatalf("event projection: %s", data)
	}
}
