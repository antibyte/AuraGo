//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

const EXE_SUFFIX = ".exe"

func prepareCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008, // 0x00000008 is DETACHED_PROCESS
	}
	return cmd
}
