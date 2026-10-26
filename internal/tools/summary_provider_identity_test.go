package tools

import (
	"aurago/internal/config"
	"testing"
)

func TestSummaryPreservesSelectedProviderIdentity(t *testing.T) {
	var cfg config.Config
	first := config.ProviderEntry{ID: "rest-first", Type: "cloudflare-rest", AccountID: "account", GatewayID: "gateway-first", APIKey: "fixture-first", Model: "google/gemini-2.5-flash"}
	selected := first
	selected.ID, selected.GatewayID, selected.APIKey = "rest-selected", "gateway-selected", "fixture-selected"
	cfg.Providers = []config.ProviderEntry{first, selected}
	cfg.Tools.WebScraper.SummaryProvider = selected.ID
	cfg.ResolveProviders()
	got := ResolveSummaryLLMConfig(&cfg, SummaryLLMConfig{ProviderID: cfg.Tools.WebScraper.SummaryProvider, APIKey: cfg.Tools.WebScraper.SummaryAPIKey, BaseURL: cfg.Tools.WebScraper.SummaryBaseURL, Model: cfg.Tools.WebScraper.SummaryModel})
	if got.ProviderID != selected.ID || got.GatewayID != selected.GatewayID {
		t.Fatalf("summary chose provider %q and gateway %q instead of saved provider %q and gateway %q", got.ProviderID, got.GatewayID, selected.ID, selected.GatewayID)
	}
}

func TestSummaryPreservesResolvedCredentialOverride(t *testing.T) {
	cfg := &config.Config{Providers: []config.ProviderEntry{{ID: "main", Type: "openai", APIKey: "vault-fixture", BaseURL: "https://api.openai.com/v1", Model: "gpt-4.1"}}}
	got := ResolveSummaryLLMConfig(cfg, SummaryLLMConfig{ProviderID: "main", APIKey: "environment-fixture", BaseURL: "https://api.openai.com/v1", Model: "gpt-4.1-mini"})
	if got.APIKey != "environment-fixture" || got.Model != "gpt-4.1-mini" || got.ProviderType != "openai" {
		t.Fatal("explicit resolved credential or model override lost")
	}
}
