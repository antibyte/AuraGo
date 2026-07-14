package virtualcomputers

import (
	"aurago/internal/config"
	"aurago/internal/security"
)

func FromAuraConfig(cfg *config.Config) ToolConfig {
	if cfg == nil {
		return ToolConfig{}
	}
	vc := cfg.VirtualComputers
	for _, value := range []string{vc.S3AccessKeyID, vc.S3SecretKey} {
		security.RegisterSensitive(value)
	}
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
		Storage: StorageConfig{
			Endpoint: vc.Storage.Endpoint,
			Bucket:   vc.Storage.Bucket,
			Region:   vc.Storage.Region,
			UseSSL:   vc.Storage.UseSSL,
		},
		LedgerPath:          cfg.SQLite.VirtualComputersPath,
		BoringdURL:          vc.ControlPlane.BoringdURL,
		BoringToken:         vc.BoringToken,
		BoringAnthropicKey:  vc.BoringAnthropicKey,
		BoringOpenRouterKey: vc.BoringOpenRouterKey,
		S3AccessKeyID:       vc.S3AccessKeyID,
		S3SecretKey:         vc.S3SecretKey,
		DefaultTemplate:     vc.DefaultTemplate,
		DefaultTTLSeconds:   vc.DefaultTTLSeconds,
		MaxTTLSeconds:       vc.MaxTTLSeconds,
		MaxRunningMachines:  vc.MaxRunningMachines,
		MaxForks:            vc.MaxForks,
		AllowInternet:       vc.AllowInternet,
		AllowPersistent:     vc.AllowPersistent,
		AllowPublish:        vc.AllowPublish,
		AllowVolumes:        vc.AllowVolumes,
		AllowAgentTasks:     vc.AllowAgentTasks,
	}
}
