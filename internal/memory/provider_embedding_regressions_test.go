package memory

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
)

type embeddingOAuthSecretReader struct{}

func (embeddingOAuthSecretReader) ReadSecret(string) (string, error) {
	return `{"access_token":"mainSyntheticOAuthToken123"}`, nil
}

func TestProviderLegacyEmbeddingRuntimeKeepsEndpointAndCredentials(t *testing.T) {
	for _, mode := range []string{"internal", "external"} {
		t.Run(mode, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Providers = []config.ProviderEntry{{ID: "main", Type: "openai", BaseURL: "https://main.example.invalid/v1", Model: "fixture", AuthType: "oauth2", RuntimeOAuthToken: "mainSyntheticOAuthToken123"}}
			cfg.LLM.Provider = "main"
			cfg.Embeddings.Provider = mode
			cfg.Embeddings.InternalModel = "internal-embedding"
			cfg.Embeddings.ExternalURL = "http://localhost:11435/v1"
			cfg.Embeddings.ExternalModel = "external-embedding"
			cfg.Embeddings.LegacyAPIKey = "syntheticEmbeddingKey123"
			cfg.ResolveProviders()
			cfg.ApplyOAuthTokens(embeddingOAuthSecretReader{})
			runtime := buildEmbeddingRuntimeFromConfig(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
			wantURL, wantKey, wantModel := cfg.Embeddings.ExternalURL, cfg.Embeddings.LegacyAPIKey, cfg.Embeddings.ExternalModel
			if mode == "internal" {
				wantURL, wantKey, wantModel = cfg.Providers[0].BaseURL, cfg.LLM.APIKey, cfg.Embeddings.InternalModel
			}
			if runtime.BaseURL != wantURL || runtime.APIKey != wantKey || runtime.Model != wantModel {
				t.Fatal("legacy embedding endpoint, credentials or model changed during provider resolution")
			}
		})
	}
}
