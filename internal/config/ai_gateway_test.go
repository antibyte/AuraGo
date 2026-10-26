package config

import "testing"

func TestAIGatewayValidationPreservesMetadataInsteadOfTruncating(t *testing.T) {
	for _, metadata := range []map[string]string{
		{"a": "1", "b": "2", "c": "3", "d": "4", "e": "5", "f": "6"},
		{"a": "1", " a ": "2"},
		{"": "value"},
		{"a": ""},
	} {
		cfg := &Config{}
		cfg.AIGateway.Metadata = metadata
		if ValidateAIGatewayConfig(cfg) == nil {
			t.Fatal("invalid metadata accepted")
		}
		NormalizeAIGatewayConfig(cfg)
		if len(metadata) > 5 && len(cfg.AIGateway.Metadata) != len(metadata) {
			t.Fatal("metadata silently discarded")
		}
	}
	cfg := &Config{}
	cfg.AIGateway.Metadata = map[string]string{"Sprache": "Grüße\r\n世界"}
	if err := ValidateAIGatewayConfig(cfg); err != nil {
		t.Fatal("JSON-safe Unicode metadata rejected", err)
	}
}

func TestAIGatewayPathSegmentsAndRequestTimeoutValidation(t *testing.T) {
	for _, account := range []string{"", "a/b", "a?b"} {
		if ValidateGatewayProvider(ProviderEntry{ID: "workers", Type: "workers-ai", AccountID: account}) == nil {
			t.Fatal("Workers AI accepted an invalid or implicit account")
		}
	}
	for _, value := range []string{"a/b", "a?b", "a#b", "..", "a b", "a\r\nb"} {
		cfg := &Config{}
		cfg.AIGateway.Enabled = true
		cfg.AIGateway.AccountID = "account"
		cfg.AIGateway.GatewayID = value
		if ValidateAIGatewayConfig(cfg) == nil {
			t.Fatalf("unsafe gateway segment accepted: %q", value)
		}
	}
	for _, timeout := range []int{-1, 300001} {
		cfg := &Config{}
		cfg.AIGateway.RequestTimeoutMS = timeout
		if ValidateAIGatewayConfig(cfg) == nil {
			t.Fatal("invalid timeout accepted")
		}
	}
}
