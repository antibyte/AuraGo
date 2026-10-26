package decisions

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestResidentProcessReapsDescendantsAndOpenPipes(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"tree", "orphan"} {
		t.Run(mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "resident")
			t.Setenv(nativeDiscoveryHelperEnv, mode)
			t.Setenv(nativeDiscoveryMarkerEnv, marker)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			command := exec.CommandContext(ctx, executable)
			configureHiddenProcess(command)
			command.Stdout, command.Stderr = io.Discard, io.Discard
			cleanup, err := startRuntimeCommand(command)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			wait := make(chan error, 1)
			go func() { err := command.Wait(); cleanup(); wait <- err; close(wait) }()
			deadline := time.Now().Add(5 * time.Second)
			for {
				if _, err := os.ReadFile(marker + ".child"); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("descendant never started")
				}
				time.Sleep(10 * time.Millisecond)
			}
			session := nativeRuntimeSession{command: command, cancel: cancel, wait: wait}
			if mode == "tree" {
				if err := session.Close(t.Context()); err != nil {
					t.Fatal(err)
				}
			} else {
				select {
				case <-wait:
				case <-time.After(4 * time.Second):
					t.Fatal("open descendant pipes held Wait")
				}
			}
			before, _ := os.ReadFile(marker + ".child")
			time.Sleep(100 * time.Millisecond)
			after, _ := os.ReadFile(marker + ".child")
			if string(before) != string(after) {
				t.Fatal("resident descendant survived cleanup")
			}
		})
	}
}
