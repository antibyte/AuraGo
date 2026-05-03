package desktop

import (
	"path/filepath"
	"strings"

	"aurago/internal/config"
)

// ConfigFromAuraConfig extracts the desktop service settings from AuraGo config.
func ConfigFromAuraConfig(cfg *config.Config) Config {
	if cfg == nil {
		return Config{
			MaxFileSizeMB: 50,
			ControlLevel:  ControlConfirmDestructive,
			MaxWSClients:  8,
		}
	}
	desktopCfg := cfg.VirtualDesktop
	maxFileSizeMB := desktopCfg.MaxFileSizeMB
	if maxFileSizeMB <= 0 {
		maxFileSizeMB = 50
	}
	controlLevel := strings.TrimSpace(desktopCfg.ControlLevel)
	if controlLevel == "" {
		controlLevel = ControlConfirmDestructive
	}
	maxWSClients := desktopCfg.MaxWSClients
	if maxWSClients <= 0 {
		maxWSClients = 8
	}
	workspaceDir := strings.TrimSpace(desktopCfg.WorkspaceDir)
	if workspaceDir == "" {
		workspaceDir = filepath.Join(cfg.Directories.WorkspaceDir, "virtual_desktop")
	}
	dbPath := strings.TrimSpace(cfg.SQLite.VirtualDesktopPath)
	if dbPath == "" {
		dbPath = filepath.Join(cfg.Directories.DataDir, "virtual_desktop.db")
	}
	dataDir := strings.TrimSpace(cfg.Directories.DataDir)
	if dataDir == "" {
		dataDir = "data"
	}
	documentDir := strings.TrimSpace(cfg.Tools.DocumentCreator.OutputDir)
	if documentDir == "" {
		documentDir = filepath.Join(dataDir, "documents")
	}
	return Config{
		Enabled:            desktopCfg.Enabled,
		ReadOnly:           desktopCfg.ReadOnly,
		AllowAgentControl:  desktopCfg.AllowAgentControl,
		AllowGeneratedApps: desktopCfg.AllowGeneratedApps,
		AllowPythonJobs:    desktopCfg.AllowPythonJobs,
		WorkspaceDir:       workspaceDir,
		DBPath:             dbPath,
		DataDir:            dataDir,
		DocumentDir:        documentDir,
		MediaRegistryPath:  strings.TrimSpace(cfg.SQLite.MediaRegistryPath),
		ImageGalleryPath:   strings.TrimSpace(cfg.SQLite.ImageGalleryPath),
		MaxFileSizeMB:      maxFileSizeMB,
		ControlLevel:       controlLevel,
		MaxWSClients:       maxWSClients,
	}
}
