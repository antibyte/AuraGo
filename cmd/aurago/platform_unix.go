//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func attachDetachedAttributes(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}
