package agent

import (
	"aurago/internal/config"
	"aurago/internal/tools"
)

func configureToolRuntimePermissions(cfg *config.Config) {
	if cfg == nil {
		return
	}
	packageManagerEnabled := cfg.Agent.AllowPackageManager && cfg.PackageManager.Enabled && (!cfg.Runtime.IsDocker || cfg.Agent.SudoEnabled)
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{
		AllowShell:                 cfg.Agent.AllowShell,
		AllowPython:                cfg.Agent.AllowPython,
		AllowFilesystemWrite:       cfg.Agent.AllowFilesystemWrite,
		AllowNetworkRequests:       cfg.Agent.AllowNetworkRequests,
		DockerEnabled:              cfg.Docker.Enabled,
		DockerReadOnly:             cfg.Docker.ReadOnly,
		SchedulerEnabled:           cfg.Tools.Scheduler.Enabled,
		SchedulerReadOnly:          cfg.Tools.Scheduler.ReadOnly,
		MissionsEnabled:            cfg.Tools.Missions.Enabled,
		MissionsReadOnly:           cfg.Tools.Missions.ReadOnly,
		MQTTEnabled:                cfg.MQTT.Enabled,
		MQTTReadOnly:               cfg.MQTT.ReadOnly,
		PackageManagerEnabled:      packageManagerEnabled,
		PackageManagerReadOnly:     cfg.PackageManager.ReadOnly,
		PackageManagerAllowInstall: cfg.PackageManager.AllowInstall,
		PackageManagerAllowRemove:  cfg.PackageManager.AllowRemove,
		PackageManagerAllowUpgrade: cfg.PackageManager.AllowUpgrade,
	})
}
