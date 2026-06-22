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
			MaxFileSizeMB:            50,
			ControlLevel:             ControlConfirmDestructive,
			MaxWSClients:             8,
			RemoteMaxSessionMinutes:  60,
			RemoteIdleTimeoutMinutes: 5,
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
	remoteMaxSessionMinutes := desktopCfg.RemoteMaxSessionMinutes
	if remoteMaxSessionMinutes <= 0 {
		remoteMaxSessionMinutes = 60
	}
	remoteIdleTimeoutMinutes := desktopCfg.RemoteIdleTimeoutMinutes
	if remoteIdleTimeoutMinutes <= 0 {
		remoteIdleTimeoutMinutes = 5
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
		Enabled:                  desktopCfg.Enabled,
		ReadOnly:                 desktopCfg.ReadOnly,
		AllowAgentControl:        desktopCfg.AllowAgentControl,
		AllowGeneratedApps:       desktopCfg.AllowGeneratedApps,
		AllowPythonJobs:          desktopCfg.AllowPythonJobs,
		WorkspaceDir:             workspaceDir,
		DockerHost:               strings.TrimSpace(cfg.Docker.Host),
		DBPath:                   dbPath,
		DataDir:                  dataDir,
		DocumentDir:              documentDir,
		MediaRegistryPath:        strings.TrimSpace(cfg.SQLite.MediaRegistryPath),
		ImageGalleryPath:         strings.TrimSpace(cfg.SQLite.ImageGalleryPath),
		MaxFileSizeMB:            maxFileSizeMB,
		ControlLevel:             controlLevel,
		MaxWSClients:             maxWSClients,
		RemoteMaxSessionMinutes:  remoteMaxSessionMinutes,
		RemoteIdleTimeoutMinutes: remoteIdleTimeoutMinutes,
		CodeStudio: CodeStudioConfig{
			Enabled:         desktopCfg.CodeStudio.Enabled,
			Image:           strings.TrimSpace(desktopCfg.CodeStudio.Image),
			AutoStart:       desktopCfg.CodeStudio.AutoStart,
			AutoStopMinutes: desktopCfg.CodeStudio.AutoStopMinutes,
			MaxMemoryMB:     desktopCfg.CodeStudio.MaxMemoryMB,
			MaxCPUCores:     desktopCfg.CodeStudio.MaxCPUCores,
		},
		OpenSCAD: OpenSCADConfig{
			Enabled:                 desktopCfg.OpenSCAD.Enabled,
			Image:                   strings.TrimSpace(desktopCfg.OpenSCAD.Image),
			AutoStart:               desktopCfg.OpenSCAD.AutoStart,
			AutoStopMinutes:         desktopCfg.OpenSCAD.AutoStopMinutes,
			MaxMemoryMB:             desktopCfg.OpenSCAD.MaxMemoryMB,
			MaxCPUCores:             desktopCfg.OpenSCAD.MaxCPUCores,
			MaxConcurrentJobs:       desktopCfg.OpenSCAD.MaxConcurrentJobs,
			GeometryBackend:         strings.TrimSpace(desktopCfg.OpenSCAD.GeometryBackend),
			DefaultExports:          append([]string(nil), desktopCfg.OpenSCAD.DefaultExports...),
			MaxSourceKB:             desktopCfg.OpenSCAD.MaxSourceKB,
			MaxOutputMB:             desktopCfg.OpenSCAD.MaxOutputMB,
			RenderTimeoutSeconds:    desktopCfg.OpenSCAD.RenderTimeoutSeconds,
			MaxRenderTimeoutSeconds: desktopCfg.OpenSCAD.MaxRenderTimeoutSeconds,
			JobRetentionDays:        desktopCfg.OpenSCAD.JobRetentionDays,
		},
	}
}
