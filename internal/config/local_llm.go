package config

import (
	"fmt"
	"strings"
)

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

// ValidateLocalLLMConfig enforces the immutable managed-provider and v1 routing contract.
func ValidateLocalLLMConfig(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("local_llm config is required")
	}
	local := &cfg.LocalLLM
	if !oneOf(local.Backend, "auto", "cuda", "sycl", "vulkan", "cpu") {
		return fmt.Errorf("local_llm.backend must be auto, cuda, sycl, vulkan, or cpu")
	}
	if !oneOf(local.ModelVariant, "q4_k_m", "q8_0") {
		return fmt.Errorf("local_llm.model_variant must be q4_k_m or q8_0")
	}
	if !oneOf(local.MTP, "off", "auto", "mtp2") {
		return fmt.Errorf("local_llm.mtp must be off, auto, or mtp2")
	}
	if !oneOf(fmt.Sprint(local.ContextSize), "16384", "32768") {
		return fmt.Errorf("local_llm.context_size must be 16384 or 32768")
	}
	if local.IdleTimeoutMinutes < 1 || local.IdleTimeoutMinutes > 1440 {
		return fmt.Errorf("local_llm.idle_timeout_minutes must be between 1 and 1440")
	}
	if local.ListenPort < 1 || local.ListenPort > 65535 {
		return fmt.Errorf("local_llm.listen_port must be between 1 and 65535")
	}

	for _, provider := range cfg.Providers {
		if strings.EqualFold(strings.TrimSpace(provider.ID), LocalLLMProviderID) {
			return fmt.Errorf("provider id %q is reserved for AuraGo's managed local model", LocalLLMProviderID)
		}
	}

	primaryLocal := strings.EqualFold(strings.TrimSpace(cfg.LLM.Provider), LocalLLMProviderID)
	fallbackLocal := cfg.FallbackLLM.Enabled &&
		strings.EqualFold(strings.TrimSpace(cfg.FallbackLLM.Provider), LocalLLMProviderID)
	if (primaryLocal || fallbackLocal) && !local.Enabled {
		return fmt.Errorf("local_llm cannot be disabled while %q is an active provider", LocalLLMProviderID)
	}
	if primaryLocal {
		if !cfg.FallbackLLM.Enabled || strings.TrimSpace(cfg.FallbackLLM.Provider) == "" || fallbackLocal {
			return fmt.Errorf("AuraGo-Qwen primary role requires one regular fallback provider")
		}
		if cfg.FindProvider(cfg.FallbackLLM.Provider) == nil {
			return fmt.Errorf("AuraGo-Qwen primary role references an unknown regular fallback provider")
		}
	}
	if fallbackLocal {
		if strings.TrimSpace(cfg.LLM.Provider) == "" || primaryLocal || cfg.FindProvider(cfg.LLM.Provider) == nil {
			return fmt.Errorf("AuraGo-Qwen fallback role requires one regular primary provider")
		}
	}
	return nil
}
