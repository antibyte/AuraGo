package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

var LLMRouterAreas = []string{"general", "easy", "normal", "complex", "coding", "research", "creativity", "security", "writing"}

type LLMRouterTarget struct {
	Provider string `yaml:"provider" json:"provider"`
	Model    string `yaml:"model" json:"model"`
}

type LLMRouterConfig struct {
	Enabled               bool                       `yaml:"enabled" json:"enabled"`
	HelperFallback        bool                       `yaml:"helper_fallback" json:"helper_fallback"`
	HelperTimeoutMS       int                        `yaml:"helper_timeout_ms" json:"helper_timeout_ms"`
	HelperMaxCallsPerHour int                        `yaml:"helper_max_calls_per_hour" json:"helper_max_calls_per_hour"`
	Areas                 map[string]LLMRouterTarget `yaml:"areas" json:"areas"`
}

func DefaultLLMRouterConfig() LLMRouterConfig {
	c := LLMRouterConfig{HelperFallback: true, HelperTimeoutMS: 1500, HelperMaxCallsPerHour: 20, Areas: map[string]LLMRouterTarget{}}
	for _, area := range LLMRouterAreas {
		c.Areas[area] = LLMRouterTarget{}
	}
	return c
}

func (c *LLMRouterConfig) UnmarshalYAML(node *yaml.Node) error {
	type plain LLMRouterConfig
	v := plain(DefaultLLMRouterConfig())
	if err := node.Decode(&v); err != nil {
		return fmt.Errorf("llm_router: %w", err)
	}
	// Older programmatic Config snapshots serialize a zero-valued new section.
	// Preserve their explicit off/zero switches while supplying a valid deadline.
	if v.HelperTimeoutMS == 0 {
		v.HelperTimeoutMS = DefaultLLMRouterConfig().HelperTimeoutMS
	}
	*c = LLMRouterConfig(v)
	return nil
}

func ValidLLMRouterArea(area string) bool {
	for _, known := range LLMRouterAreas {
		if known == area {
			return true
		}
	}
	return false
}

// ValidateLLMRouterConfig tolerates stale provider references only on startup.
func ValidateLLMRouterConfig(cfg *Config, strictReferences bool) error {
	c := cfg.LLMRouter
	missingSection := !c.Enabled && !c.HelperFallback && c.HelperTimeoutMS == 0 && c.HelperMaxCallsPerHour == 0 && c.Areas == nil
	if !missingSection && (c.HelperTimeoutMS < 250 || c.HelperTimeoutMS > 5000) {
		return fmt.Errorf("llm_router.helper_timeout_ms must be between 250 and 5000")
	}
	if c.HelperMaxCallsPerHour < 0 || c.HelperMaxCallsPerHour > 120 {
		return fmt.Errorf("llm_router.helper_max_calls_per_hour must be between 0 and 120")
	}
	for area, target := range c.Areas {
		if !ValidLLMRouterArea(area) {
			return fmt.Errorf("llm_router.areas: unknown area %q", area)
		}
		if strings.TrimSpace(target.Provider) == "" {
			if strings.TrimSpace(target.Model) != "" {
				return fmt.Errorf("llm_router.areas.%s: model requires a provider", area)
			}
			continue
		}
		p := cfg.FindProvider(target.Provider)
		if p == nil {
			if strictReferences {
				return fmt.Errorf("llm_router.areas.%s: provider does not exist", area)
			}
			continue
		}
		if ok, _ := SpeechLabChatProviderEligibility(p); !ok {
			return fmt.Errorf("llm_router.areas.%s: provider must support chat", area)
		}
		if strings.TrimSpace(target.Model) == "" && strings.TrimSpace(p.Model) == "" {
			return fmt.Errorf("llm_router.areas.%s: provider has no model", area)
		}
	}
	return nil
}

func (c LLMRouterConfig) HasAssignments() bool {
	for _, t := range c.Areas {
		if strings.TrimSpace(t.Provider) != "" {
			return true
		}
	}
	return false
}
