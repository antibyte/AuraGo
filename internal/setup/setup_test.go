package setup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("fallback not valid YAML: %v", err)
	}
	if _, ok := raw["server"]; !ok {
		t.Fatalf("fallback missing server section: %s", data)
	}
}

func TestEnsureConfigFileCreatesValidMinimalConfig(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	configPath := filepath.Join(installDir, "config.yaml")

	if err := ensureConfigFile(installDir, configPath, slog.Default()); err != nil {
		t.Fatalf("ensureConfigFile: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// The minimal config must load successfully.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("fallback config is not valid YAML: %v", err)
	}

	// And contain at least the server section so port/host have defaults.
	if _, ok := raw["server"]; !ok {
		t.Errorf("fallback config missing 'server' section, got keys: %v", keys(raw))
	}
}

func keys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestConfigAllowsSudoUnrestricted(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("agent:\n  sudo_unrestricted: true # intentional\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if !configAllowsSudoUnrestricted(configPath) {
		t.Fatal("expected sudo_unrestricted=true to be detected")
	}
}

func TestExtractTarGzStripsSetuidAndSetgidBits(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Build an in-memory tar.gz containing a file with setuid+setgid+sticky bits.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\necho pwned\n")
	hdr := &tar.Header{
		Name:     "evil.sh",
		Mode:     0o7777 | int64(os.ModeSetuid) | int64(os.ModeSetgid) | int64(os.ModeSticky),
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatalf("Write body: %v", err)
	}
	tw.Close()
	gz.Close()

	archivePath := filepath.Join(dir, "resources.dat")
	if err := os.WriteFile(archivePath, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := extractTarGz(archivePath, dir); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "evil.sh"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	mode := info.Mode()
	if mode&os.ModeSetuid != 0 {
		t.Errorf("setuid bit preserved: mode=%v", mode)
	}
	if mode&os.ModeSetgid != 0 {
		t.Errorf("setgid bit preserved: mode=%v", mode)
	}
	if mode&os.ModeSticky != 0 {
		t.Errorf("sticky bit preserved: mode=%v", mode)
	}
	// Windows does not honor POSIX mode bits via os.OpenFile/os.Stat, so the
	// exact permission value can only be asserted on POSIX platforms.
	if runtime.GOOS != "windows" {
		if perm := mode.Perm(); perm != 0o640 {
			t.Errorf("perm = %o, want 0o640", perm)
		}
	}
}

func TestBuildSystemdUnitEscapesInstallDirWithQuotes(t *testing.T) {
	t.Parallel()

	// Use a path containing a double-quote to simulate a malicious installDir.
	installDir := `/opt/aurago/odd"path`
	exePath := installDir + "/aurago"

	unit, err := buildSystemdUnit(
		"AuraGo AI Agent",
		"alice",
		installDir,
		exePath,
		"/etc/aurago/master.key",
		installDir+" /etc/aurago",
		false,
		false,
	)
	if err != nil {
		t.Fatalf("buildSystemdUnit: %v", err)
	}

	// The raw, unescaped quote must NOT appear in the unit file.
	if strings.Contains(unit, `odd"path`) {
		t.Errorf("unit contains unescaped installDir; raw quote leaked: %s", unit)
	}

	// The escaped form (with backslash before the quote) MUST appear.
	if !strings.Contains(unit, `odd\"path`) {
		t.Errorf("unit does not contain escaped path: %s", unit)
	}
}

func TestBuildSystemdUnitEmptyArgsRejected(t *testing.T) {
	t.Parallel()

	_, err := buildSystemdUnit("AuraGo", "", "/opt/aurago", "/opt/aurago/aurago", "/opt/aurago/.env", "/opt/aurago", false, false)
	if err == nil {
		t.Fatal("expected error when user is empty")
	}
}
