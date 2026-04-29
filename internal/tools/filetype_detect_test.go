package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDetectFileTypeInWorkspaceRejectsProjectEscape(t *testing.T) {
	projectRoot := t.TempDir()
	workspaceDir := filepath.Join(projectRoot, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll workspace: %v", err)
	}
	outside := filepath.Join(filepath.Dir(projectRoot), "outside.bin")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatalf("WriteFile outside: %v", err)
	}
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspaceDir

	raw := DetectFileTypeInWorkspace(filepath.Join("..", "..", "..", "outside.bin"), false, cfg)

	var result fileTypeResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "error" || len(result.Files) == 0 || !strings.Contains(result.Files[0].Error, "path traversal") {
		t.Fatalf("result = %+v, want project escape error", result)
	}
}

func TestDetectFileTypeInWorkspaceAllowsWorkspaceFile(t *testing.T) {
	projectRoot := t.TempDir()
	workspaceDir := filepath.Join(projectRoot, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "sample.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile sample: %v", err)
	}
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspaceDir

	raw := DetectFileTypeInWorkspace("sample.txt", false, cfg)

	var result fileTypeResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" || result.Total != 1 || result.Files[0].MIME == "" {
		t.Fatalf("result = %+v, want one detected file", result)
	}
}
