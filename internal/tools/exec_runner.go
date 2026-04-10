package tools

import (
	"fmt"
	"os/exec"
	"time"
)

// ForegroundOptions contains options for foreground execution.
type ForegroundOptions struct {
	// Timeout is the maximum duration before the process is killed.
	Timeout time.Duration
	// Graceful indicates whether to use SIGTERM-first kill (true for shell).
	Graceful bool
	// ScrubOutput indicates whether to apply secret scrubbing to stdout/stderr.
	ScrubOutput bool
	// KillWait is how long to wait after kill before giving up.
	KillWait time.Duration
	// ErrMsg is the error message template; %s is replaced with the timeout value.
	// If empty, a default message is used.
	ErrMsg string
}

// ForegroundRunner handles the common foreground execution pattern:
// bounded stdout/stderr buffers, process start, goroutine wait, timeout kill,
// and optional secret scrubbing.
type ForegroundRunner struct {
	cmd    *exec.Cmd
	stdout *BoundedBuffer
	stderr *BoundedBuffer
	opts   ForegroundOptions
}

// NewForegroundRunner creates a new foreground execution runner.
func NewForegroundRunner(cmd *exec.Cmd, opts ForegroundOptions) *ForegroundRunner {
	if opts.Timeout <= 0 {
		opts.Timeout = GetForegroundTimeout()
	}
	if opts.KillWait <= 0 {
		// Default kill wait: 8s for shell (graceful), 10s for python
		opts.KillWait = 10 * time.Second
	}
	return &ForegroundRunner{
		cmd:    cmd,
		stdout: NewBoundedBuffer(1024 * 1024),
		stderr: NewBoundedBuffer(1024 * 1024),
		opts:   opts,
	}
}

// Run executes the command and waits for completion or timeout.
// Returns stdout, stderr, and any error (including timeout error).
func (r *ForegroundRunner) Run() (string, string, error) {
	r.cmd.Stdout = r.stdout
	r.cmd.Stderr = r.stderr

	if err := r.cmd.Start(); err != nil {
		return "", "", err
	}

	done := make(chan error, 1)
	go func() { done <- r.cmd.Wait() }()

	timer := time.NewTimer(r.opts.Timeout)
	defer timer.Stop()

	select {
	case err := <-done:
		if r.opts.ScrubOutput {
			out, errOut := ScrubSecretOutput(r.stdout.String(), r.stderr.String())
			return out, errOut, err
		}
		return r.stdout.String(), r.stderr.String(), err
	case <-timer.C:
		if r.opts.Graceful {
			KillProcessTreeGraceful(r.cmd.Process.Pid, 2)
		} else {
			KillProcessTree(r.cmd.Process.Pid)
		}
		select {
		case <-done:
		case <-time.After(r.opts.KillWait):
		}
		errMsg := r.opts.ErrMsg
		if errMsg == "" {
			errMsg = "TIMEOUT: command exceeded %s limit and was killed"
		}
		timeoutErr := fmt.Errorf(errMsg, r.opts.Timeout)
		if r.opts.ScrubOutput {
			out, errOut := ScrubSecretOutput(r.stdout.String(), r.stderr.String())
			return out, errOut, timeoutErr
		}
		return r.stdout.String(), r.stderr.String(), timeoutErr
	}
}

// BackgroundOptions contains options for background execution.
type BackgroundOptions struct {
	Registry *ProcessRegistry
	Cleanup  func() // Optional cleanup function called when process terminates.
}

// BackgroundRunner handles the common background execution pattern
// via registerManagedBackgroundProcess.
type BackgroundRunner struct {
	cmd  *exec.Cmd
	opts BackgroundOptions
}

// NewBackgroundRunner creates a new background execution runner.
func NewBackgroundRunner(cmd *exec.Cmd, opts BackgroundOptions) *BackgroundRunner {
	return &BackgroundRunner{
		cmd:  cmd,
		opts: opts,
	}
}

// Run starts the command in the background and registers it.
// Returns the PID of the started process.
func (r *BackgroundRunner) Run() (int, error) {
	return registerManagedBackgroundProcess(r.cmd, r.opts.Registry, r.opts.Cleanup)
}
