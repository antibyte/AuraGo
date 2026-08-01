package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
)

func TestSpeechLabChatHandlerReturnsStructuredUnavailableError(t *testing.T) {
	cfg := &config.Config{SpeechLab: config.SpeechLabConfig{
		Enabled: true, BaseURL: "http://127.0.0.1:8765", ChatInputEnabled: true, ChatLLMProviderID: "missing",
	}}
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	body := bytes.NewBufferString(`{"model":"aurago","messages":[{"role":"user","content":"Hallo"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set(speechLabChatInputHeader, "1")
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
	turnCfg, client, err := speechLabChatTurnRuntime(true, false, "", cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if client == nil || factoryProvider != "ollama" || turnCfg.LLM.Provider != "fast" || turnCfg.LLM.Model != "fast-model" {
		t.Fatalf("selected provider was not snapshotted: cfg=%+v provider=%q", turnCfg.LLM, factoryProvider)
	}
	if turnCfg.FallbackLLM.Enabled {
		t.Fatal("fallback remained enabled for the selected Speech Lab provider")
	}
	if !cfg.FallbackLLM.Enabled || cfg.LLM.Provider != "" {
		t.Fatal("base configuration was mutated")
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
		gotCfg, gotClient, err := speechLabChatTurnRuntime(tc.marked, tc.followUp, tc.missionID, cfg, defaultClient)
		if err != nil || gotCfg != cfg || gotClient != defaultClient {
			t.Fatalf("ordinary/internal turn was rerouted: %+v err=%v", tc, err)
		}
	}
}

func TestSpeechLabChatTurnRuntimeFailsClosed(t *testing.T) {
	cfg := &config.Config{SpeechLab: config.SpeechLabConfig{
		Enabled: true, BaseURL: "http://127.0.0.1:8765", ChatInputEnabled: true, ChatLLMProviderID: "cloud",
	}, Providers: []config.ProviderEntry{{ID: "cloud", Type: "openai", Model: "fast-model"}}}
	if _, _, err := speechLabChatTurnRuntime(true, false, "", cfg, llm.NewClient(&config.Config{})); !errors.Is(err, errSpeechLabChatLLMUnavailable) {
		t.Fatalf("credential-less provider did not fail closed: %v", err)
	}
	cfg.SpeechLab.ChatLLMProviderID = "missing"
	if _, _, err := speechLabChatTurnRuntime(true, false, "", cfg, llm.NewClient(&config.Config{})); !errors.Is(err, errSpeechLabChatLLMUnavailable) {
		t.Fatalf("missing provider did not fail closed: %v", err)
	}
}
