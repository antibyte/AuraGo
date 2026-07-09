package tools

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWriteScript_SizeLimitRejected(t *testing.T) {
	toolsDir := t.TempDir()
	oversized := strings.Repeat("x", maxScriptBytes+1)

	_, cleanup, err := writeScript(oversized, toolsDir)
	if err == nil {
		if cleanup != nil {
			cleanup()
		}
		t.Fatal("expected error for oversized script, got nil")
	}
	if !strings.Contains(err.Error(), "script too large") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWriteScript_AcceptsNormalScript(t *testing.T) {
	toolsDir := t.TempDir()
	code := `print("hello")`

	path, cleanup, err := writeScript(code, toolsDir)
	if err != nil {
		t.Fatalf("unexpected error for normal script: %v", err)
	}
	defer cleanup()

	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		t.Errorf("script file does not exist at %s", path)
	}
}

func TestWriteScript_CleanupRemovesFile(t *testing.T) {
	toolsDir := t.TempDir()
	code := `print("cleanup test")`

	path, cleanup, err := writeScript(code, toolsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cleanup()

	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("expected script file to be removed after cleanup, but it still exists: %s", path)
	}
}

func TestWriteScript_AtMaxSize(t *testing.T) {
	// A script exactly at the limit must be accepted.
	toolsDir := t.TempDir()
	atLimit := strings.Repeat("x", maxScriptBytes)

	path, cleanup, err := writeScript(atLimit, toolsDir)
	if err != nil {
		t.Fatalf("script at exactly maxScriptBytes should be accepted, got: %v", err)
	}
	defer cleanup()

	info, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("stat failed: %v", statErr)
	}
	if int(info.Size()) != maxScriptBytes {
		t.Errorf("file size = %d, want %d", info.Size(), maxScriptBytes)
	}
}

func TestBuildToolBridgeSDKPreludeRegistersAuragoModule(t *testing.T) {
	prelude := BuildToolBridgeSDKPrelude(7)
	for _, want := range []string{
		`_call_limit = 7`,
		`def call_tool(self, tool_name, parameters=None, timeout=60):`,
		`_aurago_sys.modules["aurago"] = _AuraGoModule()`,
		`AURAGO_TOOL_BRIDGE_URL`,
		`X-Internal-Token`,
	} {
		if !strings.Contains(prelude, want) {
			t.Fatalf("prelude missing %q:\n%s", want, prelude)
		}
	}
}

func TestNormalizeToolBridgeCallLimit(t *testing.T) {
	if got := normalizeToolBridgeCallLimit(0); got != defaultToolBridgeCallLimit {
		t.Fatalf("default limit = %d, want %d", got, defaultToolBridgeCallLimit)
	}
	if got := normalizeToolBridgeCallLimit(999); got != maxToolBridgeCallLimit {
		t.Fatalf("clamped limit = %d, want %d", got, maxToolBridgeCallLimit)
	}
	if got := normalizeToolBridgeCallLimit(3); got != 3 {
		t.Fatalf("explicit limit = %d, want 3", got)
	}
}

func TestValidPackageNameAcceptsExtrasWithHyphen(t *testing.T) {
	valid := []string{
		"llm-sandbox[mcp-docker]",
		"llm-sandbox[mcp-podman]",
		"package[extra_one,extra-two]>=1.2.3",
	}
	for _, pkg := range valid {
		t.Run(pkg, func(t *testing.T) {
			if !validPackageName.MatchString(pkg) {
				t.Fatalf("expected package specifier %q to be accepted", pkg)
			}
		})
	}
}

func TestValidPackageNameRejectsUnsafeSpecifiers(t *testing.T) {
	invalid := []string{
		"--index-url=https://example.invalid package",
		"../package",
		"package; rm -rf /",
		"package[extra]; echo bad",
	}
	for _, pkg := range invalid {
		t.Run(pkg, func(t *testing.T) {
			if validPackageName.MatchString(pkg) {
				t.Fatalf("expected package specifier %q to be rejected", pkg)
			}
		})
	}
}

func TestSandboxExecuteCode_ManagerNil(t *testing.T) {
	// Ensure the package-level shorthand returns a descriptive error when no manager is set.
	// Temporarily clear the global manager.
	sandboxMgrMu.Lock()
	old := globalSandboxMgr
	globalSandboxMgr = nil
	sandboxMgrMu.Unlock()
	t.Cleanup(func() {
		sandboxMgrMu.Lock()
		globalSandboxMgr = old
		sandboxMgrMu.Unlock()
	})

	_, err := SandboxExecuteCode(`print("hi")`, "python", nil, 5, nil)
	if err == nil {
		t.Fatal("expected error when sandbox manager is nil")
	}
}

func TestExecutePythonDoesNotInheritSensitiveEnv(t *testing.T) {
	workspaceDir := t.TempDir()
	toolsDir := t.TempDir()
	installFakePython(t, workspaceDir)
	setSensitiveEnvForSubprocessTest(t)
	t.Setenv("AURAGO_FAKE_PYTHON", "1")

	stdout, stderr, err := ExecutePython(`print("unused")`, workspaceDir, toolsDir)
	if err != nil {
		t.Fatalf("ExecutePython() error = %v, stderr = %q", err, stderr)
	}
	assertCleanFakePythonOutput(t, stdout)
}

func TestRunToolDoesNotInheritSensitiveEnv(t *testing.T) {
	workspaceDir := t.TempDir()
	toolsDir := t.TempDir()
	installFakePython(t, workspaceDir)
	setSensitiveEnvForSubprocessTest(t)
	t.Setenv("AURAGO_FAKE_PYTHON", "1")
	if err := os.WriteFile(filepath.Join(toolsDir, "tool.py"), []byte(`print("unused")`), 0o600); err != nil {
		t.Fatalf("write fake tool: %v", err)
	}

	stdout, stderr, err := RunTool("tool.py", nil, workspaceDir, toolsDir)
	if err != nil {
		t.Fatalf("RunTool() error = %v, stderr = %q", err, stderr)
	}
	assertCleanFakePythonOutput(t, stdout)
}

func TestExecutePythonBackgroundDoesNotInheritSensitiveEnv(t *testing.T) {
	workspaceDir := t.TempDir()
	toolsDir := t.TempDir()
	installFakePython(t, workspaceDir)
	setSensitiveEnvForSubprocessTest(t)
	t.Setenv("AURAGO_FAKE_PYTHON", "1")
	t.Setenv("AURAGO_FAKE_PYTHON_SLEEP_MS", "500")
	registry := NewProcessRegistry(testBackgroundTaskLogger())

	pid, err := ExecutePythonBackground(`print("unused")`, workspaceDir, toolsDir, registry)
	if err != nil {
		t.Fatalf("ExecutePythonBackground() error = %v", err)
	}
	defer registry.Terminate(pid)

	output := waitForProcessOutput(t, registry, pid)
	assertCleanFakePythonOutput(t, output)
}

func TestCreateVenvDoesNotInheritSensitiveEnv(t *testing.T) {
	workspaceDir := t.TempDir()
	binDir := t.TempDir()
	installFakeExecutable(t, filepath.Join(binDir, pythonExecutableName()))
	setSensitiveEnvForSubprocessTest(t)
	t.Setenv("AURAGO_FAKE_PYTHON", "1")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := createVenv(workspaceDir, testBackgroundTaskLogger())
	if err != nil {
		t.Fatalf("createVenv() error = %v", err)
	}
}

func TestInstallPackageDoesNotInheritSensitiveEnv(t *testing.T) {
	workspaceDir := t.TempDir()
	installFakePip(t, workspaceDir)
	setSensitiveEnvForSubprocessTest(t)
	t.Setenv("AURAGO_FAKE_PYTHON", "1")

	stdout, stderr, err := InstallPackage("requests", workspaceDir)
	if err != nil {
		t.Fatalf("InstallPackage() error = %v, stdout = %q stderr = %q", err, stdout, stderr)
	}
	assertCleanFakePythonOutput(t, stdout)
}

func installFakePython(t *testing.T, workspaceDir string) {
	t.Helper()
	installFakeExecutable(t, fakePythonPath(workspaceDir))
}

func installFakePip(t *testing.T, workspaceDir string) {
	t.Helper()
	installFakeExecutable(t, fakePipPath(workspaceDir))
}

func fakePythonPath(workspaceDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(workspaceDir, "venv", "Scripts", "python.exe")
	}
	return filepath.Join(workspaceDir, "venv", "bin", "python")
}

func fakePipPath(workspaceDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(workspaceDir, "venv", "Scripts", "pip.exe")
	}
	return filepath.Join(workspaceDir, "venv", "bin", "pip")
}

func pythonExecutableName() string {
	if runtime.GOOS == "windows" {
		return "python.exe"
	}
	return "python"
}

func installFakeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fake executable dir: %v", err)
	}
	src, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatalf("open test binary for fake executable: %v", err)
	}
	defer src.Close()
	dst, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatalf("create fake executable: %v", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		t.Fatalf("copy fake executable: %v", err)
	}
	if err := dst.Close(); err != nil {
		t.Fatalf("close fake executable: %v", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatalf("chmod fake executable: %v", err)
		}
	}
}

func setSensitiveEnvForSubprocessTest(t *testing.T) {
	t.Helper()
	t.Setenv("AURAGO_MASTER_KEY", strings.Repeat("a", 64))
	t.Setenv("OPENAI_API_KEY", "sk-test-should-not-leak")
	t.Setenv("CUSTOM_PASSWORD", "password-should-not-leak")
}

func assertCleanFakePythonOutput(t *testing.T, output string) {
	t.Helper()
	if strings.Contains(output, "leaked:") {
		t.Fatalf("subprocess inherited sensitive environment: %q", output)
	}
	if !strings.Contains(output, "env-clean") {
		t.Fatalf("expected fake python clean-env marker, got %q", output)
	}
}

func waitForProcessOutput(t *testing.T, registry *ProcessRegistry, pid int) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		if info, ok := registry.Get(pid); ok {
			last = info.ReadOutput()
			if strings.TrimSpace(last) != "" {
				return last
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for process %d output, last=%q", pid, last)
	return ""
}
