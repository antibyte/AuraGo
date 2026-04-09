package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

func registerManagedBackgroundProcess(cmd *exec.Cmd, registry *ProcessRegistry, cleanup func()) (int, error) {
	info := &ProcessInfo{
		Output:    &bytes.Buffer{},
		StartedAt: time.Now(),
		Alive:     true,
		State:     ProcessStateStarting,
	}
	cmd.Stdout = info
	cmd.Stderr = info

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	info.PID = cmd.Process.Pid
	info.Process = cmd.Process
	info.State = ProcessStateRunning
	registry.Register(info)

	go superviseBackgroundProcess(cmd, info, registry, cleanup)
	return info.PID, nil
}

func superviseBackgroundProcess(cmd *exec.Cmd, info *ProcessInfo, registry *ProcessRegistry, cleanup func()) {
	defer func() {
		registry.Remove(info.PID)
		if cleanup != nil {
			cleanup()
		}
	}()

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	var err error
	timedOut := false
	timeout := GetBackgroundTimeout()
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case err = <-waitDone:
		case <-timer.C:
			timedOut = true
			KillProcessTree(info.PID)
			select {
			case err = <-waitDone:
			case <-time.After(10 * time.Second):
				err = fmt.Errorf("process %d refused to die after kill", info.PID)
			}
		}
	} else {
		err = <-waitDone
	}

	info.mu.Lock()
	defer info.mu.Unlock()
	info.Alive = false
	info.TerminatedAt = time.Now()

	if timedOut {
		info.State = ProcessStateTimedOut
		info.TimedOut = true
		info.ErrorReason = fmt.Sprintf("process exceeded background timeout of %s", timeout)
		info.Output.WriteString(fmt.Sprintf("\n[process terminated after exceeding background timeout of %s]", timeout))
	} else if err != nil {
		// Extract exit code if available
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				info.ExitCode = status.ExitStatus()
			}
		}
		info.State = ProcessStateCrashed
		info.ErrorReason = err.Error()
		info.Output.WriteString(fmt.Sprintf("\n[process exited with error: %v]", err))
	} else {
		// Normal exit
		info.State = ProcessStateExited
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				info.ExitCode = status.ExitStatus()
			}
		}
	}
}
