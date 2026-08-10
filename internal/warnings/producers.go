package warnings

import (
	"context"
	"fmt"
	"log/slog"

	"aurago/internal/config"
	"aurago/internal/llm"
)

// RegisterBuiltinProducers populates the registry with warnings that can be detected
// immediately at startup. Call this after all subsystems have been initialised.
func RegisterBuiltinProducers(reg *Registry, cfg *config.Config, vdb VectorDBHealth, logger *slog.Logger) {
	if reg == nil {
		return
	}

	checkTokenBudgetFallback(reg, cfg, logger)
	checkVectorDBDisabled(reg, cfg, logger)
	NewVectorDBMonitor(reg, cfg, vdb, logger).Start()
}

// checkTokenBudgetFallback emits a warning when either model limit could not be
// resolved through an override, the registry, or the provider probe.
func checkTokenBudgetFallback(reg *Registry, cfg *config.Config, logger *slog.Logger) {
	if reg == nil || cfg == nil {
		return
	}

	route := llm.ModelRoute{
		ProviderID:   cfg.LLM.Provider,
		ProviderType: cfg.LLM.ProviderType,
		BaseURL:      cfg.LLM.BaseURL,
		APIKey:       cfg.LLM.APIKey,
		Model:        cfg.LLM.Model,
		Primary:      true,
	}
	if provider := cfg.FindProvider(cfg.LLM.Provider); provider != nil {
		route.ContextWindowOverride = provider.ContextWindow
		route.MaxOutputTokensOverride = provider.MaxOutputTokens
		if route.Model == "" {
			route.Model = provider.Model
		}
	}
	limits := llm.ResolveModelLimits(context.Background(), route, cfg.Agent.ContextWindow, logger)
	if !limits.Unknown {
		return
	}

	reg.Add(Warning{
		ID:       "token_budget_fallback",
		Severity: SeverityWarning,
		Title:    "Model Limit Fallback",
		Description: fmt.Sprintf(
			"Model limits for %q could not be fully resolved. Unresolved values use conservative defaults of %d context tokens and %d output tokens. "+
				"Set provider-level context_window and max_output_tokens overrides if the provider does not expose reliable metadata.",
			route.Model, llm.ConservativeContextWindow, llm.ConservativeOutputTokens,
		),
		Category: CategoryPerformance,
	})
	if logger != nil {
		logger.Warn("Registered warning: conservative model limits", "model", route.Model,
			"context_source", limits.ContextSource, "output_source", limits.OutputSource)
	}
}

// checkVectorDBDisabled emits a warning when long-term memory is intentionally
// disabled via configuration (empty provider or provider=disabled).
func checkVectorDBDisabled(reg *Registry, cfg *config.Config, logger *slog.Logger) {
	if embeddingsConfigured(cfg) {
		return
	}
	reg.Add(Warning{
		ID:          "vectordb_disabled",
		Severity:    SeverityInfo,
		Title:       "Long-Term Memory Disabled",
		Description: "The embedding provider is disabled in configuration. The agent will not be able to store or search long-term memory.",
		Category:    CategorySystem,
	})
	if logger != nil {
		logger.Info("Registered warning: vector DB / long-term memory disabled by configuration")
	}
}
