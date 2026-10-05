package networkshares

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"aurago/internal/sudoticket"
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
	cmd, viaSudo, err := newRunnerCommand(ctx, options, privileged, name, args, stdin)
	if err != nil {
		return nil, err
	}
	lease, authOut, err := acquireSudoTicket(ctx, options, viaSudo)
	if err != nil {
		if message := limitMessage(authOut); message != "" {
			return nil, fmt.Errorf("%s failed: %w: %s", name, err, message)
		}
		return nil, fmt.Errorf("%s failed: %w", name, err)
	}
	defer lease.Release()
	var stdout, stderr cappedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := limitMessage(stderr.String())
		if message == "" {
			message = limitMessage(stdout.String())
		}
		if message != "" {
			return nil, lease.Explain(fmt.Errorf("%s failed: %s", name, message))
		}
		return nil, lease.Explain(fmt.Errorf("%s failed: %w", name, err))
	}
	return stdout.Bytes(), nil
}

// newRunnerCommand builds the process for one runner call: the platform
// decides about elevation, and stdin is the platform's command input. viaSudo
// reports a `sudo -n` command (Linux, see platformCommand).
func newRunnerCommand(ctx context.Context, options Options, privileged bool, name string, args []string, stdin []byte) (*exec.Cmd, bool, error) {
	commandName, commandArgs, commandInput, viaSudo, err := platformCommand(options, privileged, name, args, stdin)
	if err != nil {
		return nil, false, err
	}
	cmd := exec.CommandContext(ctx, commandName, commandArgs...)
	if len(commandInput) > 0 {
		cmd.Stdin = bytes.NewReader(commandInput)
	}
	return cmd, viaSudo, nil
}

// acquireSudoTicket holds the shared sudo ticket for a `sudo -n` run when a
// Vault password is configured; sudoticket.Acquire validates the password only
// when `sudo -n true` does not already work. Other commands, and sudo without
// a password, get a nil lease: sudo -n then relies on root rules or NOPASSWD.
func acquireSudoTicket(ctx context.Context, options Options, viaSudo bool) (*sudoticket.Lease, string, error) {
	if !viaSudo || options.SudoPassword == "" {
		return nil, "", nil
	}
	return sudoticket.Acquire(ctx, "", options.SudoPassword)
}

// limitMessage trims command output for an error message.
func limitMessage(output string) string {
	message := strings.TrimSpace(output)
	if len(message) > 400 {
		message = message[:400]
	}
	return message
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
