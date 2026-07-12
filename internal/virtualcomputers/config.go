package virtualcomputers

import "aurago/internal/config"

func FromAuraConfig(cfg *config.Config) ToolConfig {
	if cfg == nil {
		return ToolConfig{}
	}
	vc := cfg.VirtualComputers
	return ToolConfig{
		Enabled:   vc.Enabled,
		Provider:  vc.Provider,
		AutoSetup: vc.AutoSetup,
		ReadOnly:  vc.ReadOnly,
		ToolGate:  cfg.Tools.VirtualComputers.Enabled,
		ControlPlane: ControlPlaneConfig{
			Mode:         vc.ControlPlane.Mode,
			Host:         vc.ControlPlane.Host,
			SSHPort:      vc.ControlPlane.SSHPort,
			CredentialID: vc.ControlPlane.CredentialID,
			InstallDir:   vc.ControlPlane.InstallDir,
			BoringdURL:   vc.ControlPlane.BoringdURL,
		},
		BoringdURL:         vc.ControlPlane.BoringdURL,
		BoringToken:        vc.BoringToken,
		DefaultTemplate:    vc.DefaultTemplate,
		DefaultTTLSeconds:  vc.DefaultTTLSeconds,
		MaxTTLSeconds:      vc.MaxTTLSeconds,
		MaxRunningMachines: vc.MaxRunningMachines,
		MaxForks:           vc.MaxForks,
		AllowInternet:      vc.AllowInternet,
		AllowPersistent:    vc.AllowPersistent,
		AllowPublish:       vc.AllowPublish,
		AllowVolumes:       vc.AllowVolumes,
		AllowAgentTasks:    vc.AllowAgentTasks,
	}
}
