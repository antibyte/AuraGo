//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

const EXE_SUFFIX = ""

func prepareCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	return cmd
}
