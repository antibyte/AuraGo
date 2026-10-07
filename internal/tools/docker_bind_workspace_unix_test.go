//go:build !windows

package tools

import (
	"os"
	"path/filepath"
	"testing"
)

// F-C6 accepts binds inside the workspace early. A symlink inside the
// workspace that leads out of it must still be refused, and a symlink that
// stays inside must still be accepted.
func TestValidateDockerBindMountRefusesASymlinkOutOfTheWorkspaceAfterTheEarlyAccept(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	outside := filepath.Join(root, "outside")
	for _, dir := range []string{filepath.Join(workspace, "stack", "data"), outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(workspace, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(workspace, "stack", "data"), filepath.Join(workspace, "inner")); err != nil {
		t.Fatal(err)
	}
	cfg := DockerConfig{WorkspaceDir: workspace}
	if dockerBindWithinWorkspace(workspace, filepath.Join(workspace, "escape", "x")) {
		t.Fatal("a symlink out of the workspace was accepted early")
	}
	if err := validateDockerBindMount(cfg, filepath.Join(workspace, "escape")+":/x"); err == nil {
		t.Fatal("a bind through a symlink out of the workspace was accepted")
	}
	if !dockerBindWithinWorkspace(workspace, filepath.Join(workspace, "inner")) {
		t.Fatal("a symlink that stays inside the workspace was not accepted early")
	}
	if err := validateDockerBindMount(cfg, filepath.Join(workspace, "inner")+":/x"); err != nil {
		t.Fatalf("a bind through a symlink inside the workspace was refused: %v", err)
	}
	// A workspace that is itself a symlink to a sensitive location accepts
	// nothing early.
	if err := os.Symlink("/etc", filepath.Join(root, "etc-link")); err != nil {
		t.Fatal(err)
	}
	if dockerBindWithinWorkspace(filepath.Join(root, "etc-link"), "/etc/hostname") {
		t.Fatal("a workspace that resolves to /etc accepted a bind early")
	}
}
