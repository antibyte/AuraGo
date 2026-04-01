package agent

import (
	"testing"

	"aurago/internal/config"
)

func TestResolveConsolidationModelPrefersOverride(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "main-model"
	cfg.Consolidation.Model = "cheap-model"

	if got := resolveConsolidationModel(cfg); got != "cheap-model" {
		t.Fatalf("resolveConsolidationModel() = %q, want %q", got, "cheap-model")
	}
}

func TestResolveConsolidationModelPrefersHelperLLM(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "main-model"
	cfg.Consolidation.Model = "cheap-model"
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProviderType = "custom"
	cfg.LLM.HelperResolvedModel = "helper-model"

	if got := resolveConsolidationModel(cfg); got != "helper-model" {
		t.Fatalf("resolveConsolidationModel() = %q, want %q", got, "helper-model")
	}
}

func TestResolveConsolidationModelFallsBackToMainModel(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "main-model"

	if got := resolveConsolidationModel(cfg); got != "main-model" {
		t.Fatalf("resolveConsolidationModel() = %q, want %q", got, "main-model")
	}
}

func TestResolveHelperBackedLLMFallsBackToProvidedClient(t *testing.T) {
	cfg := &config.Config{}
	fallback := &fakeActivityDigestClient{}

	gotClient, gotModel := resolveHelperBackedLLM(cfg, fallback, "fallback-model")
	if gotClient != fallback {
		t.Fatalf("resolveHelperBackedLLM() client = %#v, want fallback", gotClient)
	}
	if gotModel != "fallback-model" {
		t.Fatalf("resolveHelperBackedLLM() model = %q, want %q", gotModel, "fallback-model")
	}
}

func TestResolveHelperBackedLLMPrefersHelperLLM(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProviderType = "custom"
	cfg.LLM.HelperBaseURL = "https://example.com/v1"
	cfg.LLM.HelperAPIKey = "helper-key"
	cfg.LLM.HelperResolvedModel = "helper-model"

	fallback := &fakeActivityDigestClient{}
	gotClient, gotModel := resolveHelperBackedLLM(cfg, fallback, "fallback-model")
	if gotClient == nil {
		t.Fatal("resolveHelperBackedLLM() client = nil, want helper client")
	}
	if gotClient == fallback {
		t.Fatal("resolveHelperBackedLLM() returned fallback client, want helper client")
	}
	if gotModel != "helper-model" {
		t.Fatalf("resolveHelperBackedLLM() model = %q, want %q", gotModel, "helper-model")
	}
}
