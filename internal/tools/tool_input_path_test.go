package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestResolveToolInputPathAllowsWorkspaceBoundFiles(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(repoRoot, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll workspace: %v", err)
	}
	allowedFile := filepath.Join(repoRoot, "data", "sample.txt")
	if err := os.MkdirAll(filepath.Dir(allowedFile), 0o755); err != nil {
		t.Fatalf("MkdirAll data: %v", err)
	}
	if err := os.WriteFile(allowedFile, []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspaceDir

	resolved, err := resolveToolInputPath("../../data/sample.txt", cfg)
	if err != nil {
		t.Fatalf("resolveToolInputPath: %v", err)
	}
	if resolved != allowedFile {
		t.Fatalf("resolved path = %q, want %q", resolved, allowedFile)
	}
}

func TestResolveToolInputPathRejectsProjectEscape(t *testing.T) {
	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(repoRoot, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll workspace: %v", err)
	}
	outsideFile := filepath.Join(filepath.Dir(repoRoot), "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("nope"), 0o644); err != nil {
		t.Fatalf("WriteFile outside: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspaceDir

	_, err := resolveToolInputPath(filepath.Join("..", "..", "..", "outside.txt"), cfg)
	if err == nil {
		t.Fatal("expected project escape to be rejected")
	}
	if !strings.Contains(err.Error(), "escapes the project root") {
		t.Fatalf("unexpected error: %v", err)
	}
}
