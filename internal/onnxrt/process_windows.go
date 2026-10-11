//go:build windows

package onnxrt

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps native workers from opening a console window
// (CREATE_NO_WINDOW).
func hideWindow(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
}
