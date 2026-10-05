package tools

import (
	"context"
	"os/exec"
	"regexp"
	"strings"

	"aurago/internal/security"
	"aurago/internal/sudoticket"
)

// Privileged commands never read the Vault password on their stdin. The
// password only reaches `sudo -S -v` behind the process-wide ticket in package
// sudoticket; the command itself then runs with `sudo -n` and a closed stdin.

var sudoPasswordPromptPattern = regexp.MustCompile(`^\[sudo\][^:\r\n]*:\s*`)

// newSudoRunCommand runs command under the cached sudo timestamp. -n fails
// instead of prompting and stdin stays closed, so the password is never on
// the command's input.
func newSudoRunCommand(command, dir string) *exec.Cmd {
	cmd := exec.Command("sudo", "-n", "/bin/sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdin = nil
	return cmd
}

// sudoLease holds the shared sudo ticket and the scrubber registration of its
// password. Explain comes from the embedded ticket lease.
type sudoLease struct {
	*sudoticket.Lease
	unregister func()
}

// Release drops the ticket before it unregisters the password, so everything
// produced under the ticket is scrubbed first. It is idempotent.
func (l *sudoLease) Release() {
	l.Lease.Release()
	l.unregister()
}

// withSudoTicket registers password with the scrubber and acquires the shared
// sudo ticket. On failure it returns sudo's output scrubbed.
func withSudoTicket(password, dir string) (*sudoLease, string, error) {
	unregister := security.RegisterScopedSensitiveExact(password)
	lease, authOut, err := sudoticket.Acquire(context.Background(), dir, password)
	if err != nil {
		authOut = security.Scrub(authOut)
		unregister()
		return nil, authOut, err
	}
	return &sudoLease{Lease: lease, unregister: unregister}, "", nil
}

func normalizeSudoStderr(stderr string) string {
	trimmed := strings.TrimSpace(stderr)
	if trimmed == "" {
		return ""
	}
	return strings.TrimSpace(sudoPasswordPromptPattern.ReplaceAllString(trimmed, ""))
}
