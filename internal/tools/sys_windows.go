//go:build windows

package tools

import (
	"errors"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// SetupCmd applies Windows-specific process attributes.
func SetupCmd(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}

// SetSkillLimits is a no-op on Windows. Resource limits are enforced via
// context timeout only; Windows Job Objects would require cgo or external tools.
func SetSkillLimits(cmd *exec.Cmd, memoryMB, cpuSeconds int) {
	// No-op: Windows does not support POSIX rlimits.
	// The context-based timeout still applies.
}

// ApplySkillLimits is a no-op on Windows.
func ApplySkillLimits(pid, memoryMB, cpuSeconds int) {
	// No-op: Windows does not support POSIX rlimits.
}

// signalZero is a placeholder on Windows — Signal(0) is not supported.
// daemon_runner.go uses os.FindProcess instead on Windows.
var signalZero = syscall.Signal(0)

// KillProcessTree forcefully terminates a process and all its children on Windows.
// Uses taskkill /F /T to traverse and kill the full process subtree.
func KillProcessTree(pid int) {
	_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}

// KillProcessTreeGraceful performs a two-stage process termination on Windows:
// Stage 1: taskkill /T (terminate tree, no /F) - gives processes a chance to clean up
// Stage 2: taskkill /F /T (force) if still alive after grace period
// Default grace period is 2 seconds.
// The grace wait watches the process handle opened at this call. A later OpenProcess
// on the same PID could see a recycled process. os.Process.Wait would block until
// exit, ignore the grace deadline, and race with the runner's own Wait.
func KillProcessTreeGraceful(pid int, gracePeriodSeconds int) {
	if gracePeriodSeconds <= 0 {
		gracePeriodSeconds = 2
	}
	if pid <= 0 {
		return
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// The PID is already gone. Do not taskkill: the number may belong to
		// someone else. Any other open failure cannot be polled, so force-kill.
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return
		}
		_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
		return
	}
	defer windows.CloseHandle(handle)
	if !windowsHandleRunning(handle) {
		return
	}

	// A rejected graceful taskkill (typical for a console process) falls through
	// to the force kill when this handle is still active. An accepted one waits
	// out the grace period first.
	err = exec.Command("taskkill", "/T", "/PID", strconv.Itoa(pid)).Run()
	if err == nil {
		deadline := time.Now().Add(time.Duration(gracePeriodSeconds) * time.Second)
		for time.Now().Before(deadline) {
			if !windowsHandleRunning(handle) {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	} else if !windowsHandleRunning(handle) {
		return
	}

	_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}

// windowsHandleRunning reports whether the process opened at the call is still
// active. GetExitCodeProcess on that handle stays bound to the original process
// after the PID is reused. 259 is the Win32 STILL_ACTIVE code; x/sys/windows
// does not export it. A query error counts as running so the force kill still happens.
func windowsHandleRunning(handle windows.Handle) bool {
	var code uint32
	if err := windows.GetExitCodeProcess(handle, &code); err != nil {
		return true
	}
	const stillActive uint32 = 259
	return code == stillActive
}

// applyDaemonLimits is a no-op on Windows (no POSIX rlimits).
func applyDaemonLimits(pid, memoryMB int) {}

// killDaemonProcessGroup is a no-op on Windows: there is no process group and
// PIDs are reused at once, so taskkill on an exited daemon PID could hit an
// unrelated process tree.
func killDaemonProcessGroup(pid int) {}
