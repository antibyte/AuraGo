package agent

import (
	"testing"

	"aurago/internal/config"
)

func TestPhononRuntimeConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.TTS.Provider, cfg.TTS.Language = "phonon", "auto"
	cfg.Personality.CorePersonality = "servant"
	cfg.TTS.Phonon.VoiceID = "chosen-voice"
	cfg.Server.UILanguage = "de"
	got := buildRuntimeTTSConfig(cfg, "pt-BR")
	if !isTTSConfigured(cfg) || got.Provider != "phonon" || got.Language != "pt-BR" || got.Phonon.Persona != "servant" || got.Phonon.VoiceID != "chosen-voice" {
		t.Fatal("agent lost local speech settings")
	}
}
