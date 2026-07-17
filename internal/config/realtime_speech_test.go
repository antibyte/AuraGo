package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealtimeSpeechDefaultsAndValidation(t *testing.T) {
	cfg := RealtimeSpeechConfig{}
	NormalizeRealtimeSpeechConfig(&cfg)
	if cfg.ParkAfterSeconds != 5 {
		t.Fatalf("ParkAfterSeconds = %d, want 5", cfg.ParkAfterSeconds)
	}
	if err := ValidateRealtimeSpeechConfig(cfg); err != nil {
		t.Fatalf("empty disabled config should be valid: %v", err)
	}
	cfg.ParkAfterSeconds = 4
	if err := ValidateRealtimeSpeechConfig(cfg); err == nil {
		t.Fatal("expected park interval below 5 seconds to fail")
	}
	cfg.ParkAfterSeconds = 61
	if err := ValidateRealtimeSpeechConfig(cfg); err == nil {
		t.Fatal("expected park interval above 60 seconds to fail")
	}
}

func TestRealtimeSpeechVaultKeyRejectsUnsafeIDs(t *testing.T) {
	if got := RealtimeSpeechProfileAPIKeyVaultKey("living-room"); got != "realtime_speech_profile_living-room_api_key" {
		t.Fatalf("vault key = %q", got)
	}
	for _, unsafe := range []string{"", "../key", "Upper", "with space", strings.Repeat("a", 65)} {
		if got := RealtimeSpeechProfileAPIKeyVaultKey(unsafe); got != "" {
			t.Fatalf("unsafe ID %q produced vault key %q", unsafe, got)
		}
	}
}

func TestMigrateAndHydrateRealtimeSpeechAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `realtime_speech:
  enabled: true
  default_profile: main
  park_after_seconds: 5
  profiles:
    - id: main
      name: Main voice
      provider: openai
      model: gpt-realtime-2.1
      voice: marin
      enabled: true
      api_key: realtime-secret
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	vault := &testSecretVault{data: make(map[string]string)}
	MigratePlaintextSecretsToVault(path, vault, slog.Default())
	cleaned, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cleaned), "realtime-secret") || strings.Contains(string(cleaned), "api_key:") {
		t.Fatalf("plaintext realtime speech key survived migration:\n%s", cleaned)
	}
	if vault.data["realtime_speech_profile_main_api_key"] != "realtime-secret" {
		t.Fatalf("vault value = %q", vault.data["realtime_speech_profile_main_api_key"])
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded.ApplyVaultSecrets(vault)
	if loaded.RealtimeSpeech.Profiles[0].APIKey != "realtime-secret" {
		t.Fatal("realtime speech API key was not hydrated from vault")
	}
}

func TestLoadRejectsInvalidRealtimeSpeechParkInterval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("realtime_speech:\n  park_after_seconds: 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "park_after_seconds") {
		t.Fatalf("Load error = %v, want park interval validation", err)
	}
}
