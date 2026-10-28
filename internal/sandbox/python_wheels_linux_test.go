//go:build linux

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSandboxHelperBootstrapsDistributionPip(t *testing.T) {
	caps := Detect()
	if caps.LandlockABI < 3 {
		t.Skip("Landlock ABI 3 or newer is required")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("system Python unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	wheelDir, err := exec.CommandContext(ctx, python, "-I", "-c", "import ensurepip,sysconfig; print(sysconfig.get_config_var('WHEEL_PKG_DIR') or '')").Output()
	if err != nil || filepath.Clean(strings.TrimSpace(string(wheelDir))) != "/usr/share/python-wheels" {
		t.Skip("Python does not use the distribution wheel directory")
	}
	if _, err := os.Stat("/usr/share/python-wheels"); err != nil {
		t.Skip("distribution wheels unavailable")
	}
	runtimeDir := t.TempDir()
	create := exec.CommandContext(ctx, python, "-I", "-m", "venv", "--without-pip", filepath.Join(runtimeDir, "venv"))
	if output, err := create.CombinedOutput(); err != nil {
		t.Fatalf("create private venv: %v\n%s", err, output)
	}
	privatePython := filepath.Join(runtimeDir, "venv", "bin", "python")
	if err := exec.CommandContext(ctx, privatePython, "-I", "-m", "pip", "--version").Run(); err == nil {
		t.Fatal("fixture unexpectedly already contains pip")
	}
	sb := NewLandlockSandbox(ShellSandboxConfig{MaxMemoryMB: 512, MaxCPUSeconds: 30, MaxFileSizeMB: 100}, caps, runtimeDir, nil)
	env, err := sb.buildEnv(runtimeDir, []string{
		"AURAGO_TEST_HELPER_MODE=exec",
		"AURAGO_SBX_EXEC_BIN=" + privatePython,
		`AURAGO_SBX_EXEC_ARGS=["-I","-m","ensurepip","--upgrade"]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSandboxHelperEnforcesIsolationAndLowMemoryLimit$")
	command.Env = env
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bootstrap pip under Landlock: %v\n%s", err, output)
	}
	if output, err := exec.CommandContext(ctx, privatePython, "-I", "-m", "pip", "--version").CombinedOutput(); err != nil {
		t.Fatalf("private pip remains unavailable: %v\n%s", err, output)
	}
}
