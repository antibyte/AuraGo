package tools

import (
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"aurago/internal/sandbox"
)

// shellKillWait is the time to wait after kill before giving up (for shell).
const shellKillWait = 8 * time.Second

var sudoPasswordPromptPattern = regexp.MustCompile(`^\[sudo\][^:\r\n]*:\s*`)

// Security notes for shell execution:
//
// Shell command execution is an intentional, core agent capability.
// The primary security layer is the sandbox (Landlock on Linux) which restricts
// filesystem and process operations. Without a sandbox, shell access is effectively
// root-equivalent on the host — this is by design for a home-lab autonomous agent.
//
// Hardening strategies applied:
//   - Sandbox is used automatically when available (Linux with Landlock).
//   - Workspace directory is restricted and enforced via getAbsWorkspace.
//   - All processes are killed on timeout via KillProcessTree.
//   - Bounded stdout/stderr buffers prevent memory exhaustion.
//   - PowerShell on Windows runs with -NoProfile -NonInteractive.
//
// Residual risks:
//   - On Windows and non-Landlock Linux: sandbox unavailable, full shell access.
//   - The allow_shell config toggle controls whether shell execution is permitted.
//   - Users must trust the LLM provider when shell is enabled.
//
// If you need stricter isolation, consider:
//   - Running AuraGo inside a Docker container with appropriate capabilities dropped.
//   - Using landlock-based sandbox on Linux (requires kernel >= 5.13).

// ExecuteShell runs a command in the shell (PS on Windows, sh on Unix) and returns stdout/stderr.
// Uses a manual timer + KillProcessTree to reliably terminate the full process subtree on timeout,
// avoiding the Windows issue where exec.CommandContext only kills the parent shell but not grandchildren
// (e.g., an ssh process spawned by powershell that holds pipes open indefinitely).
func ExecuteShell(command, workspaceDir string) (string, string, error) {
	var cmd *exec.Cmd
	absWorkDir := getAbsWorkspace(workspaceDir)

	if runtime.GOOS == "windows" {
		slog.Warn("Shell execution is running WITHOUT sandbox protection on Windows. Commands execute with full user privileges.")
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

	runner := NewForegroundRunner(cmd, ForegroundOptions{
		Timeout:  GetForegroundTimeout(),
		Graceful: true,
		KillWait: shellKillWait,
		ErrMsg:   "TIMEOUT: shell command exceeded %s limit",
	})

	return runner.Run()
}

// ExecuteShellBackground starts a command in the shell in the background and registers it.
func ExecuteShellBackground(command, workspaceDir string, registry *ProcessRegistry) (int, error) {
	var cmd *exec.Cmd
	absWorkDir := getAbsWorkspace(workspaceDir)

	if runtime.GOOS == "windows" {
		slog.Warn("Shell execution is running WITHOUT sandbox protection on Windows. Commands execute with full user privileges.")
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

	runner := NewBackgroundRunner(cmd, BackgroundOptions{
		Registry: registry,
	})

	pid, err := runner.Run()
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

	runner := NewForegroundRunner(cmd, ForegroundOptions{
		Timeout:  GetForegroundTimeout(),
		Graceful: true,
		KillWait: shellKillWait,
		ErrMsg:   "TIMEOUT: sudo command exceeded %s limit",
	})

	stdout, stderr, err := runner.Run()
	if err != nil {
		// Apply sudo-specific stderr normalization for password prompt
		stderr = normalizeSudoStderr(stderr)
	}
	return stdout, stderr, err
}

func normalizeSudoStderr(stderr string) string {
	trimmed := strings.TrimSpace(stderr)
	if trimmed == "" {
		return ""
	}
	return strings.TrimSpace(sudoPasswordPromptPattern.ReplaceAllString(trimmed, ""))
}
