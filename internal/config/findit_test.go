package config

import (
	"gopkg.in/yaml.v3"
	"path/filepath"
	"slices"
	"testing"
)

func TestFindItDefaultsAndBackupRegistration(t *testing.T) {
	var cfg Config
	if cfg.FindIt.Enabled || cfg.FindIt.AgentAccess {
		t.Fatal("FindIt must be opt-in")
	}
	if err := yaml.Unmarshal([]byte("findit:\n  enabled: true\n  agent_access: true\n  readonly: true\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.FindIt.Enabled || !cfg.FindIt.AgentAccess || !cfg.FindIt.ReadOnly {
		t.Fatal("FindIt policy did not round-trip")
	}
	cfg.Directories.DataDir = t.TempDir()
	if !slices.Contains(SQLiteDatabasePaths(&cfg), filepath.Join(cfg.Directories.DataDir, "findit.db")) {
		t.Fatal("FindIt database omitted from backups")
	}
	if cfg.Clone().FindIt != cfg.FindIt {
		t.Fatal("FindIt policy not cloned")
	}
}
