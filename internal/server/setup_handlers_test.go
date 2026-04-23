package server

import (
	"aurago/internal/config"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	setupCSRFMu.Lock()
	setupCSRFToken = ""
	setupCSRFMu.Unlock()

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
	setupCSRFMu.Lock()
	setupCSRFToken = ""
	setupCSRFMu.Unlock()

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
	setupCSRFMu.Lock()
	setupCSRFToken = "test-csrf-token-12345"
	setupCSRFMu.Unlock()

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
	setupCSRFMu.Lock()
	setupCSRFToken = "correct-token"
	setupCSRFMu.Unlock()

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

	foundMiniMax := false
	for _, raw := range profiles {
		profile := raw.(map[string]interface{})
		if profile["id"] != "minimax_coding" {
			continue
		}
		foundMiniMax = true
		if profile["key_placeholder"] != "sk-..." {
			t.Fatalf("minimax key_placeholder = %v, want sk-...", profile["key_placeholder"])
		}
		if profile["base_url"] != "https://api.minimax.io/v1" {
			t.Fatalf("minimax base_url = %v, want international endpoint", profile["base_url"])
		}
		if profile["alt_base_url"] != "https://api.minimaxi.com/v1" {
			t.Fatalf("minimax alt_base_url = %v, want China endpoint", profile["alt_base_url"])
		}
		if profile["highspeed_model"] != "MiniMax-M2.7-highspeed" {
			t.Fatalf("minimax highspeed_model = %v, want MiniMax-M2.7-highspeed", profile["highspeed_model"])
		}
		models, ok := profile["models"].(map[string]interface{})
		if !ok {
			t.Fatal("expected minimax models map in response")
		}
		imageGen, ok := models["image_generation"].(map[string]interface{})
		if !ok {
			t.Fatal("expected minimax image_generation config in response")
		}
		if imageGen["provider_type"] != "minimax" {
			t.Fatalf("minimax image_generation provider_type = %v, want minimax", imageGen["provider_type"])
		}
		if imageGen["base_url"] != "https://api.minimax.io/v1/image_generation" {
			t.Fatalf("minimax image_generation base_url = %v, want international image endpoint", imageGen["base_url"])
		}
		if imageGen["alt_base_url"] != "https://api.minimaxi.com/v1/image_generation" {
			t.Fatalf("minimax image_generation alt_base_url = %v, want China image endpoint", imageGen["alt_base_url"])
		}
		if imageGen["model"] != "image-01" {
			t.Fatalf("minimax image_generation model = %v, want image-01", imageGen["model"])
		}
	}
	if !foundMiniMax {
		t.Fatal("expected minimax_coding profile in response")
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

func TestHandleSetupSaveAcceptsMiniMaxQuickPatch(t *testing.T) {
	t.Parallel()

	setupCSRFMu.Lock()
	setupCSRFToken = "minimax-setup-token"
	setupCSRFMu.Unlock()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	input, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template: %v", err)
	}
	if err := os.WriteFile(configPath, input, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	s := &Server{
		Cfg:    &config.Config{ConfigPath: configPath},
		Logger: slog.Default(),
	}
	s.Cfg.Server.UILanguage = "de"
	s.Cfg.Auth.Enabled = true

	patch := map[string]interface{}{
		"auth": map[string]interface{}{
			"enabled":        true,
			"admin_password": "supersecret",
		},
		"providers": []interface{}{
			map[string]interface{}{
				"id":                      "main",
				"type":                    "openai",
				"name":                    "MiniMax Coding Plan",
				"base_url":                "https://api.minimax.io/v1",
				"api_key":                 "sk-test",
				"model":                   "MiniMax-M2.7",
				"native_function_calling": true,
			},
			map[string]interface{}{
				"id":                      "vision",
				"type":                    "openai",
				"name":                    "MiniMax Coding Plan Vision",
				"base_url":                "https://api.minimax.io/v1",
				"api_key":                 "sk-test",
				"model":                   "MiniMax-M2.7",
				"native_function_calling": true,
			},
			map[string]interface{}{
				"id":                      "whisper",
				"type":                    "openai",
				"name":                    "MiniMax Coding Plan Whisper",
				"base_url":                "https://api.minimax.io/v1",
				"api_key":                 "sk-test",
				"model":                   "MiniMax-M2.7",
				"native_function_calling": true,
			},
			map[string]interface{}{
				"id":                      "embeddings",
				"type":                    "openai",
				"name":                    "MiniMax Coding Plan Embeddings",
				"base_url":                "https://api.minimax.io/v1",
				"api_key":                 "sk-test",
				"model":                   "minimax-embedding",
				"native_function_calling": false,
			},
			map[string]interface{}{
				"id":                      "helper",
				"type":                    "openai",
				"name":                    "MiniMax Coding Plan Helper",
				"base_url":                "https://api.minimax.io/v1",
				"api_key":                 "sk-test",
				"model":                   "MiniMax-M2.1",
				"native_function_calling": true,
			},
			map[string]interface{}{
				"id":                      "image_gen",
				"type":                    "minimax",
				"name":                    "MiniMax Coding Plan Image Gen",
				"base_url":                "https://api.minimax.io/v1/image_generation",
				"api_key":                 "sk-test",
				"model":                   "image-01",
				"native_function_calling": true,
			},
			map[string]interface{}{
				"id":                      "music_gen",
				"type":                    "minimax",
				"name":                    "MiniMax Coding Plan Music Gen",
				"base_url":                "https://api.minimax.io/v1",
				"api_key":                 "sk-test",
				"model":                   "music-01",
				"native_function_calling": true,
			},
		},
		"agent": map[string]interface{}{
			"system_language": "Deutsch",
		},
		"llm": map[string]interface{}{
			"provider":             "main",
			"use_native_functions": true,
			"helper_enabled":       true,
			"helper_provider":      "helper",
		},
		"embeddings": map[string]interface{}{
			"provider": "embeddings",
		},
		"vision": map[string]interface{}{
			"provider": "vision",
		},
		"whisper": map[string]interface{}{
			"provider": "whisper",
			"mode":     "multimodal",
		},
		"image_generation": map[string]interface{}{
			"enabled":  true,
			"provider": "image_gen",
		},
		"music_generation": map[string]interface{}{
			"enabled":  true,
			"provider": "music_gen",
		},
		"tts": map[string]interface{}{
			"provider": "minimax",
			"minimax": map[string]interface{}{
				"api_key":  "sk-test",
				"model_id": "speech-02-turbo",
				"voice_id": "male-qn-qingse",
			},
		},
	}

	body, err := json.Marshal(patch)
	if err != nil {
		t.Fatalf("marshal patch: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "minimax-setup-token")
	rec := httptest.NewRecorder()

	handleSetupSave(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}
