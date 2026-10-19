package tools

import (
	"archive/zip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestRegisteredKeyPathSurvivesEnvironmentRemovalAndReload(t *testing.T) {
	workdir := filepath.Join(t.TempDir(), "agent_workspace", "workdir")
	secrets := filepath.Join(workdir, "secrets")
	if err := os.MkdirAll(secrets, 0o700); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(secrets, "arbitrary-key-name.txt")
	if err := os.WriteFile(key, []byte("fixture-only"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AURAGO_MASTER_KEY_FILE", key)
	if err := RegisterProtectedSystemFile(key); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AURAGO_MASTER_KEY_FILE", "")
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workdir
	perms := defaultRuntimePermissionsForTests()
	perms.ProtectedSystemFiles = protectedSystemFilesFromConfig(cfg)
	ConfigureRuntimePermissions(perms)
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	if !IsProtectedSystemPath(key, workdir, cfg) {
		t.Fatal("reloaded config lost the registered key path")
	}
	if _, err := OpenToolInputFile(key, cfg); err == nil {
		t.Fatal("raw tool input opened the private key")
	}
	if _, err := rootedToolReadFile(key); err == nil {
		t.Fatal("rooted wrapper opened the private key")
	}
	for _, operation := range []string{"read_file", "write_file", "delete", "move"} {
		out := ExecuteFilesystem(operation, key, filepath.Join(workdir, "moved.txt"), "changed", nil, workdir, 0, 0)
		if !strings.Contains(out, "protected AuraGo") {
			t.Fatalf("%s was not blocked: %s", operation, out)
		}
	}
	if out := ExecuteFilesystem("delete", secrets, "", "", nil, workdir, 0, 0); !strings.Contains(out, "protected AuraGo") {
		t.Fatalf("parent deletion was not blocked: %s", out)
	}
	if data, err := os.ReadFile(key); err != nil || string(data) != "fixture-only" {
		t.Fatal("private key changed")
	}
}

func TestRegisteredKeyPathProtectsSymlinkAndResolvedTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "actual.txt")
	link := filepath.Join(dir, "chosen.txt")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := RegisterProtectedSystemFile(link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{target, link} {
		if err := requireUnprotectedProcessPath(path, false); err == nil {
			t.Fatalf("unprotected key alias: %s", path)
		}
	}
}

func TestSupervisorRuntimeNeverAdoptsAgentVenvAndBlocksWrappers(t *testing.T) {
	jail := filepath.Join(t.TempDir(), "agent_workspace")
	workdir := filepath.Join(jail, "workdir")
	private := filepath.Join(jail, "private", "sandbox-supervisor")
	if err := os.MkdirAll(workdir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := sandboxSupervisorRuntimeDir(filepath.Join(workdir, "venv"), workdir); err == nil {
		t.Fatal("agent venv was adopted")
	}
	if _, err := sandboxSupervisorRuntimeDir("", workdir); err == nil {
		t.Fatal("missing private runtime silently fell back")
	}
	if _, err := sandboxSupervisorRuntimeDir(private, workdir); err != nil {
		t.Fatal(err)
	}
	perms := defaultRuntimePermissionsForTests()
	ConfigureRuntimePermissions(perms)
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	startup := filepath.Join(private, "venv", "lib", "site-packages", "sitecustomize.py")
	if err := os.MkdirAll(filepath.Dir(startup), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(startup, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{startup, filepath.Join(private, "venv", "lib", "evil.pth"), filepath.Join(private, "venv", "llm_sandbox", "docker.py")} {
		if _, err := secureResolve(workdir, path); err == nil {
			t.Fatalf("runtime path writable through native tools: %s", path)
		}
		if err := rootedToolWriteFileAtomic(path, []byte("changed")); err == nil {
			t.Fatalf("runtime path writable through rooted wrapper: %s", path)
		}
	}
	for _, operation := range []string{"delete", "move"} {
		out := ExecuteFilesystem(operation, filepath.Dir(private), filepath.Join(jail, "relocated"), "", nil, workdir, 0, 0)
		if !strings.Contains(out, "protected AuraGo") {
			t.Fatalf("parent %s was not blocked: %s", operation, out)
		}
	}
	archive := filepath.Join(workdir, "attempt.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	entry, err := z.Create("private/sandbox-supervisor/venv/lib/site-packages/sitecustomize.py")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("changed")); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if out := ExecuteArchive(workdir, "extract", archive, jail, "", ""); !strings.Contains(out, "protected AuraGo") {
		t.Fatalf("archive overwrote runtime: %s", out)
	}
	if data, err := os.ReadFile(startup); err != nil || string(data) != "original" {
		t.Fatal("runtime startup hook changed")
	}
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workdir
	if _, err := OpenToolInputFile(startup, cfg); err == nil {
		t.Fatal("upload input opened private runtime")
	}
	if _, err := readIsolatedScript(startup, workdir); err == nil {
		t.Fatal("run_tool wrapper opened private runtime")
	}
	requests := 0
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer fixture.Close()
	previousURL := virustotalBaseURL
	virustotalBaseURL = fixture.URL
	defer func() { virustotalBaseURL = previousURL }()
	out := ExecuteVirusTotalScanWithOptions("fixture-only", VirusTotalOptions{FilePath: startup, Mode: "upload", Config: cfg})
	if requests != 0 || !strings.Contains(out, "protected AuraGo") {
		t.Fatalf("private runtime reached upload: calls=%d result=%s", requests, out)
	}
	// This is still an agent environment for explicit host compatibility, never a supervisor.
	if _, err := secureResolve(workdir, filepath.Join(workdir, "venv", "agent-owned.py")); err != nil {
		t.Fatalf("unrelated agent venv changed: %v", err)
	}
}

func TestSupervisorRuntimeRejectsSymlinkIntoAgentState(t *testing.T) {
	dir := t.TempDir()
	workdir := filepath.Join(dir, "agent_workspace", "workdir")
	data := filepath.Join(dir, "data")
	for _, path := range []string{workdir, data} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	runtimeDir := filepath.Join(data, "sandbox-supervisor")
	if err := os.Symlink(workdir, runtimeDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := sandboxSupervisorRuntimeDir(runtimeDir, workdir); err == nil {
		t.Fatal("private runtime adopted an agent-owned directory through a symlink")
	}
}

func TestSupervisorPythonIgnoresCWDAndPythonPathHooks(t *testing.T) {
	python := findSystemPython()
	if python == "" {
		t.Skip("Python unavailable")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sitecustomize.py"), []byte("raise RuntimeError('untrusted startup hook')"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "aurago_untrusted_runtime_fixture.py"), []byte("raise RuntimeError('untrusted package')"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PYTHONPATH", dir)
	cmd := supervisorPythonCommand(context.Background(), python, "-c", "import importlib.util,sys; assert sys.flags.isolated; assert importlib.util.find_spec('aurago_untrusted_runtime_fixture') is None; print('isolated')")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil || strings.TrimSpace(string(output)) != "isolated" {
		t.Fatalf("supervisor accepted Python hooks: %v %s", err, output)
	}
	if cmd.Args[1] != "-I" {
		t.Fatal("missing isolated interpreter flag")
	}
	// A bootstrap on PATH inside the jail must never become a trusted interpreter.
	venvBin := filepath.Join(dir, "agent_workspace", "workdir", "venv", "bin")
	if err := os.MkdirAll(venvBin, 0o700); err != nil {
		t.Fatal(err)
	}
	name := "python3"
	if filepath.Ext(python) == ".exe" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(venvBin, name), []byte("untrusted"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", venvBin)
	if got := trustedSupervisorBootstrapPython(filepath.Dir(filepath.Dir(venvBin))); got != "" {
		t.Fatalf("agent interpreter accepted: %s", got)
	}
}
