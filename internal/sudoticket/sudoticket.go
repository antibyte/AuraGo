// Package sudoticket shares one sudo timestamp ("ticket") between every
// privileged call in this process, so the Vault sudo password never reaches a
// command's stdin.
//
// The password only reaches `sudo -S -v`, which runs no command and leaves a
// ticket; the command itself then runs with `sudo -n`, which refuses instead of
// prompting. sudo keys the ticket by this process (its tty, or its pid when
// there is none), so every privileged call shares one ticket. A reference count
// keeps one call's cleanup from revoking a ticket that another call has
// validated but not used yet.
//
// The package is a leaf (standard library and internal/sandbox only):
// internal/tools imports internal/virtualcomputers and internal/networkshares,
// and internal/security imports internal/config, which imports
// internal/networkshares. Callers that show sudo output scrub it themselves.
package sudoticket

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"aurago/internal/sandbox"
)

// ErrTimestampDisabled reports a sudoers policy without timestamps
// (timestamp_timeout=0): a validated password never covers the next command.
var ErrTimestampDisabled = errors.New("sudo on this host requires a password for every command (timestamp caching disabled); enable a sudo timestamp or a NOPASSWD rule for the AuraGo user")

const (
	probeTimeout = 10 * time.Second
	dropTimeout  = 10 * time.Second
)

// validateTimeout bounds the password check; tests shorten it.
var validateTimeout = 30 * time.Second

// Processes are the sudo invocations behind the ticket.
type Processes struct {
	// Validate checks the password with `sudo -S -v`, leaving a ticket.
	Validate func(ctx context.Context, dir, password string) ([]byte, error)
	// Probe reports whether `sudo -n -v` accepts the current ticket.
	Probe func(ctx context.Context, dir string) error
	// Drop invalidates the ticket with `sudo -k`.
	Drop func()
}

// ticket counts the calls that currently rely on the shared ticket. Only the
// last holder drops it.
var ticket = struct {
	sync.Mutex
	holders   int
	processes Processes
}{processes: Processes{Validate: runValidate, Probe: runProbe, Drop: runDrop}}

// ReplaceProcessesForTesting swaps the sudo invocations so tests in any
// package can exercise the ticket without running sudo. It returns a function
// that restores the previous invocations.
func ReplaceProcessesForTesting(p Processes) (restore func()) {
	ticket.Lock()
	defer ticket.Unlock()
	previous := ticket.processes
	ticket.processes = p
	return func() {
		ticket.Lock()
		defer ticket.Unlock()
		ticket.processes = previous
	}
}

// Lease is one call's hold on the shared ticket.
type Lease struct {
	once     sync.Once
	probeErr error
}

// Acquire validates password into the shared ticket and returns this call's
// lease on it. On failure it returns sudo's output unscrubbed; callers that
// show it must scrub it. ctx bounds the validation and the probe.
//
// The `sudo -n -v` probe is advisory: hosts with `Defaults verifypw=always`
// refuse it even with a fresh ticket, although `sudo -n <command>` runs under
// that ticket. A failed probe therefore still returns a lease; the command runs
// with sudo -n, which refuses before running anything when no ticket covers
// it, and Lease.Explain attaches ErrTimestampDisabled if that run fails.
func Acquire(ctx context.Context, dir, password string) (*Lease, string, error) {
	ticket.Lock()
	defer ticket.Unlock()

	validateCtx, cancel := context.WithTimeout(ctx, validateTimeout)
	defer cancel()
	if out, err := ticket.processes.Validate(validateCtx, dir, password); err != nil {
		switch {
		case errors.Is(validateCtx.Err(), context.DeadlineExceeded):
			return nil, string(out), fmt.Errorf("sudo authentication timed out: %w", validateCtx.Err())
		case validateCtx.Err() != nil:
			return nil, string(out), fmt.Errorf("sudo authentication canceled: %w", validateCtx.Err())
		}
		return nil, string(out), fmt.Errorf("sudo authentication failed: %w", err)
	}
	probeCtx, cancelProbe := context.WithTimeout(ctx, probeTimeout)
	defer cancelProbe()
	lease := &Lease{probeErr: ticket.processes.Probe(probeCtx, dir)}
	ticket.holders++
	return lease, "", nil
}

// Release gives up the hold; the last holder drops the ticket. Release is
// idempotent, and on a nil Lease (no ticket was needed) it does nothing.
func (l *Lease) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		ticket.Lock()
		defer ticket.Unlock()
		ticket.holders--
		if ticket.holders == 0 {
			ticket.processes.Drop()
		}
	})
}

// Explain returns err unchanged unless the timestamp probe failed in Acquire.
// Then a failed sudo -n run may be sudo refusing the command for lack of a
// password, so Explain attaches ErrTimestampDisabled. A nil Lease returns err.
func (l *Lease) Explain(err error) error {
	if l == nil || err == nil || l.probeErr == nil {
		return err
	}
	return fmt.Errorf("%w; %w (sudo -n -v: %v)", err, ErrTimestampDisabled, l.probeErr)
}

// newValidateCommand refreshes the sudo timestamp with the Vault password.
// -v runs no command, so the password line on stdin can never reach a child
// process. -k must not be added: combined with -v, sudo authenticates without
// writing the timestamp and the following sudo -n is always refused.
func newValidateCommand(ctx context.Context, dir, password string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "sudo", "-S", "-p", "", "-v")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(password + "\n")
	return cmd
}

func runValidate(ctx context.Context, dir, password string) ([]byte, error) {
	cmd := newValidateCommand(ctx, dir, password)
	prepare(cmd)
	return cmd.CombinedOutput()
}

// runProbe succeeds only when a ticket or a NOPASSWD policy covers the next
// sudo -n call. It never prompts, so it reads no password, and its exit status
// does not depend on the sudo message locale.
func runProbe(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "sudo", "-n", "-v")
	cmd.Dir = dir
	cmd.Stdin = nil
	prepare(cmd)
	return cmd.Run()
}

// runDrop invalidates the ticket. A failed drop is not fatal: the ticket still
// expires with timestamp_timeout.
func runDrop() {
	ctx, cancel := context.WithTimeout(context.Background(), dropTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sudo", "-k")
	cmd.Stdin = nil
	prepare(cmd)
	if err := cmd.Run(); err != nil {
		slog.Warn("sudo ticket could not be dropped; it expires with timestamp_timeout", "error", err)
	}
}

// prepare keeps host secrets out of sudo's environment and puts it in its own
// process group, as internal/tools does for every child process.
func prepare(cmd *exec.Cmd) {
	if cmd.Env == nil {
		cmd.Env = sandbox.FilterEnv(os.Environ())
	}
	setProcessGroup(cmd)
}
