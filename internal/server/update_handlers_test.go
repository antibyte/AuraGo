package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestHandleUpdateInstallRequiresSelfUpdatePermission(t *testing.T) {
	s := &Server{
		Cfg:    &config.Config{},
		Logger: slog.Default(),
	}
	s.Cfg.ConfigPath = t.TempDir() + "/config.yaml"
	s.Cfg.Agent.AllowSelfUpdate = false

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/updates/install", nil)

	handleUpdateInstall(s)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("response missing error: %#v", body)
	}
}

func TestHandleUpdateInstallKeepsMissingScriptPathWhenAllowed(t *testing.T) {
	s := &Server{
		Cfg:    &config.Config{},
		Logger: slog.Default(),
	}
	s.Cfg.ConfigPath = t.TempDir() + "/config.yaml"
	s.Cfg.Agent.AllowSelfUpdate = true

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/updates/install", nil)

	handleUpdateInstall(s)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
