package config

import "strings"

// TTSVoiceProviderConfig stores cloud voice settings; credentials remain vault-only.
type TTSVoiceProviderConfig struct {
	APIKey  string `yaml:"-" vault:"api_key"`
	VoiceID string `yaml:"voice_id"`
}

// TTSModelProviderConfig also selects a model for OpenAI-compatible speech APIs.
type TTSModelProviderConfig struct {
	APIKey  string `yaml:"-" vault:"api_key"`
	VoiceID string `yaml:"voice_id"`
	ModelID string `yaml:"model_id"`
}

// SanoTTSLanguage follows explicit speech settings, then the request/user language.
// Unsupported languages are resolved to English by the sanoTTS voice catalog.
func (c *Config) SanoTTSLanguage(requested string) string {
	for _, lang := range []string{c.TTS.Language, requested, c.Server.UILanguage, c.Agent.SystemLanguage} {
		lang = strings.TrimSpace(lang)
		if lang != "" && !strings.EqualFold(lang, "auto") {
			return lang
		}
	}
	return "en"
}
