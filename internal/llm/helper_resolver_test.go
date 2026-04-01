package llm

import (
	"testing"

	"aurago/internal/config"
)

func TestResolveHelperLLMReturnsResolvedFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = "helper"
	cfg.LLM.HelperProviderType = "openrouter"
	cfg.LLM.HelperBaseURL = "https://openrouter.ai/api/v1"
	cfg.LLM.HelperAPIKey = "secret"
	cfg.LLM.HelperResolvedModel = "google/gemini-2.0-flash-001"

	got := ResolveHelperLLM(cfg)

	if !got.Enabled {
		t.Fatal("expected helper LLM to be enabled")
	}
	if got.ProviderID != "helper" {
		t.Fatalf("ProviderID = %q, want helper", got.ProviderID)
	}
	if got.ProviderType != "openrouter" {
		t.Fatalf("ProviderType = %q, want openrouter", got.ProviderType)
	}
	if got.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("BaseURL = %q", got.BaseURL)
	}
	if got.APIKey != "secret" {
		t.Fatalf("APIKey = %q", got.APIKey)
	}
	if got.Model != "google/gemini-2.0-flash-001" {
		t.Fatalf("Model = %q", got.Model)
	}
}

func TestIsHelperLLMAvailableRequiresExplicitResolution(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = "helper"
	cfg.LLM.HelperResolvedModel = "cheap-model"

	if IsHelperLLMAvailable(cfg) {
		t.Fatal("expected helper LLM to be unavailable without provider type")
	}

	cfg.LLM.HelperProviderType = "openai"
	if !IsHelperLLMAvailable(cfg) {
		t.Fatal("expected helper LLM to become available once explicitly resolved")
	}
}
