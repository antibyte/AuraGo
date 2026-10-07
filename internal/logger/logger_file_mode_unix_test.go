//go:build !windows

package logger

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// A log path may point somewhere that is not a regular file (a symlink to
// /dev/null, a pipe to a collector). Its mode belongs to whoever set it up and
// must not be tightened: under root that would chmod /dev/null itself.
func TestLogFileLeavesNonRegularTargetModeAlone(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "collector.fifo")
	if err := syscall.Mkfifo(fifo, 0o666); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	if err := os.Chmod(fifo, 0o666); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "aurago.log")
	if err := os.Symlink(fifo, logPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	// Opening a FIFO for writing waits for a reader.
	reader := make(chan *os.File, 1)
	go func() {
		r, err := os.Open(fifo)
		if err != nil {
			t.Error(err)
		}
		reader <- r
	}()
	lf, err := SetupFileOnly(false, logPath, true)
	if err != nil {
		t.Fatalf("SetupFileOnly: %v", err)
	}
	_ = lf.Close()
	if r := <-reader; r != nil {
		_ = r.Close()
	}

	info, err := os.Stat(fifo)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o666 {
		t.Fatalf("non-regular log target mode = %v after open, want 0666 untouched", got)
	}
}
