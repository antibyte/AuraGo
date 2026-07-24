package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGameMakerDefaultsAreSecureAndPathsAreResolved(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GameMaker.Enabled {
		t.Fatal("game maker must be disabled by default")
	}
	if !cfg.GameMaker.ReadOnly {
		t.Fatal("game maker must be read-only by default")
	}
	if cfg.GameMaker.AllowCreate || cfg.GameMaker.AllowEdit || cfg.GameMaker.AllowDelete || cfg.GameMaker.AllowMediaGeneration {
		t.Fatalf("game maker write permissions must be disabled by default: %+v", cfg.GameMaker)
	}
	if !filepath.IsAbs(cfg.GameMaker.WorkspacePath) {
		t.Fatalf("workspace path must be resolved: %q", cfg.GameMaker.WorkspacePath)
	}
	if !filepath.IsAbs(cfg.SQLite.GameMakerPath) {
		t.Fatalf("database path must be resolved: %q", cfg.SQLite.GameMakerPath)
	}
}

func TestGameMakerExplicitPermissionsRoundTripThroughLoad(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")
	const input = `
game_maker:
  enabled: true
  readonly: false
  allow_create: true
  allow_edit: true
  allow_delete: true
  allow_media_generation: true
  workspace_path: custom-games
  max_projects: 7
  max_files_per_project: 80
  max_file_size_kb: 512
  max_asset_size_mb: 12
  max_project_size_mb: 24
  job_timeout_seconds: 90
sqlite:
  game_maker_path: databases/games.db
`
	if err := os.WriteFile(configPath, []byte(input), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.GameMaker.Enabled || cfg.GameMaker.ReadOnly ||
		!cfg.GameMaker.AllowCreate || !cfg.GameMaker.AllowEdit ||
		!cfg.GameMaker.AllowDelete || !cfg.GameMaker.AllowMediaGeneration {
		t.Fatalf("explicit permissions were not loaded: %+v", cfg.GameMaker)
	}
	if got, want := cfg.GameMaker.WorkspacePath, filepath.Join(configDir, "custom-games"); got != want {
		t.Fatalf("workspace path = %q, want %q", got, want)
	}
	if got, want := cfg.SQLite.GameMakerPath, filepath.Join(configDir, "databases", "games.db"); got != want {
		t.Fatalf("database path = %q, want %q", got, want)
	}
	if cfg.GameMaker.MaxProjects != 7 || cfg.GameMaker.MaxFilesPerProject != 80 ||
		cfg.GameMaker.MaxFileSizeKB != 512 || cfg.GameMaker.MaxAssetSizeMB != 12 ||
		cfg.GameMaker.MaxProjectSizeMB != 24 ||
		cfg.GameMaker.JobTimeoutSeconds != 90 {
		t.Fatalf("limits were not loaded: %+v", cfg.GameMaker)
	}
}
