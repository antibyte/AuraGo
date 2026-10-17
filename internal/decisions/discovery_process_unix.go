//go:build !windows

package decisions

import (
	"os/exec"
	"syscall"
)

func runDiscoveryCommand(command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
	if err := command.Start(); err != nil {
		return err
	}
	defer syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	return command.Wait()
}
