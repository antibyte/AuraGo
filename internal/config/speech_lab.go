package config

import (
	"fmt"
	"net/url"
	"strings"

	"aurago/internal/llm/catalog"
)

const (
	DefaultSpeechLabBaseURL        = "http://s2s-vulkan:8765"
	DefaultManagedSpeechLabBaseURL = "http://127.0.0.1:8765"
	DefaultSpeechLabTimeoutSeconds = 60
)

// SpeechLabDeploymentConfig controls the optional AuraGo-owned Docker bundle.
// External mode remains the backwards-compatible default for configurations
// that already point at a separately managed s2s instance.
type SpeechLabDeploymentConfig struct {
	Mode       string `yaml:"mode,omitempty" json:"mode,omitempty"`
	Bundle     string `yaml:"bundle,omitempty" json:"bundle,omitempty"`
	AutoStart  bool   `yaml:"auto_start" json:"auto_start"`
	AutoUpdate bool   `yaml:"auto_update" json:"auto_update"`
}

// SpeechLabConfig integrates the external s2s-vulkan speech orchestrator.
// Speech Lab is an optional, surface-scoped ASR/TTS provider; AuraGo retains
// ownership of conversations, LLM calls, tools, and memory.
type SpeechLabConfig struct {
	Enabled           bool                      `yaml:"enabled" json:"enabled"`
	BaseURL           string                    `yaml:"base_url" json:"base_url"`
	AdvancedUIURL     string                    `yaml:"advanced_ui_url,omitempty" json:"advanced_ui_url,omitempty"`
	Language          string                    `yaml:"language" json:"language"`
	ChatLLMProviderID string                    `yaml:"chat_llm_provider_id,omitempty" json:"chat_llm_provider_id,omitempty"`
	TimeoutSeconds    int                       `yaml:"timeout_seconds" json:"timeout_seconds"`
	SIPEnabled        bool                      `yaml:"sip_enabled" json:"sip_enabled"`
	ChatInputEnabled  bool                      `yaml:"chat_input_enabled" json:"chat_input_enabled"`
	ChatOutputEnabled bool                      `yaml:"chat_output_enabled" json:"chat_output_enabled"`
	Deployment        SpeechLabDeploymentConfig `yaml:"deployment,omitempty" json:"deployment,omitempty"`

	// Legacy aliases from the pre-release Speech Lab integration. They are
	// accepted on load but never emitted through JSON or new configuration.
	LegacyUseForSIP       *bool  `yaml:"use_for_sip,omitempty" json:"-"`
	LegacyUseForChatVoice *bool  `yaml:"use_for_chat_voice,omitempty" json:"-"`
	Voice                 string `yaml:"voice,omitempty" json:"-"`
}

// NormalizeSpeechLabConfig applies defaults and legacy aliases. rawConfig is
// the original YAML so canonical fields always win when both forms exist.
func NormalizeSpeechLabConfig(cfg *SpeechLabConfig, rawConfig []byte) {
	if cfg == nil {
		return
	}
	if !yamlHasPath(rawConfig, "speech_lab", "sip_enabled") && cfg.LegacyUseForSIP != nil {
		cfg.SIPEnabled = *cfg.LegacyUseForSIP
	}
	if !yamlHasPath(rawConfig, "speech_lab", "chat_output_enabled") && cfg.LegacyUseForChatVoice != nil {
		cfg.ChatOutputEnabled = *cfg.LegacyUseForChatVoice
	}
	cfg.LegacyUseForSIP = nil
	cfg.LegacyUseForChatVoice = nil

	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if strings.TrimSpace(cfg.Deployment.Mode) == "" {
		// New/default configurations use the managed bundle. Existing explicit
		// base URLs remain external and are never silently replaced.
		if !yamlHasPath(rawConfig, "speech_lab", "base_url") {
			cfg.Deployment.Mode = "managed"
		} else {
			cfg.Deployment.Mode = "external"
		}
	}
	cfg.Deployment.Mode = strings.ToLower(strings.TrimSpace(cfg.Deployment.Mode))
	if cfg.Deployment.Mode != "managed" && cfg.Deployment.Mode != "external" {
		cfg.Deployment.Mode = "external"
	}
	cfg.Deployment.Bundle = strings.TrimSpace(cfg.Deployment.Bundle)
	if cfg.Deployment.Bundle == "" {
		cfg.Deployment.Bundle = "stable"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultSpeechLabBaseURL
	}
	cfg.AdvancedUIURL = strings.TrimRight(strings.TrimSpace(cfg.AdvancedUIURL), "/")
	cfg.Language = strings.TrimSpace(cfg.Language)
	if cfg.Language == "" {
		cfg.Language = "de"
	}
	// Voice is owned by the active Speech Lab runtime stack. Legacy YAML values
	// remain loadable but are deliberately ignored and removed on the next save.
	cfg.Voice = ""
	cfg.ChatLLMProviderID = strings.TrimSpace(cfg.ChatLLMProviderID)
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = DefaultSpeechLabTimeoutSeconds
	}
}

// ValidateSpeechLabConfig validates syntax and bounded scalar fields. Network
// destinations are additionally restricted by the Speech Lab HTTP transport at
// dial time, after DNS resolution.
func ValidateSpeechLabConfig(cfg SpeechLabConfig) error {
	if mode := strings.ToLower(strings.TrimSpace(cfg.Deployment.Mode)); mode != "" && mode != "managed" && mode != "external" {
		return fmt.Errorf("speech_lab.deployment.mode must be managed or external")
	}
	if len(strings.TrimSpace(cfg.Deployment.Bundle)) > 64 {
		return fmt.Errorf("speech_lab.deployment.bundle is too long")
	}
	if err := validateSpeechLabURL("base_url", cfg.BaseURL, true); err != nil {
		return err
	}
	if err := validateSpeechLabURL("advanced_ui_url", cfg.AdvancedUIURL, false); err != nil {
		return err
	}
	if cfg.TimeoutSeconds < 1 || cfg.TimeoutSeconds > DefaultSpeechLabTimeoutSeconds {
		return fmt.Errorf("speech_lab.timeout_seconds must be between 1 and 60")
	}
	if len(cfg.Language) > 35 {
		return fmt.Errorf("speech_lab.language is too long")
	}
	if len(cfg.ChatLLMProviderID) > 128 {
		return fmt.Errorf("speech_lab.chat_llm_provider_id is too long")
	}
	return nil
}

// ValidateSpeechLabProviderReference verifies the optional provider selected
// exclusively for direct webchat turns transcribed by Speech Lab.
func ValidateSpeechLabProviderReference(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("runtime configuration is unavailable")
	}
	id := strings.TrimSpace(cfg.SpeechLab.ChatLLMProviderID)
	if id == "" {
		return nil
	}
	provider := cfg.FindProvider(id)
	if provider == nil {
		return fmt.Errorf("speech_lab.chat_llm_provider_id references an unknown provider")
	}
	if eligible, reason := SpeechLabChatProviderEligibility(provider); !eligible {
		switch reason {
		case "missing_model":
			return fmt.Errorf("speech_lab.chat_llm_provider_id references a provider without a model")
		case "media_provider":
			return fmt.Errorf("speech_lab.chat_llm_provider_id references a media-only provider")
		default:
			return fmt.Errorf("speech_lab.chat_llm_provider_id references an unsupported chat provider")
		}
	}
	return nil
}

// SpeechLabChatProviderEligibility reports whether an existing provider can
// drive a complete chat-agent turn. Authentication readiness is evaluated by
// the server against the Vault and provider-specific runtime managers.
func SpeechLabChatProviderEligibility(provider *ProviderEntry) (bool, string) {
	if provider == nil {
		return false, "unknown_provider"
	}
	providerType := catalog.NormalizeProviderID(provider.Type)
	if !catalog.IsRuntimeProviderType(providerType) {
		return false, "unsupported_provider_type"
	}
	switch providerType {
	case "stability", "ideogram", "vision", "agnes":
		return false, "media_provider"
	}
	if strings.TrimSpace(provider.Model) == "" {
		return false, "missing_model"
	}
	return true, "available"
}

func validateSpeechLabURL(field, raw string, required bool) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if required {
			return fmt.Errorf("speech_lab.%s is required", field)
		}
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("speech_lab.%s must be an absolute HTTP(S) URL", field)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("speech_lab.%s must use http or https", field)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("speech_lab.%s must not contain credentials, query, or fragment", field)
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return fmt.Errorf("speech_lab.%s must not contain an API path", field)
	}
	return nil
}

// Active reports whether Speech Lab is enabled and callable.
func (c SpeechLabConfig) Active() bool {
	return c.Enabled && strings.TrimSpace(c.BaseURL) != ""
}
