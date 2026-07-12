package tools

import (
	"fmt"
	"os/exec"
	"time"
)

func registerManagedBackgroundProcess(cmd *exec.Cmd, registry *ProcessRegistry, cleanup func()) (int, error) {
	info := &ProcessInfo{
		StartedAt: time.Now(),
		Alive:     true,
		State:     ProcessStateStarting,
	}
	cmd.Stdout = info
	cmd.Stderr = info
	SetupCmd(cmd)
	timeout := GetBackgroundTimeout()

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	info.PID = cmd.Process.Pid
	info.Process = cmd.Process
	info.State = ProcessStateRunning
	registry.Register(info)

	go superviseBackgroundProcess(cmd, info, registry, cleanup, timeout)
	return info.PID, nil
}

func superviseBackgroundProcess(cmd *exec.Cmd, info *ProcessInfo, registry *ProcessRegistry, cleanup func(), timeout time.Duration) {
	defer func() {
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
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case err = <-waitDone:
		case <-timer.C:
			timedOut = true
			KillProcessTree(info.PID)
			if info.Process != nil {
				_ = killProcess(info.Process)
			}
			select {
			case err = <-waitDone:
			case <-time.After(10 * time.Second):
				err = fmt.Errorf("process %d refused to die after kill", info.PID)
			}
		}
	} else {
		err = <-waitDone
	}

	var systemMessage string
	info.mu.Lock()
	info.Alive = false
	info.TerminatedAt = time.Now()

	if info.State == ProcessStateTerminated {
		// Terminate already recorded the authoritative state.
	} else if timedOut {
		info.State = ProcessStateTimedOut
		info.TimedOut = true
		info.ErrorReason = fmt.Sprintf("process exceeded background timeout of %s", timeout)
		systemMessage = fmt.Sprintf("[process terminated after exceeding background timeout of %s]", timeout)
	} else if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			info.ExitCode = exitErr.ExitCode()
		}
		info.State = ProcessStateCrashed
		info.ErrorReason = err.Error()
		systemMessage = fmt.Sprintf("[process exited with error: %v]", err)
	} else {
		info.State = ProcessStateExited
	}
	info.mu.Unlock()
	if systemMessage != "" {
		_ = info.WriteSystemMessage(systemMessage)
	}
	registry.pruneCompleted(time.Now())
}
