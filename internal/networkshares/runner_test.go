package networkshares

import (
	"bytes"
	"context"
	"os/exec"
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
	t.Cleanup(sudoticket.ReplaceProcessesForTesting(sudoticket.Processes{
		Validate: func(_ context.Context, _ string, password string) ([]byte, error) {
			validated = append(validated, password)
			return nil, nil
		},
		Probe: func(context.Context, string) error { return nil },
		Drop:  func() { drops++ },
	}))
	withPassword := Options{SudoPassword: "vault-sudo-secret"}
	sudoCmd := exec.Command("sudo", "-n", "--", "true")

	for _, tc := range []struct {
		name    string
		options Options
		cmd     *exec.Cmd
	}{
		{"command without sudo", withPassword, exec.Command("net", "conf", "list")},
		{"sudo without password", Options{}, sudoCmd},
	} {
		lease, _, err := acquireSudoTicket(context.Background(), tc.options, tc.cmd)
		if err != nil || lease != nil || len(validated) != 0 {
			t.Fatalf("%s: lease=%v err=%v validations=%d, want no ticket", tc.name, lease != nil, err, len(validated))
		}
	}

	lease, _, err := acquireSudoTicket(context.Background(), withPassword, sudoCmd)
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
}
