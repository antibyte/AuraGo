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
