//go:build !windows

package tools

import (
	"os/exec"
	"syscall"
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
