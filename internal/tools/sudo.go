package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"
)

// Privileged commands never read the Vault password on their stdin. The
// password only reaches `sudo -S -v`, which runs no command and leaves a sudo
// timestamp ("ticket"); the command itself then runs with `sudo -n` and a
// closed stdin. sudo keys the ticket by this process (its tty, or its pid when
// there is none), so every privileged call shares one ticket.

var sudoPasswordPromptPattern = regexp.MustCompile(`^\[sudo\][^:\r\n]*:\s*`)

// errSudoTimestampDisabled reports a sudoers policy without timestamps
// (timestamp_timeout=0): a validated password never covers the next command.
var errSudoTimestampDisabled = errors.New("sudo on this host requires a password for every command (timestamp caching disabled); enable a sudo timestamp or a NOPASSWD rule for the AuraGo user")

const (
	sudoProbeTimeout = 10 * time.Second
	sudoDropTimeout  = 10 * time.Second
)

// sudoValidateTimeout bounds the password check; tests shorten it.
var sudoValidateTimeout = 30 * time.Second

// The sudo processes behind the ticket. Tests swap them to exercise the
// refcount without running sudo.
var (
	runSudoValidate = func(ctx context.Context, dir, password string) ([]byte, error) {
		cmd := newSudoValidateCommand(ctx, dir, password)
		ensureFilteredEnv(cmd)
		SetupCmd(cmd)
		return cmd.CombinedOutput()
	}
	runSudoProbe = probeSudoTimestamp
	runSudoDrop  = dropSudoTimestamp
)

// newSudoValidateCommand refreshes the sudo timestamp with the Vault password.
// -v runs no command, so the password line on stdin can never reach a child
// process. -k must not be added: combined with -v, sudo authenticates without
// writing the timestamp and the following sudo -n is always refused.
func newSudoValidateCommand(ctx context.Context, dir, password string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "sudo", "-S", "-p", "", "-v")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(password + "\n")
	return cmd
}

// newSudoRunCommand runs command under the cached sudo timestamp. -n fails
// instead of prompting and stdin stays closed, so the password is never on
// the command's input.
func newSudoRunCommand(command, dir string) *exec.Cmd {
	cmd := exec.Command("sudo", "-n", "/bin/sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdin = nil
	return cmd
}

// probeSudoTimestamp succeeds only when a ticket or a NOPASSWD policy covers
// the next sudo -n call. It never prompts, so it reads no password, and its
// exit status does not depend on the sudo message locale.
func probeSudoTimestamp(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), sudoProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sudo", "-n", "-v")
	cmd.Dir = dir
	cmd.Stdin = nil
	ensureFilteredEnv(cmd)
	SetupCmd(cmd)
	return cmd.Run()
}

// dropSudoTimestamp invalidates the ticket created by newSudoValidateCommand.
// A failed drop is not fatal: the ticket still expires with timestamp_timeout.
func dropSudoTimestamp() {
	ctx, cancel := context.WithTimeout(context.Background(), sudoDropTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sudo", "-k")
	cmd.Stdin = nil
	ensureFilteredEnv(cmd)
	SetupCmd(cmd)
	if err := cmd.Run(); err != nil {
		slog.Warn("sudo ticket could not be dropped; it expires with timestamp_timeout", "error", err)
	}
}

// sudoTicket counts the calls that currently rely on the shared ticket. Only
// the last holder drops it, or one call's cleanup could revoke a ticket that
// another call has validated but not used yet.
var sudoTicket struct {
	sync.Mutex
	holders int
}

// acquireSudoTicket validates password into the shared sudo timestamp and
// returns a release function that drops the timestamp once no other call
// holds it. On failure it returns the scrubbed sudo output.
func acquireSudoTicket(dir, password string) (func(), string, error) {
	sudoTicket.Lock()
	defer sudoTicket.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), sudoValidateTimeout)
	defer cancel()
	if out, err := runSudoValidate(ctx, dir, password); err != nil {
		if ctx.Err() != nil {
			return nil, security.Scrub(string(out)), fmt.Errorf("sudo authentication timed out: %w", ctx.Err())
		}
		return nil, security.Scrub(string(out)), fmt.Errorf("sudo authentication failed: %w", err)
	}
	if err := runSudoProbe(dir); err != nil {
		if sudoTicket.holders == 0 {
			runSudoDrop()
		}
		return nil, "", fmt.Errorf("%w (sudo -n -v: %v)", errSudoTimestampDisabled, err)
	}
	sudoTicket.holders++

	var once sync.Once
	return func() {
		once.Do(func() {
			sudoTicket.Lock()
			defer sudoTicket.Unlock()
			sudoTicket.holders--
			if sudoTicket.holders == 0 {
				runSudoDrop()
			}
		})
	}, "", nil
}

// withSudoTicket registers password with the scrubber and acquires the shared
// sudo ticket. The single release drops the ticket before it unregisters the
// password, so everything produced under the ticket is scrubbed first.
func withSudoTicket(password, dir string) (func(), string, error) {
	unregister := security.RegisterScopedSensitiveExact(password)
	dropTicket, authOut, err := acquireSudoTicket(dir, password)
	if err != nil {
		unregister()
		return nil, authOut, err
	}
	return func() {
		dropTicket()
		unregister()
	}, "", nil
}

func normalizeSudoStderr(stderr string) string {
	trimmed := strings.TrimSpace(stderr)
	if trimmed == "" {
		return ""
	}
	return strings.TrimSpace(sudoPasswordPromptPattern.ReplaceAllString(trimmed, ""))
}
