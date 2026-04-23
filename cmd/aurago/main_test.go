package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvSetsMissingVariablesOnly(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(envPath, []byte("AURAGO_TEST_LOAD=from-file\nAURAGO_TEST_KEEP=from-file\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("AURAGO_TEST_KEEP", "existing")
	_ = os.Unsetenv("AURAGO_TEST_LOAD")

	loadDotEnv(envPath, slog.Default())

	if got := os.Getenv("AURAGO_TEST_LOAD"); got != "from-file" {
		t.Fatalf("AURAGO_TEST_LOAD = %q, want from-file", got)
	}
	if got := os.Getenv("AURAGO_TEST_KEEP"); got != "existing" {
		t.Fatalf("AURAGO_TEST_KEEP = %q, want existing", got)
	}
}

func TestFindLegacyVaultPathReturnsPreviousDefaultVault(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")
	legacyVault := filepath.Join(configDir, "data", "vault.bin")
	if err := os.MkdirAll(filepath.Dir(legacyVault), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(legacyVault, []byte("vault"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got := findLegacyVaultPath(configPath, filepath.Join(t.TempDir(), "new-data"))
	if got != legacyVault {
		t.Fatalf("findLegacyVaultPath() = %q, want %q", got, legacyVault)
	}
}

func TestFindLegacyVaultPathIgnoresCurrentVaultLocation(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")
	currentDataDir := filepath.Join(configDir, "data")
	legacyVault := filepath.Join(currentDataDir, "vault.bin")
	if err := os.MkdirAll(filepath.Dir(legacyVault), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(legacyVault, []byte("vault"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if got := findLegacyVaultPath(configPath, currentDataDir); got != "" {
		t.Fatalf("findLegacyVaultPath() = %q, want empty", got)
	}
}
