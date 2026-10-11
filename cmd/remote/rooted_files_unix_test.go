//go:build unix

package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/remote"
	"golang.org/x/sys/unix"
)

func TestRootedEditorsRejectFIFOWithoutBlocking(t *testing.T) {
	for _, operation := range []string{remote.OpFileRead, remote.OpFileWrite, remote.OpJsonEdit, remote.OpYamlEdit, remote.OpXmlEdit, remote.OpFileEdit} {
		t.Run(operation, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "document")
			if err := unix.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			executor := NewExecutor(slog.Default(), 1)
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			done := make(chan remote.ResultPayload, 1)
			go func() {
				done <- executor.ExecuteContext(ctx, remote.CommandPayload{Operation: operation, Args: map[string]interface{}{"path": path, "operation": "validate", "content": "e30="}}, false, []string{dir})
			}()
			select {
			case result := <-done:
				if result.Status == "ok" {
					t.Fatal("FIFO accepted as a regular file")
				}
			case <-time.After(time.Second):
				// Release a regressed blocking open before failing, without leaking its goroutine.
				peer, err := os.OpenFile(path, os.O_RDWR|unix.O_NONBLOCK, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer peer.Close()
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Fatal("failed to release FIFO operation")
				}
				t.Fatal("file operation blocked after cancellation")
			}
		})
	}
}
