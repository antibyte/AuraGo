package server

import (
	"path/filepath"
	"testing"

	"aurago/internal/desktop"
)

func TestOpenSCADDockerAdapterUsesJobRootForBindValidation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := desktop.Config{
		WorkspaceDir: filepath.Join(root, "desktop"),
		DataDir:      filepath.Join(root, "data"),
		DBPath:       filepath.Join(root, "virtual_desktop.db"),
	}

	codeAdapter := newCodeStudioDockerAdapter(cfg, nil)
	if codeAdapter.cfg.WorkspaceDir != cfg.WorkspaceDir {
		t.Fatalf("code studio WorkspaceDir = %q, want %q", codeAdapter.cfg.WorkspaceDir, cfg.WorkspaceDir)
	}

	openSCADAdapter := newOpenSCADDockerAdapter(cfg, nil)
	want := filepath.Join(cfg.DataDir, "openscad", "jobs")
	if openSCADAdapter.cfg.WorkspaceDir != want {
		t.Fatalf("openscad WorkspaceDir = %q, want %q", openSCADAdapter.cfg.WorkspaceDir, want)
	}
}
