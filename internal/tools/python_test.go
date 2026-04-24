package tools

import (
	"os"
	"strings"
	"testing"
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
