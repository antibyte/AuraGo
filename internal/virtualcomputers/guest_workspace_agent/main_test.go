//go:build linux

package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceShellAndPTYUsePortableShell(t *testing.T) {
	for _, usePTY := range []bool{false, true} {
		svc := &service{jobs: make(map[string]*job)}
		created, err := svc.createJob(startJobRequest{
			Command: "printf AURAGO_WORKSPACE_OK", WorkingDir: t.TempDir(),
			TimeoutSeconds: 5, PTY: usePTY,
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if created.cmd.Path != "/bin/sh" || created.cmd.Args[1] != "-c" {
			t.Fatalf("job requires a non-portable shell: %v", created.cmd.Args)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		select {
		case <-created.done:
		case <-ctx.Done():
			_ = svc.cancelJob(created.public.ID)
			t.Fatal("shell job did not finish")
		}
		cancel()
		if got := created.snapshot(); got.State != "completed" || got.ExitCode == nil || *got.ExitCode != 0 {
			t.Fatalf("PTY=%v: job failed: %+v", usePTY, got)
		}
		if !usePTY && !strings.Contains(created.output.all(), "AURAGO_WORKSPACE_OK") {
			t.Fatal("shell verification output is missing")
		}
	}
}

func TestTryAcquireDoesNotQueuePastCapacity(t *testing.T) {
	slots := make(chan struct{}, 1)
	if !tryAcquire(slots) {
		t.Fatal("first operation should acquire the available slot")
	}
	if tryAcquire(slots) {
		t.Fatal("operation beyond capacity must be rejected without queuing")
	}
	release(slots)
	if !tryAcquire(slots) {
		t.Fatal("released slot should be reusable")
	}
	release(slots)
}
