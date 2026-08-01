package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeSpeechLabConfigDefaultsAndLegacyAliases(t *testing.T) {
	useSIP := true
	useChat := true
	cfg := SpeechLabConfig{
		LegacyUseForSIP:       &useSIP,
		LegacyUseForChatVoice: &useChat,
	}
	NormalizeSpeechLabConfig(&cfg, []byte("speech_lab:\n  use_for_sip: true\n  use_for_chat_voice: true\n"))
	if cfg.BaseURL != DefaultSpeechLabBaseURL || cfg.TimeoutSeconds != 60 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.Voice != "" {
		t.Fatalf("legacy voice must not become a runtime default: %q", cfg.Voice)
	}
	if !cfg.SIPEnabled || !cfg.ChatOutputEnabled || cfg.ChatInputEnabled {
		t.Fatalf("unexpected surface migration: %+v", cfg)
	}
	if cfg.LegacyUseForSIP != nil || cfg.LegacyUseForChatVoice != nil {
		t.Fatal("legacy aliases must be cleared after normalization")
	}
}

func TestNormalizeSpeechLabConfigDiscardsLegacyVoiceAndTrimsProvider(t *testing.T) {
	cfg := SpeechLabConfig{Voice: "M1", ChatLLMProviderID: " fast "}
	NormalizeSpeechLabConfig(&cfg, []byte("speech_lab:\n  voice: M1\n  chat_llm_provider_id: fast\n"))
	if cfg.Voice != "" {
		t.Fatalf("legacy voice was retained: %q", cfg.Voice)
	}
	if cfg.ChatLLMProviderID != "fast" {
		t.Fatalf("provider ID was not normalized: %q", cfg.ChatLLMProviderID)
	}
}

func TestValidateSpeechLabProviderReference(t *testing.T) {
	cfg := &Config{Providers: []ProviderEntry{{ID: "fast", Type: "openai", Model: "fast-model"}}}
	if err := ValidateSpeechLabProviderReference(cfg); err != nil {
		t.Fatalf("valid provider rejected: %v", err)
	}
	cfg.SpeechLab.ChatLLMProviderID = "fast"
	if err := ValidateSpeechLabProviderReference(cfg); err != nil {
		t.Fatalf("known provider rejected: %v", err)
	}
	cfg.SpeechLab.ChatLLMProviderID = "missing"
	if err := ValidateSpeechLabProviderReference(cfg); err == nil {
		t.Fatal("unknown provider was accepted")
	}
}

func TestNormalizeSpeechLabCanonicalFieldsWin(t *testing.T) {
	legacy := true
	cfg := SpeechLabConfig{LegacyUseForSIP: &legacy}
	NormalizeSpeechLabConfig(&cfg, []byte("speech_lab:\n  sip_enabled: false\n  use_for_sip: true\n"))
	if cfg.SIPEnabled {
		t.Fatal("canonical sip_enabled must win over the legacy alias")
	}
}

func TestValidateSpeechLabConfig(t *testing.T) {
	valid := SpeechLabConfig{BaseURL: "http://127.0.0.1:8765", TimeoutSeconds: 60}
	if err := ValidateSpeechLabConfig(valid); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	for _, raw := range []string{
		"file:///tmp/s2s",
		"http://user:pass@127.0.0.1:8765",
		"http://127.0.0.1:8765/ready",
		"http://127.0.0.1:8765?x=1",
	} {
		cfg := valid
		cfg.BaseURL = raw
		if err := ValidateSpeechLabConfig(cfg); err == nil {
			t.Fatalf("invalid base URL accepted: %s", raw)
		}
	}
	tooSlow := valid
	tooSlow.TimeoutSeconds = DefaultSpeechLabTimeoutSeconds + 1
	if err := ValidateSpeechLabConfig(tooSlow); err == nil {
		t.Fatal("inference timeout above 60 seconds was accepted")
	}
}

func TestLoadSpeechLabEnvironmentOverride(t *testing.T) {
	t.Setenv("AURAGO_SPEECH_LAB_BASE_URL", "http://127.0.0.1:9876/")
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("speech_lab:\n  enabled: true\n  base_url: http://127.0.0.1:8765\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SpeechLab.BaseURL != "http://127.0.0.1:9876" {
		t.Fatalf("env override not applied: %q", cfg.SpeechLab.BaseURL)
	}
}

func TestLoadSpeechLabLegacyVoiceIsIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("speech_lab:\n  enabled: true\n  base_url: http://127.0.0.1:8765\n  voice: M1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SpeechLab.Voice != "" {
		t.Fatalf("legacy voice became a runtime default: %q", cfg.SpeechLab.Voice)
	}
}
