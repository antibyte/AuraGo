//go:build !linux

package sandbox

import (
	"fmt"
	"os"
)

// RunExecHelper and RunHelper fail closed on non-Linux platforms. Normal
// startup only enables the shell sandbox on Linux, so reaching these helper
// modes elsewhere means sandboxing cannot be applied safely.
func RunExecHelper() {
	fmt.Fprintln(os.Stderr, "sandbox: RunExecHelper not supported on this platform")
	os.Exit(126)
}

func RunHelper(command string) {
	fmt.Fprintln(os.Stderr, "sandbox: RunHelper not supported on this platform")
	os.Exit(126)
}
