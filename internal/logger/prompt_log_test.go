package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResetPromptLogTruncatesExistingFileAndCreatesDirectory(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), "missing", "log")
	path := PromptLogPath(logDir)

	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("old prompt data"), 0600); err != nil {
		t.Fatalf("seed prompt log: %v", err)
	}

	if err := ResetPromptLog(logDir); err != nil {
		t.Fatalf("reset prompt log: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat prompt log: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("prompt log size = %d, want 0", info.Size())
	}
}

func TestAppendPromptLogEntryCreatesDirectoryAndWritesJSONLine(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), "missing", "log")

	if err := AppendPromptLogEntry(logDir, map[string]string{"role": "user"}); err != nil {
		t.Fatalf("append prompt log entry: %v", err)
	}

	data, err := os.ReadFile(PromptLogPath(logDir))
	if err != nil {
		t.Fatalf("read prompt log: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `"role":"user"`) {
		t.Fatalf("prompt log entry = %q, want JSON role", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("prompt log entry should end with newline: %q", got)
	}
}
