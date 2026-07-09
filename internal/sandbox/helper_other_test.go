//go:build !linux

package sandbox

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRunHelperUnsupportedOnNonLinux(t *testing.T) {
	if os.Getenv("AURAGO_TEST_RUN_UNSUPPORTED_HELPER") == "1" {
		RunHelper("echo should-not-run")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestRunHelperUnsupportedOnNonLinux")
	cmd.Env = append(os.Environ(), "AURAGO_TEST_RUN_UNSUPPORTED_HELPER=1")
	out, err := cmd.CombinedOutput()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("RunHelper subprocess error = %v, output = %s", err, out)
	}
	if exitErr.ExitCode() != 126 {
		t.Fatalf("exit code = %d, want 126; output = %s", exitErr.ExitCode(), out)
	}
	if !strings.Contains(string(out), "RunHelper not supported") {
		t.Fatalf("output = %q, want unsupported message", out)
	}
}
