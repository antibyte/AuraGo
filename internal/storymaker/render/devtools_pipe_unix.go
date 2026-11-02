//go:build !windows

package render

import (
	"os"
	"os/exec"
	"syscall"
)

// attachDevToolsPipe gives the browser its ends of the pipe as file descriptors 3 (commands in) and 4
// (answers out), which is where --remote-debugging-pipe looks, and puts it in its own process group so
// that a kill reaches its helper processes.
func attachDevToolsPipe(cmd *exec.Cmd, in, out *os.File) error {
	cmd.ExtraFiles = []*os.File{in, out}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

// killProcessTree kills the browser and its process group.
func killProcessTree(p *os.Process) {
	if p == nil {
		return
	}
	_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
	_ = p.Kill()
}
