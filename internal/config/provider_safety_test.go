package config

import (
	"errors"
	"strings"
	"testing"
)

type providerSafetyReader map[string]string

func (r providerSafetyReader) ReadSecret(key string) (string, error) {
	value, ok := r[key]
	if !ok {
		return "", errors.New("missing")
	}
	return value, nil
}

func TestProviderCredentialOwnershipAcrossExplicitAndInheritedSlots(t *testing.T) {
	for _, sameOrigin := range []bool{false, true} {
		cfg := &Config{}
		mainURL, otherURL := "https://main.example.invalid/v1", "http://localhost:11434/v1"
		if sameOrigin {
			otherURL = mainURL
		}
		cfg.Providers = []ProviderEntry{{ID: "main", Type: "openai", BaseURL: mainURL, APIKey: "mainSyntheticKey123456", Model: "main-model"}, {ID: "local", Type: "ollama", BaseURL: otherURL, Model: "local-model"}}
		cfg.LLM.Provider = "main"
		cfg.Vision.Provider = "local"
		cfg.Whisper.Provider = "local"
		cfg.Embeddings.Provider = "local"
		cfg.CoAgents.LLM.Provider = "local"
		cfg.CoAgents.Specialists.Coder.LLM.Provider = "local"
		cfg.A2A.LLM.Provider = "local"
		cfg.LLMGuardian.Provider = "local"
		cfg.MissionPreparation.Provider = "local"
		cfg.ResolveProviders()
		for _, binding := range cfg.providerCredentialBindings() {
			if binding.id == "local" && *binding.key != "" {
				t.Fatalf("other provider inherited a credential, same origin=%v", sameOrigin)
			}
		}
		cfg.CoAgents.LLM.Provider = ""
		cfg.CoAgents.Specialists.Coder.LLM.Provider = ""
		cfg.Vision.Provider = ""
		cfg.Whisper.Provider = ""
		cfg.Embeddings.Provider = ""
		cfg.A2A.LLM.Provider = ""
		cfg.LLMGuardian.Provider = ""
		cfg.MissionPreparation.Provider = ""
		cfg.Providers[0].AuthType = "oauth2"
		cfg.Providers[0].APIKey = ""
		cfg.ResolveProviders()
		cfg.ApplyOAuthTokens(providerSafetyReader{"oauth_main": `{"access_token":"mainOAuthToken123456"}`})
		for _, binding := range cfg.providerCredentialBindings() {
			if binding.id == "main" && *binding.key != "mainOAuthToken123456" {
				t.Fatalf("inherited route lost OAuth credential: %v", binding.id)
			}
		}
	}
}

func TestProviderMissingSelectionsNeverReuseResolvedSecrets(t *testing.T) {
	cfg := &Config{}
	cfg.Providers = []ProviderEntry{{ID: "main", Type: "openai", BaseURL: "https://main.example.invalid/v1", APIKey: "oldSyntheticKey123456", Model: "model"}}
	cfg.LLM.Provider = "main"
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = "main"
	cfg.LLM.HelperModel = "model"
	cfg.ResolveProviders()
	cfg.Providers = nil
	// Prevent the legacy inline migration from creating a replacement provider.
	cfg.Providers = []ProviderEntry{{ID: "unrelated", Type: "ollama"}}
	cfg.ResolveProviders()
	for _, binding := range cfg.providerCredentialBindings() {
		if binding.id == "main" && (*binding.key != "" || !strings.HasPrefix(*binding.url, "invalid:")) {
			t.Fatal("dead reference retained a live route")
		}
	}
	if len(cfg.MissingProviderReferences()) < 2 {
		t.Fatal("dead selections were not diagnosed")
	}
}

func TestProviderReferencesIncludeDormantAndAuxiliarySelections(t *testing.T) {
	cfg := &Config{}
	cfg.LLM.Provider = "selected"
	cfg.LLM.HelperProvider = "selected"
	cfg.FallbackLLM.Provider = "selected"
	cfg.CoAgents.LLM.Provider = "selected"
	cfg.CoAgents.Specialists.Security.LLM.Provider = "selected"
	cfg.SIP.Voice.AgentProviderID = "selected"
	cfg.SIP.Voice.Classic.ASRProviderID = "selected"
	cfg.SpeechLab.ChatLLMProviderID = "selected"
	cfg.MemoryAnalysis.Provider = "selected"
	cfg.CYDCreator.Model = "selected"
	cfg.VirtualComputers.AgentProvider = "selected"
	cfg.YepAPI.Provider = "selected"
	cfg.Decisions.Provider = "selected"
	paths := map[string]bool{}
	for _, ref := range cfg.ProviderReferences() {
		paths[ref.Path] = true
	}
	for _, path := range []string{"llm.provider", "llm.helper_provider", "fallback_llm.provider", "co_agents.llm.provider", "co_agents.specialists.security.llm.provider", "sip.voice.agent_provider_id", "sip.voice.classic.asr_provider_id", "speech_lab.chat_llm_provider_id", "memory_analysis.provider", "cyd_creator.model", "virtual_computers.agent_provider", "yepapi.provider", "decisions.provider"} {
		if !paths[path] {
			t.Fatalf("missing dormant reference %s", path)
		}
	}
	cfg.LLM.Provider = LocalLLMProviderID
	cfg.FallbackLLM.Provider = LocalLLMProviderID
	cfg.Embeddings.Provider = "local-granite"
	for _, ref := range cfg.ProviderReferences() {
		if ref.ProviderID == LocalLLMProviderID || ref.ProviderID == "local-granite" {
			t.Fatal("built-in selection treated as deletable provider")
		}
	}
}
