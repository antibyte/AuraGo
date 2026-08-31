package virtualcomputers

import (
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"
)

func FromAuraConfig(cfg *config.Config) ToolConfig {
	if cfg == nil {
		return ToolConfig{}
	}
	vc := cfg.VirtualComputers
	anthropicKey := ""
	openRouterKey := ""
	if vc.AllowAgentTasks {
		providerID := strings.TrimSpace(vc.AgentProvider)
		if providerID == "" {
			// Preserve installations configured before provider references were added.
			anthropicKey = vc.BoringAnthropicKey
			openRouterKey = vc.BoringOpenRouterKey
		} else if provider := cfg.FindProvider(providerID); provider != nil &&
			strings.EqualFold(strings.TrimSpace(provider.Type), "anthropic") &&
			!strings.EqualFold(strings.TrimSpace(provider.AuthType), "oauth2") {
			anthropicKey = provider.APIKey
		}
	}
	storage := EffectiveStorageFromConfig(vc)
	accessKey, secretKey := EffectiveCredentials(vc)
	for _, value := range []string{
		anthropicKey, openRouterKey,
		vc.S3AccessKeyID, vc.S3SecretKey,
		vc.GarageAccessKeyID, vc.GarageSecretKey, vc.GarageRPCSecret,
		accessKey, secretKey,
	} {
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
		Storage:             storage,
		LedgerPath:          cfg.SQLite.VirtualComputersPath,
		BoringdURL:          vc.ControlPlane.BoringdURL,
		BoringToken:         vc.BoringToken,
		BoringAnthropicKey:  anthropicKey,
		BoringOpenRouterKey: openRouterKey,
		S3AccessKeyID:       accessKey,
		S3SecretKey:         secretKey,
		GarageRPCSecret:     vc.GarageRPCSecret,
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
		AgentControl: AgentControlConfig{
			Enabled:                     vc.AgentControl.Enabled,
			DefaultTemplate:             vc.AgentControl.DefaultTemplate,
			MaxActiveWorkspaces:         vc.AgentControl.MaxActiveWorkspaces,
			IdleTTLSeconds:              vc.AgentControl.IdleTTLSeconds,
			MaxWorkspaceSeconds:         vc.AgentControl.MaxWorkspaceSeconds,
			MaxJobSeconds:               vc.AgentControl.MaxJobSeconds,
			MaxJobOutputBytes:           int64(vc.AgentControl.MaxJobOutputMB) * 1024 * 1024,
			JobsPerWorkspace:            vc.AgentControl.JobsPerWorkspace,
			BrowserSessionsPerWorkspace: vc.AgentControl.BrowserSessionsPerWorkspace,
			NetworkProfile:              vc.AgentControl.Network.DefaultProfile,
			AllowedPrivateCIDRs:         append([]string(nil), vc.AgentControl.Network.AllowedPrivateCIDRs...),
			CredentialsEnabled:          vc.AgentControl.Credentials.Enabled,
			CredentialGrantTTLSeconds:   vc.AgentControl.Credentials.GrantTTLSeconds,
		},
	}
}
