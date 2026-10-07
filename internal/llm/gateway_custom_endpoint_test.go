package llm

import (
	"aurago/internal/config"
	"testing"
)

func TestGatewayPreservesExplicitCustomProviderEndpoints(t *testing.T) {
	cfg := &config.Config{}
	cfg.AIGateway.Enabled = true
	cfg.AIGateway.AccountID = "fixture-account"
	cfg.AIGateway.GatewayID = "fixture-gateway"
	cfg.AIGateway.Token = "fixture-only"
	for _, provider := range []string{"openai", "anthropic", "workers-ai", "openrouter", "deepseek", "groq", "mistral", "xai"} {
		for _, endpoint := range []string{"https://proxy.example/v1", "https://api.openai.com/v1/private", "https://api.openai.com:8443/v1"} {
			got := buildOpenAIClientConfig(cfg, resolvedProvider{ProviderType: provider, BaseURL: endpoint, AccountID: "fixture-account"})
			if got.BaseURL != normalizeProviderBaseURL(provider, endpoint) {
				t.Errorf("%s custom endpoint rewritten: %s", provider, got.BaseURL)
			}
		}
	}
}
