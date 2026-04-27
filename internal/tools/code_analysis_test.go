package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeAnalyzerExtractStructureInWorkspaceRejectsOutsidePath(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace): %v", err)
	}
	outside := filepath.Join(base, "outside.go")
	if err := os.WriteFile(outside, []byte("package main\nfunc Secret() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(outside): %v", err)
	}

	_, err := NewCodeAnalyzer().ExtractStructureInWorkspace(workspace, outside)
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("ExtractStructureInWorkspace() error = %v, want outside-workspace rejection", err)
	}
}

func TestCodeAnalyzerSymbolSearchInWorkspaceRejectsTraversal(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace): %v", err)
	}

	_, err := NewCodeAnalyzer().SymbolSearchInWorkspace(workspace, "..", "Secret")
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("SymbolSearchInWorkspace() error = %v, want outside-workspace rejection", err)
	}
}
