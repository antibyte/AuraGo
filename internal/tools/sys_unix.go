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

// KillProcessTree kills the entire process group rooted at pid on Unix.
func KillProcessTree(pid int) {
	// Negative PID sends signal to the whole process group.
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
