package setup

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestNeedsSetupUsesExplicitConfigPath(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  port: 8088\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if NeedsSetup(installDir, configPath) {
		t.Fatal("expected explicit config path to suppress setup mode")
	}
}

func TestEnsureMasterKeyUsesEnvironmentAndRepairsEnvFile(t *testing.T) {
	installDir := t.TempDir()
	envPath := filepath.Join(installDir, ".env")
	if err := os.WriteFile(envPath, []byte("AURAGO_MASTER_KEY=broken\n"), 0o600); err != nil {
		t.Fatalf("write invalid .env: %v", err)
	}

	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	t.Setenv("AURAGO_MASTER_KEY", validKey)

	if err := ensureMasterKey(installDir, slog.Default()); err != nil {
		t.Fatalf("ensureMasterKey() error = %v", err)
	}

	if got := readEnvKey(envPath, "AURAGO_MASTER_KEY"); got != validKey {
		t.Fatalf(".env key = %q, want %q", got, validKey)
	}
}

func TestEnsureMasterKeyLoadsValidEnvFileIntoProcess(t *testing.T) {
	installDir := t.TempDir()
	envPath := filepath.Join(installDir, ".env")
	validKey := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	if err := os.WriteFile(envPath, []byte("AURAGO_MASTER_KEY="+validKey+"\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	_ = os.Unsetenv("AURAGO_MASTER_KEY")
	if err := ensureMasterKey(installDir, slog.Default()); err != nil {
		t.Fatalf("ensureMasterKey() error = %v", err)
	}

	if got := os.Getenv("AURAGO_MASTER_KEY"); got != validKey {
		t.Fatalf("AURAGO_MASTER_KEY = %q, want %q", got, validKey)
	}
}

func TestEnsureConfigFileCopiesTemplateWhenMissing(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	configPath := filepath.Join(installDir, "config.yaml")
	template := []byte("server:\n  host: 127.0.0.1\n")
	if err := os.WriteFile(filepath.Join(installDir, "config_template.yaml"), template, 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}

	if err := ensureConfigFile(installDir, configPath, slog.Default()); err != nil {
		t.Fatalf("ensureConfigFile() error = %v", err)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(got) != string(template) {
		t.Fatalf("config contents = %q, want %q", string(got), string(template))
	}
}

func TestEnsureConfigFileCreatesMinimalFallbackWithoutTemplate(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	configPath := filepath.Join(installDir, "config.yaml")

	if err := ensureConfigFile(installDir, configPath, slog.Default()); err != nil {
		t.Fatalf("ensureConfigFile() error = %v", err)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(got) != "{}\n" {
		t.Fatalf("config contents = %q, want minimal fallback", string(got))
	}
}
