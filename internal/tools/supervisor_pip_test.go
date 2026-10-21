package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSupervisorRuntimeRepairsPythonWithoutPip(t *testing.T) {
	python := findSystemPython()
	if python == "" {
		t.Skip("Python unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	runtimeDir := filepath.Join(t.TempDir(), "sandbox-supervisor")
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	create := supervisorPythonCommand(ctx, python, "-m", "venv", "--without-pip", filepath.Join(runtimeDir, "venv"))
	if output, err := create.CombinedOutput(); err != nil {
		t.Fatalf("create incomplete private venv: %v %s", err, output)
	}
	probe := func() error {
		return supervisorPythonCommand(ctx, GetPythonBin(runtimeDir), "-m", "pip", "--version").Run()
	}
	if err := probe(); err == nil {
		t.Fatal("fixture unexpectedly already contains pip")
	}
	for range 2 {
		if err := ensureSupervisorPip(ctx, runtimeDir); err != nil {
			t.Fatal(err)
		}
		if err := probe(); err != nil {
			t.Fatalf("private supervisor pip remains unavailable: %v", err)
		}
	}
}
