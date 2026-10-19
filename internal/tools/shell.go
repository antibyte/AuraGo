package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"aurago/internal/sandbox"
	"aurago/internal/security"
)

// shellKillWait is the time to wait after kill before giving up (for shell).
const shellKillWait = 8 * time.Second

// shellSandboxDockerHost is the configured Docker endpoint published at
// startup. It is not a global Landlock rule.
var shellSandboxDockerHost atomic.Value

// SetShellSandboxDockerHost records the Docker host commands may receive.
// An empty host disables the grant.
func SetShellSandboxDockerHost(host string) {
	shellSandboxDockerHost.Store(host)
}

func currentShellSandboxDockerHost() string {
	host, _ := shellSandboxDockerHost.Load().(string)
	return host
}

// ApplyShellSandboxDockerGrant appends DOCKER_HOST when Docker permission
// allows it and the command already has an environment. A unix:// host also
// appends its socket to AURAGO_SBX_RW and AURAGO_SBX_RW_REQUIRED. A nil
// environment stays nil so a later ensureFilteredShellEnv can install the
// filtered process environment. A tcp host sets DOCKER_HOST and adds no
// filesystem path.
func ApplyShellSandboxDockerGrant(cmd *exec.Cmd) {
	if cmd == nil || cmd.Env == nil {
		return
	}
	host := currentShellSandboxDockerHost()
	if host == "" || requireDockerPermission() != nil {
		return
	}
	if !shellEnvHasKey(cmd.Env, "DOCKER_HOST") {
		cmd.Env = append(cmd.Env, "DOCKER_HOST="+host)
	}
	cmd.Env = sandbox.AddUnixSocketGrant(cmd.Env)
}

func shellEnvHasKey(env []string, key string) bool {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return true
		}
	}
	return false
}

// Security notes for shell execution:
//
// Shell command execution is an intentional, core agent capability.
// When explicitly enabled, the shell sandbox uses Landlock on Linux to restrict
// filesystem access and applies process resource limits. Without isolation,
// commands inherit the AuraGo process user's permissions, subject to other guards.
//
// Hardening strategies applied:
//   - Landlock requires shell_sandbox.enabled and a functional Linux backend.
//   - Desktop Notes write-path checks apply within active isolation; disabled
//     isolation or explicit unsafe fallback permits otherwise authorized processes.
//   - Workspace directory is restricted and enforced via getAbsWorkspace.
//   - All processes are killed on timeout via KillProcessTree.
//   - Bounded stdout/stderr buffers prevent memory exhaustion.
//   - PowerShell on Windows runs with -NoProfile -NonInteractive.
//   - Dangerous commands are blocked via pattern matching.
//   - Unsandboxed shells get ensureFilteredShellEnv: no host secrets, and no
//     Docker client variables (DOCKER_HOST, ...) unless the Docker tool is permitted.
//
// IMPORTANT: The allow_shell config option controls whether shell execution is
// permitted. It MUST be set to false by default (the config system enforces this).
// Shell execution should only be enabled in trusted home-lab environments.
//
// Residual risks:
//   - If shell_sandbox.allow_unsafe_fallback is enabled, commands execute with host privileges.
//   - Pattern matching may not catch all variations of dangerous commands.
//   - Users must trust the LLM provider when shell is enabled.
//
// If you need stricter isolation, consider:
//   - Running AuraGo inside a Docker container with appropriate capabilities dropped.
//   - Using landlock-based sandbox on Linux (requires kernel >= 5.13).

// ExecuteShell runs a command in the shell (PS on Windows, sh on Unix) and returns stdout/stderr.
// ctx is forwarded to the foreground runner. When it ends, the process tree is killed the same
// way the runner's own timeout kills it. A nil ctx means context.Background.
// Uses a manual timer + KillProcessTree to reliably terminate the full process subtree on timeout,
// avoiding the Windows issue where exec.CommandContext only kills the parent shell but not grandchildren
// (e.g., an ssh process spawned by powershell that holds pipes open indefinitely).
func ExecuteShell(ctx context.Context, command, workspaceDir string) (string, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if isolatedExecutionRequired(ctx, true) {
		if err := requireIsolatedPermission(ctx, true); err != nil {
			return "", "", err
		}
		if err := ValidateShellCommandPolicy(command); err != nil {
			return "", "", err
		}
		return runIsolatedWorker(ctx, isolatedShellCode(command), "python", GetForegroundTimeout())
	}
	if err := requireHostShellExecutionContext(ctx); err != nil {
		return "", "", err
	}
	// Security: Check for dangerous commands before execution
	if err := ValidateShellCommandPolicy(command); err != nil {
		slog.Warn("[ExecuteShell] blocked shell command", "reason", err.Error(), "command", command)
		return "", "", err
	}

	var cmd *exec.Cmd
	absWorkDir := getAbsWorkspace(workspaceDir)
	sb := sandbox.Get()

	if sb.Name() == "blocked" {
		cmd = sb.PrepareCommand(command, absWorkDir)
		slog.Warn("[ExecuteShell] blocked because shell sandbox is unavailable and unsafe fallback is disabled")
	} else if runtime.GOOS == "windows" {
		slog.Warn("Shell execution is running WITHOUT sandbox protection on Windows. Commands execute with full user privileges.")
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	} else if sb.Available() {
		cmd = sb.PrepareCommand(command, absWorkDir)
		slog.Debug("[ExecuteShell] using sandbox", "backend", sb.Name())
	} else {
		cmd = exec.Command("/bin/sh", "-c", command)
	}

	cmd.Dir = absWorkDir
	ApplyShellSandboxDockerGrant(cmd)
	ensureFilteredShellEnv(cmd)
	SetupCmd(cmd)

	slog.Debug("[ExecuteShell]", "command", command, "dir", cmd.Dir)

	runner := NewForegroundRunner(cmd, ForegroundOptions{
		Timeout:  GetForegroundTimeout(),
		Graceful: true,
		KillWait: shellKillWait,
		ErrMsg:   "TIMEOUT: shell command exceeded %s limit",
	})

	return runner.Run(ctx)
}

// ExecuteShellBackground starts a command in the shell in the background and registers it.
func ExecuteShellBackground(command, workspaceDir string, registry *ProcessRegistry) (int, error) {
	if err := requireHostShellExecutionContext(context.Background()); err != nil {
		return 0, err
	}
	// Security: Check for dangerous commands before execution
	if err := ValidateShellCommandPolicy(command); err != nil {
		slog.Warn("[ExecuteShellBackground] blocked shell command", "reason", err.Error(), "command", command)
		return 0, err
	}

	var cmd *exec.Cmd
	absWorkDir := getAbsWorkspace(workspaceDir)
	sb := sandbox.Get()

	if sb.Name() == "blocked" {
		cmd = sb.PrepareCommand(command, absWorkDir)
		slog.Warn("[ExecuteShellBackground] blocked because shell sandbox is unavailable and unsafe fallback is disabled")
	} else if runtime.GOOS == "windows" {
		slog.Warn("Shell execution is running WITHOUT sandbox protection on Windows. Commands execute with full user privileges.")
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	} else if sb.Available() {
		cmd = sb.PrepareCommand(command, absWorkDir)
		slog.Debug("[ExecuteShellBackground] using sandbox", "backend", sb.Name())
	} else {
		cmd = exec.Command("/bin/sh", "-c", command)
	}

	cmd.Dir = absWorkDir
	ApplyShellSandboxDockerGrant(cmd)
	ensureFilteredShellEnv(cmd)
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

// ExecuteSudo runs a command on Unix via a validated sudo timestamp; the Vault
// password is only ever given to `sudo -v`, never to the command's stdin.
// It returns stdout, stderr, and any execution or timeout error.
// On Windows this is a no-op and returns an unsupported error.
func ExecuteSudo(command, workspaceDir, password string) (string, string, error) {
	if err := requireHostShellExecutionContext(context.Background()); err != nil {
		return "", "", err
	}
	if sandbox.IsActive() {
		return "", "", fmt.Errorf("execute_sudo is disabled while shell sandbox is active; run privileged maintenance outside the sandboxed shell path")
	}
	if runtime.GOOS == "windows" {
		return "", "", fmt.Errorf("execute_sudo is not supported on Windows")
	}

	// Security: Check for dangerous commands before execution
	if err := ValidateShellCommandPolicy(command); err != nil {
		slog.Warn("[ExecuteSudo] blocked shell command", "reason", err.Error(), "command", command)
		return "", "", err
	}

	absWorkDir := getAbsWorkspace(workspaceDir)
	lease, authOut, err := withSudoTicket(password, absWorkDir)
	if err != nil {
		return "", normalizeSudoStderr(authOut), err
	}
	defer lease.Release()

	cmd := newSudoRunCommand(command, absWorkDir)
	ensureFilteredShellEnv(cmd)
	SetupCmd(cmd)

	slog.Debug("[ExecuteSudo]", "command", command, "dir", cmd.Dir)

	runner := NewForegroundRunner(cmd, ForegroundOptions{
		Timeout:  GetForegroundTimeout(),
		Graceful: true,
		KillWait: shellKillWait,
		ErrMsg:   "TIMEOUT: sudo command exceeded %s limit",
	})

	stdout, stderr, err := runner.Run(context.Background())
	stdout = security.Scrub(stdout)
	stderr = security.Scrub(stderr)
	if err != nil {
		stderr = normalizeSudoStderr(stderr)
	}
	return stdout, stderr, lease.Explain(err)
}
