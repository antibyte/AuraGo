package memory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/config"

	chromem "github.com/philippgille/chromem-go"
)

type embeddingRuntimeConfig struct {
	EmbeddingFunc chromem.EmbeddingFunc
	Provider      string
	BaseURL       string
	APIKey        string
	Model         string
	Fingerprint   string
	Disabled      bool
	Local         bool
}

func buildEmbeddingFuncFromConfig(cfg *config.Config, logger *slog.Logger) (chromem.EmbeddingFunc, string, bool) {
	runtime := buildEmbeddingRuntimeFromConfig(cfg, logger)
	return runtime.EmbeddingFunc, runtime.Provider, runtime.Disabled
}

func buildEmbeddingRuntimeFromConfig(cfg *config.Config, logger *slog.Logger) embeddingRuntimeConfig {
	if cfg == nil {
		return disabledEmbeddingRuntime("disabled")
	}

	provider := strings.TrimSpace(cfg.Embeddings.Provider)
	if provider == "disabled" || provider == "" {
		if logger != nil {
			logger.Info("VectorDB: embeddings disabled by configuration")
		}
		return disabledEmbeddingRuntime("disabled")
	}

	embedURL := cfg.Embeddings.BaseURL
	embedKey := cfg.Embeddings.APIKey
	embedModel := cfg.Embeddings.Model

	if provider == "internal" {
		// Always use main LLM credentials; embeddings.api_key is irrelevant here.
		embedURL = cfg.LLM.BaseURL
		embedKey = cfg.LLM.APIKey
		if embedModel == "" {
			embedModel = cfg.Embeddings.InternalModel
		}
	}
	if provider == "external" {
		if embedURL == "" {
			embedURL = cfg.Embeddings.ExternalURL
		}
		if embedModel == "" {
			embedModel = cfg.Embeddings.ExternalModel
		}
	}

	if embedModel == "" {
		embedModel = "text-embedding-3-small"
	}

	if logger != nil {
		logger.Info("Embedding runtime configured", "provider", provider, "url", embedURL, "model", embedModel)
		if embedKey == "" {
			vaultKey := "provider_" + provider + "_api_key"
			logger.Warn("[Embedding] API key is empty - check vault entry",
				"provider", provider, "vault_key", vaultKey,
				"hint", "Re-enter the API key via Config UI -> Providers -> "+provider)
		}
	}

	return embeddingRuntimeConfig{
		EmbeddingFunc: withEmbeddingRetry(chromem.NewEmbeddingFuncOpenAICompat(embedURL, embedKey, embedModel, nil), logger),
		Provider:      provider,
		BaseURL:       embedURL,
		APIKey:        embedKey,
		Model:         embedModel,
		Fingerprint:   buildEmbeddingFingerprint(provider, embedURL, embedModel),
		Disabled:      false,
		Local:         isLocalEmbeddingProvider(cfg),
	}
}

func disabledEmbeddingRuntime(provider string) embeddingRuntimeConfig {
	return embeddingRuntimeConfig{
		EmbeddingFunc: func(_ context.Context, _ string) ([]float32, error) {
			return nil, fmt.Errorf("embeddings are disabled")
		},
		Provider:    provider,
		Fingerprint: buildEmbeddingFingerprint(provider, "", ""),
		Disabled:    true,
	}
}
