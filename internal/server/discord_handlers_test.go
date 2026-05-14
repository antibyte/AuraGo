package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestDiscordHealthReportsMissingToken(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.Enabled = true
	s := &Server{Cfg: cfg}

	req := httptest.NewRequest(http.MethodGet, "/api/health/discord", nil)
	rec := httptest.NewRecorder()

	handleDiscordHealth(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["status"] != "missing_token" || body["connected"] != false {
		t.Fatalf("body = %#v, want missing_token disconnected", body)
	}
}

func TestDiscordHealthReportsDisabledAsOK(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}

	req := httptest.NewRequest(http.MethodGet, "/api/health/discord", nil)
	rec := httptest.NewRecorder()

	handleDiscordHealth(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}
