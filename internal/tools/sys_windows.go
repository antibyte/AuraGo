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

// KillProcessTree forcefully terminates a process and all its children on Windows.
// Uses taskkill /F /T to traverse and kill the full process subtree.
func KillProcessTree(pid int) {
	_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}
