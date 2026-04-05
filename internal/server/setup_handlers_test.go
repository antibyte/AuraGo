package server

import (
	"aurago/internal/config"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestNeedsSetupRequiresPasswordWhenAuthEnabled(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.LLM.APIKey = "configured"
	cfg.Auth.Enabled = true

	if !needsSetup(cfg) {
		t.Fatal("expected setup to remain required while auth is enabled and no password is set")
	}

	cfg.Auth.PasswordHash = "hash"
	if needsSetup(cfg) {
		t.Fatal("expected setup to be complete once provider and password are configured")
	}
}

func TestExtractSetupAdminPasswordStripsTemporaryField(t *testing.T) {
	t.Parallel()

	patch := map[string]interface{}{
		"auth": map[string]interface{}{
			"enabled":        true,
			"admin_password": "supersecret",
		},
	}

	password, authEnabled, err := extractSetupAdminPassword(patch, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !authEnabled {
		t.Fatal("expected auth to stay enabled")
	}
	if password != "supersecret" {
		t.Fatalf("unexpected password %q", password)
	}

	authPatch := patch["auth"].(map[string]interface{})
	if _, exists := authPatch["admin_password"]; exists {
		t.Fatal("expected temporary admin_password field to be removed before config merge")
	}
}

func TestExtractSetupAdminPasswordAllowsExistingPasswordToRemain(t *testing.T) {
	t.Parallel()

	patch := map[string]interface{}{
		"auth": map[string]interface{}{
			"enabled": true,
		},
	}

	password, authEnabled, err := extractSetupAdminPassword(patch, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !authEnabled {
		t.Fatal("expected auth to stay enabled")
	}
	if password != "" {
		t.Fatalf("expected empty password when keeping existing one, got %q", password)
	}
}

func TestHandleSetupStatusReturnsCSRFToken(t *testing.T) {
	// Reset global CSRF state for this test.
	setupCSRFOnce = sync.Once{}
	setupCSRFToken = ""

	s := &Server{Cfg: &config.Config{}}
	// Config has no provider → needsSetup returns true

	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	rec := httptest.NewRecorder()
	handleSetupStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body["needs_setup"] != true {
		t.Fatal("expected needs_setup=true")
	}
	token, ok := body["csrf_token"].(string)
	if !ok || len(token) < 16 {
		t.Fatalf("expected csrf_token of sufficient length, got %q", token)
	}
}

func TestHandleSetupStatusNoCSRFWhenConfigured(t *testing.T) {
	setupCSRFOnce = sync.Once{}
	setupCSRFToken = ""

	s := &Server{Cfg: &config.Config{}}
	s.Cfg.LLM.APIKey = "configured"

	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	rec := httptest.NewRecorder()
	handleSetupStatus(s).ServeHTTP(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body["needs_setup"] != false {
		t.Fatal("expected needs_setup=false")
	}
	if _, exists := body["csrf_token"]; exists {
		t.Fatal("CSRF token should not be returned when setup is complete")
	}
}

func TestHandleSetupSaveRejectsWithoutCSRF(t *testing.T) {
	setupCSRFOnce = sync.Once{}
	setupCSRFToken = "test-csrf-token-12345"
	setupCSRFOnce.Do(func() {}) // mark as done so the token is used as-is

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleSetupSave(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "CSRF") {
		t.Fatalf("expected CSRF error message, got %q", rec.Body.String())
	}
}

func TestHandleSetupSaveRejectsWrongCSRF(t *testing.T) {
	setupCSRFOnce = sync.Once{}
	setupCSRFToken = "correct-token"
	setupCSRFOnce.Do(func() {})

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "wrong-token")
	rec := httptest.NewRecorder()
	handleSetupSave(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestHandleSetupProfilesReturnsProfiles(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodGet, "/api/setup/profiles", nil)
	rec := httptest.NewRecorder()
	handleSetupProfiles(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}

	profiles, ok := body["profiles"].([]interface{})
	if !ok {
		t.Fatal("expected profiles array in response")
	}
	if len(profiles) < 2 {
		t.Fatalf("expected at least 2 profiles, got %d", len(profiles))
	}

	// Verify first profile has required fields
	first := profiles[0].(map[string]interface{})
	for _, field := range []string{"id", "name", "description", "icon"} {
		if _, ok := first[field]; !ok {
			t.Fatalf("missing field %q in first profile", field)
		}
	}
}

func TestHandleSetupProfilesRejectsPost(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodPost, "/api/setup/profiles", nil)
	rec := httptest.NewRecorder()
	handleSetupProfiles(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}
