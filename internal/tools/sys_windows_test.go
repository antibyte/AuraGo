//go:build windows

package tools

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

// TestWindowsProcessHandleStaysDeadAfterExit holds the process open, kills it,
// and checks GetExitCodeProcess on that handle. A recycled PID cannot make this
// handle look active again.
func TestWindowsProcessHandleStaysDeadAfterExit(t *testing.T) {
	skipIfWindowsPowerShellUnavailable(t)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pid := cmd.Process.Pid
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("OpenProcess: %v", err)
	}
	defer windows.CloseHandle(handle)

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill: %v", err)
	}
	_ = cmd.Wait()

	if windowsHandleRunning(handle) {
		t.Fatal("held handle still reports STILL_ACTIVE (259) after the process was killed")
	}
}
