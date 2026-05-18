package tools

import "aurago/internal/shellpolicy"

// ValidateShellCommandPolicy rejects shell commands that fall into high-risk classes.
func ValidateShellCommandPolicy(command string) error {
	return shellpolicy.ValidateCommand(command)
}
