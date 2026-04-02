package tools

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"aurago/internal/sandbox"
)

var sudoPasswordPromptPattern = regexp.MustCompile(`^\[sudo\][^:\r\n]*:\s*`)

// ExecuteShell runs a command in the shell (PS on Windows, sh on Unix) and returns stdout/stderr.
// Uses a manual timer + KillProcessTree to reliably terminate the full process subtree on timeout,
// avoiding the Windows issue where exec.CommandContext only kills the parent shell but not grandchildren
// (e.g., an ssh process spawned by powershell that holds pipes open indefinitely).
func ExecuteShell(command, workspaceDir string) (string, string, error) {
	var cmd *exec.Cmd
	absWorkDir := getAbsWorkspace(workspaceDir)

	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	} else if sb := sandbox.Get(); sb.Available() {
		cmd = sb.PrepareCommand(command, absWorkDir)
		slog.Debug("[ExecuteShell] using sandbox", "backend", sb.Name())
	} else {
		cmd = exec.Command("/bin/sh", "-c", command)
	}

	cmd.Dir = absWorkDir
	SetupCmd(cmd)

	slog.Debug("[ExecuteShell]", "command", command, "dir", cmd.Dir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(GetForegroundTimeout())
	defer timer.Stop()

	select {
	case err := <-done:
		return stdout.String(), stderr.String(), err
	case <-timer.C:
		KillProcessTree(cmd.Process.Pid)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
		return stdout.String(), stderr.String(), fmt.Errorf("TIMEOUT: shell command exceeded %s limit", GetForegroundTimeout())
	}
}

// ExecuteShellBackground starts a command in the shell in the background and registers it.
func ExecuteShellBackground(command, workspaceDir string, registry *ProcessRegistry) (int, error) {
	var cmd *exec.Cmd
	absWorkDir := getAbsWorkspace(workspaceDir)

	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	} else if sb := sandbox.Get(); sb.Available() {
		cmd = sb.PrepareCommand(command, absWorkDir)
		slog.Debug("[ExecuteShellBackground] using sandbox", "backend", sb.Name())
	} else {
		cmd = exec.Command("/bin/sh", "-c", command)
	}

	cmd.Dir = absWorkDir
	SetupCmd(cmd)

	slog.Debug("[ExecuteShellBackground]", "command", command, "dir", cmd.Dir)

	pid, err := registerManagedBackgroundProcess(cmd, registry, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to start background shell process: %w", err)
	}
	return pid, nil
}

// ExecuteSudo runs a command via `sudo -S` (reads password from stdin) on Unix,
// returning stdout, stderr, and any execution or timeout error.
// On Windows this is a no-op and returns an unsupported error.
func ExecuteSudo(command, workspaceDir, password string) (string, string, error) {
	if runtime.GOOS == "windows" {
		return "", "", fmt.Errorf("execute_sudo is not supported on Windows")
	}

	// sudo -S reads the password from stdin; -n would fail if a password is needed.
	// We also suppress the interactive password prompt so stderr stays stable across locales.
	cmd := exec.Command("sudo", "-S", "-p", "", "/bin/sh", "-c", command)
	cmd.Dir = getAbsWorkspace(workspaceDir)
	cmd.Stdin = strings.NewReader(password + "\n")
	SetupCmd(cmd)

	slog.Debug("[ExecuteSudo]", "command", command, "dir", cmd.Dir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(GetForegroundTimeout())
	defer timer.Stop()

	select {
	case err := <-done:
		stderrStr := normalizeSudoStderr(stderr.String())
		return stdout.String(), stderrStr, err
	case <-timer.C:
		KillProcessTree(cmd.Process.Pid)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
		return stdout.String(), stderr.String(), fmt.Errorf("TIMEOUT: sudo command exceeded %s limit", GetForegroundTimeout())
	}
}

func normalizeSudoStderr(stderr string) string {
	trimmed := strings.TrimSpace(stderr)
	if trimmed == "" {
		return ""
	}
	return strings.TrimSpace(sudoPasswordPromptPattern.ReplaceAllString(trimmed, ""))
}
