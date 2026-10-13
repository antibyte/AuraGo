package config

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const (
	ProviderUsageGeneral   = "general"
	ProviderUsageDecisions = "decisions"
)

// NormalizeProviderUsage canonicalizes the optional provider usage scope.
func NormalizeProviderUsage(usage string) string {
	return strings.ToLower(strings.TrimSpace(usage))
}

// DecisionsConfig controls AuraGo's optional local or OpenRouter Decisions
// backend. Model and runtime pins for the local backend are deliberately owned
// by the application rather than being overridable through user configuration.
type DecisionsConfig struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	Backend            string `yaml:"backend" json:"backend"`
	Provider           string `yaml:"provider" json:"provider"`
	Model              string `yaml:"model" json:"model"`
	Deployment         string `yaml:"deployment" json:"deployment"`
	CPUFallbackAllowed bool   `yaml:"cpu_fallback_allowed" json:"cpu_fallback_allowed"`
}

const (
	DecisionsBackendLocal    = "local"
	DecisionsBackendProvider = "provider"

	DecisionsDeploymentAuto   = "auto"
	DecisionsDeploymentNative = "native"
	DecisionsDeploymentDocker = "docker"
)

// Normalize applies safe defaults without enabling the service or CPU use.
func (c DecisionsConfig) Normalize() DecisionsConfig {
	c.Provider = strings.TrimSpace(c.Provider)
	c.Model = strings.TrimSpace(c.Model)
	if c.Backend == "" {
		c.Backend = DecisionsBackendLocal
	}
	if c.Deployment == "" {
		c.Deployment = DecisionsDeploymentAuto
	}
	return c
}

// NormalizeDecisionsConfig applies safe defaults in place.
func NormalizeDecisionsConfig(c *DecisionsConfig) {
	if c == nil {
		return
	}
	*c = c.Normalize()
}

// Validate rejects deployment modes that cannot be safely managed.
func (c DecisionsConfig) Validate() error {
	c = c.Normalize()
	switch c.Backend {
	case DecisionsBackendLocal, DecisionsBackendProvider:
	default:
		return fmt.Errorf("unsupported decisions backend %q", c.Backend)
	}
	switch c.Deployment {
	case DecisionsDeploymentAuto, DecisionsDeploymentNative, DecisionsDeploymentDocker:
		return nil
	default:
		return fmt.Errorf("unsupported decisions deployment %q", c.Deployment)
	}
}

// ValidateDecisionsConfig checks the selected managed deployment policy.
func ValidateDecisionsConfig(c DecisionsConfig) error {
	return c.Validate()
}

// ValidateProviderEntryUsage rejects unknown provider scopes and prevents a
// Decisions-only profile from masquerading as a general-purpose provider.
func ValidateProviderEntryUsage(provider ProviderEntry) error {
	switch NormalizeProviderUsage(provider.Usage) {
	case "", ProviderUsageGeneral:
		return nil
	case ProviderUsageDecisions:
		if !strings.EqualFold(strings.TrimSpace(provider.Type), "openrouter") {
			return fmt.Errorf("provider %q with decisions-only usage must use OpenRouter", provider.ID)
		}
		return nil
	default:
		return fmt.Errorf("provider %q has unsupported usage %q", provider.ID, provider.Usage)
	}
}

// ValidateDecisionsProvider validates the optional external Decisions route.
// Incomplete provider drafts are allowed while Decisions is disabled.
func ValidateDecisionsProvider(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("decisions configuration is unavailable")
	}
	if err := ValidateDecisionsConfig(cfg.Decisions); err != nil {
		return err
	}
	for _, provider := range cfg.Providers {
		if err := ValidateProviderEntryUsage(provider); err != nil {
			return err
		}
	}
	if err := validateDecisionsOnlyProviderReferences(cfg); err != nil {
		return err
	}

	settings := cfg.Decisions.Normalize()
	if !settings.Enabled || settings.Backend != DecisionsBackendProvider {
		return nil
	}
	providerID := strings.TrimSpace(settings.Provider)
	if providerID == "" {
		return fmt.Errorf("decisions.provider is required when the provider backend is enabled")
	}
	if strings.TrimSpace(settings.Model) == "" {
		return fmt.Errorf("decisions.model is required when the provider backend is enabled")
	}
	provider := cfg.FindProvider(providerID)
	if provider == nil {
		return fmt.Errorf("decisions.provider %q does not reference a configured provider", providerID)
	}
	if !strings.EqualFold(strings.TrimSpace(provider.Type), "openrouter") {
		return fmt.Errorf("decisions.provider %q must use OpenRouter", providerID)
	}
	if !validDecisionsOpenRouterURL(provider.BaseURL) {
		return fmt.Errorf("decisions.provider %q must use the canonical OpenRouter URL", providerID)
	}
	return nil
}

// validateDecisionsOnlyProviderReferences prevents a profile reserved for the
// Decisions API from being selected by general chat and other provider-backed
// integrations. The dedicated Decisions reference is intentionally omitted.
func validateDecisionsOnlyProviderReferences(cfg *Config) error {
	type reference struct{ path, id string }
	refs := []reference{
		{"llm.provider", cfg.LLM.Provider},
		{"llm.helper_provider", cfg.LLM.HelperProvider},
		{"fallback_llm.provider", cfg.FallbackLLM.Provider},
		{"vision.provider", cfg.Vision.Provider},
		{"whisper.provider", cfg.Whisper.Provider},
		{"sip.voice.agent_provider_id", cfg.SIP.Voice.AgentProviderID},
		{"sip.voice.classic.asr_provider_id", cfg.SIP.Voice.Classic.ASRProviderID},
		{"embeddings.provider", cfg.Embeddings.Provider},
		{"llm_guardian.provider", cfg.LLMGuardian.Provider},
		{"mission_preparation.provider", cfg.MissionPreparation.Provider},
		{"flows.ai_provider", cfg.Flows.AIProvider},
		{"cyd_creator.model", cfg.CYDCreator.Model},
		{"image_generation.provider", cfg.ImageGeneration.Provider},
		{"music_generation.provider", cfg.MusicGeneration.Provider},
		{"video_generation.provider", cfg.VideoGeneration.Provider},
		{"a2a.llm.provider", cfg.A2A.LLM.Provider},
		{"co_agents.llm.provider", cfg.CoAgents.LLM.Provider},
		{"co_agents.specialists.researcher.llm.provider", cfg.CoAgents.Specialists.Researcher.LLM.Provider},
		{"co_agents.specialists.coder.llm.provider", cfg.CoAgents.Specialists.Coder.LLM.Provider},
		{"co_agents.specialists.designer.llm.provider", cfg.CoAgents.Specialists.Designer.LLM.Provider},
		{"co_agents.specialists.security.llm.provider", cfg.CoAgents.Specialists.Security.LLM.Provider},
		{"co_agents.specialists.writer.llm.provider", cfg.CoAgents.Specialists.Writer.LLM.Provider},
	}
	for area, target := range cfg.LLMRouter.Areas {
		refs = append(refs, reference{"llm_router.areas." + area + ".provider", target.Provider})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].path < refs[j].path })
	for _, ref := range refs {
		if provider := cfg.FindProvider(strings.TrimSpace(ref.id)); provider != nil && NormalizeProviderUsage(provider.Usage) == ProviderUsageDecisions {
			return fmt.Errorf("provider %q has decisions-only usage and cannot be referenced by %s", provider.ID, ref.path)
		}
	}
	return nil
}

func validDecisionsOpenRouterURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Scheme != "https" || u.Host != "openrouter.ai" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return false
	}
	return u.Path == "" || u.Path == "/api/v1" || u.Path == "/api/v1/"
}
