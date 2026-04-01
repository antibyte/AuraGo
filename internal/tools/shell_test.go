package tools

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestNormalizeSudoStderrRemovesLocalizedPrompt(t *testing.T) {
	stderr := "[sudo] Passwort fuer andi: permission denied"
	got := normalizeSudoStderr(stderr)
	if got != "permission denied" {
		t.Fatalf("normalizeSudoStderr() = %q, want %q", got, "permission denied")
	}
}

func TestNormalizeSudoStderrLeavesRegularErrors(t *testing.T) {
	stderr := "permission denied"
	got := normalizeSudoStderr(stderr)
	if got != stderr {
		t.Fatalf("normalizeSudoStderr() = %q, want %q", got, stderr)
	}
}

func TestExecuteShell(t *testing.T) {
	workspaceDir, err := os.MkdirTemp("", "shell_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(workspaceDir)

	var command string
	if runtime.GOOS == "windows" {
		command = "Write-Output 'hello world'"
	} else {
		command = "echo 'hello world'"
	}

	stdout, stderr, err := ExecuteShell(command, workspaceDir)
	if err != nil {
		t.Errorf("ExecuteShell failed: %v, stderr: %s", err, stderr)
	}

	if !strings.Contains(stdout, "hello world") {
		t.Errorf("expected stdout to contain 'hello world', got: %s", stdout)
	}
}

func TestExecuteShellError(t *testing.T) {
	workspaceDir, _ := os.MkdirTemp("", "shell_test_err")
	defer os.RemoveAll(workspaceDir)

	command := "non-existent-command-12345"
	_, _, err := ExecuteShell(command, workspaceDir)
	if err == nil {
		t.Error("expected error for non-existent command, got nil")
	}
}
