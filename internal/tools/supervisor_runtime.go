package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// The agent venv may already contain model-written startup hooks. Never adopt
// it as a trusted supervisor, even after applying a new write restriction.
func sandboxSupervisorRuntimeDir(runtimeDir, agentWorkspace string) (string, error) {
	if runtimeDir == "" {
		return "", fmt.Errorf("private sandbox supervisor runtime is not configured")
	}
	abs, err := filepath.Abs(runtimeDir)
	if err != nil {
		return "", err
	}
	parent, err := secureResolveFinalPath(filepath.Dir(abs))
	if err != nil {
		return "", err
	}
	abs, err = secureResolveFinalPath(abs)
	if err != nil {
		return "", err
	}
	if !pathHasProtectedPrefix(parent, abs) || pathsEqualFold(parent, abs) {
		return "", fmt.Errorf("sandbox supervisor runtime escapes its private data directory")
	}
	agentVenv, err := filepath.Abs(filepath.Join(agentWorkspace, "venv"))
	if err != nil {
		return "", err
	}
	agentVenv, err = secureResolveFinalPath(agentVenv)
	if err != nil {
		return "", err
	}
	if pathHasProtectedPrefix(agentVenv, abs) || pathHasProtectedPrefix(abs, agentVenv) {
		return "", fmt.Errorf("sandbox supervisor must not reuse or contain the agent venv")
	}
	if err := registerProtectedProcessPath(runtimeDir, true); err != nil {
		return "", err
	}
	return abs, nil
}

func supervisorPythonCommand(ctx context.Context, python string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, python, append([]string{"-I"}, args...)...)
	ensureFilteredEnv(cmd)
	SetupCmd(cmd)
	return cmd
}

func trustedSupervisorBootstrapPython(agentWorkspace string) string {
	_, jail := filesystemRoots(agentWorkspace)
	names := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		names = []string{"python", "python3"}
	}
	for _, name := range names {
		candidate, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		abs, err := filepath.Abs(candidate)
		if err != nil || pathHasProtectedPrefix(jail, abs) {
			continue
		}
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil || pathHasProtectedPrefix(jail, resolved) {
			continue
		}
		return abs
	}
	return ""
}

// AutoInstall is explicit operator provisioning of a separate, protected runtime.
// It never invokes pip or startup hooks from the agent's existing environment.
func installSandboxPackage(runtimeDir, agentWorkspace, backend string, logger *slog.Logger) error {
	pkg, err := llmSandboxRequirement(backend)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return err
	}
	python := GetPythonBin(runtimeDir)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	run := func(command *exec.Cmd) error {
		command.Dir = runtimeDir
		_, _, err := NewForegroundRunner(command, ForegroundOptions{Timeout: 5 * time.Minute, ScrubOutput: true}).Run(ctx)
		return err
	}
	if _, err := os.Stat(python); os.IsNotExist(err) {
		bootstrap := trustedSupervisorBootstrapPython(agentWorkspace)
		if bootstrap == "" {
			return fmt.Errorf("provision private supervisor with a trusted system Python outside the agent filesystem jail")
		}
		if err := run(supervisorPythonCommand(ctx, bootstrap, "-m", "venv", filepath.Join(runtimeDir, "venv"))); err != nil {
			return fmt.Errorf("create private supervisor venv: %w", err)
		}
	} else if err != nil {
		return err
	}
	if err := run(supervisorPythonCommand(ctx, python, "-m", "pip", "--isolated", "install", "--disable-pip-version-check", pkg)); err != nil {
		return fmt.Errorf("provision pinned supervisor package: %w", err)
	}
	logger.Info("[Sandbox] Private supervisor package provisioned", "package", pkg)
	return nil
}
