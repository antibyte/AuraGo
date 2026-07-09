package tools

import (
	"os"
	"os/exec"

	"aurago/internal/sandbox"
)

// ensureFilteredEnv prevents child processes from inheriting host secrets by default.
func ensureFilteredEnv(cmd *exec.Cmd) {
	if cmd == nil || cmd.Env != nil {
		return
	}
	cmd.Env = sandbox.FilterEnv(os.Environ())
}
