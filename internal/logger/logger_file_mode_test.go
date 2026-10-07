package logger

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Log lines carry prompts, paths and tool output, so log files are owner-only.
// A legacy world-readable file is tightened on open; group access an operator
// granted on purpose (for a log shipper) survives a restart.
func TestLogFileIsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes do not apply on Windows")
	}
	dir := t.TempDir()

	mode := func(path string) os.FileMode {
		t.Helper()
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return info.Mode().Perm()
	}
	open := func(path string, appendMode bool) {
		t.Helper()
		lf, err := SetupWithFile(false, path, appendMode)
		if err != nil {
			t.Fatalf("SetupWithFile(%s): %v", path, err)
		}
		lf.Logger.Info("fixture line")
		if err := lf.Close(); err != nil {
			t.Fatal(err)
		}
	}

	fresh := filepath.Join(dir, "fresh", "aurago.log")
	open(fresh, false)
	if got := mode(fresh); got != 0o600 {
		t.Fatalf("new log file mode = %v, want 0600", got)
	}

	for _, appendMode := range []bool{false, true} {
		legacy := filepath.Join(dir, "legacy.log")
		if err := os.WriteFile(legacy, []byte("old\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(legacy, 0o644); err != nil {
			t.Fatal(err)
		}
		open(legacy, appendMode)
		if got := mode(legacy); got != 0o600 {
			t.Fatalf("append=%v: world-readable log file mode = %v after open, want 0600", appendMode, got)
		}
	}

	shipped := filepath.Join(dir, "shipped.log")
	if err := os.WriteFile(shipped, nil, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shipped, 0o640); err != nil {
		t.Fatal(err)
	}
	open(shipped, true)
	if got := mode(shipped); got != 0o640 {
		t.Fatalf("operator-granted group access = %v after open, want 0640 kept", got)
	}

	webAccess := filepath.Join(dir, "web_access.log")
	lf, err := SetupFileOnly(false, webAccess, false)
	if err != nil {
		t.Fatal(err)
	}
	_ = lf.Close()
	if got := mode(webAccess); got != 0o600 {
		t.Fatalf("file-only log mode = %v, want 0600", got)
	}
}
