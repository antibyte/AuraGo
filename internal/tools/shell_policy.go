package tools

import (
	"runtime"

	"aurago/internal/shellpolicy"
)

// ValidateShellCommandPolicy rejects shell commands that fall into high-risk classes.
func ValidateShellCommandPolicy(command string) error {
	if runtime.GOOS == "windows" {
		return shellpolicy.ValidateCommandWithEscape(command, '`')
	}
	return validateIsolatedShellCommandPolicy(command)
}

// Container workers always execute POSIX sh, even on a Windows host.
func validateIsolatedShellCommandPolicy(command string) error {
	return shellpolicy.ValidateCommand(command)
}
