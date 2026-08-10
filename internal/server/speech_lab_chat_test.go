package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
)

type speechLabTestSecrets map[string]string

func (s speechLabTestSecrets) ReadSecret(key string) (string, error) {
	value, ok := s[key]
	if !ok {
		return "", errors.New("secret not found")
	}
	return value, nil
}

func TestSpeechLabChatHandlerReturnsStructuredUnavailableError(t *testing.T) {
	cfg := &config.Config{SpeechLab: config.SpeechLabConfig{
		Enabled: true, BaseURL: "http://127.0.0.1:8765", ChatInputEnabled: true, ChatLLMProviderID: "missing",
	}}
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	body := bytes.NewBufferString(`{"model":"aurago","messages":[{"role":"user","content":"Hallo"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set(speechLabChatTurnTokenHeader, s.speechLabTokens().Issue("default", "Hallo"))
	rec := httptest.NewRecorder()
	handleChatCompletions(s, nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["error"] != "speech_lab_llm_unavailable" {
		t.Fatalf("structured error = %#v", payload)
	}
}

func TestSpeechLabChatTurnRuntimeUsesSelectedProviderWithoutFallback(t *testing.T) {
	originalFactory := speechLabChatClientFactory
	t.Cleanup(func() { speechLabChatClientFactory = originalFactory })
	var factoryProvider string
	speechLabChatClientFactory = func(_ *config.Config, providerType, _, _, _ string) llm.ChatClient {
		factoryProvider = providerType
		return llm.NewClient(&config.Config{})
	}
	cfg := &config.Config{
		SpeechLab: config.SpeechLabConfig{
			Enabled: true, BaseURL: "http://127.0.0.1:8765", ChatInputEnabled: true, ChatLLMProviderID: "fast",
		},
		Providers: []config.ProviderEntry{{ID: "fast", Type: "ollama", BaseURL: "http://127.0.0.1:11434/v1", Model: "fast-model"}},
	}
	cfg.FallbackLLM.Enabled = true
	cfg.LLM.HelperEnabled = true
	turnCfg, client, err := speechLabChatTurnRuntime(true, false, "", cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if client == nil || factoryProvider != "ollama" || turnCfg.LLM.Provider != "fast" || turnCfg.LLM.Model != "fast-model" {
		t.Fatalf("selected provider was not snapshotted: cfg=%+v provider=%q", turnCfg.LLM, factoryProvider)
	}
	if turnCfg.FallbackLLM.Enabled {
		t.Fatal("fallback remained enabled for the selected Speech Lab provider")
	}
	if turnCfg.LLM.HelperEnabled {
		t.Fatal("helper routing remained enabled for the selected Speech Lab provider")
	}
	compressionClient, compressionModel := llm.ResolveHelperBackedClient(turnCfg, client, turnCfg.LLM.Model)
	if compressionClient != client || compressionModel != "fast-model" {
		t.Fatalf("context compression escaped the selected provider: client=%T model=%q", compressionClient, compressionModel)
	}
	if !cfg.FallbackLLM.Enabled || !cfg.LLM.HelperEnabled || cfg.LLM.Provider != "" {
		t.Fatal("base configuration was mutated")
	}
}

func TestSpeechLabChatTurnRuntimeResolvesStaticAndOAuthCredentials(t *testing.T) {
	originalFactory := speechLabChatClientFactory
	t.Cleanup(func() { speechLabChatClientFactory = originalFactory })
	var capturedKey string
	speechLabChatClientFactory = func(_ *config.Config, _, _, apiKey, _ string) llm.ChatClient {
		capturedKey = apiKey
		return llm.NewClient(&config.Config{})
	}
	cfg := &config.Config{
		SpeechLab: config.SpeechLabConfig{Enabled: true, BaseURL: "http://127.0.0.1:8765", ChatInputEnabled: true, ChatLLMProviderID: "fast"},
		Providers: []config.ProviderEntry{{ID: "fast", Type: "openai", Model: "fast-model"}},
	}
	secrets := speechLabTestSecrets{"provider_fast_api_key": "static-vault-key"}
	if _, _, err := speechLabChatTurnRuntime(true, false, "", cfg, nil, secrets); err != nil || capturedKey != "static-vault-key" {
		t.Fatalf("static Vault key was not resolved: key=%q err=%v", capturedKey, err)
	}

	cfg.Providers[0].AuthType = "oauth2"
	secrets = speechLabTestSecrets{"oauth_fast": `{"access_token":"oauth-access","expiry":"` + time.Now().Add(time.Hour).UTC().Format(time.RFC3339) + `"}`}
	if _, _, err := speechLabChatTurnRuntime(true, false, "", cfg, nil, secrets); err != nil || capturedKey != "oauth-access" {
		t.Fatalf("OAuth token was not resolved: key=%q err=%v", capturedKey, err)
	}
	secrets["oauth_fast"] = `{"access_token":"expired","expiry":"` + time.Now().Add(-time.Hour).UTC().Format(time.RFC3339) + `"}`
	if _, _, err := speechLabChatTurnRuntime(true, false, "", cfg, nil, secrets); !errors.Is(err, errSpeechLabChatLLMUnavailable) {
		t.Fatalf("expired OAuth token did not fail closed: %v", err)
	}
}

func TestSpeechLabRuntimeChatProviderUsesCopilotAuthManagerState(t *testing.T) {
	provider := &config.ProviderEntry{ID: "copilot-fast", Type: "github-copilot", BaseURL: "https://api.githubcopilot.com", Model: "copilot/gpt-5-mini"}
	status, _ := speechLabRuntimeChatProviderWithAuth(provider, nil, time.Now, func() bool { return true })
	if !status.Eligible || !status.Configured || status.Reason != "available" {
		t.Fatalf("configured Copilot manager was rejected: %+v", status)
	}
	status, _ = speechLabRuntimeChatProviderWithAuth(provider, nil, time.Now, func() bool { return false })
	if !status.Eligible || status.Configured || status.Reason != "copilot_not_authenticated" {
		t.Fatalf("unauthenticated Copilot state = %+v", status)
	}
}

func TestSpeechLabChatTurnRuntimeIgnoresUnmarkedAndInternalTurns(t *testing.T) {
	defaultClient := llm.NewClient(&config.Config{})
	cfg := &config.Config{SpeechLab: config.SpeechLabConfig{
		Enabled: true, BaseURL: "http://127.0.0.1:8765", ChatInputEnabled: true, ChatLLMProviderID: "missing",
	}}
	for _, tc := range []struct {
		marked    bool
		followUp  bool
		missionID string
	}{{false, false, ""}, {true, true, ""}, {true, false, "mission"}} {
		gotCfg, gotClient, err := speechLabChatTurnRuntime(tc.marked, tc.followUp, tc.missionID, cfg, defaultClient, nil)
		if err != nil || gotCfg != cfg || gotClient != defaultClient {
			t.Fatalf("ordinary/internal turn was rerouted: %+v err=%v", tc, err)
		}
	}
}

func TestSpeechLabChatTurnRuntimeFailsClosed(t *testing.T) {
	cfg := &config.Config{SpeechLab: config.SpeechLabConfig{
		Enabled: true, BaseURL: "http://127.0.0.1:8765", ChatInputEnabled: true, ChatLLMProviderID: "cloud",
	}, Providers: []config.ProviderEntry{{ID: "cloud", Type: "openai", Model: "fast-model"}}}
	if _, _, err := speechLabChatTurnRuntime(true, false, "", cfg, llm.NewClient(&config.Config{}), nil); !errors.Is(err, errSpeechLabChatLLMUnavailable) {
		t.Fatalf("credential-less provider did not fail closed: %v", err)
	}
	cfg.SpeechLab.ChatLLMProviderID = "missing"
	if _, _, err := speechLabChatTurnRuntime(true, false, "", cfg, llm.NewClient(&config.Config{}), nil); !errors.Is(err, errSpeechLabChatLLMUnavailable) {
		t.Fatalf("missing provider did not fail closed: %v", err)
	}
}

func TestSpeechLabTurnTokenIsBoundToSessionTranscriptAndSingleUse(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	registry := newSpeechLabTurnTokenRegistry(func() time.Time { return now })
	token := registry.Issue("session-a", "Hallo Welt")
	if token == "" {
		t.Fatal("Issue() returned an empty token")
	}
	if registry.Consume(token, "session-b", "Hallo Welt") {
		t.Fatal("token accepted for another session")
	}
	if registry.Consume(token, "session-a", "Hallo Welt") {
		t.Fatal("mismatched attempt did not consume the one-time token")
	}
	token = registry.Issue("session-a", "Hallo Welt")
	if !registry.Consume(token, "session-a", "Hallo Welt") {
		t.Fatal("matching token was rejected")
	}
	if registry.Consume(token, "session-a", "Hallo Welt") {
		t.Fatal("token was reusable")
	}
	token = registry.Issue("session-a", "Hallo Welt")
	now = now.Add(5 * time.Minute)
	if registry.Consume(token, "session-a", "Hallo Welt") {
		t.Fatal("expired token was accepted")
	}
}

func TestSpeechLabTurnTokenReservationReleaseAndCommit(t *testing.T) {
	registry := newSpeechLabTurnTokenRegistry(nil)
	token := registry.Issue("default", "hello")
	reservation, ok := registry.Reserve(token, "default", "hello")
	if !ok || reservation == nil {
		t.Fatal("valid token could not be reserved")
	}
	if _, second := registry.Reserve(token, "default", "hello"); second {
		t.Fatal("reserved token was concurrently reservable")
	}
	reservation.release()
	reservation, ok = registry.Reserve(token, "default", "hello")
	if !ok || !reservation.commit() {
		t.Fatal("released token could not be reserved and committed")
	}
	if registry.Consume(token, "default", "hello") {
		t.Fatal("committed token remained reusable")
	}
}

func TestSpeechLabTurnTokenRegistryIsGloballyBounded(t *testing.T) {
	registry := newSpeechLabTurnTokenRegistry(nil)
	for index := 0; index < speechLabTurnTokenMax+20; index++ {
		if token := registry.Issue("default", fmt.Sprintf("transcript-%d", index)); token == "" {
			t.Fatalf("Issue(%d) returned an empty token", index)
		}
	}
	registry.mu.Lock()
	count := len(registry.tokens)
	registry.mu.Unlock()
	if count != speechLabTurnTokenMax {
		t.Fatalf("token count = %d, want %d", count, speechLabTurnTokenMax)
	}
}
