package tools

import (
	"aurago/internal/config"
	"testing"
)

func TestSummaryRetainsGatewayConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.AIGateway.Enabled = true
	cfg.AIGateway.GatewayID = "fixture-gateway"
	for _, helper := range []bool{false, true} {
		cfg.LLM.HelperEnabled = helper
		cfg.LLM.HelperResolvedModel = "fixture-model"
		cfg.LLM.HelperProviderType = "workers-ai"
		cfg.LLM.HelperAccountID = "fixture-account"
		resolved := ResolveSummaryLLMConfig(cfg, SummaryLLMConfig{ProviderType: "workers-ai", AccountID: "fixture-account"})
		if resolved.Config != cfg || resolved.AccountID != "fixture-account" {
			t.Fatal("summary lost routing configuration or provider account")
		}
	}
}

func TestSummaryResolvesPrimaryCloudflareAccountAndGateway(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers = []config.ProviderEntry{{ID: "main", Type: "cloudflare-rest", AccountID: "account", GatewayID: "gateway", APIKey: "fixture-token", Model: "google/gemini-2.5-flash"}}
	cfg.LLM.Provider = "main"
	cfg.ResolveProviders()
	resolved := ResolveSummaryLLMConfig(cfg, SummaryLLMConfig{APIKey: cfg.LLM.APIKey, BaseURL: cfg.LLM.BaseURL, Model: cfg.LLM.Model})
	if resolved.ProviderID != "main" || resolved.ProviderType != "cloudflare-rest" || resolved.AccountID != "account" || resolved.GatewayID != "gateway" {
		t.Fatal("primary fallback lost Cloudflare identity")
	}
}
