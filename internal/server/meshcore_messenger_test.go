package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/meshcore"
)

func TestMeshCoreMessengerAdministrativeBoundary(t *testing.T) {
	s, client := meshCoreTestServer(t)
	s.Cfg.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(s.Cfg.ConfigPath, []byte("server:\n  port: 8088\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.initMeshCore(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { s.MeshCore.Close(); meshcore.SetDefaultManager(nil) }()
	mux := http.NewServeMux()
	registerMeshCoreRoutes(mux, s)
	call := func(method, action, body, origin string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/api/meshcore/messenger/"+action, strings.NewReader(body))
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "bootstrap", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"conversations":[]`) || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("bootstrap: %d %s", w.Code, w.Body)
	}
	for _, action := range []string{"send", "manage", "invitation", "reveal", "settings", "conversation", "device-settings"} {
		if w := call("POST", action, `{}`, "https://untrusted.example"); w.Code != 403 {
			t.Fatalf("origin %s: %d", action, w.Code)
		}
		if w := call("GET", action, "", ""); w.Code != 405 {
			t.Fatalf("method %s: %d", action, w.Code)
		}
		if w := call("POST", action, `{"raw_command":1}`, ""); w.Code != 400 {
			t.Fatalf("raw %s: %d", action, w.Code)
		}
	}
	if w := call("POST", "send", `{"id":"valid-request-123456","conversation":"`+strings.Repeat("a", 64)+`","text":"hello"}`, ""); w.Code != 409 || !strings.Contains(w.Body.String(), "not_connected") {
		t.Fatalf("disabled send: %d %s", w.Code, w.Body)
	}
	if w := call("POST", "invitation", `{}`, ""); w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "secret=") {
		t.Fatal("export response cache or disclosure")
	}
	if w := call("POST", "device-settings", `{"section":"clock"}`, ""); w.Code != 409 || !strings.Contains(w.Body.String(), "permission_denied") {
		t.Fatal("device gate", w.Code, w.Body)
	}
	if w := call("POST", "diagnostics", `{"identity":"`+strings.Repeat("a", 64)+`","target":"`+strings.Repeat("b", 64)+`","kind":"telemetry"}`, ""); w.Code != 409 || !strings.Contains(w.Body.String(), "permission_denied") {
		t.Fatal("diagnostic gate", w.Code, w.Body)
	}
	if w := call("POST", "diagnostics", `{}`, "https://untrusted.example"); w.Code != 403 {
		t.Fatal("diagnostic origin", w.Code)
	}
	revision := meshCoreSettingsRevision(s.ConfigSnapshot().MeshCore)
	if w := call("POST", "settings", `{"history_days":120,"history_messages":15000,"revision":"`+revision+`","allow_device_settings":true,"allow_remote_diagnostics":true}`, ""); w.Code != 200 {
		t.Fatalf("settings: %d %s", w.Code, w.Body)
	}
	if w := call("POST", "settings", `{"history_days":120,"history_messages":15000,"revision":"`+revision+`"}`, ""); w.Code != 409 {
		t.Fatal("app conflict", w.Code)
	}
	data, err := os.ReadFile(s.Cfg.ConfigPath)
	if err != nil || !strings.Contains(string(data), "port: 8088") || !strings.Contains(string(data), "history_days: 120") || s.ConfigSnapshot().MeshCore.HistoryMessages != 15000 {
		t.Fatalf("config persistence: %s %v", data, err)
	}
	if client.requestCount() != 0 {
		t.Fatal("human messenger invoked an LLM")
	}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "test"
	s.Cfg.WebConfig.Enabled = true
	for _, action := range []string{"bootstrap", "messages", "conversations", "invitation", "send", "reveal", "manage", "settings", "conversation", "device", "device-settings", "diagnostics"} {
		method := "POST"
		if action == "bootstrap" || action == "messages" || action == "conversations" || action == "device" || action == "diagnostics" {
			method = "GET"
		}
		if w := call(method, action, `{}`, ""); w.Code != 401 {
			t.Fatalf("unauthenticated %s: %d", action, w.Code)
		}
	}
}
