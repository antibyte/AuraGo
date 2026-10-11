//go:build windows

package onnxrt

import (
	"os/exec"
	"testing"
)

func TestConfigureWorkerCommandHidesWindow(t *testing.T) {
	command := exec.Command("aurago-test-worker")
	ConfigureWorkerCommand(command, t.TempDir())
	if command.SysProcAttr == nil || !command.SysProcAttr.HideWindow || command.SysProcAttr.CreationFlags&0x08000000 == 0 {
		t.Fatalf("SysProcAttr = %+v", command.SysProcAttr)
	}
}

func TestConfigureWorkerCommandWithoutRootsHidesWindow(t *testing.T) {
	command := exec.Command("aurago-test-worker")
	ConfigureWorkerCommand(command)
	if command.SysProcAttr == nil || !command.SysProcAttr.HideWindow || command.SysProcAttr.CreationFlags&0x08000000 == 0 {
		t.Fatalf("SysProcAttr = %+v", command.SysProcAttr)
	}
}
