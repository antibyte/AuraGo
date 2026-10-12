//go:build !windows

package ruview

import (
	"os/exec"
	"syscall"
)

func launchNative(cmd *exec.Cmd) (*nativeProcess, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &nativeProcess{cmd: cmd, done: make(chan error, 1), kill: func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }}
	go func() { p.done <- cmd.Wait() }()
	return p, nil
}
