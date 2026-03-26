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

// SetSkillLimits applies resource constraints (memory, CPU) for skill execution
// on Unix via POSIX rlimits. This prevents runaway skills from consuming all
// system resources during their execution window.
func SetSkillLimits(cmd *exec.Cmd, memoryMB, cpuSeconds int) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true

	if memoryMB <= 0 {
		memoryMB = 1024 // default 1 GB
	}
	if cpuSeconds <= 0 {
		cpuSeconds = 120 // default 2 minutes
	}

	memBytes := uint64(memoryMB) * 1024 * 1024
	cmd.SysProcAttr.Rlimit = []syscall.Rlimit{
		{Resource: syscall.RLIMIT_AS, Cur: memBytes, Max: memBytes},
		{Resource: syscall.RLIMIT_CPU, Cur: uint64(cpuSeconds), Max: uint64(cpuSeconds)},
		{Resource: syscall.RLIMIT_NPROC, Cur: 50, Max: 50},
	}
}

// KillProcessTree kills the entire process group rooted at pid on Unix.
func KillProcessTree(pid int) {
	// Negative PID sends signal to the whole process group.
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
