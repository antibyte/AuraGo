package llm

import (
	"strings"

	"aurago/internal/config"
)

// HelperLLMConfig captures the resolved helper-LLM runtime settings.
type HelperLLMConfig struct {
	Enabled      bool
	ProviderID   string
	ProviderType string
	BaseURL      string
	APIKey       string
	Model        string
}

// ResolveHelperLLM returns the resolved helper-LLM configuration without
// falling back to the main LLM. Helper features must be explicitly configured.
func ResolveHelperLLM(cfg *config.Config) HelperLLMConfig {
	if cfg == nil {
		return HelperLLMConfig{}
	}

	resolved := HelperLLMConfig{
		Enabled:      cfg.LLM.HelperEnabled,
		ProviderID:   strings.TrimSpace(cfg.LLM.HelperProvider),
		ProviderType: strings.TrimSpace(cfg.LLM.HelperProviderType),
		BaseURL:      strings.TrimSpace(cfg.LLM.HelperBaseURL),
		APIKey:       strings.TrimSpace(cfg.LLM.HelperAPIKey),
		Model:        strings.TrimSpace(cfg.LLM.HelperResolvedModel),
	}

	return resolved
}

// IsHelperLLMAvailable reports whether the helper LLM is explicitly enabled
// and fully resolved for runtime use.
func IsHelperLLMAvailable(cfg *config.Config) bool {
	resolved := ResolveHelperLLM(cfg)
	return resolved.Enabled &&
		resolved.ProviderID != "" &&
		resolved.ProviderType != "" &&
		resolved.Model != ""
}
