package agent

import (
	"aurago/internal/config"
	"aurago/internal/tools"
)

func configureToolRuntimePermissions(cfg *config.Config) {
	if cfg == nil {
		return
	}
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{
		AllowShell:           cfg.Agent.AllowShell,
		AllowPython:          cfg.Agent.AllowPython,
		AllowFilesystemWrite: cfg.Agent.AllowFilesystemWrite,
		AllowNetworkRequests: cfg.Agent.AllowNetworkRequests,
		DockerEnabled:        cfg.Docker.Enabled,
		SchedulerEnabled:     cfg.Tools.Scheduler.Enabled,
		SchedulerReadOnly:    cfg.Tools.Scheduler.ReadOnly,
	})
}
