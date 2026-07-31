package config

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	DefaultSpeechLabBaseURL        = "http://s2s-vulkan:8765"
	DefaultSpeechLabTimeoutSeconds = 60
)

// SpeechLabConfig integrates the external s2s-vulkan speech orchestrator.
// Speech Lab is an optional, surface-scoped ASR/TTS provider; AuraGo retains
// ownership of conversations, LLM calls, tools, and memory.
type SpeechLabConfig struct {
	Enabled           bool   `yaml:"enabled" json:"enabled"`
	BaseURL           string `yaml:"base_url" json:"base_url"`
	AdvancedUIURL     string `yaml:"advanced_ui_url,omitempty" json:"advanced_ui_url,omitempty"`
	Language          string `yaml:"language" json:"language"`
	Voice             string `yaml:"voice" json:"voice"`
	TimeoutSeconds    int    `yaml:"timeout_seconds" json:"timeout_seconds"`
	SIPEnabled        bool   `yaml:"sip_enabled" json:"sip_enabled"`
	ChatInputEnabled  bool   `yaml:"chat_input_enabled" json:"chat_input_enabled"`
	ChatOutputEnabled bool   `yaml:"chat_output_enabled" json:"chat_output_enabled"`

	// Legacy aliases from the pre-release Speech Lab integration. They are
	// accepted on load but never emitted through JSON or new configuration.
	LegacyUseForSIP       *bool `yaml:"use_for_sip,omitempty" json:"-"`
	LegacyUseForChatVoice *bool `yaml:"use_for_chat_voice,omitempty" json:"-"`
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
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultSpeechLabBaseURL
	}
	cfg.AdvancedUIURL = strings.TrimRight(strings.TrimSpace(cfg.AdvancedUIURL), "/")
	cfg.Language = strings.TrimSpace(cfg.Language)
	if cfg.Language == "" {
		cfg.Language = "de"
	}
	cfg.Voice = strings.TrimSpace(cfg.Voice)
	if cfg.Voice == "" {
		cfg.Voice = "M1"
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = DefaultSpeechLabTimeoutSeconds
	}
}

// ValidateSpeechLabConfig validates syntax and bounded scalar fields. Network
// destinations are additionally restricted by the Speech Lab HTTP transport at
// dial time, after DNS resolution.
func ValidateSpeechLabConfig(cfg SpeechLabConfig) error {
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
	if len(cfg.Voice) > 128 {
		return fmt.Errorf("speech_lab.voice is too long")
	}
	return nil
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
