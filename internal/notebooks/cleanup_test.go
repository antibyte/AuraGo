package notebooks

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

type cleanupFailureRuntime struct {
	Runtime
	calls atomic.Int32
}

func (r *cleanupFailureRuntime) Remove(context.Context, string) error {
	if r.calls.Add(1) == 1 {
		return errors.New("fixture Docker temporarily unavailable")
	}
	return nil
}

func TestKernelCleanupFailureRetainsOwnedContainerAndSlot(t *testing.T) {
	s, d, p, _, _ := documentFixture(t)
	if _, err := d.Save(context.Background(), p.ID, "book.ipynb", "*", []byte(preservedNotebook)); err != nil {
		t.Fatal(err)
	}
	runtime := &cleanupFailureRuntime{}
	k := NewKernels(context.Background(), s, d, runtime)
	defer k.Close()
	_, cancel := context.WithCancel(context.Background())
	k.mu.Lock()
	k.sessions["owned"] = &Session{ID: "owned", Project: p.ID, File: "book.ipynb", Status: "idle", container: "owned-container", cancel: cancel}
	k.mu.Unlock()
	if err := k.Stop("owned", "permission_revoked"); err == nil {
		t.Fatal("cleanup error was hidden")
	}
	k.mu.Lock()
	container, status := k.sessions["owned"].container, k.sessions["owned"].Status
	k.mu.Unlock()
	if container != "owned-container" || status != "stopping" {
		t.Fatalf("failed cleanup lost its retry handle or reported completion: %q %q", container, status)
	}
	if _, err := k.Start(p.ID, "book.ipynb", nil, false); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatal("unconfirmed cleanup freed a session slot", err)
	}
	if err := k.Stop("owned", "permission_revoked"); err != nil {
		t.Fatal(err)
	}
	if runtime.calls.Load() != 2 || k.List()[0].Status != "stopped" {
		t.Fatal("explicit cleanup retry did not complete")
	}
}
