package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTTSCloudConfigVaultRoundTrip(t *testing.T) {
	for _, provider := range []string{"xai", "openai", "stepfun", "resemble"} {
		t.Run(provider, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			content := "tts:\n  provider: " + provider + "\n  " + provider + ":\n    api_key: fixture-tts-secret\n    voice_id: custom-voice\n"
			if provider == "openai" || provider == "stepfun" {
				content += "    model_id: custom-model\n"
			}
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			vault := &testSecretVault{data: map[string]string{}}
			MigratePlaintextSecretsToVault(path, vault, slog.Default())
			if vault.data["tts_"+provider+"_api_key"] != "fixture-tts-secret" {
				t.Fatal("TTS key was not moved to the expected vault slot")
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			cfg.ApplyVaultSecrets(vault)
			var key, voice, model string
			switch provider {
			case "xai":
				key, voice = cfg.TTS.XAI.APIKey, cfg.TTS.XAI.VoiceID
			case "openai":
				key, voice, model = cfg.TTS.OpenAI.APIKey, cfg.TTS.OpenAI.VoiceID, cfg.TTS.OpenAI.ModelID
			case "stepfun":
				key, voice, model = cfg.TTS.StepFun.APIKey, cfg.TTS.StepFun.VoiceID, cfg.TTS.StepFun.ModelID
			case "resemble":
				key, voice = cfg.TTS.Resemble.APIKey, cfg.TTS.Resemble.VoiceID
			}
			if cfg.TTS.Provider != provider || key != "fixture-tts-secret" || voice != "custom-voice" {
				t.Fatal("TTS selection, voice or vault key was lost on reload")
			}
			if (provider == "openai" || provider == "stepfun") && model != "custom-model" {
				t.Fatal("custom TTS model was lost on reload")
			}
			if err := cfg.Save(path); err != nil {
				t.Fatal(err)
			}
			stored, err := os.ReadFile(path)
			if err != nil || strings.Contains(string(stored), "fixture-tts-secret") {
				t.Fatalf("secret leaked into saved YAML or read failed: %v", err)
			}
		})
	}
}

func TestFreshInstallTTSAndLanguage(t *testing.T) {
	data, err := os.ReadFile("../../config_template.yaml")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TTS.Provider != "phonon" || cfg.TTS.Language != "auto" {
		t.Fatalf("fresh installation TTS = %q/%q", cfg.TTS.Provider, cfg.TTS.Language)
	}
	cfg.Server.UILanguage = "de"
	if got := cfg.SanoTTSLanguage(""); got != "de" {
		t.Fatalf("user language = %q", got)
	}
	if got := cfg.SanoTTSLanguage("fr-CA"); got != "fr-CA" {
		t.Fatalf("request language = %q", got)
	}
	cfg.TTS.Language = "it"
	if got := cfg.SanoTTSLanguage("de"); got != "it" {
		t.Fatalf("explicit speech language = %q", got)
	}
	// Existing choices, including explicitly disabled speech, survive loading.
	for _, provider := range []string{"", "google", "supertonic"} {
		if err := os.WriteFile(path, []byte("tts:\n  provider: \""+provider+"\"\n  language: de\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load(path)
		if err != nil || loaded.TTS.Provider != provider || loaded.TTS.Language != "de" {
			t.Fatalf("existing provider %q was changed: %v", provider, err)
		}
	}
}
