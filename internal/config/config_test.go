package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAbsolutePaths(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	workspacePath := "/tmp/workspace"
	if os.PathSeparator == '\\' {
		workspacePath = "C:\\absolute\\path\\workspace"
	}

	configContent := `
directories:
  data_dir: './data'
  workspace_dir: '` + workspacePath + `'
  skills_dir: '../skills'
sqlite:
  short_term_path: './data/short_term.db'
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Calculate expected paths
	absConfigDir, _ := filepath.Abs(tmpDir)
	expectedDataDir := filepath.Join(absConfigDir, "./data")
	expectedWorkspaceDir := workspacePath
	expectedSkillsDir := filepath.Join(absConfigDir, "../skills")
	expectedShortTermPath := filepath.Join(absConfigDir, "./data/short_term.db")

	if cfg.Directories.DataDir != expectedDataDir {
		t.Errorf("expected DataDir %s, got %s", expectedDataDir, cfg.Directories.DataDir)
	}
	if cfg.Directories.WorkspaceDir != expectedWorkspaceDir {
		t.Errorf("expected WorkspaceDir %s, got %s", expectedWorkspaceDir, cfg.Directories.WorkspaceDir)
	}
	if cfg.Directories.SkillsDir != expectedSkillsDir {
		t.Errorf("expected SkillsDir %s, got %s", expectedSkillsDir, cfg.Directories.SkillsDir)
	}
	if cfg.SQLite.ShortTermPath != expectedShortTermPath {
		t.Errorf("expected ShortTermPath %s, got %s", expectedShortTermPath, cfg.SQLite.ShortTermPath)
	}
}
