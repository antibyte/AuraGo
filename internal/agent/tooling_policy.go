package agent

import (
	"strings"

	"aurago/internal/config"
	"aurago/internal/prompts"
)

// ModelCapabilities describes provider/model-specific behavior that affects
// tool calling and prompt construction. It centralizes compatibility quirks so
// the agent loop does not need to hardcode them inline.
type ModelCapabilities struct {
	ProviderType              string
	Model                     string
	IsOllama                  bool
	IsDeepSeek                bool
	AutoEnableNativeFunctions bool
	SupportsStructuredOutputs bool
	SupportsParallelToolCalls bool
}

// ToolingPolicy resolves the effective runtime behavior after combining user
// config with model/provider capabilities.
type ToolingPolicy struct {
	Capabilities               ModelCapabilities
	TelemetryScope             AgentTelemetryScope
	TelemetryProfile           string
	TelemetrySnapshot          AgentTelemetryScopeSnapshot
	IntentFamily               string
	FamilyTelemetry            AgentTelemetryToolFamilySnapshot
	UseNativeFunctions         bool
	AutoEnabledNativeFunctions bool
	StructuredOutputsRequested bool
	StructuredOutputsEnabled   bool
	ParallelToolCallsEnabled   bool
	DockerEnabled              bool
	SandboxEnabled             bool
	HomepageEnabled            bool
	HomepageAllowLocalServer   bool
	WOLEnabled                 bool
	EffectiveMaxToolGuides     int
	EffectiveGuideStrategy     prompts.DynamicGuideStrategy
}

type promptContextOptions struct {
	IsErrorState          bool
	RequiresCoding        bool
	IsMaintenanceMode     bool
	SurgeryPlan           string
	WebhooksDefinitions   string
	ActiveProcesses       string
	SpecialistsAvailable  bool
	SpecialistsStatus     string
	SpecialistsSuggestion string
}

func resolveModelCapabilities(cfg *config.Config) ModelCapabilities {
	if cfg == nil {
		return ModelCapabilities{}
	}
	providerType := strings.TrimSpace(cfg.LLM.ProviderType)
	model := strings.TrimSpace(cfg.LLM.Model)
	lowerProvider := strings.ToLower(providerType)
	lowerModel := strings.ToLower(model)
	isOllama := lowerProvider == "ollama"
	isDeepSeek := strings.Contains(lowerModel, "deepseek")

	return ModelCapabilities{
		ProviderType:              providerType,
		Model:                     model,
		IsOllama:                  isOllama,
		IsDeepSeek:                isDeepSeek,
		AutoEnableNativeFunctions: isDeepSeek,
		SupportsStructuredOutputs: !isOllama,
		SupportsParallelToolCalls: !isOllama,
	}
}

func buildToolingPolicy(cfg *config.Config, userQuery string) ToolingPolicy {
	caps := resolveModelCapabilities(cfg)
	if cfg == nil {
		return ToolingPolicy{Capabilities: caps}
	}
	scope := AgentTelemetryScope{
		ProviderType: caps.ProviderType,
		Model:        caps.Model,
	}
	scopedTelemetry, _ := GetScopedAgentTelemetrySnapshot(scope)

	dockerEnabled := cfg.Docker.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK)
	sandboxEnabled := cfg.Sandbox.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK)
	homepageEnabled := cfg.Homepage.Enabled && (dockerEnabled || cfg.Homepage.AllowLocalServer)
	wolEnabled := cfg.Tools.WOL.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.BroadcastOK)

	useNativeFunctions := cfg.LLM.UseNativeFunctions || caps.AutoEnableNativeFunctions
	effectiveMaxToolGuides := cfg.Agent.MaxToolGuides
	if effectiveMaxToolGuides <= 0 {
		effectiveMaxToolGuides = 5
	}
	telemetryProfile := "default"
	guideStrategy := prompts.DynamicGuideStrategy{}
	intentFamily := inferToolFamilyFromQuery(userQuery)
	familyTelemetry := scopedTelemetry.ToolFamilies[intentFamily]
	if scopedTelemetry.ToolCalls >= 8 && scopedTelemetry.FailureRate >= 0.45 {
		telemetryProfile = "conservative"
		guideStrategy = prompts.DynamicGuideStrategy{
			PreferSemantics:              true,
			DisableStatisticalHeuristics: true,
			DisableFrequencyHeuristics:   true,
		}
		if effectiveMaxToolGuides > 2 {
			effectiveMaxToolGuides -= 2
		}
		if effectiveMaxToolGuides < 2 {
			effectiveMaxToolGuides = 2
		}
	} else if intentFamily != "" && familyTelemetry.ToolCalls >= 4 && familyTelemetry.FailureRate >= 0.5 {
		telemetryProfile = "family_guarded"
		guideStrategy = prompts.DynamicGuideStrategy{
			PreferSemantics:              true,
			DisableStatisticalHeuristics: true,
			DisableFrequencyHeuristics:   true,
		}
		if effectiveMaxToolGuides > 3 {
			effectiveMaxToolGuides--
		}
	}

	return ToolingPolicy{
		Capabilities:               caps,
		TelemetryScope:             scope,
		TelemetryProfile:           telemetryProfile,
		TelemetrySnapshot:          scopedTelemetry,
		IntentFamily:               intentFamily,
		FamilyTelemetry:            familyTelemetry,
		UseNativeFunctions:         useNativeFunctions,
		AutoEnabledNativeFunctions: !cfg.LLM.UseNativeFunctions && caps.AutoEnableNativeFunctions,
		StructuredOutputsRequested: cfg.LLM.StructuredOutputs,
		StructuredOutputsEnabled:   cfg.LLM.StructuredOutputs && caps.SupportsStructuredOutputs,
		ParallelToolCallsEnabled:   caps.SupportsParallelToolCalls,
		DockerEnabled:              dockerEnabled,
		SandboxEnabled:             sandboxEnabled,
		HomepageEnabled:            homepageEnabled,
		HomepageAllowLocalServer:   cfg.Homepage.AllowLocalServer,
		WOLEnabled:                 wolEnabled,
		EffectiveMaxToolGuides:     effectiveMaxToolGuides,
		EffectiveGuideStrategy:     guideStrategy,
	}
}

func applyTelemetryAwarePromptTier(policy ToolingPolicy, flags prompts.ContextFlags, baseTier string) string {
	if policy.TelemetryProfile != "conservative" && policy.TelemetryProfile != "family_guarded" {
		return baseTier
	}
	if baseTier != "full" {
		return baseTier
	}
	if flags.IsMission || flags.IsErrorState || flags.RequiresCoding {
		return baseTier
	}
	if len(flags.PredictedGuides) > 0 {
		return baseTier
	}
	if flags.MessageCount < 5 {
		return baseTier
	}
	return "compact"
}

func buildPromptContextFlags(runCfg RunConfig, policy ToolingPolicy, opts promptContextOptions) prompts.ContextFlags {
	cfg := runCfg.Config
	if cfg == nil {
		return prompts.ContextFlags{}
	}

	return prompts.ContextFlags{
		ActiveProcesses:          opts.ActiveProcesses,
		IsErrorState:             opts.IsErrorState,
		RequiresCoding:           opts.RequiresCoding,
		SystemLanguage:           cfg.Agent.SystemLanguage,
		LifeboatEnabled:          cfg.Maintenance.LifeboatEnabled,
		IsMaintenanceMode:        opts.IsMaintenanceMode,
		SurgeryPlan:              opts.SurgeryPlan,
		CorePersonality:          cfg.Personality.CorePersonality,
		TokenBudget:              config.CalculateAdaptiveSystemPromptTokenBudget(cfg),
		IsDebugMode:              cfg.Agent.DebugMode || GetDebugMode(),
		IsCoAgent:                isCoAgentSession(runCfg.SessionID),
		DiscordEnabled:           cfg.Discord.Enabled,
		EmailEnabled:             cfg.Email.Enabled || len(cfg.EmailAccounts) > 0,
		DockerEnabled:            policy.DockerEnabled,
		HomeAssistantEnabled:     cfg.HomeAssistant.Enabled,
		WebDAVEnabled:            cfg.WebDAV.Enabled,
		KoofrEnabled:             cfg.Koofr.Enabled,
		PaperlessNGXEnabled:      cfg.PaperlessNGX.Enabled,
		ChromecastEnabled:        cfg.Chromecast.Enabled,
		CoAgentEnabled:           cfg.CoAgents.Enabled,
		GoogleWorkspaceEnabled:   cfg.GoogleWorkspace.Enabled,
		OneDriveEnabled:          cfg.OneDrive.Enabled,
		JellyfinEnabled:          cfg.Jellyfin.Enabled,
		ProxmoxEnabled:           cfg.Proxmox.Enabled,
		OllamaEnabled:            cfg.Ollama.Enabled,
		TailscaleEnabled:         cfg.Tailscale.Enabled,
		CloudflareTunnelEnabled:  cfg.CloudflareTunnel.Enabled,
		AnsibleEnabled:           cfg.Ansible.Enabled,
		InvasionControlEnabled:   cfg.InvasionControl.Enabled && runCfg.InvasionDB != nil,
		GitHubEnabled:            cfg.GitHub.Enabled,
		MQTTEnabled:              cfg.MQTT.Enabled,
		AdGuardEnabled:           cfg.AdGuard.Enabled,
		MCPEnabled:               cfg.MCP.Enabled && cfg.Agent.AllowMCP,
		SandboxEnabled:           policy.SandboxEnabled,
		MeshCentralEnabled:       cfg.MeshCentral.Enabled,
		HomepageEnabled:          policy.HomepageEnabled,
		HomepageAllowLocalServer: policy.HomepageAllowLocalServer,
		NetlifyEnabled:           cfg.Netlify.Enabled,
		WebhooksEnabled:          cfg.Webhooks.Enabled,
		WebhooksDefinitions:      opts.WebhooksDefinitions,
		VirusTotalEnabled:        cfg.VirusTotal.Enabled,
		BraveSearchEnabled:       cfg.BraveSearch.Enabled,
		ImageGenerationEnabled:   cfg.ImageGeneration.Enabled,
		RemoteControlEnabled:     cfg.RemoteControl.Enabled && runCfg.RemoteHub != nil,
		MemoryEnabled:            cfg.Tools.Memory.Enabled,
		KnowledgeGraphEnabled:    cfg.Tools.KnowledgeGraph.Enabled,
		SecretsVaultEnabled:      cfg.Tools.SecretsVault.Enabled,
		SchedulerEnabled:         cfg.Tools.Scheduler.Enabled,
		NotesEnabled:             cfg.Tools.Notes.Enabled,
		JournalEnabled:           cfg.Tools.Journal.Enabled,
		MissionsEnabled:          cfg.Tools.Missions.Enabled,
		StopProcessEnabled:       cfg.Tools.StopProcess.Enabled,
		InventoryEnabled:         cfg.Tools.Inventory.Enabled,
		MemoryMaintenanceEnabled: cfg.Tools.MemoryMaintenance.Enabled,
		WOLEnabled:               policy.WOLEnabled,
		MediaRegistryEnabled:     cfg.MediaRegistry.Enabled && runCfg.MediaRegistryDB != nil,
		HomepageRegistryEnabled:  cfg.Homepage.Enabled && runCfg.HomepageRegistryDB != nil,
		AllowShell:               cfg.Agent.AllowShell,
		AllowPython:              cfg.Agent.AllowPython,
		AllowFilesystemWrite:     cfg.Agent.AllowFilesystemWrite,
		AllowNetworkRequests:     cfg.Agent.AllowNetworkRequests,
		AllowRemoteShell:         cfg.Agent.AllowRemoteShell,
		AllowSelfUpdate:          cfg.Agent.AllowSelfUpdate,
		IsEgg:                    cfg.EggMode.Enabled,
		InternetExposed:          cfg.Server.HTTPS.Enabled,
		IsDocker:                 cfg.Runtime.IsDocker,
		DocumentCreatorEnabled:   cfg.Tools.DocumentCreator.Enabled,
		WebCaptureEnabled:        cfg.Tools.WebCapture.Enabled,
		NetworkPingEnabled:       cfg.Tools.NetworkPing.Enabled,
		WebScraperEnabled:        cfg.Tools.WebScraper.Enabled,
		S3Enabled:                cfg.S3.Enabled,
		NetworkScanEnabled:       cfg.Tools.NetworkScan.Enabled,
		FormAutomationEnabled:    cfg.Tools.FormAutomation.Enabled,
		UPnPScanEnabled:          cfg.Tools.UPnPScan.Enabled,
		FritzBoxSystemEnabled:    cfg.FritzBox.Enabled && cfg.FritzBox.System.Enabled,
		FritzBoxNetworkEnabled:   cfg.FritzBox.Enabled && cfg.FritzBox.Network.Enabled,
		FritzBoxTelephonyEnabled: cfg.FritzBox.Enabled && cfg.FritzBox.Telephony.Enabled,
		FritzBoxSmartHomeEnabled: cfg.FritzBox.Enabled && cfg.FritzBox.SmartHome.Enabled,
		FritzBoxStorageEnabled:   cfg.FritzBox.Enabled && cfg.FritzBox.Storage.Enabled,
		FritzBoxTVEnabled:        cfg.FritzBox.Enabled && cfg.FritzBox.TV.Enabled,
		A2AEnabled:               cfg.A2A.Server.Enabled || cfg.A2A.Client.Enabled,
		TelnyxEnabled:            cfg.Telnyx.Enabled,
		AdditionalPrompt:         cfg.Agent.AdditionalPrompt,
		MessageSource:            runCfg.MessageSource,
		ToolsDir:                 cfg.Directories.ToolsDir,
		SkillsDir:                cfg.Directories.SkillsDir,
		SpecialistsAvailable:     opts.SpecialistsAvailable,
		SpecialistsStatus:        opts.SpecialistsStatus,
		SpecialistsSuggestion:    opts.SpecialistsSuggestion,
	}
}

func buildToolFeatureFlags(runCfg RunConfig, policy ToolingPolicy) ToolFeatureFlags {
	cfg := runCfg.Config
	if cfg == nil {
		return ToolFeatureFlags{}
	}

	return ToolFeatureFlags{
		HomeAssistantEnabled:         cfg.HomeAssistant.Enabled,
		DockerEnabled:                policy.DockerEnabled,
		CoAgentEnabled:               cfg.CoAgents.Enabled,
		SudoEnabled:                  cfg.Agent.SudoEnabled && !cfg.Runtime.IsDocker,
		WebhooksEnabled:              cfg.Webhooks.Enabled,
		JellyfinEnabled:              cfg.Jellyfin.Enabled,
		ProxmoxEnabled:               cfg.Proxmox.Enabled,
		OllamaEnabled:                cfg.Ollama.Enabled,
		TailscaleEnabled:             cfg.Tailscale.Enabled,
		AnsibleEnabled:               cfg.Ansible.Enabled,
		InvasionControlEnabled:       cfg.InvasionControl.Enabled && runCfg.InvasionDB != nil,
		GitHubEnabled:                cfg.GitHub.Enabled,
		MQTTEnabled:                  cfg.MQTT.Enabled,
		AdGuardEnabled:               cfg.AdGuard.Enabled,
		MCPEnabled:                   cfg.MCP.Enabled && cfg.Agent.AllowMCP,
		SandboxEnabled:               policy.SandboxEnabled,
		MeshCentralEnabled:           cfg.MeshCentral.Enabled,
		HomepageEnabled:              policy.HomepageEnabled,
		HomepageAllowLocalServer:     policy.HomepageAllowLocalServer,
		NetlifyEnabled:               cfg.Netlify.Enabled,
		FirewallEnabled:              cfg.Firewall.Enabled && cfg.Runtime.FirewallAccessOK,
		EmailEnabled:                 cfg.Email.Enabled || len(cfg.EmailAccounts) > 0,
		CloudflareTunnelEnabled:      cfg.CloudflareTunnel.Enabled,
		GoogleWorkspaceEnabled:       cfg.GoogleWorkspace.Enabled,
		OneDriveEnabled:              cfg.OneDrive.Enabled,
		VirusTotalEnabled:            cfg.VirusTotal.Enabled,
		ImageGenerationEnabled:       cfg.ImageGeneration.Enabled,
		RemoteControlEnabled:         cfg.RemoteControl.Enabled && runCfg.RemoteHub != nil,
		MemoryEnabled:                cfg.Tools.Memory.Enabled,
		KnowledgeGraphEnabled:        cfg.Tools.KnowledgeGraph.Enabled,
		SecretsVaultEnabled:          cfg.Tools.SecretsVault.Enabled,
		SchedulerEnabled:             cfg.Tools.Scheduler.Enabled,
		NotesEnabled:                 cfg.Tools.Notes.Enabled,
		JournalEnabled:               cfg.Tools.Journal.Enabled,
		MissionsEnabled:              cfg.Tools.Missions.Enabled,
		StopProcessEnabled:           cfg.Tools.StopProcess.Enabled,
		InventoryEnabled:             cfg.Tools.Inventory.Enabled,
		MemoryMaintenanceEnabled:     cfg.Tools.MemoryMaintenance.Enabled,
		WOLEnabled:                   policy.WOLEnabled,
		MediaRegistryEnabled:         cfg.MediaRegistry.Enabled && runCfg.MediaRegistryDB != nil,
		HomepageRegistryEnabled:      cfg.Homepage.Enabled && runCfg.HomepageRegistryDB != nil,
		ContactsEnabled:              cfg.Tools.Contacts.Enabled && runCfg.ContactsDB != nil,
		MemoryAnalysisEnabled:        cfg.MemoryAnalysis.Enabled,
		DocumentCreatorEnabled:       cfg.Tools.DocumentCreator.Enabled,
		WebCaptureEnabled:            cfg.Tools.WebCapture.Enabled,
		NetworkPingEnabled:           cfg.Tools.NetworkPing.Enabled,
		WebScraperEnabled:            cfg.Tools.WebScraper.Enabled,
		S3Enabled:                    cfg.S3.Enabled,
		NetworkScanEnabled:           cfg.Tools.NetworkScan.Enabled,
		FormAutomationEnabled:        cfg.Tools.FormAutomation.Enabled,
		UPnPScanEnabled:              cfg.Tools.UPnPScan.Enabled,
		FritzBoxSystemEnabled:        cfg.FritzBox.Enabled && cfg.FritzBox.System.Enabled,
		FritzBoxNetworkEnabled:       cfg.FritzBox.Enabled && cfg.FritzBox.Network.Enabled,
		FritzBoxTelephonyEnabled:     cfg.FritzBox.Enabled && cfg.FritzBox.Telephony.Enabled,
		FritzBoxSmartHomeEnabled:     cfg.FritzBox.Enabled && cfg.FritzBox.SmartHome.Enabled,
		FritzBoxStorageEnabled:       cfg.FritzBox.Enabled && cfg.FritzBox.Storage.Enabled,
		FritzBoxTVEnabled:            cfg.FritzBox.Enabled && cfg.FritzBox.TV.Enabled,
		TelnyxSMSEnabled:             cfg.Telnyx.Enabled && !cfg.Telnyx.ReadOnly,
		TelnyxCallEnabled:            cfg.Telnyx.Enabled && !cfg.Telnyx.ReadOnly,
		SQLConnectionsEnabled:        cfg.SQLConnections.Enabled && runCfg.SQLConnectionsDB != nil,
		PythonSecretInjectionEnabled: cfg.Tools.PythonSecretInjection.Enabled,
		AllowShell:                   cfg.Agent.AllowShell,
		AllowPython:                  cfg.Agent.AllowPython,
		AllowFilesystemWrite:         cfg.Agent.AllowFilesystemWrite,
		AllowNetworkRequests:         cfg.Agent.AllowNetworkRequests,
		AllowRemoteShell:             cfg.Agent.AllowRemoteShell,
		AllowSelfUpdate:              cfg.Agent.AllowSelfUpdate,
	}
}

func isCoAgentSession(sessionID string) bool {
	return strings.HasPrefix(sessionID, "coagent-") || strings.HasPrefix(sessionID, "specialist-")
}
