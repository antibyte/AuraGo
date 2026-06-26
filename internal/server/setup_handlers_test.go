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
	"time"
)

type panicVectorDB struct{}

func addSetupCSRFTokenForTest(s *Server, token string) {
	s.SetupCSRFMu.Lock()
	defer s.SetupCSRFMu.Unlock()
	if s.SetupCSRFTokens == nil {
		s.SetupCSRFTokens = make(map[string]time.Time)
	}
	s.SetupCSRFTokens[token] = time.Now().Add(setupCSRFTokenTTL)
}

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
func (panicVectorDB) Count() int       { return 0 }
func (panicVectorDB) IsDisabled() bool { panic("boom") }
func (panicVectorDB) IsReady() bool    { return true }
func (panicVectorDB) Close() error     { return nil }
func (panicVectorDB) StoreCheatsheet(id, name, content string, attachments ...string) error {
	return errors.New("not implemented")
}
func (panicVectorDB) DeleteCheatsheet(id string) error {
	return errors.New("not implemented")
}
func (panicVectorDB) RegisterCollections(collections []string) {}

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

func TestNeedsSetupRequiresOAuthTokenForOAuthProvider(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Providers: []config.ProviderEntry{{
			ID:            "main",
			Type:          "openai",
			BaseURL:       "https://api.example/v1",
			Model:         "model",
			AuthType:      "oauth2",
			OAuthAuthURL:  "https://accounts.example/authorize",
			OAuthTokenURL: "https://accounts.example/token",
			OAuthClientID: "client-id",
		}},
	}
	cfg.LLM.Provider = "main"

	if !needsSetup(cfg) {
		t.Fatal("expected setup to remain required until an OAuth access token is applied")
	}
}

func TestNeedsSetupAcceptsOAuthProviderWithAppliedToken(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Providers: []config.ProviderEntry{{
			ID:            "main",
			Type:          "openai",
			BaseURL:       "https://api.example/v1",
			Model:         "model",
			AuthType:      "oauth2",
			OAuthAuthURL:  "https://accounts.example/authorize",
			OAuthTokenURL: "https://accounts.example/token",
			OAuthClientID: "client-id",
		}},
	}
	cfg.LLM.Provider = "main"
	cfg.LLM.APIKey = "oauth-access-token"

	if needsSetup(cfg) {
		t.Fatal("expected setup to be complete once an OAuth access token is applied")
	}
}

func TestValidateSetupAdminPasswordStripsTemporaryField(t *testing.T) {
	t.Parallel()

	patch := map[string]interface{}{
		"auth": map[string]interface{}{
			"enabled":        true,
			"admin_password": "supersecret",
		},
	}

	authPatch, _ := patch["auth"].(map[string]interface{})
	password, authEnabled, err := validateSetupAdminPassword(authPatch, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !authEnabled {
		t.Fatal("expected auth to stay enabled")
	}
	if password != "supersecret" {
		t.Fatalf("unexpected password %q", password)
	}

	// validateSetupAdminPassword must NOT mutate the patch.
	if _, exists := authPatch["admin_password"]; !exists {
		t.Fatal("validateSetupAdminPassword should not strip admin_password; stripSetupAdminPassword does that")
	}

	// Now strip and verify it's gone.
	stripSetupAdminPassword(authPatch)
	if _, exists := authPatch["admin_password"]; exists {
		t.Fatal("expected stripSetupAdminPassword to remove admin_password")
	}
}

func TestValidateSetupAdminPasswordAllowsExistingPasswordToRemain(t *testing.T) {
	t.Parallel()

	patch := map[string]interface{}{
		"auth": map[string]interface{}{
			"enabled": true,
		},
	}

	authPatch, _ := patch["auth"].(map[string]interface{})
	password, authEnabled, err := validateSetupAdminPassword(authPatch, true, true)
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

func TestApplySetupProfileConfigPatchAppliesMiniMaxDefaults(t *testing.T) {
	t.Parallel()

	patch := map[string]interface{}{
		"_setup_profile_id": "minimax_coding",
		"llm": map[string]interface{}{
			"provider": "main",
		},
	}
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}

	applySetupProfileConfigPatch(patch, s)

	if _, exists := patch["_setup_profile_id"]; exists {
		t.Fatal("expected setup profile marker to be removed before config merge")
	}
	llmPatch, ok := patch["llm"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected llm patch map, got %T", patch["llm"])
	}
	if got := llmPatch["structured_outputs"]; got != true {
		t.Fatalf("structured_outputs = %v, want true from minimax config_patch", got)
	}
	if got := llmPatch["provider"]; got != "main" {
		t.Fatalf("provider = %v, want client value main", got)
	}
}

func TestHandleSetupStatusReturnsCSRFToken(t *testing.T) {
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

func TestSetupCSRFTokensAllowMultipleTabs(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}

	tokenA := issueSetupCSRFToken(s)
	tokenB := issueSetupCSRFToken(s)

	if tokenA == "" || tokenB == "" || tokenA == tokenB {
		t.Fatalf("expected distinct non-empty tokens, got %q and %q", tokenA, tokenB)
	}
	if !validateSetupCSRFToken(s, tokenA, true) {
		t.Fatal("expected first token to validate")
	}
	if validateSetupCSRFToken(s, tokenA, false) {
		t.Fatal("expected consumed first token to be rejected")
	}
	if !validateSetupCSRFToken(s, tokenB, false) {
		t.Fatal("expected second token to remain valid")
	}
}

func TestValidateSetupTestBaseURLRejectsPrivateProviderURL(t *testing.T) {
	if err := validateSetupTestBaseURL("openai", "https://127.0.0.1/v1"); err == nil {
		t.Fatal("expected private provider URL to be rejected")
	}
}

func TestValidateSetupTestBaseURLAllowsKnownProviderURL(t *testing.T) {
	if err := validateSetupTestBaseURL("openrouter", "https://openrouter.ai/api/v1"); err != nil {
		t.Fatalf("expected known provider URL to be allowed: %v", err)
	}
}

func TestValidateSetupTestBaseURLAllowsCustomPublicHTTPSURL(t *testing.T) {
	oldValidate := validateSetupProviderSSRF
	validateSetupProviderSSRF = func(rawURL string) error {
		if rawURL != "https://llm.example.com/v1" {
			t.Fatalf("SSRF validator got %q", rawURL)
		}
		return nil
	}
	t.Cleanup(func() { validateSetupProviderSSRF = oldValidate })

	if err := validateSetupTestBaseURL("custom", "https://llm.example.com/v1"); err != nil {
		t.Fatalf("expected custom public HTTPS URL to be allowed: %v", err)
	}
}

func TestValidateSetupTestBaseURLRejectsCustomURLWhenSSRFValidatorBlocks(t *testing.T) {
	oldValidate := validateSetupProviderSSRF
	validateSetupProviderSSRF = func(rawURL string) error {
		if rawURL != "https://internal.example/v1" {
			t.Fatalf("SSRF validator got %q", rawURL)
		}
		return errors.New("access to internal address 127.0.0.1 is blocked")
	}
	t.Cleanup(func() { validateSetupProviderSSRF = oldValidate })

	if err := validateSetupTestBaseURL("custom", "https://internal.example/v1"); err == nil {
		t.Fatal("expected custom URL blocked by SSRF validator to be rejected")
	}
}

func TestValidateSetupTestBaseURLAllowsDockerOllamaURL(t *testing.T) {
	if got := setupOllamaBaseURL(true); got != "http://host.docker.internal:11434/v1" {
		t.Fatalf("docker Ollama base URL = %q", got)
	}
	if err := validateSetupTestBaseURL("ollama", setupOllamaBaseURL(true)); err != nil {
		t.Fatalf("expected Docker Ollama URL to be allowed: %v", err)
	}
}

func TestValidateSetupTestBaseURLRejectsOllamaUnexpectedPort(t *testing.T) {
	if err := validateSetupTestBaseURL("ollama", "http://host.docker.internal:8080/v1"); err == nil {
		t.Fatal("expected unexpected Ollama port to be rejected")
	}
}

func TestHandleSetupTestConnectionRejectsWithoutCSRF(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodPost, "/api/setup/test", strings.NewReader(`{"provider_type":"openrouter","base_url":"https://openrouter.ai/api/v1","api_key":"sk-test","model":"test-model"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handleSetupTestConnection(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestHandleSetupSaveRejectsWithoutCSRF(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	addSetupCSRFTokenForTest(s, "test-csrf-token-12345")

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
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	addSetupCSRFTokenForTest(s, "correct-token")

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
	foundStepFun := false
	for _, raw := range profiles {
		profile := raw.(map[string]interface{})
		if profile["id"] == "stepfun_step_plan" {
			foundStepFun = true
			if profile["name"] != "StepFun Step Plan" {
				t.Fatalf("stepfun name = %v, want StepFun Step Plan", profile["name"])
			}
			if profile["base_url"] != "https://api.stepfun.ai/step_plan/v1" {
				t.Fatalf("stepfun base_url = %v, want step_plan endpoint", profile["base_url"])
			}
			if profile["main_model"] != "step-3.5-flash" {
				t.Fatalf("stepfun main_model = %v, want step-3.5-flash", profile["main_model"])
			}
			models, ok := profile["models"].(map[string]interface{})
			if !ok {
				t.Fatal("expected stepfun models map in response")
			}
			helper, ok := models["helper"].(map[string]interface{})
			if !ok {
				t.Fatal("expected stepfun helper config in response")
			}
			if helper["model"] != "step-3.5-flash-2603" {
				t.Fatalf("stepfun helper model = %v, want step-3.5-flash-2603", helper["model"])
			}
		}
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
		videoGen, ok := models["video_generation"].(map[string]interface{})
		if !ok {
			t.Fatal("expected minimax video_generation config in response")
		}
		if videoGen["provider_type"] != "minimax" {
			t.Fatalf("minimax video_generation provider_type = %v, want minimax", videoGen["provider_type"])
		}
		if videoGen["base_url"] != "https://api.minimax.io/v1" {
			t.Fatalf("minimax video_generation base_url = %v, want international video endpoint", videoGen["base_url"])
		}
		if videoGen["alt_base_url"] != "https://api.minimaxi.com/v1" {
			t.Fatalf("minimax video_generation alt_base_url = %v, want China video endpoint", videoGen["alt_base_url"])
		}
		if videoGen["model"] != "MiniMax-Hailuo-2.3" {
			t.Fatalf("minimax video_generation model = %v, want MiniMax-Hailuo-2.3", videoGen["model"])
		}
	}
	if !foundMiniMax {
		t.Fatal("expected minimax_coding profile in response")
	}
	if !foundStepFun {
		t.Fatal("expected stepfun_step_plan profile in response")
	}
}

func TestSetupProviderHostAllowsStepFunStepPlan(t *testing.T) {
	t.Parallel()

	if !isAllowedSetupProviderHost("api.stepfun.ai") {
		t.Fatal("expected api.stepfun.ai to be allowed for StepFun setup profile connection tests")
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
	addSetupCSRFTokenForTest(s, "minimax-setup-token")

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
			map[string]interface{}{
				"id":                      "video_gen",
				"type":                    "minimax",
				"name":                    "MiniMax Coding Plan Video Gen",
				"base_url":                "https://api.minimax.io/v1",
				"api_key":                 "sk-test",
				"model":                   "MiniMax-Hailuo-2.3",
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
		"video_generation": map[string]interface{}{
			"enabled":                  true,
			"provider":                 "video_gen",
			"default_duration_seconds": 6,
			"default_resolution":       "768P",
		},
		"tts": map[string]interface{}{
			"provider": "minimax",
			"minimax": map[string]interface{}{
				"api_key":  "sk-test",
				"model_id": "speech-2.8-hd",
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

func TestHandleSetupSaveAcceptsMiniMaxQuickPatchAgainstTemplateConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	input, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
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
	addSetupCSRFTokenForTest(s, "minimax-current-config-token")

	patch := map[string]interface{}{
		"auth": map[string]interface{}{
			"enabled":        true,
			"admin_password": "supersecret",
		},
		"providers": []interface{}{
			map[string]interface{}{"id": "main", "type": "minimax", "name": "MiniMax Coding Plan", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-M2.7", "native_function_calling": true},
			map[string]interface{}{"id": "whisper", "type": "minimax", "name": "MiniMax Coding Plan Whisper", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-M2.7", "native_function_calling": true},
			map[string]interface{}{"id": "helper", "type": "minimax", "name": "MiniMax Coding Plan Helper", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-M2.5", "native_function_calling": true},
			map[string]interface{}{"id": "image_gen", "type": "minimax", "name": "MiniMax Coding Plan Image Gen", "base_url": "https://api.minimax.io/v1/image_generation", "api_key": "sk-test", "model": "image-01", "native_function_calling": true},
			map[string]interface{}{"id": "music_gen", "type": "minimax", "name": "MiniMax Coding Plan Music Gen", "base_url": "https://api.minimax.io/v1/music_generation", "api_key": "sk-test", "model": "music-2.6", "native_function_calling": true},
			map[string]interface{}{"id": "video_gen", "type": "minimax", "name": "MiniMax Coding Plan Video Gen", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-Hailuo-2.3", "native_function_calling": true},
		},
		"agent":            map[string]interface{}{"system_language": "Deutsch"},
		"llm":              map[string]interface{}{"provider": "main", "use_native_functions": true, "helper_enabled": true, "helper_provider": "helper", "structured_outputs": true},
		"whisper":          map[string]interface{}{"provider": "whisper", "mode": "multimodal"},
		"image_generation": map[string]interface{}{"enabled": true, "provider": "image_gen"},
		"music_generation": map[string]interface{}{"enabled": true, "provider": "music_gen"},
		"video_generation": map[string]interface{}{"enabled": true, "provider": "video_gen", "default_duration_seconds": 6, "default_resolution": "768P"},
		"tts":              map[string]interface{}{"provider": "minimax", "minimax": map[string]interface{}{"api_key": "sk-test", "model_id": "speech-2.8-hd", "voice_id": "English_PlayfulGirl"}},
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
	addSetupCSRFTokenForTest(s, "minimax-panic-token")

	patch := map[string]interface{}{
		"auth": map[string]interface{}{
			"enabled":        true,
			"admin_password": "supersecret",
		},
		"providers": []interface{}{
			map[string]interface{}{"id": "main", "type": "minimax", "name": "MiniMax Coding Plan", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-M2.7", "native_function_calling": true},
			map[string]interface{}{"id": "whisper", "type": "minimax", "name": "MiniMax Coding Plan Whisper", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-M2.7", "native_function_calling": true},
			map[string]interface{}{"id": "helper", "type": "minimax", "name": "MiniMax Coding Plan Helper", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-M2.5", "native_function_calling": true},
			map[string]interface{}{"id": "image_gen", "type": "minimax", "name": "MiniMax Coding Plan Image Gen", "base_url": "https://api.minimax.io/v1/image_generation", "api_key": "sk-test", "model": "image-01", "native_function_calling": true},
			map[string]interface{}{"id": "music_gen", "type": "minimax", "name": "MiniMax Coding Plan Music Gen", "base_url": "https://api.minimax.io/v1/music_generation", "api_key": "sk-test", "model": "music-2.6", "native_function_calling": true},
			map[string]interface{}{"id": "video_gen", "type": "minimax", "name": "MiniMax Coding Plan Video Gen", "base_url": "https://api.minimax.io/v1", "api_key": "sk-test", "model": "MiniMax-Hailuo-2.3", "native_function_calling": true},
		},
		"agent":            map[string]interface{}{"system_language": "Deutsch"},
		"llm":              map[string]interface{}{"provider": "main", "use_native_functions": true, "helper_enabled": true, "helper_provider": "helper", "structured_outputs": true},
		"whisper":          map[string]interface{}{"provider": "whisper", "mode": "multimodal"},
		"image_generation": map[string]interface{}{"enabled": true, "provider": "image_gen"},
		"music_generation": map[string]interface{}{"enabled": true, "provider": "music_gen"},
		"video_generation": map[string]interface{}{"enabled": true, "provider": "video_gen", "default_duration_seconds": 6, "default_resolution": "768P"},
		"tts":              map[string]interface{}{"provider": "minimax", "minimax": map[string]interface{}{"api_key": "sk-test", "model_id": "speech-2.8-hd", "voice_id": "English_PlayfulGirl"}},
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
