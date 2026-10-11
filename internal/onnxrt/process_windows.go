//go:build windows

package onnxrt

import (
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW.
const createNoWindow = 0x08000000

// hideWindow keeps native workers from opening a console window. It adds to
// the caller's SysProcAttr instead of replacing it.
func hideWindow(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.HideWindow = true
	command.SysProcAttr.CreationFlags |= createNoWindow
}
