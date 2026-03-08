//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func attachDetachedAttributes(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000200, // CREATE_NEW_PROCESS_GROUP
		HideWindow:    true,
	}
}
