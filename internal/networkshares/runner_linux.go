//go:build linux

package networkshares

import (
	"fmt"
	"os"
)

// platformCommand runs privileged commands without root through `sudo -n`,
// with the caller's stdin only, and reports that with viaSudo. The Vault
// password never reaches the command: the runner holds the shared sudo ticket
// for the run instead (see acquireSudoTicket).
func platformCommand(options Options, privileged bool, name string, args []string, stdin []byte) (string, []string, []byte, bool, error) {
	if !privileged || os.Geteuid() == 0 {
		return name, args, stdin, false, nil
	}
	if !options.SudoEnabled || !options.SudoUnrestricted || options.NoNewPrivileges || options.ProtectSystemStrict {
		return "", nil, nil, false, codedError(ErrorPermissionDenied, "Host-wide share changes require unrestricted sudo and a writable system configuration.", nil)
	}
	sudoArgs := append([]string{"-n", "--", name}, args...)
	return "sudo", sudoArgs, stdin, true, nil
}

func platformElevated() bool {
	return os.Geteuid() == 0
}

func elevationReason() string {
	return fmt.Sprintf("Host-wide share changes require root or unrestricted sudo.")
}
