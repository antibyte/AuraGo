package memory

import (
	"context"
	"fmt"
	"log/slog"

	"aurago/internal/config"

	chromem "github.com/philippgille/chromem-go"
)

func buildEmbeddingFuncFromConfig(cfg *config.Config, logger *slog.Logger) (chromem.EmbeddingFunc, string, bool) {
	provider := cfg.Embeddings.Provider

	if provider == "disabled" || provider == "" {
		return func(_ context.Context, _ string) ([]float32, error) {
			return nil, fmt.Errorf("embeddings are disabled")
		}, "disabled", true
	}

	embedURL := cfg.Embeddings.BaseURL
	embedKey := cfg.Embeddings.APIKey
	embedModel := cfg.Embeddings.Model

	if provider == "internal" {
		// Always use main LLM credentials — embeddings.api_key is irrelevant here.
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
	}

	return chromem.NewEmbeddingFuncOpenAICompat(embedURL, embedKey, embedModel, nil), provider, false
}
