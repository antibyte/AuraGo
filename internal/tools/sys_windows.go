//go:build windows

package tools

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// SetupCmd applies Windows-specific process attributes.
func SetupCmd(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}

// SetSkillLimits is a no-op on Windows. Resource limits are enforced via
// context timeout only; Windows Job Objects would require cgo or external tools.
func SetSkillLimits(cmd *exec.Cmd, memoryMB, cpuSeconds int) {
	// No-op: Windows does not support POSIX rlimits.
	// The context-based timeout still applies.
}

// ApplySkillLimits is a no-op on Windows.
func ApplySkillLimits(pid, memoryMB, cpuSeconds int) {
	// No-op: Windows does not support POSIX rlimits.
}

// signalZero is a placeholder on Windows — Signal(0) is not supported.
// daemon_runner.go uses os.FindProcess instead on Windows.
var signalZero = syscall.Signal(0)

// KillProcessTree forcefully terminates a process and all its children on Windows.
// Uses taskkill /F /T to traverse and kill the full process subtree.
func KillProcessTree(pid int) {
	_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}

// KillProcessTreeGraceful performs a two-stage process termination on Windows:
// Stage 1: taskkill /T (terminate tree, no /F) - gives processes a chance to clean up
// Stage 2: taskkill /F /T (force) if still alive after grace period
// Default grace period is 2 seconds.
func KillProcessTreeGraceful(pid int, gracePeriodSeconds int) {
	if gracePeriodSeconds <= 0 {
		gracePeriodSeconds = 2
	}

	// Stage 1: Graceful termination (no /F flag)
	err := exec.Command("taskkill", "/T", "/PID", strconv.Itoa(pid)).Run()
	if err == nil {
		// Wait for process to die gracefully
		deadline := time.Now().Add(time.Duration(gracePeriodSeconds) * time.Second)
		for time.Now().Before(deadline) {
			// Check if process still exists
			proc, err := os.FindProcess(pid)
			if err != nil || proc == nil {
				return
			}
			// On Windows, we can't easily check if process is still running
			// without a syscall. Try to get the exit code - if process is
			// gone, this will fail.
			_, err = proc.Wait()
			if err != nil {
				// Process exited (or error - either way it's gone)
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Stage 2: Force kill
	_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}
