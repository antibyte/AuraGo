package server

import (
	"testing"

	"aurago/internal/config"
)

func TestGlassSpeechUsesConfiguredProvider(t *testing.T) {
	cfg := &config.Config{}
	cfg.TTS.Provider = "phonon"
	cfg.TTS.Language = "de"
	cfg.TTS.Phonon.VoiceID = "de-voice"
	cfg.SpeechLab.Enabled = true
	cfg.SpeechLab.BaseURL = "http://127.0.0.1:9"
	cfg.SpeechLab.ChatOutputEnabled = true

	if !glassTTSReady(cfg) {
		t.Fatal("phonon should be ready")
	}
	got := glassTTSConfig(cfg)
	if got.Provider != "phonon" || got.Language != "de" || got.Phonon.VoiceID != "de-voice" || !got.NoProviderFallback {
		t.Fatalf("provider=%q lang=%q voice=%q fallback=%v", got.Provider, got.Language, got.Phonon.VoiceID, got.NoProviderFallback)
	}
}

func TestGlassSpeechIgnoresBrokenSanoFallback(t *testing.T) {
	cfg := &config.Config{}
	cfg.TTS.Provider = "sanotts"
	if !glassTTSReady(cfg) {
		t.Fatal("an explicitly selected sanoTTS provider is ready")
	}
	cfg.TTS.Provider = ""
	if glassTTSReady(cfg) {
		t.Fatal("empty provider must not pretend sanoTTS is active")
	}
}
