//go:build !windows

package tools

import (
	"os/exec"
	"syscall"
	"time"
)

// SetupCmd sets process group on Unix so all children share the same PGID,
// enabling reliable whole-tree termination via KillProcessTree.
func SetupCmd(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// SetSkillLimits configures the command for process-group management on Unix.
// Actual resource limits are applied after process start via ApplySkillLimits.
func SetSkillLimits(cmd *exec.Cmd, memoryMB, cpuSeconds int) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// signalZero is used by daemon health checks — Signal(0) checks process existence without an actual signal.
var signalZero = syscall.Signal(0)

// KillProcessTree kills the entire process group rooted at pid on Unix.
func KillProcessTree(pid int) {
	// Negative PID sends signal to the whole process group.
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}

// KillProcessTreeGraceful performs a two-stage process termination:
// Stage 1: Send SIGTERM to allow graceful shutdown (default 2 second grace period)
// Stage 2: Send SIGKILL if process still alive
// This gives processes a chance to clean up temp files, flush buffers, etc.
func KillProcessTreeGraceful(pid int, gracePeriodSeconds int) {
	if gracePeriodSeconds <= 0 {
		gracePeriodSeconds = 2
	}

	// Stage 1: SIGTERM (graceful)
	_ = syscall.Kill(-pid, syscall.SIGTERM)

	// Wait for graceful shutdown with polling
	deadline := time.Now().Add(time.Duration(gracePeriodSeconds) * time.Second)
	for time.Now().Before(deadline) {
		// Check if process is still alive by sending signal 0
		err := syscall.Kill(pid, syscall.Signal(0))
		if err != nil {
			// ESRCH means process doesn't exist - it died
			if err == syscall.ESRCH {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Stage 2: SIGKILL (force)
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
