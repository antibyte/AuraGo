//go:build unix

package server

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Opening a FIFO blocks until a writer appears, so openRegularFileInRoot must
// refuse it from Lstat, before any open.
func TestOpenRegularFileInRootRefusesFIFOWithoutOpeningIt(t *testing.T) {
	dir := t.TempDir()
	if err := unix.Mkfifo(filepath.Join(dir, "pipe.mp3"), 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		f, _, err := openRegularFileInRoot(dir, "pipe.mp3")
		if f != nil {
			f.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("err = %v, want os.ErrNotExist", err)
		}
	case <-time.After(5 * time.Second):
		// Unblock the stuck open so the test binary can exit.
		if w, err := os.OpenFile(filepath.Join(dir, "pipe.mp3"), os.O_WRONLY, 0); err == nil {
			w.Close()
		}
		t.Fatal("openRegularFileInRoot blocked opening a FIFO")
	}
}
