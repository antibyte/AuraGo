package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppliesSafeManusDefaults(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("providers: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Manus.Enabled || !cfg.Manus.ReadOnly || cfg.Manus.AllowCreateTasks || cfg.Manus.AllowSendMessages ||
		cfg.Manus.AllowStopTasks || cfg.Manus.AllowFileUploads || cfg.Manus.AllowFileDownloads {
		t.Fatalf("unsafe Manus defaults: %#v", cfg.Manus)
	}
	if cfg.Manus.DefaultAgentProfile != "manus-1.6" || cfg.Manus.RequestTimeoutSeconds != 60 ||
		cfg.Manus.PollIntervalSeconds != 5 || cfg.Manus.MaxWaitSeconds != 60 ||
		cfg.Manus.MaxResultBytes != 262144 || cfg.Manus.MaxFileSizeMB != 20 {
		t.Fatalf("unexpected Manus defaults: %#v", cfg.Manus)
	}
}

func TestApplyVaultSecretsLoadsManusAPIKey(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.ApplyVaultSecrets(&testSecretVault{data: map[string]string{"manus_api_key": "manus-secret"}})
	if cfg.Manus.APIKey != "manus-secret" {
		t.Fatalf("Manus.APIKey = %q", cfg.Manus.APIKey)
	}
}

func TestConfigSaveOmitsManusAPIKey(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("providers: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Manus: ManusConfig{Enabled: true, APIKey: "must-not-leak"}}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "must-not-leak") || strings.Contains(string(raw), "api_key") {
		t.Fatalf("Manus API key leaked into YAML:\n%s", raw)
	}
}

func TestMigratePlaintextManusAPIKeyToVault(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("manus:\n  enabled: true\n  api_key: legacy-manus-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	vault := &testSecretVault{data: map[string]string{}}
	MigratePlaintextSecretsToVault(path, vault, slog.Default())
	if vault.data["manus_api_key"] != "legacy-manus-secret" {
		t.Fatalf("vault Manus key = %q", vault.data["manus_api_key"])
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "legacy-manus-secret") || strings.Contains(string(raw), "api_key") {
		t.Fatalf("legacy Manus key remains in YAML:\n%s", raw)
	}
}
