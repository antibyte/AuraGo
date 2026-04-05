package warnings

import (
	"fmt"
	"log/slog"

	"aurago/internal/config"
)

// RegisterBuiltinProducers populates the registry with warnings that can be detected
// immediately at startup. Call this after all subsystems have been initialised.
func RegisterBuiltinProducers(reg *Registry, cfg *config.Config, logger *slog.Logger) {
	if reg == nil {
		return
	}

	checkTokenBudgetFallback(reg, cfg, logger)
	checkVectorDBDisabled(reg, cfg, logger)
}

// checkTokenBudgetFallback emits a warning when the token budget had to fall back
// to the minimal 8192 default because context-window auto-detection failed.
func checkTokenBudgetFallback(reg *Registry, cfg *config.Config, logger *slog.Logger) {
	if cfg.Agent.SystemPromptTokenBudgetAuto && cfg.Agent.ContextWindow <= 0 {
		model := cfg.LLM.Model
		if model == "" {
			for _, p := range cfg.Providers {
				if p.ID == cfg.LLM.Provider {
					model = p.Model
					break
				}
			}
		}
		reg.Add(Warning{
			ID:       "token_budget_fallback",
			Severity: SeverityWarning,
			Title:    "Token Budget Fallback",
			Description: fmt.Sprintf(
				"Context window auto-detection failed for model %q. The system prompt token budget fell back to 8192 tokens, which significantly limits agent capabilities. "+
					"You can set llm.context_window manually in config.yaml to resolve this.",
				model,
			),
			Category: CategoryPerformance,
		})
		logger.Warn("Registered warning: token budget fallback to 8192", "model", model)
	}
}

// checkVectorDBDisabled emits a warning when the vector database / long-term memory
// is not available (embedding provider set to "disabled").
func checkVectorDBDisabled(reg *Registry, cfg *config.Config, logger *slog.Logger) {
	if cfg.Embeddings.Provider == "disabled" || cfg.Embeddings.Provider == "" {
		reg.Add(Warning{
			ID:          "vectordb_disabled",
			Severity:    SeverityInfo,
			Title:       "Long-Term Memory Disabled",
			Description: "The embedding provider is disabled. The agent will not be able to store or search long-term memory.",
			Category:    CategorySystem,
		})
		logger.Info("Registered warning: vector DB / long-term memory disabled")
	}
}
