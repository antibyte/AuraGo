package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if cfg.TTS.Provider != "sanotts" || cfg.TTS.Language != "auto" {
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
