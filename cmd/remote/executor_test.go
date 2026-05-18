package main

import (
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

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

func TestExecutorResolveAllowedPathReturnsResolvedSymlinkTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink setup not supported: %v", err)
	}

	executor := NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB)
	resolved, err := executor.resolveAllowedPath(link, []string{root})
	if err != nil {
		t.Fatalf("resolveAllowedPath() error = %v", err)
	}
	if filepath.Clean(resolved) != filepath.Clean(target) {
		t.Fatalf("resolveAllowedPath() = %q, want resolved target %q", resolved, target)
	}
}

func TestExecutorFileWriteRejectsSymlinkPrefixEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink setup not supported: %v", err)
	}
	target := filepath.Join(link, "nested", "created.txt")

	executor := NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB)
	result := executor.Execute(remote.CommandPayload{
		CommandID: "cmd-symlink-prefix",
		Operation: remote.OpFileWrite,
		Args: map[string]interface{}{
			"path":    target,
			"content": base64.StdEncoding.EncodeToString([]byte("secret")),
		},
	}, false, []string{root})

	if result.Status != "denied" || !strings.Contains(result.Error, "outside allowed directories") {
		t.Fatalf("status=%q error=%q, want denied symlink escape", result.Status, result.Error)
	}
	if _, err := os.Stat(filepath.Join(outside, "nested", "created.txt")); !os.IsNotExist(err) {
		t.Fatalf("symlink escape created outside file, stat error = %v", err)
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

func TestExecutorTrimCommandResultCacheSkipsCurrentAndRemovesCompletedEntries(t *testing.T) {
	executor := NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB)
	executor.commandResults = make(map[string]*commandExecutionRecord)
	executor.commandOrder = append(executor.commandOrder, "current")
	executor.commandResults["current"] = runningCommandRecord()

	for i := 0; i < maxCachedCommandResults+5; i++ {
		id := "done-" + strconv.Itoa(i)
		executor.commandOrder = append(executor.commandOrder, id)
		executor.commandResults[id] = finishedCommandRecord(id)
	}

	executor.trimCommandResultCacheLocked("current")

	if len(executor.commandOrder) > maxCachedCommandResults {
		t.Fatalf("commandOrder length = %d, want <= %d", len(executor.commandOrder), maxCachedCommandResults)
	}
	if executor.commandResults["current"] == nil {
		t.Fatal("current command record was evicted")
	}
}

func TestExecutorTrimCommandResultCacheForcesRemovalWhenAllRunning(t *testing.T) {
	executor := NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB)
	executor.commandResults = make(map[string]*commandExecutionRecord)

	// Fill cache with only running (not done) entries
	for i := 0; i < maxCachedCommandResults+10; i++ {
		id := "running-" + strconv.Itoa(i)
		executor.commandOrder = append(executor.commandOrder, id)
		executor.commandResults[id] = runningCommandRecord()
	}

	currentID := "running-5"
	executor.trimCommandResultCacheLocked(currentID)

	if len(executor.commandOrder) > maxCachedCommandResults {
		t.Fatalf("commandOrder length = %d, want <= %d", len(executor.commandOrder), maxCachedCommandResults)
	}
	if executor.commandResults[currentID] == nil {
		t.Fatal("current command record was evicted")
	}

	// Verify that force-removed entries have their done channel closed
	removedID := "running-0"
	if executor.commandResults[removedID] != nil {
		t.Fatalf("oldest running entry %q should have been force-removed", removedID)
	}
}

func TestCommandExecutionRecordCloseDoneIsIdempotent(t *testing.T) {
	record := runningCommandRecord()
	record.closeDone()
	select {
	case <-record.done:
	default:
		t.Fatal("closeDone did not close the channel")
	}
	// Second close must not panic
	record.closeDone()
}

func TestExecutorFileReadAdvRejectsOversizedFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "big.txt")
	if err := os.WriteFile(target, make([]byte, 2*1024*1024), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	executor := NewExecutor(slog.Default(), 1)
	result := executor.Execute(remote.CommandPayload{
		CommandID: "cmd-readadv",
		Operation: remote.OpFileReadAdv,
		Args: map[string]interface{}{
			"path":       target,
			"operation":  "tail",
			"line_count": 10.0,
		},
	}, false, []string{root})

	if result.Status != "error" || !strings.Contains(result.Error, "max_file_size_mb") {
		t.Fatalf("status=%q error=%q, want max_file_size_mb error", result.Status, result.Error)
	}
}

func TestExecutorShellExecTimeoutReturnsPromptly(t *testing.T) {
	executor := NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB)
	start := time.Now()
	_, err := executor.shellExec(slowShellCommand(), "", 50*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "command timed out after") {
		t.Fatalf("shellExec timeout error = %v, want timeout error", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("shellExec timeout returned after %v, want prompt return", elapsed)
	}
}

func finishedCommandRecord(commandID string) *commandExecutionRecord {
	done := make(chan struct{})
	close(done)
	return &commandExecutionRecord{
		done:   done,
		result: remote.ResultPayload{CommandID: commandID, Status: "ok"},
	}
}

func runningCommandRecord() *commandExecutionRecord {
	return &commandExecutionRecord{done: make(chan struct{})}
}

func slowShellCommand() string {
	if runtime.GOOS == "windows" {
		return "ping 127.0.0.1 -n 6 >NUL"
	}
	return "sleep 5"
}
