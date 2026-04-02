//go:build !linux

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
)

// RunHelper on non-Linux platforms simply executes the command directly without
// any sandboxing. This path should not normally be reached because the sandbox
// is only enabled on Linux, but it provides a safe fallback.
func RunExecHelper() {
	fmt.Fprintln(os.Stderr, "sandbox: RunExecHelper not supported on this platform")
	os.Exit(126)
}

func RunHelper(command string) {
	workDir := os.Getenv("AURAGO_SBX_WORKDIR")
	if workDir != "" {
		_ = os.Chdir(workDir)
	}

	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "sandbox: exec: %v\n", err)
		os.Exit(126)
	}
}
