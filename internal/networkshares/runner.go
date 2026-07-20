package networkshares

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const maxCommandOutputBytes = 512 * 1024

type commandRunner interface {
	Run(ctx context.Context, options Options, privileged bool, name string, args []string, stdin []byte) ([]byte, error)
	LookPath(file string) (string, error)
}

type execCommandRunner struct{}

func (execCommandRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (execCommandRunner) Run(ctx context.Context, options Options, privileged bool, name string, args []string, stdin []byte) ([]byte, error) {
	commandName, commandArgs, commandInput, err := platformCommand(options, privileged, name, args, stdin)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, commandName, commandArgs...)
	if len(commandInput) > 0 {
		cmd.Stdin = bytes.NewReader(commandInput)
	}
	var stdout, stderr cappedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if len(message) > 400 {
			message = message[:400]
		}
		if message != "" {
			return nil, fmt.Errorf("%s failed: %s", name, message)
		}
		return nil, fmt.Errorf("%s failed: %w", name, err)
	}
	return stdout.Bytes(), nil
}

type cappedBuffer struct {
	data []byte
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := maxCommandOutputBytes - len(b.data)
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}

func (b *cappedBuffer) Bytes() []byte {
	return append([]byte(nil), b.data...)
}

func (b *cappedBuffer) String() string {
	return string(b.data)
}
