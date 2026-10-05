//go:build linux || darwin || freebsd

package tools

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A FIFO swapped in for the file after the check must not hang the open until some process
// writes to it: the open step has to come back with an error.
func TestOpenWithinOutgoingRootDoesNotBlockOnAFifo(t *testing.T) {
	env := c103NewEnv(t)
	fifo := filepath.Join(env.workspace, "swapped.pdf")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("cannot create a FIFO: %v", err)
	}
	root := canonicalExistingRoot(env.workspace)

	done := make(chan error, 1)
	go func() {
		file, err := openWithinOutgoingRoot(env.cfg, root, filepath.Join(root, "swapped.pdf"))
		if file != nil {
			file.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a FIFO must not be handed out as an attachment")
		}
	case <-time.After(5 * time.Second):
		// Let the stuck open finish so the goroutine does not outlive the test.
		if unblock, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			unblock.Close()
		}
		t.Fatal("opening a FIFO blocked")
	}
	c103Refused(t, fifo, env.cfg)
}
