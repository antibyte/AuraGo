package server

import (
	"strings"

	"aurago/internal/config"
	"aurago/internal/llm"
)

func gameVisualPrimaryRoute(cfg *config.Config) (config.ProviderEntry, bool) {
	p := config.ProviderEntry{ID: cfg.LLM.Provider, Type: cfg.LLM.ProviderType, BaseURL: cfg.LLM.BaseURL, APIKey: cfg.LLM.APIKey, AccountID: cfg.LLM.AccountID, Model: cfg.LLM.Model}
	if source := cfg.FindProvider(cfg.LLM.Provider); source != nil {
		p = *source
		p.Model = cfg.LLM.Model
	}
	return p, !strings.EqualFold(p.Type, "agnes") && llm.ResolveProviderCapabilities(p, llm.CapabilityFallback{Multimodal: cfg.LLM.Multimodal}).Multimodal
}
func gameVisualRoute(cfg *config.Config) (config.ProviderEntry, string) {
	if p, ok := gameVisualPrimaryRoute(cfg); ok {
		return p, ""
	}
	if p := cfg.FindProvider(cfg.Vision.Provider); p != nil && strings.TrimSpace(cfg.Vision.Provider) != "" {
		if strings.EqualFold(p.Type, "agnes") {
			return config.ProviderEntry{}, "public_url_required"
		}
		if llm.ResolveProviderCapabilities(*p, llm.CapabilityFallback{Multimodal: true}).Multimodal {
			return *p, ""
		}
	}
	if strings.EqualFold(cfg.LLM.ProviderType, "agnes") {
		return config.ProviderEntry{}, "public_url_required"
	}
	return config.ProviderEntry{}, "no_vision_route"
}
