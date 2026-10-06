//go:build !windows

package tools

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestDockerComposeOutputRefusesSpecialFileTargets(t *testing.T) {
	workspace := t.TempDir()
	resolvedWorkspace, err := secureResolveFinalPath(workspace)
	if err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(resolvedWorkspace, "pipe.yml")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("cannot create a FIFO here: %v", err)
	}
	cfg := DockerConfig{WorkspaceDir: workspace}
	// Validation refuses the FIFO, so Compose is never asked to open it ...
	if plan, err := planComposeOutput(t, cfg, "config -o pipe.yml"); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("FIFO target accepted as %+v (err %v)", plan, err)
	}
	// ... and publishing refuses one that appeared after validation, without
	// opening it (opening a FIFO for writing would block).
	staged := filepath.Join(t.TempDir(), "out")
	if err := os.WriteFile(staged, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := dockerComposeOutputPlan{root: resolvedWorkspace, rel: "pipe.yml", target: fifo}
	if published, err := plan.publish(staged); err == nil || published || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("publish over a FIFO: published = %v, err = %v", published, err)
	}
}

func TestDockerComposeOutputAllowsColonInNamesOffWindows(t *testing.T) {
	// The NTFS stream check is Windows-only: ':' is an ordinary character in a
	// Linux file name.
	workspace := t.TempDir()
	plan, err := planComposeOutput(t, DockerConfig{WorkspaceDir: workspace}, "config -o stack:v1.yml")
	if err != nil || !plan.writesFile() || filepath.Base(plan.target) != "stack:v1.yml" {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
}
