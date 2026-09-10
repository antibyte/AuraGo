package config

import "strings"

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
