package memory

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"aurago/internal/config"
	"aurago/internal/embeddings"

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
	Embedder      embeddings.Embedder
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
	if provider == embeddings.LocalGraniteProvider {
		cacheDir := filepath.Join(cfg.Directories.DataDir, "embeddings")
		local, err := embeddings.NewLocalGranite(embeddings.LocalOptions{
			CacheDir:        cacheDir,
			ResetMarkerPath: filepath.Join(cfg.Directories.DataDir, "embeddings_reset_pending.json"),
			Backend:         cfg.Embeddings.Local.Backend,
			ContextSize:     cfg.Embeddings.Local.ContextSize,
			BatchSize:       cfg.Embeddings.Local.BatchSize,
			Logger:          logger,
		})
		if err != nil {
			if logger != nil {
				logger.Error("Local Granite runtime configuration failed", "error", err)
			}
			return disabledEmbeddingRuntime(provider)
		}
		embeddingFunc := func(ctx context.Context, text string) ([]float32, error) {
			vectors, err := local.Embed(ctx, []string{text})
			if err != nil {
				return nil, err
			}
			if len(vectors) != 1 {
				return nil, fmt.Errorf("local Granite returned %d vectors for one text", len(vectors))
			}
			return vectors[0], nil
		}
		return embeddingRuntimeConfig{
			EmbeddingFunc: embeddingFunc,
			Provider:      provider,
			Model:         embeddings.GraniteModelID,
			Fingerprint:   local.Fingerprint(),
			Disabled:      false,
			Local:         true,
			Embedder:      local,
		}
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

	embeddingFunc := withEmbeddingRetry(chromem.NewEmbeddingFuncOpenAICompat(embedURL, embedKey, embedModel, nil), logger)
	fingerprint := buildEmbeddingFingerprint(provider, embedURL, embedModel)
	return embeddingRuntimeConfig{
		EmbeddingFunc: embeddingFunc,
		Provider:      provider,
		BaseURL:       embedURL,
		APIKey:        embedKey,
		Model:         embedModel,
		Fingerprint:   fingerprint,
		Disabled:      false,
		Local:         isLocalEmbeddingProvider(cfg),
		Embedder:      embeddings.NewProviderFuncEmbedder(embeddingFunc, provider, embedModel, fingerprint),
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
