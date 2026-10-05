package networkshares

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"aurago/internal/sudoticket"
)

func TestCappedBufferLimitsCapturedOutput(t *testing.T) {
	var buffer cappedBuffer
	input := bytes.Repeat([]byte("x"), maxCommandOutputBytes+4096)
	written, err := buffer.Write(input)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if written != len(input) || len(buffer.Bytes()) != maxCommandOutputBytes {
		t.Fatalf("written=%d captured=%d", written, len(buffer.Bytes()))
	}
}

func TestRunnerAcquiresSudoTicketOnlyForSudoWithPassword(t *testing.T) {
	var validated []string
	drops := 0
	nopasswd := false
	t.Cleanup(sudoticket.ReplaceProcessesForTesting(sudoticket.Processes{
		Passwordless: func(context.Context, string) error {
			if nopasswd {
				return nil
			}
			return errors.New("sudo: a password is required")
		},
		Validate: func(_ context.Context, _ string, password string) ([]byte, error) {
			validated = append(validated, password)
			return nil, nil
		},
		Probe: func(context.Context, string) error { return nil },
		Drop:  func() { drops++ },
	}))
	withPassword := Options{SudoPassword: "vault-sudo-secret"}

	for _, tc := range []struct {
		name    string
		options Options
		viaSudo bool
	}{
		{"command without sudo", withPassword, false},
		{"sudo without password", Options{}, true},
	} {
		lease, _, err := acquireSudoTicket(context.Background(), tc.options, tc.viaSudo)
		if err != nil || lease != nil || len(validated) != 0 {
			t.Fatalf("%s: lease=%v err=%v validations=%d, want no ticket", tc.name, lease != nil, err, len(validated))
		}
	}

	lease, _, err := acquireSudoTicket(context.Background(), withPassword, true)
	if err != nil || lease == nil {
		t.Fatalf("sudo with password: lease=%v err=%v, want a ticket", lease != nil, err)
	}
	if len(validated) != 1 || validated[0] != "vault-sudo-secret" || drops != 0 {
		t.Fatalf("validations=%v drops=%d, want the password validated once", len(validated), drops)
	}
	lease.Release()
	if drops != 1 {
		t.Fatalf("drops = %d after release, want 1", drops)
	}

	// A NOPASSWD host holds the ticket without a PAM call, so a stale Vault
	// password does not matter.
	nopasswd = true
	lease, _, err = acquireSudoTicket(context.Background(), withPassword, true)
	if err != nil || lease == nil || len(validated) != 1 {
		t.Fatalf("NOPASSWD host: lease=%v err=%v validations=%d, want a lease without validation", lease != nil, err, len(validated))
	}
	lease.Release()
}
