package tools

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"aurago/internal/sandbox"
)

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

	timer := time.NewTimer(ForegroundTimeout)
	defer timer.Stop()

	select {
	case err := <-done:
		return stdout.String(), stderr.String(), err
	case <-timer.C:
		KillProcessTree(cmd.Process.Pid)
		<-done // drain so the goroutine exits cleanly
		return stdout.String(), stderr.String(), fmt.Errorf("TIMEOUT: shell command exceeded %s limit", ForegroundTimeout)
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

	info := &ProcessInfo{
		Output:    &bytes.Buffer{},
		StartedAt: time.Now(),
		Alive:     true,
	}

	cmd.Stdout = info
	cmd.Stderr = info

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start background shell process: %w", err)
	}

	info.PID = cmd.Process.Pid
	info.Process = cmd.Process
	registry.Register(info)

	go func() {
		err := cmd.Wait()
		info.mu.Lock()
		info.Alive = false
		if err != nil {
			info.Output.WriteString(fmt.Sprintf("\n[process exited with error: %v]", err))
		}
		info.mu.Unlock()
		registry.Remove(info.PID)
	}()

	return info.PID, nil
}

// ExecuteSudo runs a command via `sudo -S` (reads password from stdin) on Unix,
// returning stdout, stderr, and any execution or timeout error.
// On Windows this is a no-op and returns an unsupported error.
func ExecuteSudo(command, workspaceDir, password string) (string, string, error) {
	if runtime.GOOS == "windows" {
		return "", "", fmt.Errorf("execute_sudo is not supported on Windows")
	}

	// sudo -S reads the password from stdin; -n would fail if a password is needed.
	// We pass the password followed by a newline.
	cmd := exec.Command("sudo", "-S", "/bin/sh", "-c", command)
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

	timer := time.NewTimer(ForegroundTimeout)
	defer timer.Stop()

	select {
	case err := <-done:
		// sudo writes the password prompt to stderr; strip it from the output
		stderrStr := strings.TrimPrefix(stderr.String(), "[sudo] password for "+"\n")
		stderrStr = strings.TrimSpace(stderrStr)
		return stdout.String(), stderrStr, err
	case <-timer.C:
		KillProcessTree(cmd.Process.Pid)
		<-done
		return stdout.String(), stderr.String(), fmt.Errorf("TIMEOUT: sudo command exceeded %s limit", ForegroundTimeout)
	}
}
