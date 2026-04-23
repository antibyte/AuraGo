package server

import (
	"aurago/internal/config"
	"aurago/internal/memory"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type panicVectorDB struct{}

func (panicVectorDB) StoreDocument(concept, content string) ([]string, error) {
	return nil, errors.New("not implemented")
}
func (panicVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	return "", errors.New("not implemented")
}
func (panicVectorDB) StoreDocumentInCollection(concept, content, collection string) ([]string, error) {
	return nil, errors.New("not implemented")
}
func (panicVectorDB) StoreDocumentWithEmbeddingInCollection(concept, content string, embedding []float32, collection string) (string, error) {
	return "", errors.New("not implemented")
}
func (panicVectorDB) StoreBatch(items []memory.ArchiveItem) ([]string, error) {
	return nil, errors.New("not implemented")
}
func (panicVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	return nil, nil, errors.New("not implemented")
}
func (panicVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	return nil, nil, errors.New("not implemented")
}
func (panicVectorDB) GetByIDFromCollection(id, collection string) (string, error) {
	return "", errors.New("not implemented")
}
func (panicVectorDB) GetByID(id string) (string, error) {
	return "", errors.New("not implemented")
}
func (panicVectorDB) DeleteDocument(id string) error {
	return errors.New("not implemented")
}
func (panicVectorDB) DeleteDocumentFromCollection(id, collection string) error {
	return errors.New("not implemented")
}
func (panicVectorDB) Count() int { return 0 }
func (panicVectorDB) IsDisabled() bool { panic("boom") }
func (panicVectorDB) Close() error { return nil }
func (panicVectorDB) StoreCheatsheet(id, name, content string, attachments ...string) error {
	return errors.New("not implemented")
}
func (panicVectorDB) DeleteCheatsheet(id string) error {
	return errors.New("not implemented")
}

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
				"id":                      "whisper",
				"type":                    "openai",
				"name":                    "MiniMax Coding Plan Whisper",
				"base_url":                "https://api.minimax.io/v1",
				"api_key":                 "sk-test",
				"model":                   "MiniMax-M2.7",
				"native_function_calling": true,
			},
			map[string]interface{}{
				"id":                      "helper",
				"type":                    "openai",
				"name":                    "MiniMax Coding Plan Helper",
				"base_url":                "https://api.minimax.io/v1",
				"api_key":                 "sk-test",
				"model":                   "MiniMax-M2.5",
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
				"base_url":                "https://api.minimax.io/v1/music_generation",
				"api_key":                 "sk-test",
				"model":                   "music-2.6",
				"native_function_calling": true,
			},
		},
		"agent": map[string]interface{}{
			"system_language": "Deutsch",
		},
		"server": map[string]interface{}{
			"ui_language": "de",
		},
		"llm": map[string]interface{}{
			"provider":             "main",
			"use_native_functions": true,
			"helper_enabled":       true,
			"helper_provider":      "helper",
			"structured_outputs":   true,
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
				"model_id": "speech-02-hd",
				"voice_id": "English_PlayfulGirl",
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

	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if loaded.Server.UILanguage != "de" {
		t.Fatalf("ui_language = %q, want de", loaded.Server.UILanguage)
	}
	if loaded.Agent.SystemLanguage != "Deutsch" {
		t.Fatalf("system_language = %q, want Deutsch", loaded.Agent.SystemLanguage)
	}
	if !loaded.LLM.StructuredOutputs {
		t.Fatal("structured_outputs should be enabled for minimax quick setup")
	}
}

func TestHandleSetupSaveAcceptsMiniMaxQuickPatchAgainstCurrentConfig(t *testing.T) {
	setupCSRFMu.Lock()
	setupCSRFToken = "minimax-current-config-token"
	setupCSRFMu.Unlock()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	input, err := os.ReadFile(filepath.Join("..", "..", "config.yaml"))
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
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
			map[string]interface{}{"id": "main", "type": "openai", "name": "MiniMax Coding Plan", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-M2.7", "native_function_calling": true},
			map[string]interface{}{"id": "whisper", "type": "openai", "name": "MiniMax Coding Plan Whisper", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-M2.7", "native_function_calling": true},
			map[string]interface{}{"id": "helper", "type": "openai", "name": "MiniMax Coding Plan Helper", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-M2.5", "native_function_calling": true},
			map[string]interface{}{"id": "image_gen", "type": "minimax", "name": "MiniMax Coding Plan Image Gen", "base_url": "https://api.minimax.io/v1/image_generation", "api_key": "sk-test", "model": "image-01", "native_function_calling": true},
			map[string]interface{}{"id": "music_gen", "type": "minimax", "name": "MiniMax Coding Plan Music Gen", "base_url": "https://api.minimax.io/v1/music_generation", "api_key": "sk-test", "model": "music-2.6", "native_function_calling": true},
		},
		"agent": map[string]interface{}{"system_language": "Deutsch"},
		"llm": map[string]interface{}{"provider": "main", "use_native_functions": true, "helper_enabled": true, "helper_provider": "helper", "structured_outputs": true},
		"whisper": map[string]interface{}{"provider": "whisper", "mode": "multimodal"},
		"image_generation": map[string]interface{}{"enabled": true, "provider": "image_gen"},
		"music_generation": map[string]interface{}{"enabled": true, "provider": "music_gen"},
		"tts": map[string]interface{}{"provider": "minimax", "minimax": map[string]interface{}{"api_key": "sk-test", "model_id": "speech-02-hd", "voice_id": "English_PlayfulGirl"}},
	}

	body, err := json.Marshal(patch)
	if err != nil {
		t.Fatalf("marshal patch: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "minimax-current-config-token")
	rec := httptest.NewRecorder()

	handleSetupSave(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSetupSaveReturnsRestartRequiredWhenHotReloadPanics(t *testing.T) {
	setupCSRFMu.Lock()
	setupCSRFToken = "minimax-panic-token"
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
		Cfg:         &config.Config{ConfigPath: configPath},
		Logger:      slog.Default(),
		LongTermMem: panicVectorDB{},
	}
	s.Cfg.Server.UILanguage = "de"
	s.Cfg.Auth.Enabled = true

	patch := map[string]interface{}{
		"auth": map[string]interface{}{
			"enabled":        true,
			"admin_password": "supersecret",
		},
		"providers": []interface{}{
			map[string]interface{}{"id": "main", "type": "openai", "name": "MiniMax Coding Plan", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-M2.7", "native_function_calling": true},
			map[string]interface{}{"id": "whisper", "type": "openai", "name": "MiniMax Coding Plan Whisper", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-M2.7", "native_function_calling": true},
			map[string]interface{}{"id": "helper", "type": "openai", "name": "MiniMax Coding Plan Helper", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-M2.5", "native_function_calling": true},
			map[string]interface{}{"id": "image_gen", "type": "minimax", "name": "MiniMax Coding Plan Image Gen", "base_url": "https://api.minimax.io/v1/image_generation", "api_key": "sk-test", "model": "image-01", "native_function_calling": true},
			map[string]interface{}{"id": "music_gen", "type": "minimax", "name": "MiniMax Coding Plan Music Gen", "base_url": "https://api.minimax.io/v1/music_generation", "api_key": "sk-test", "model": "music-2.6", "native_function_calling": true},
		},
		"agent": map[string]interface{}{"system_language": "Deutsch"},
		"llm": map[string]interface{}{"provider": "main", "use_native_functions": true, "helper_enabled": true, "helper_provider": "helper", "structured_outputs": true},
		"whisper": map[string]interface{}{"provider": "whisper", "mode": "multimodal"},
		"image_generation": map[string]interface{}{"enabled": true, "provider": "image_gen"},
		"music_generation": map[string]interface{}{"enabled": true, "provider": "music_gen"},
		"tts": map[string]interface{}{"provider": "minimax", "minimax": map[string]interface{}{"api_key": "sk-test", "model_id": "speech-02-hd", "voice_id": "English_PlayfulGirl"}},
	}

	body, err := json.Marshal(patch)
	if err != nil {
		t.Fatalf("marshal patch: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "minimax-panic-token")
	rec := httptest.NewRecorder()

	handleSetupSave(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Status       string   `json:"status"`
		NeedsRestart bool     `json:"needs_restart"`
		Reasons      []string `json:"restart_reason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Status != "saved" || !resp.NeedsRestart {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
