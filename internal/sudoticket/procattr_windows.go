//go:build windows

package sudoticket

import (
	"os/exec"
	"syscall"
)

// setProcessGroup matches tools.SetupCmd on Windows. Nothing on Windows calls
// Acquire; Windows 11 24H2 ships an optional sudo.exe with different flags.
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}
