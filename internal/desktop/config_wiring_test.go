package desktop

import (
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func TestConfigFromAuraConfigResolvesDesktopDefaults(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join(t.TempDir(), "virtual_desktop")
	cfg.SQLite.VirtualDesktopPath = filepath.Join(t.TempDir(), "virtual_desktop.db")

	got := ConfigFromAuraConfig(cfg)
	if !got.Enabled || !got.AllowAgentControl {
		t.Fatalf("feature flags not preserved: %+v", got)
	}
	if got.WorkspaceDir != cfg.VirtualDesktop.WorkspaceDir {
		t.Fatalf("workspace = %q, want %q", got.WorkspaceDir, cfg.VirtualDesktop.WorkspaceDir)
	}
	if got.DBPath != cfg.SQLite.VirtualDesktopPath {
		t.Fatalf("db path = %q, want %q", got.DBPath, cfg.SQLite.VirtualDesktopPath)
	}
	if got.MaxFileSizeMB <= 0 {
		t.Fatalf("max file size should have a safe default, got %d", got.MaxFileSizeMB)
	}
	if got.ControlLevel == "" {
		t.Fatal("control level should have a default")
	}
}
