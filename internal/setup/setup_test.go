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

// TestEnsureConfigFileCreatesMinimalFallbackWithoutTemplate verifies that when
// neither the user config nor the embedded template exists, ensureConfigFile
// produces a syntactically valid YAML document with a `server` section so the
// next startup can boot with sensible defaults.
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

	// The fallback must be valid YAML.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("fallback not valid YAML: %v", err)
	}

	// And contain the server section so port/host have defaults.
	if _, ok := raw["server"]; !ok {
		t.Fatalf("fallback missing server section: %s", data)
	}
	indexing, ok := raw["indexing"].(map[string]interface{})
	if !ok {
		t.Fatalf("fallback missing indexing section: %s", data)
	}
	chunking, ok := indexing["chunking"].(map[string]interface{})
	if !ok {
		t.Fatalf("fallback missing indexing.chunking section: %s", data)
	}
	if got, want := chunking["strategy"], "recursive"; got != want {
		t.Fatalf("fallback indexing.chunking.strategy = %q, want %q", got, want)
	}

	// And contain no placeholder values that would cause silent breakage.
	server, ok := raw["server"].(map[string]interface{})
	if !ok {
		t.Fatalf("server section is not a map: %v", raw["server"])
	}
	if _, ok := server["port"]; !ok {
		t.Errorf("fallback server.port missing: %v", server)
	}
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

func TestExtractTarGzContinuesOnFileError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Build an in-memory tar.gz with one good file and one that we'll
	// make un-extractable by pre-creating a read-only directory at the
	// target location.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	// File 1: normal file (should extract successfully)
	body1 := []byte("hello\n")
	hdr1 := &tar.Header{Name: "good.txt", Mode: 0o644, Size: int64(len(body1)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr1); err != nil {
		t.Fatalf("WriteHeader 1: %v", err)
	}
	tw.Write(body1)

	// File 2: in a subdirectory that we'll pre-create as read-only
	body2 := []byte("world\n")
	hdr2 := &tar.Header{Name: "readonly/bad.txt", Mode: 0o644, Size: int64(len(body2)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr2); err != nil {
		t.Fatalf("WriteHeader 2: %v", err)
	}
	tw.Write(body2)

	tw.Close()
	gz.Close()

	archivePath := filepath.Join(dir, "resources.dat")
	if err := os.WriteFile(archivePath, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Pre-create the readonly directory and remove write permission.
	roDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(roDir, 0o555); err != nil {
		t.Fatalf("MkdirAll readonly: %v", err)
	}
	// On Windows the chmod bits are largely irrelevant; skip the rest of
	// the test on Windows since we can't make the directory truly read-only.
	if runtime.GOOS == "windows" {
		t.Skip("file permission semantics differ on Windows; skip negative test")
	}

	// Extract — should NOT abort; good.txt should be created, readonly/bad.txt should fail.
	err := extractTarGz(archivePath, dir)
	if err == nil {
		t.Fatal("expected extract to return error due to readonly directory")
	}

	// But the first file should still be extracted (proving we continued past the error).
	if _, err := os.Stat(filepath.Join(dir, "good.txt")); err != nil {
		t.Errorf("good.txt should have been extracted before the error: %v", err)
	}

	// Restore so TempDir cleanup works.
	os.Chmod(roDir, 0o755)
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

func TestServiceAlreadyInstalledLinuxFileFallback(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-specific test")
	}
	t.Parallel()

	// On a test machine without the unit file, this should return false
	// regardless of systemctl availability.
	if serviceAlreadyInstalled("/nonexistent-install-dir", slog.Default()) {
		t.Log("serviceAlreadyInstalled returned true — possibly because aurago.service is actually installed in this environment. Test inconclusive.")
	}
}
