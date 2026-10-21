package config

import "testing"

func TestProviderLegacyEmbeddingModesRemainBuiltin(t *testing.T) {
	for _, mode := range []string{"internal", "external", "disabled", "local-granite", "local-ollama-embeddings"} {
		t.Run(mode, func(t *testing.T) {
			cfg := &Config{}
			cfg.Providers = []ProviderEntry{{ID: "main", Type: "ollama", BaseURL: "http://localhost:11434/v1", Model: "fixture"}}
			cfg.LLM.Provider = "main"
			cfg.Embeddings.Provider = mode
			cfg.Embeddings.ExternalURL = "http://localhost:11435/v1"
			cfg.Embeddings.LegacyAPIKey = "syntheticEmbeddingKey123"
			for range 2 {
				cfg.ResolveProviders()
				if cfg.Embeddings.ProviderType == "invalid-provider" {
					t.Fatal("built-in embedding mode was blocked as a missing provider")
				}
				for _, ref := range cfg.ProviderReferences() {
					if ref.Path == "embeddings.provider" {
						t.Fatal("built-in embedding mode was treated as a provider reference")
					}
				}
			}
		})
	}
}
