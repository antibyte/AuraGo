package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecureResolveRejectsProtectedSystemPaths(t *testing.T) {
	jail := filepath.Join(t.TempDir(), "agent_workspace")
	workdir := filepath.Join(jail, "workdir")
	dataDir := filepath.Join(jail, "data")
	for _, dir := range []string{workdir, dataDir, filepath.Join(jail, "skills")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(jail, "config.yaml")
	if err := os.WriteFile(configPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	perms := defaultRuntimePermissionsForTests()
	perms.ProtectedDataDir = dataDir
	perms.ProtectedSystemFiles = []string{configPath}
	ConfigureRuntimePermissions(perms)
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })

	for _, path := range []string{
		".env",
		filepath.Join("deploy", "prod.env"),
		"AURAGO_MASTER.KEY",
		filepath.Join("..", "data", "short_term.db"),
		filepath.Join("..", "CONFIG.YAML"),
		filepath.Join(workdir, "secrets.env"),
	} {
		if _, err := secureResolve(workdir, path); err == nil || !strings.Contains(err.Error(), "protected AuraGo") {
			t.Fatalf("secureResolve(%q) error = %v, want protected-path denial", path, err)
		}
	}
	for _, path := range []string{"notes.txt", filepath.Join("..", "skills", "helper.py")} {
		if _, err := secureResolve(workdir, path); err != nil {
			t.Fatalf("secureResolve(%q) = %v, want allowed", path, err)
		}
	}
}

func TestSecureResolveNameChecksWorkWithoutRuntimeSnapshot(t *testing.T) {
	ClearRuntimePermissionsForTest()
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	workdir := filepath.Join(t.TempDir(), "agent_workspace", "workdir")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".env", "vault.bin", "vault.bin.lock", "aurago_master.key"} {
		if _, err := secureResolve(workdir, path); err == nil {
			t.Fatalf("secureResolve(%q) succeeded without a runtime snapshot", path)
		}
	}
}

func TestFileReaderAdvancedCannotReadWorkspaceEnv(t *testing.T) {
	workdir := filepath.Join(t.TempDir(), "agent_workspace", "workdir")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workdir, ".env"), []byte("DEPLOY_TOKEN=leak-me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := ExecuteFileReaderAdvanced("head", ".env", "", 0, 0, 5, workdir)
	if strings.Contains(out, "leak-me") || !strings.Contains(out, "protected AuraGo") {
		t.Fatalf("file_reader_advanced output = %s", out)
	}
}

func TestIntersectRuntimePermissionsKeepsProtectedPaths(t *testing.T) {
	got := intersectRuntimePermissions(
		RuntimePermissions{ProtectedDataDir: "/srv/aurago/data", ProtectedSystemFiles: []string{"/srv/aurago/config.yaml"}},
		RuntimePermissions{ProtectedSystemFiles: []string{"/srv/aurago/data/vault.bin"}},
	)
	if got.ProtectedDataDir != "/srv/aurago/data" {
		t.Fatalf("ProtectedDataDir = %q", got.ProtectedDataDir)
	}
	if len(got.ProtectedSystemFiles) != 2 {
		t.Fatalf("ProtectedSystemFiles = %v", got.ProtectedSystemFiles)
	}
}
