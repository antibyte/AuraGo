package config

import (
	"os"
	"path/filepath"
	"slices"
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

func TestGetSpecialist(t *testing.T) {
	cfg := &Config{}
	cfg.CoAgents.Specialists.Coder.Enabled = true

	tests := []struct {
		role    string
		wantNil bool
	}{
		{"researcher", false},
		{"coder", false},
		{"designer", false},
		{"security", false},
		{"writer", false},
		{"unknown", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			got := cfg.GetSpecialist(tt.role)
			if (got == nil) != tt.wantNil {
				t.Errorf("GetSpecialist(%q) nil=%v, wantNil=%v", tt.role, got == nil, tt.wantNil)
			}
		})
	}

	// Verify the coder specialist we set is actually enabled
	coder := cfg.GetSpecialist("coder")
	if coder == nil || !coder.Enabled {
		t.Error("expected coder specialist to be enabled")
	}
}

func TestValidSpecialistRoles(t *testing.T) {
	expected := []string{"researcher", "coder", "designer", "security", "writer"}
	for _, role := range expected {
		if !ValidSpecialistRoles[role] {
			t.Errorf("expected %q in ValidSpecialistRoles", role)
		}
	}
	if ValidSpecialistRoles["unknown"] {
		t.Error("unexpected role 'unknown' in ValidSpecialistRoles")
	}
}

func TestLoadUpgradesLegacyIndexingExtensions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
indexing:
  enabled: true
  directories:
    - ./knowledge
  extensions:
    - .txt
    - .md
    - .json
    - .csv
    - .log
    - .yaml
    - .yml
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	for _, want := range []string{".pdf", ".docx", ".xlsx", ".pptx", ".odt", ".rtf"} {
		if !slices.Contains(cfg.Indexing.Extensions, want) {
			t.Fatalf("expected upgraded indexing extensions to include %s, got %v", want, cfg.Indexing.Extensions)
		}
	}
}

func TestLoadKeepsCustomIndexingExtensions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
indexing:
  enabled: true
  directories:
    - ./knowledge
  extensions:
    - .txt
    - .md
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Indexing.Extensions) != 2 || cfg.Indexing.Extensions[0] != ".txt" || cfg.Indexing.Extensions[1] != ".md" {
		t.Fatalf("expected custom indexing extensions to stay unchanged, got %v", cfg.Indexing.Extensions)
	}
}

func TestLoadBudgetAdaptiveLimitDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
budget:
  enabled: true
  daily_limit_usd: 5
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if !cfg.Budget.AdaptiveLimit.Enabled {
		t.Fatal("expected budget.adaptive_limit.enabled to default to true")
	}
	if cfg.Budget.AdaptiveLimit.Strategy != "capability_weighted" {
		t.Fatalf("adaptive strategy = %q, want capability_weighted", cfg.Budget.AdaptiveLimit.Strategy)
	}
	if cfg.Budget.AdaptiveLimit.MinMultiplier != 1.0 {
		t.Fatalf("min multiplier = %v, want 1.0", cfg.Budget.AdaptiveLimit.MinMultiplier)
	}
	if cfg.Budget.AdaptiveLimit.MaxMultiplier != 2.5 {
		t.Fatalf("max multiplier = %v, want 2.5", cfg.Budget.AdaptiveLimit.MaxMultiplier)
	}
}

func TestLoadBudgetAdaptiveLimitExplicitDisablePreserved(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
budget:
  enabled: true
  daily_limit_usd: 5
  adaptive_limit:
    enabled: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Budget.AdaptiveLimit.Enabled {
		t.Fatal("expected explicit adaptive_limit.enabled=false to be preserved")
	}
}
