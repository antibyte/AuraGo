package tools

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"aurago/internal/security"
	"aurago/internal/sudoticket"
)

func TestNormalizeSudoStderrRemovesLocalizedPrompt(t *testing.T) {
	stderr := "[sudo] Passwort fuer andi: permission denied"
	got := normalizeSudoStderr(stderr)
	if got != "permission denied" {
		t.Fatalf("normalizeSudoStderr() = %q, want %q", got, "permission denied")
	}
}

func TestNormalizeSudoStderrLeavesRegularErrors(t *testing.T) {
	stderr := "permission denied"
	got := normalizeSudoStderr(stderr)
	if got != stderr {
		t.Fatalf("normalizeSudoStderr() = %q, want %q", got, stderr)
	}
}

func TestSudoRunCommandNeverCarriesPasswordOnStdin(t *testing.T) {
	cmd := newSudoRunCommand("id", t.TempDir())
	if cmd.Stdin != nil {
		t.Fatal("sudo run command must not attach stdin")
	}
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, " -n ") || strings.Contains(args, " -S") {
		t.Fatalf("sudo run must be non-interactive without -S, got %q", args)
	}
}

// fakeTicketSudo stands in for the sudo processes behind the shared ticket.
// The host needs a password (sudo -n true fails), so every acquire validates.
// The refcount itself is tested in package sudoticket.
type fakeTicketSudo struct {
	mu          sync.Mutex
	validateOut string
	validateErr error
	probeErr    error
	drops       int
	onDrop      func()
}

func installFakeTicketSudo(t *testing.T, f *fakeTicketSudo) {
	t.Helper()
	t.Cleanup(sudoticket.ReplaceProcessesForTesting(sudoticket.Processes{
		Passwordless: func(context.Context, string) error {
			return errors.New("sudo: a password is required")
		},
		Validate: func(context.Context, string, string) ([]byte, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.validateErr != nil {
				return []byte(f.validateOut), f.validateErr
			}
			return nil, nil
		},
		Probe: func(context.Context, string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.probeErr
		},
		Drop: func() {
			f.mu.Lock()
			f.drops++
			onDrop := f.onDrop
			f.mu.Unlock()
			if onDrop != nil {
				onDrop()
			}
		},
	}))
}

func TestWithSudoTicket(t *testing.T) {
	t.Run("failed validation returns scrubbed output", func(t *testing.T) {
		const password = "refcount-secret-pw"
		f := &fakeTicketSudo{validateErr: errors.New("exit status 1"), validateOut: "Sorry, try again. " + password}
		installFakeTicketSudo(t, f)

		lease, out, err := withSudoTicket(password, ".")
		if err == nil || lease != nil || !strings.Contains(err.Error(), "sudo authentication failed") {
			t.Fatalf("withSudoTicket() = (lease %v, err %v), want authentication failure", lease != nil, err)
		}
		if strings.Contains(out, password) || !strings.Contains(out, "Sorry, try again.") {
			t.Fatalf("auth output = %q, want the sudo message with the password scrubbed", out)
		}
		if !strings.Contains(security.Scrub(password), password) {
			t.Fatal("password must be unregistered after a failed validation")
		}
		if f.drops != 0 {
			t.Fatalf("drops = %d after failed validation, want 0", f.drops)
		}
	})

	t.Run("release drops the ticket before unregistering the password", func(t *testing.T) {
		const password = "ordering-secret-pw"
		scrubbedDuringDrop := false
		f := &fakeTicketSudo{onDrop: func() {
			scrubbedDuringDrop = !strings.Contains(security.Scrub("out "+password), password)
		}}
		installFakeTicketSudo(t, f)
		lease, _, err := withSudoTicket(password, ".")
		if err != nil {
			t.Fatalf("withSudoTicket() error = %v", err)
		}
		if strings.Contains(security.Scrub(password), password) {
			t.Fatal("password must be registered with the scrubber while the ticket is held")
		}
		lease.Release()
		lease.Release()
		if f.drops != 1 {
			t.Fatalf("drops = %d after release, want 1", f.drops)
		}
		if !scrubbedDuringDrop {
			t.Fatal("password was unregistered before the ticket was dropped")
		}
		if !strings.Contains(security.Scrub(password), password) {
			t.Fatal("password must be unregistered after release")
		}
	})

	t.Run("failed probe does not refuse the run but explains its failure", func(t *testing.T) {
		f := &fakeTicketSudo{probeErr: errors.New("exit status 1")}
		installFakeTicketSudo(t, f)
		lease, _, err := withSudoTicket("verifypw-always-pw", ".")
		if err != nil {
			t.Fatalf("withSudoTicket() error = %v, want a lease despite the failed probe", err)
		}
		defer lease.Release()
		if got := lease.Explain(nil); got != nil {
			t.Fatalf("Explain(nil) = %v, want nil", got)
		}
		if got := lease.Explain(errors.New("exit status 1")); !errors.Is(got, sudoticket.ErrTimestampDisabled) {
			t.Fatalf("Explain(runErr) = %v, want ErrTimestampDisabled attached", got)
		}
	})

	t.Run("nil lease is safe", func(t *testing.T) {
		var lease *sudoLease
		lease.Release()
		runErr := errors.New("exit status 1")
		if got := lease.Explain(runErr); got != runErr {
			t.Fatalf("nil lease Explain(runErr) = %v, want the run error unchanged", got)
		}
	})
}
