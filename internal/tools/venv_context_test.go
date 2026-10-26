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

func TestEnsureVenvCreatesMissingWorkspace(t *testing.T) {
	if findSystemPython() == "" {
		t.Skip("system Python required for venv creation")
	}
	workspace := filepath.Join(t.TempDir(), "missing", "workspace")
	if err := EnsureVenvContext(context.Background(), workspace, nil); err != nil {
		t.Fatalf("create venv in new workspace: %v", err)
	}
	for _, bin := range []string{GetPythonBin(workspace), GetPipBin(workspace)} {
		if _, err := os.Stat(bin); err != nil {
			t.Fatalf("venv entry point %s: %v", bin, err)
		}
	}
}
