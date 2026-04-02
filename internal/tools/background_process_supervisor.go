package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"time"
)

func registerManagedBackgroundProcess(cmd *exec.Cmd, registry *ProcessRegistry, cleanup func()) (int, error) {
	info := &ProcessInfo{
		Output:    &bytes.Buffer{},
		StartedAt: time.Now(),
		Alive:     true,
	}
	cmd.Stdout = info
	cmd.Stderr = info

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	info.PID = cmd.Process.Pid
	info.Process = cmd.Process
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
	info.Alive = false
	if timedOut {
		info.Output.WriteString(fmt.Sprintf("\n[process terminated after exceeding background timeout of %s]", timeout))
	}
	if err != nil {
		info.Output.WriteString(fmt.Sprintf("\n[process exited with error: %v]", err))
	}
	info.mu.Unlock()
}
