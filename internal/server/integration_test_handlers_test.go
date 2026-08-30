package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestHandleHomeAssistantTestUsesSavedReadOnlyProbe(t *testing.T) {
	const token = "home-assistant-handler-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/" {
			t.Fatalf("request = %s %s, want GET /api/", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Fatalf("unexpected authorization header")
		}
		fmt.Fprint(w, `{"message":"API running."}`)
	}))
	defer server.Close()

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.HomeAssistant.Enabled = true
	s.Cfg.HomeAssistant.URL = server.URL
	s.Cfg.HomeAssistant.AccessToken = token
	rec := httptest.NewRecorder()
	handleHomeAssistantTest(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/home-assistant/test", strings.NewReader(`{}`)))

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if rec.Code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("status = %d, body = %#v, want 200/ok", rec.Code, body)
	}
	if strings.Contains(rec.Body.String(), token) {
		t.Fatalf("response leaked access token: %s", rec.Body.String())
	}
}

func TestHandleConfiguredIntegrationTestRejectsDisabledIntegration(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Discord.BotToken = "discord-handler-token"
	rec := httptest.NewRecorder()
	handleDiscordTest(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/discord/test", nil))

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "disabled") {
		t.Fatalf("status = %d, body = %s, want disabled bad request", rec.Code, rec.Body.String())
	}
}

func TestHandleConfiguredIntegrationTestRejectsNonPost(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	rec := httptest.NewRecorder()
	handleTelegramConnectionTest(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/telegram/test", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestIntegrationTestRoutesRequireAdminSession(t *testing.T) {
	cfg := &config.Config{}
	cfg.WebConfig.Enabled = true
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "integration-test-session-secret"
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	mux := http.NewServeMux()
	s.registerConfigAPIRoutes(mux, nil)

	paths := []string{
		"/api/telegram/test",
		"/api/discord/test",
		"/api/rocketchat/test",
		"/api/home-assistant/test",
		"/api/proxmox/test",
		"/api/s3/test",
		"/api/frigate/test",
		"/api/ansible/test",
	}
	for _, path := range paths {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want %d; body=%s", path, rec.Code, http.StatusUnauthorized, rec.Body.String())
		}
	}
}
