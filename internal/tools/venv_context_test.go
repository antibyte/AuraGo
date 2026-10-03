package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCancelledVenvWaitDoesNotCreateEnvironment(t *testing.T) {
	work := t.TempDir()
	venvMu.Lock()
	defer venvMu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := EnsureVenvContext(ctx, work, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled wait=%v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "venv")); !os.IsNotExist(err) {
		t.Fatalf("cancelled caller created venv: %v", err)
	}
}
