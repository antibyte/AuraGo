//go:build !windows

package sudoticket

import (
	"os/exec"
	"syscall"
)

// setProcessGroup matches tools.SetupCmd: the sudo process gets its own
// process group.
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}
