package bluetooth

import (
	"context"
	"io"
	"os/exec"
)

// headsetProcess is a running pw-record or pw-play with its pipes.
type headsetProcess interface {
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Wait() error
	Kill() error
}

// headsetRunner starts the PipeWire tools behind a HeadsetLink.
type headsetRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	Pipe(ctx context.Context, name string, args ...string) (headsetProcess, error)
}

type execHeadsetRunner struct{}

func (execHeadsetRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func (execHeadsetRunner) Pipe(ctx context.Context, name string, args ...string) (headsetProcess, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &execHeadsetProcess{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

type execHeadsetProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (p *execHeadsetProcess) Stdin() io.WriteCloser { return p.stdin }
func (p *execHeadsetProcess) Stdout() io.ReadCloser { return p.stdout }
func (p *execHeadsetProcess) Wait() error           { return p.cmd.Wait() }

func (p *execHeadsetProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}
