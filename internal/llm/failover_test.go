package llm

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
)

func TestFailoverManagerActiveProviderAndModelTracksFallback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{}
	cfg.LLM.ProviderType = "openrouter"
	cfg.LLM.Model = "primary-model"
	cfg.FallbackLLM.Enabled = true
	cfg.FallbackLLM.ProviderType = "ollama"
	cfg.FallbackLLM.Model = "fallback-model"
	cfg.FallbackLLM.BaseURL = "http://localhost:11434/v1"
	cfg.FallbackLLM.ErrorThreshold = 1
	cfg.FallbackLLM.ProbeIntervalSeconds = 3600

	fm := NewFailoverManager(cfg, logger)
	defer fm.Stop()

	providerType, model := fm.ActiveProviderAndModel()
	if providerType != "openrouter" || model != "primary-model" {
		t.Fatalf("initial active endpoint = (%q, %q), want (%q, %q)", providerType, model, "openrouter", "primary-model")
	}

	fm.recordError(errors.New("connection timeout"))

	providerType, model = fm.ActiveProviderAndModel()
	if providerType != "ollama" || model != "fallback-model" {
		t.Fatalf("fallback active endpoint = (%q, %q), want (%q, %q)", providerType, model, "ollama", "fallback-model")
	}
}
