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
	cfg.SQLite.MediaRegistryPath = filepath.Join(t.TempDir(), "media_registry.db")
	cfg.SQLite.ImageGalleryPath = filepath.Join(t.TempDir(), "image_gallery.db")
	cfg.Directories.DataDir = filepath.Join(t.TempDir(), "data")
	cfg.Tools.DocumentCreator.OutputDir = filepath.Join(cfg.Directories.DataDir, "documents")
	cfg.Docker.Host = "tcp://docker.example:2375"
	cfg.VirtualDesktop.CodeStudio.Enabled = true
	cfg.VirtualDesktop.CodeStudio.Image = "custom/code-studio:test"
	cfg.VirtualDesktop.CodeStudio.AutoStopMinutes = 15
	cfg.VirtualDesktop.CodeStudio.MaxMemoryMB = 2048
	cfg.VirtualDesktop.CodeStudio.MaxCPUCores = 1

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
	if got.DataDir != cfg.Directories.DataDir {
		t.Fatalf("data dir = %q, want %q", got.DataDir, cfg.Directories.DataDir)
	}
	if got.DocumentDir != cfg.Tools.DocumentCreator.OutputDir {
		t.Fatalf("document dir = %q, want %q", got.DocumentDir, cfg.Tools.DocumentCreator.OutputDir)
	}
	if got.MediaRegistryPath != cfg.SQLite.MediaRegistryPath {
		t.Fatalf("media registry path = %q, want %q", got.MediaRegistryPath, cfg.SQLite.MediaRegistryPath)
	}
	if got.ImageGalleryPath != cfg.SQLite.ImageGalleryPath {
		t.Fatalf("image gallery path = %q, want %q", got.ImageGalleryPath, cfg.SQLite.ImageGalleryPath)
	}
	if got.MaxFileSizeMB <= 0 {
		t.Fatalf("max file size should have a safe default, got %d", got.MaxFileSizeMB)
	}
	if got.ControlLevel == "" {
		t.Fatal("control level should have a default")
	}
	if got.DockerHost != cfg.Docker.Host {
		t.Fatalf("docker host = %q, want %q", got.DockerHost, cfg.Docker.Host)
	}
	if got.CodeStudio.Image != cfg.VirtualDesktop.CodeStudio.Image {
		t.Fatalf("code studio image = %q, want %q", got.CodeStudio.Image, cfg.VirtualDesktop.CodeStudio.Image)
	}
	if got.CodeStudio.AutoStopMinutes != 15 || got.CodeStudio.MaxMemoryMB != 2048 || got.CodeStudio.MaxCPUCores != 1 {
		t.Fatalf("code studio config not preserved: %+v", got.CodeStudio)
	}
}
