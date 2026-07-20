//go:build linux

package networkshares

import (
	"fmt"
	"os"
)

func platformCommand(options Options, privileged bool, name string, args []string, stdin []byte) (string, []string, []byte, error) {
	if !privileged || os.Geteuid() == 0 {
		return name, args, stdin, nil
	}
	if !options.SudoEnabled || !options.SudoUnrestricted || options.NoNewPrivileges || options.ProtectSystemStrict {
		return "", nil, nil, codedError(ErrorPermissionDenied, "Host-wide share changes require unrestricted sudo and a writable system configuration.", nil)
	}
	sudoArgs := []string{"--", name}
	commandInput := stdin
	if options.SudoPassword != "" {
		sudoArgs = []string{"-S", "-p", "", "--", name}
		commandInput = append([]byte(options.SudoPassword+"\n"), stdin...)
	} else {
		sudoArgs = append([]string{"-n"}, sudoArgs...)
	}
	sudoArgs = append(sudoArgs, args...)
	return "sudo", sudoArgs, commandInput, nil
}

func platformElevated() bool {
	return os.Geteuid() == 0
}

func elevationReason() string {
	return fmt.Sprintf("Host-wide share changes require root or unrestricted sudo.")
}
