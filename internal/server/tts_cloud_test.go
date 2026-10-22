package server

import (
	"log/slog"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestTTSCloudVaultExtractionAndMaskedIndicators(t *testing.T) {
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"xai", "openai", "stepfun", "resemble"} {
		settings := map[string]interface{}{"api_key": "fixture-cloud-key", "voice_id": "custom-voice"}
		patch := map[string]interface{}{"tts": map[string]interface{}{provider: settings}}
		if err := extractSecretsToVault(patch, vault, slog.Default()); err != nil {
			t.Fatal(err)
		}
		if _, exists := settings["api_key"]; exists {
			t.Fatal("API key remains in the config patch")
		}
		stored, err := vault.ReadSecret("tts_" + provider + "_api_key")
		if err != nil || stored != "fixture-cloud-key" {
			t.Fatalf("%s key was not persisted in its vault slot", provider)
		}
		injectVaultIndicators(patch, vault)
		if settings["api_key"] != "••••••••" || settings["voice_id"] != "custom-voice" {
			t.Fatalf("%s config does not expose a masked key indicator", provider)
		}
	}
}

func TestTTSCloudChatAndAgodeskReadiness(t *testing.T) {
	cfg := &config.Config{}
	for _, provider := range []string{"xai", "openai", "stepfun", "resemble"} {
		cfg.TTS.Provider = provider
		if chatVoiceOutputTTSConfigured(cfg) || agodeskTTSConfigured(cfg) {
			t.Fatalf("%s is ready without credentials", provider)
		}
	}
	cfg.TTS.XAI = config.TTSVoiceProviderConfig{APIKey: "fixture-xai", VoiceID: "rex"}
	cfg.TTS.OpenAI = config.TTSModelProviderConfig{APIKey: "fixture-openai", VoiceID: "coral", ModelID: "tts-1"}
	cfg.TTS.StepFun = config.TTSModelProviderConfig{APIKey: "fixture-stepfun", VoiceID: "custom", ModelID: "step-tts-mini"}
	cfg.TTS.Resemble = config.TTSVoiceProviderConfig{APIKey: "fixture-resemble", VoiceID: "voice-uuid"}
	for _, provider := range []string{"xai", "openai", "stepfun", "resemble"} {
		cfg.TTS.Provider = provider
		if !chatVoiceOutputTTSConfigured(cfg) || !agodeskTTSConfigured(cfg) {
			t.Fatalf("%s is unavailable with credentials", provider)
		}
		got := buildChatVoiceOutputTTSConfig(cfg, "de")
		if got.Provider != provider || got.XAI != cfg.TTS.XAI || got.OpenAI != cfg.TTS.OpenAI || got.StepFun != cfg.TTS.StepFun || got.Resemble != cfg.TTS.Resemble {
			t.Fatalf("%s chat lost TTS settings", provider)
		}
	}
	cfg.TTS.Resemble.VoiceID = ""
	if chatVoiceOutputTTSConfigured(cfg) || agodeskTTSConfigured(cfg) {
		t.Fatal("Resemble is ready without a voice UUID")
	}
}
