//go:build !windows

package decisions

import (
	"os/exec"
	"sync"
	"syscall"
	"time"
)

func startRuntimeCommand(command *exec.Cmd) (func(), error) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.Setpgid = true
	command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
	command.WaitDelay = 2 * time.Second
	if err := command.Start(); err != nil {
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(func() { _ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }) }, nil
}
