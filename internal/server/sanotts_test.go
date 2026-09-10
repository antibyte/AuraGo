package server

import (
	"aurago/internal/config"
	"testing"
)

func TestSanoTTSChatAndDeviceConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.TTS.Provider = "sanotts"
	cfg.TTS.Language = "auto"
	cfg.Server.UILanguage = "de"
	if !chatVoiceOutputTTSConfigured(cfg) || !agodeskTTSConfigured(cfg) {
		t.Fatal("local default is unavailable to chat/device speech")
	}
	if got := buildChatVoiceOutputTTSConfig(cfg, ""); got.Provider != "sanotts" || got.Language != "de" {
		t.Fatalf("incorrect local speech config: %q/%q", got.Provider, got.Language)
	}
	cfg.TTS.Provider = "google"
	if got := buildChatVoiceOutputTTSConfig(cfg, "").Language; got != "de" {
		t.Fatalf("switching providers leaked automatic language token: %q", got)
	}
}
