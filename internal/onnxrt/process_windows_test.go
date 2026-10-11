//go:build windows

package onnxrt

import (
	"os/exec"
	"syscall"
	"testing"
)

const testCreateNoWindow = 0x08000000

func TestConfigureWorkerCommandHidesWindow(t *testing.T) {
	command := exec.Command("aurago-test-worker")
	ConfigureWorkerCommand(command, t.TempDir())
	if command.SysProcAttr == nil || !command.SysProcAttr.HideWindow || command.SysProcAttr.CreationFlags&testCreateNoWindow == 0 {
		t.Fatalf("SysProcAttr = %+v", command.SysProcAttr)
	}
}

func TestConfigureWorkerCommandWithoutRootsHidesWindow(t *testing.T) {
	command := exec.Command("aurago-test-worker")
	ConfigureWorkerCommand(command)
	if command.SysProcAttr == nil || !command.SysProcAttr.HideWindow || command.SysProcAttr.CreationFlags&testCreateNoWindow == 0 {
		t.Fatalf("SysProcAttr = %+v", command.SysProcAttr)
	}
}

// A caller's own process attributes survive; the window flags are added.
func TestConfigureWorkerCommandKeepsCallerProcessAttributes(t *testing.T) {
	command := exec.Command("aurago-test-worker")
	attributes := &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		CmdLine:       "aurago-test-worker --flag",
	}
	command.SysProcAttr = attributes
	ConfigureWorkerCommand(command, t.TempDir())
	if command.SysProcAttr != attributes {
		t.Fatal("SysProcAttr was replaced instead of merged")
	}
	if !attributes.HideWindow || attributes.CreationFlags != syscall.CREATE_NEW_PROCESS_GROUP|testCreateNoWindow {
		t.Fatalf("SysProcAttr = %+v", attributes)
	}
	if attributes.CmdLine != "aurago-test-worker --flag" {
		t.Fatalf("CmdLine = %q", attributes.CmdLine)
	}
}
