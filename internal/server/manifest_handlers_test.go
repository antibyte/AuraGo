package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestManifestStatusDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}

	req := httptest.NewRequest(http.MethodGet, "/api/manifest/status", nil)
	rec := httptest.NewRecorder()

	handleManifestStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["status"] != "disabled" {
		t.Fatalf("status = %#v, want disabled", body["status"])
	}
}

func TestManifestStatusReportsMissingSecrets(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Manifest.Enabled = true
	s.Cfg.Manifest.Mode = "managed"
	s.Cfg.Manifest.URL = "http://127.0.0.1:2099"

	req := httptest.NewRequest(http.MethodGet, "/api/manifest/status", nil)
	rec := httptest.NewRecorder()

	handleManifestStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "setup_required") || !strings.Contains(rec.Body.String(), "postgres password") {
		t.Fatalf("body = %s, want setup_required missing secret message", rec.Body.String())
	}
}

func TestManifestStartRequiresPost(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}

	req := httptest.NewRequest(http.MethodGet, "/api/manifest/start", nil)
	rec := httptest.NewRecorder()

	handleManifestStart(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status code = %d, want 405", rec.Code)
	}
}

func TestManifestTestAcceptsPatchAndReportsSetupRequired(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	body := strings.NewReader(`{"manifest":{"enabled":true,"mode":"managed","url":"http://127.0.0.1:2099"}}`)

	req := httptest.NewRequest(http.MethodPost, "/api/manifest/test", body)
	rec := httptest.NewRecorder()

	handleManifestTest(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "setup_required") {
		t.Fatalf("body = %s, want setup_required", rec.Body.String())
	}
}
