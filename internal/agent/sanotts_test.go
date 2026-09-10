package agent

import (
	"aurago/internal/config"
	"testing"
)

func TestSanoTTSRuntimeConfiguration(t *testing.T) {
	cfg := &config.Config{}
	cfg.TTS.Provider = "sanotts"
	cfg.TTS.Language = "auto"
	cfg.Server.UILanguage = "de"
	if !isTTSConfigured(cfg) || buildRuntimeTTSConfig(cfg, "").Language != "de" {
		t.Fatal("local default is unavailable or ignores the user language")
	}
	cfg.TTS.Provider = "google"
	if got := buildRuntimeTTSConfig(cfg, "").Language; got != "de" {
		t.Fatalf("switching providers leaked automatic language token: %q", got)
	}
}
