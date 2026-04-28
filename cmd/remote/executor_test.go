package main

import (
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/remote"
)

func TestExecutorValidatePathRequiresAllowedPaths(t *testing.T) {
	t.Parallel()

	executor := NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB)
	target := filepath.Join(t.TempDir(), "file.txt")
	if err := executor.validatePath(target, nil); err == nil || !strings.Contains(err.Error(), "allowed_paths") {
		t.Fatalf("validatePath() error = %v, want allowed_paths failure", err)
	}
}

func TestExecutorValidatePathRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	link := filepath.Join(root, "escape.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink setup not supported: %v", err)
	}

	executor := NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB)
	err := executor.validatePath(link, []string{root})
	if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
		t.Fatalf("validatePath() error = %v, want outside allowed directories", err)
	}
}

func TestExecutorFileReadDeniedWithoutAllowedPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "data.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	executor := NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB)
	result := executor.Execute(remote.CommandPayload{
		CommandID: "cmd-1",
		Operation: remote.OpFileRead,
		Args: map[string]interface{}{
			"path": target,
		},
	}, false, nil)

	if result.Status != "denied" || !strings.Contains(result.Error, "allowed_paths") {
		t.Fatalf("status=%q error=%q, want denied allowed_paths error", result.Status, result.Error)
	}
}

func TestExecutorFileSearchRequiresExplicitPath(t *testing.T) {
	t.Parallel()

	executor := NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB)
	result := executor.Execute(remote.CommandPayload{
		CommandID: "cmd-2",
		Operation: remote.OpFileSearch,
		Args: map[string]interface{}{
			"operation": "find",
			"pattern":   "*.go",
			"glob":      "*.go",
		},
	}, false, []string{t.TempDir()})

	if result.Status != "denied" || !strings.Contains(result.Error, "path is required") {
		t.Fatalf("status=%q error=%q, want explicit path failure", result.Status, result.Error)
	}
}

func TestExecutorShellExecBlocksDangerousCommands(t *testing.T) {
	t.Parallel()

	executor := NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB)
	result := executor.Execute(remote.CommandPayload{
		CommandID: "cmd-3",
		Operation: remote.OpShellExec,
		Args: map[string]interface{}{
			"command": "rm -rf /",
		},
	}, false, []string{t.TempDir()})

	if result.Status != "error" || !strings.Contains(result.Error, "command blocked") {
		t.Fatalf("status=%q error=%q, want blocked shell command", result.Status, result.Error)
	}
}

func TestExecutorFileWriteRejectsOversizedPayload(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "big.txt")
	executor := NewExecutor(slog.Default(), 1)
	content := base64.StdEncoding.EncodeToString(make([]byte, 2*1024*1024))

	result := executor.Execute(remote.CommandPayload{
		CommandID: "cmd-4",
		Operation: remote.OpFileWrite,
		Args: map[string]interface{}{
			"path":    target,
			"content": content,
		},
	}, false, []string{root})

	if result.Status != "error" || !strings.Contains(result.Error, "max_file_size_mb") {
		t.Fatalf("status=%q error=%q, want max_file_size_mb error", result.Status, result.Error)
	}
}

func TestExecutorDoesNotReplayDuplicateCommandID(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "data.txt")
	executor := NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB)

	first := executor.Execute(remote.CommandPayload{
		CommandID: "cmd-replay",
		Operation: remote.OpFileWrite,
		Args: map[string]interface{}{
			"path":    target,
			"content": base64.StdEncoding.EncodeToString([]byte("first")),
		},
	}, false, []string{root})
	if first.Status != "ok" {
		t.Fatalf("first status=%q error=%q", first.Status, first.Error)
	}

	second := executor.Execute(remote.CommandPayload{
		CommandID: "cmd-replay",
		Operation: remote.OpFileWrite,
		Args: map[string]interface{}{
			"path":    target,
			"content": base64.StdEncoding.EncodeToString([]byte("second")),
		},
	}, false, []string{root})
	if second.Status != first.Status || second.Output != first.Output {
		t.Fatalf("duplicate result = %#v, want cached first result %#v", second, first)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "first" {
		t.Fatalf("duplicate command replay changed file to %q, want first", got)
	}
}
