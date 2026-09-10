package server

import (
	"aurago/internal/desktop"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHASwitchboardHandlers(t *testing.T) {
	s, readToken, writeToken := testDesktopPermissionServer(t)
	admin, _, err := s.TokenManager.Create("switchboard", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.Directories.DataDir = t.TempDir()
	s.Cfg.VirtualDesktop.WorkspaceDir = filepath.Join(t.TempDir(), "workspace")
	s.Cfg.SQLite.VirtualDesktopPath = filepath.Join(t.TempDir(), "desktop.db")
	t.Cleanup(func() {
		if s.DesktopService != nil {
			_ = s.DesktopService.Close()
		}
		if s.DesktopHub != nil {
			s.DesktopHub.Close()
		}
	})
	state, haError := "off", false
	posts, reads := 0, 0
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-ha-token" {
			t.Error("missing HA token")
		}
		if haError {
			http.Error(w, "private upstream details and test-ha-token", 502)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/states":
			reads++
			_ = json.NewEncoder(w).Encode([]interface{}{
				map[string]interface{}{"entity_id": "switch.desk", "state": state, "attributes": map[string]string{"friendly_name": "<img src=x onerror=alert(1)>", "private": "hidden"}},
				map[string]string{"entity_id": "switch.other", "state": "on"},
				map[string]string{"entity_id": "light.secret", "state": "on"},
			})
		case "/api/services/switch/turn_on", "/api/services/switch/turn_off":
			if r.Method != "POST" {
				t.Error("wrong HA method")
			}
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body) != 1 || body["entity_id"] != "switch.desk" {
				t.Errorf("wrong target: %+v", body)
			}
			posts++
			state = strings.TrimPrefix(r.URL.Path, "/api/services/switch/turn_")
			_, _ = w.Write([]byte(`[{"entity_id":"sun.sun","state":"above_horizon"}]`))
		default:
			t.Errorf("unexpected HA request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer ha.Close()
	call := func(method, path, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/desktop/home-assistant/"+path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handleDesktopHomeAssistant(s)(w, r)
		return w
	}
	for _, token := range []string{"", readToken, writeToken} {
		if w := call("GET", "entities", token, ""); w.Code != 401 && w.Code != 403 {
			t.Fatalf("unauthorized: %d", w.Code)
		}
	}
	if w := call("GET", "entities", admin, ""); w.Code != 200 || !strings.Contains(w.Body.String(), "setup_required") || reads != 0 {
		t.Fatalf("setup: %d %s", w.Code, w.Body.String())
	}
	s.Cfg.HomeAssistant.Enabled = true
	s.Cfg.HomeAssistant.URL = ha.URL
	s.Cfg.HomeAssistant.AccessToken = "test-ha-token"
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetSetting(context.Background(), desktop.HASwitchboardSetting, `{"version":1,"switches":[{"entity_id":"switch.desk"},{"entity_id":"switch.deleted"}]}`, desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	if w := call("GET", "entities", admin, ""); w.Code != 200 || !strings.Contains(w.Body.String(), "switch.other") || strings.Contains(w.Body.String(), "light.secret") || strings.Contains(w.Body.String(), "hidden") {
		t.Fatalf("catalog: %d %s", w.Code, w.Body.String())
	}
	if w := call("GET", "states", admin, ""); w.Code != 200 || strings.Contains(w.Body.String(), "switch.other") || !strings.Contains(w.Body.String(), "switch.deleted") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("states: %d %s", w.Code, w.Body.String())
	}
	for _, body := range []string{
		`{"entity_id":"switch.other","state":"on"}`, `{"entity_id":"switch.desk","state":"toggle"}`,
		`{"entity_id":"switch.desk","state":"on","domain":"lock"}`, `{"entity_id":"switch.desk","state":"on"}{}`,
		`{"entity_id":"switch.desk/..","state":"on"}`, strings.Repeat("x", 1025),
	} {
		if w := call("POST", "switch", admin, body); w.Code != 400 {
			t.Fatalf("invalid command: %d %s", w.Code, w.Body.String())
		}
	}
	on := `{"entity_id":"switch.desk","state":"on"}`
	s.Cfg.HomeAssistant.ReadOnly = true
	if w := call("POST", "switch", admin, on); w.Code != 403 {
		t.Fatalf("HA readonly: %d", w.Code)
	}
	s.Cfg.HomeAssistant.ReadOnly = false
	s.Cfg.VirtualDesktop.ReadOnly = true
	if w := call("POST", "switch", admin, on); w.Code != 403 {
		t.Fatalf("desktop readonly: %d", w.Code)
	}
	s.Cfg.VirtualDesktop.ReadOnly = false
	s.Cfg.HomeAssistant.AllowedServices = []string{"switch.turn_off"}
	if w := call("POST", "switch", admin, on); w.Code != 403 {
		t.Fatalf("allowlist: %d", w.Code)
	}
	s.Cfg.HomeAssistant.AllowedServices = []string{"switch.turn_on"}
	s.Cfg.HomeAssistant.BlockedServices = []string{"SWITCH.TURN_ON"}
	if w := call("POST", "switch", admin, on); w.Code != 403 {
		t.Fatalf("blocklist: %d", w.Code)
	}
	if posts != 0 {
		t.Fatal("denied call reached HA")
	}
	s.Cfg.HomeAssistant.AllowedServices = nil
	s.Cfg.HomeAssistant.BlockedServices = nil
	state = "unavailable"
	if w := call("POST", "switch", admin, on); w.Code != 409 || posts != 0 {
		t.Fatalf("unavailable: %d", w.Code)
	}
	state = "off"
	if w := call("POST", "switch", admin, on); w.Code != 200 || posts != 1 || state != "on" || !strings.Contains(w.Body.String(), `"accepted":true`) {
		t.Fatalf("switch: %d %s", w.Code, w.Body.String())
	}
	if w := call("GET", "states", admin, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"state":"on"`) {
		t.Fatalf("confirmation: %s", w.Body.String())
	}
	haError = true
	if w := call("GET", "states", admin, ""); w.Code != 502 || strings.Contains(w.Body.String(), "test-ha-token") || strings.Contains(w.Body.String(), "private upstream") {
		t.Fatalf("error projection: %d %s", w.Code, w.Body.String())
	}
	s.Cfg.HomeAssistant.Enabled = false
	if w := call("POST", "switch", admin, on); w.Code != 503 || posts != 1 {
		t.Fatalf("disabled: %d", w.Code)
	}
}

func TestHASwitchboardCSRF(t *testing.T) {
	s, _, _ := testDesktopPermissionServer(t)
	s.Logger = slog.Default()
	handler := authMiddleware(s, handleDesktopHomeAssistant(s))
	for _, origin := range []string{"", "https://foreign.example"} {
		r := httptest.NewRequest(http.MethodPost, "https://aurago.example/api/desktop/home-assistant/switch", strings.NewReader(`{"entity_id":"switch.desk","state":"on"}`))
		r.Header.Set("Origin", origin)
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("cross-origin mutation admitted: %d", w.Code)
		}
	}
}
