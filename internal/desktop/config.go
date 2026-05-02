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
	return Config{
		Enabled:            desktopCfg.Enabled,
		ReadOnly:           desktopCfg.ReadOnly,
		AllowAgentControl:  desktopCfg.AllowAgentControl,
		AllowGeneratedApps: desktopCfg.AllowGeneratedApps,
		AllowPythonJobs:    desktopCfg.AllowPythonJobs,
		WorkspaceDir:       workspaceDir,
		DBPath:             dbPath,
		MaxFileSizeMB:      maxFileSizeMB,
		ControlLevel:       controlLevel,
		MaxWSClients:       maxWSClients,
	}
}
