package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigratePlaintextHereNowAPIKeyToVault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	legacy := "here_now:\n  enabled: true\n  readonly: false\n  api_key: legacy-here-now-secret\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	vault := &testSecretVault{data: map[string]string{}}
	MigratePlaintextSecretsToVault(path, vault, slog.Default())
	if got := vault.data["here_now_api_key"]; got != "legacy-here-now-secret" {
		t.Fatalf("Vault here.now key = %q", got)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "legacy-here-now-secret") || strings.Contains(string(raw), "api_key") {
		t.Fatalf("legacy here.now key remains in YAML:\n%s", raw)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.ApplyVaultSecrets(vault)
	if cfg.HereNow.APIKey != "legacy-here-now-secret" {
		t.Fatalf("runtime API key = %q", cfg.HereNow.APIKey)
	}
}

func TestHereNowDefaultsRemainDenyByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("here_now:\n  enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.HereNow.ReadOnly || cfg.HereNow.AllowPublish || cfg.HereNow.AllowSiteManagement || cfg.HereNow.AllowAccessManagement || cfg.HereNow.AllowDelete {
		t.Fatalf("unsafe here.now defaults: %+v", cfg.HereNow)
	}
	if cfg.HereNow.APIKey != "" {
		t.Fatal("here.now API key must not be loaded from normal YAML")
	}
}
