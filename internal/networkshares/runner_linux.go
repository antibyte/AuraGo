//go:build linux

package networkshares

import (
	"fmt"
	"os"
)

// platformCommand runs privileged commands without root through `sudo -n`,
// with the caller's stdin only. The Vault password never reaches the command:
// the runner validates it into the shared sudo ticket first (see
// acquireSudoTicket).
func platformCommand(options Options, privileged bool, name string, args []string, stdin []byte) (string, []string, []byte, error) {
	if !privileged || os.Geteuid() == 0 {
		return name, args, stdin, nil
	}
	if !options.SudoEnabled || !options.SudoUnrestricted || options.NoNewPrivileges || options.ProtectSystemStrict {
		return "", nil, nil, codedError(ErrorPermissionDenied, "Host-wide share changes require unrestricted sudo and a writable system configuration.", nil)
	}
	sudoArgs := append([]string{"-n", "--", name}, args...)
	return "sudo", sudoArgs, stdin, nil
}

func platformElevated() bool {
	return os.Geteuid() == 0
}

func elevationReason() string {
	return fmt.Sprintf("Host-wide share changes require root or unrestricted sudo.")
}
