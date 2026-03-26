//go:build windows

package tools

import (
	"os/exec"
	"strconv"
	"syscall"
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

// KillProcessTree forcefully terminates a process and all its children on Windows.
// Uses taskkill /F /T to traverse and kill the full process subtree.
func KillProcessTree(pid int) {
	_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}
