//go:build windows

package sudoticket

import (
	"os/exec"
	"syscall"
)

// setProcessGroup matches tools.SetupCmd on Windows, where sudo does not
// exist and Acquire always fails to start it.
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}
