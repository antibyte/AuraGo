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
	IsAnthropic               bool
	AutoEnableNativeFunctions bool
	SupportsStructuredOutputs bool
	SupportsParallelToolCalls bool
	// DisableNativeFunctionCalling is true for models (e.g. GLM/Zhipu, MiniMax) that do not
	// reliably produce OpenAI-compatible API-level tool_calls. These models often emit tool
	// invocations as text content using proprietary XML/JSON markers. Forcing JSON text mode
	// gives the system a consistent, parseable output format.
	DisableNativeFunctionCalling bool
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

// PromptContextOptions carries per-request runtime values that are known only
// at call time (maintenance state, webhook definitions, specialist hints, …).
// Callers outside the agent package use this to supply dynamic context when
// calling BuildPromptContextFlags.
type PromptContextOptions struct {
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

// promptContextOptions is an alias kept for internal callers.
type promptContextOptions = PromptContextOptions

type resolvedToolFeatureState struct {
	ToolFlags           ToolFeatureFlags
	WebDAVEnabled       bool
	PaperlessNGXEnabled bool
	BraveSearchEnabled  bool
	A2AEnabled          bool
	TelnyxEnabled       bool
	UnifiedMemoryBlock  bool
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
	// isAnthropic is true only for actual Claude/Anthropic models, NOT for third-party models
	// that use the Anthropic API protocol (type: anthropic) such as Kimi-for-coding or GLM variants.
	// Using lowerProvider alone would incorrectly flag any model using the Anthropic SDK.
	isAnthropic := strings.Contains(lowerModel, "claude")
	isNemotron := strings.Contains(lowerModel, "nemotron")

	// Models from providers known to NOT support OpenAI-style strict mode on
	// individual tool definitions (Function.Strict=true). Ollama supports
	// structured outputs via response_format, but currently ignores the strict
	// field in the OpenAI-compatible chat completions API. The other entries are
	// Chinese LLM providers with OpenAI-compatible APIs but without the
	// strict-mode constraint decoding extension.
	isNoStrictStructuredOutputs := isOllama ||
		strings.HasPrefix(lowerModel, "glm-") ||
		strings.Contains(lowerModel, "/glm-") ||
		strings.Contains(lowerModel, "zhipuai/") ||
		strings.HasPrefix(lowerModel, "minimax") ||
		strings.HasPrefix(lowerModel, "abab") ||
		strings.HasPrefix(lowerModel, "kimi-") ||
		strings.HasPrefix(lowerModel, "moonshot-") ||
		strings.HasPrefix(lowerModel, "qwen") ||
		strings.HasPrefix(lowerModel, "qwq") ||
		strings.HasPrefix(lowerModel, "ernie")

	// GLM (Zhipu) and MiniMax models emit tool calls as proprietary XML/JSON text
	// content rather than proper OpenAI-compatible API tool_calls. Force JSON text
	// mode for these so the prompt-based JSON extraction path is used instead.
	// Note: MiniMax M2.7+ supports native function calling — only disable for
	// older MiniMax models (abab prefix) and the legacy minimax-text series.
	isGLMFamily := strings.HasPrefix(lowerModel, "glm-") ||
		strings.Contains(lowerModel, "/glm-") ||
		strings.Contains(lowerModel, "zhipuai/") ||
		strings.HasPrefix(lowerModel, "abab") ||
		(strings.HasPrefix(lowerModel, "minimax") && !strings.Contains(lowerModel, "m2.7") && !strings.Contains(lowerModel, "minimax-m1") && !strings.Contains(lowerModel, "/text-"))

	return ModelCapabilities{
		ProviderType:                 providerType,
		Model:                        model,
		IsOllama:                     isOllama,
		IsDeepSeek:                   isDeepSeek,
		IsAnthropic:                  isAnthropic,
		AutoEnableNativeFunctions:    isDeepSeek || isAnthropic || isNemotron,
		SupportsStructuredOutputs:    !isNoStrictStructuredOutputs,
		SupportsParallelToolCalls:    !isOllama,
		DisableNativeFunctionCalling: isGLMFamily,
	}
}

// BuildToolingPolicy resolves runtime feature toggles, telemetry profile, and
// guide strategy for the given config and user query. It is exported so that
// non-agent callers (server handlers, bots) can obtain the same resolved policy
// that the agent loop uses, without duplicating the logic inline.
func BuildToolingPolicy(cfg *config.Config, userQuery string) ToolingPolicy {
	return buildToolingPolicy(cfg, userQuery)
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
	// Force JSON text mode for models known to emit tool calls in text content rather
	// than proper API tool_calls (e.g. GLM/Zhipu, MiniMax). This ensures the prompt-based
	// JSON extraction path is used regardless of what the config says.
	if caps.DisableNativeFunctionCalling {
		useNativeFunctions = false
	}
	autoEnabled := !cfg.LLM.UseNativeFunctions && caps.AutoEnableNativeFunctions
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
		AutoEnabledNativeFunctions: autoEnabled && useNativeFunctions, // only true if it actually was auto-enabled
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

// BuildPromptContextFlags is the exported entry point for building prompt
// ContextFlags from a RunConfig and a resolved ToolingPolicy. All callers
// (agent loop, server handlers, bots) must use this function instead of
// building ContextFlags inline to keep all channels consistent.
func BuildPromptContextFlags(runCfg RunConfig, policy ToolingPolicy, opts PromptContextOptions) prompts.ContextFlags {
	return buildPromptContextFlags(runCfg, policy, opts)
}

// buildToolFlagsFromConfig builds ToolFeatureFlags from a config struct.
// This is the canonical config-only source for all ToolFeatureFlags.
// It uses cfg.Runtime values (which are set at startup) for environment-aware decisions
// but does NOT check database/runtime handle availability.
//
// Use resolveToolFeatureState (which calls this helper) when runtime handle availability
// needs to be factored in (e.g. db != nil checks).
func buildToolFlagsFromConfig(cfg *config.Config) ToolFeatureFlags {
	if cfg == nil {
		return ToolFeatureFlags{}
	}

	// Compute policy-derived flags using the same logic as buildToolingPolicy.
	// These values already incorporate cfg.Runtime.IsDocker checks.
	dockerEnabled := cfg.Docker.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK)
	sandboxEnabled := cfg.Sandbox.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK)
	homepageEnabled := cfg.Homepage.Enabled && (dockerEnabled || cfg.Homepage.AllowLocalServer)
	wolEnabled := cfg.Tools.WOL.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.BroadcastOK)

	return ToolFeatureFlags{
		HomeAssistantEnabled:         cfg.HomeAssistant.Enabled,
		DockerEnabled:                dockerEnabled,
		CoAgentEnabled:               cfg.CoAgents.Enabled,
		SudoEnabled:                  cfg.Agent.SudoEnabled && !cfg.Runtime.IsDocker && !cfg.Runtime.NoNewPrivileges,
		WebhooksEnabled:              cfg.Webhooks.Enabled,
		JellyfinEnabled:              cfg.Jellyfin.Enabled,
		ObsidianEnabled:              cfg.Obsidian.Enabled,
		ChromecastEnabled:            cfg.Chromecast.Enabled,
		DiscordEnabled:               cfg.Discord.Enabled,
		TelegramEnabled:              cfg.Telegram.BotToken != "" && cfg.Telegram.UserID != 0,
		TrueNASEnabled:               cfg.TrueNAS.Enabled,
		KoofrEnabled:                 cfg.Koofr.Enabled,
		ProxmoxEnabled:               cfg.Proxmox.Enabled,
		OllamaEnabled:                cfg.Ollama.Enabled,
		TailscaleEnabled:             cfg.Tailscale.Enabled,
		AnsibleEnabled:               cfg.Ansible.Enabled,
		InvasionControlEnabled:       cfg.InvasionControl.Enabled,
		GitHubEnabled:                cfg.GitHub.Enabled,
		MQTTEnabled:                  cfg.MQTT.Enabled,
		AdGuardEnabled:               cfg.AdGuard.Enabled,
		MCPEnabled:                   cfg.MCP.Enabled && cfg.Agent.AllowMCP,
		SandboxEnabled:               sandboxEnabled,
		MeshCentralEnabled:           cfg.MeshCentral.Enabled,
		HomepageEnabled:              homepageEnabled,
		HomepageAllowLocalServer:     cfg.Homepage.AllowLocalServer,
		NetlifyEnabled:               cfg.Netlify.Enabled,
		FirewallEnabled:              cfg.Firewall.Enabled && (cfg.Runtime.FirewallAccessOK || (cfg.Agent.SudoEnabled && !cfg.Runtime.IsDocker)),
		EmailEnabled:                 cfg.Email.Enabled || len(cfg.EmailAccounts) > 0,
		CloudflareTunnelEnabled:      cfg.CloudflareTunnel.Enabled,
		GoogleWorkspaceEnabled:       cfg.GoogleWorkspace.Enabled,
		OneDriveEnabled:              cfg.OneDrive.Enabled,
		VirusTotalEnabled:            cfg.VirusTotal.Enabled,
		GolangciLintEnabled:          cfg.GolangciLint.Enabled,
		ImageGenerationEnabled:       cfg.ImageGeneration.Enabled,
		MusicGenerationEnabled:       cfg.MusicGeneration.Enabled,
		RemoteControlEnabled:         cfg.RemoteControl.Enabled,
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
		WOLEnabled:                   wolEnabled,
		MediaRegistryEnabled:         cfg.MediaRegistry.Enabled,
		HomepageRegistryEnabled:      cfg.Homepage.Enabled,
		ContactsEnabled:              cfg.Tools.Contacts.Enabled,
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
		SQLConnectionsEnabled:        cfg.SQLConnections.Enabled,
		PythonSecretInjectionEnabled: cfg.Tools.PythonSecretInjection.Enabled,
		DaemonSkillsEnabled:          cfg.Tools.DaemonSkills.Enabled,
		AllowShell:                   cfg.Agent.AllowShell,
		AllowPython:                  cfg.Agent.AllowPython,
		AllowFilesystemWrite:         cfg.Agent.AllowFilesystemWrite,
		AllowNetworkRequests:         cfg.Agent.AllowNetworkRequests,
		AllowRemoteShell:             cfg.Agent.AllowRemoteShell,
		AllowSelfUpdate:              cfg.Agent.AllowSelfUpdate,
		LDAPEnabled:                  cfg.LDAP.Enabled,
	}
}

func resolveToolFeatureState(runCfg RunConfig, policy ToolingPolicy) resolvedToolFeatureState {
	cfg := runCfg.Config
	if cfg == nil {
		return resolvedToolFeatureState{}
	}

	// Use the shared config-only helper as the base.
	// Then apply runtime-specific conditions that require database handles or runtime context.
	toolFlags := buildToolFlagsFromConfig(cfg)

	// Apply runtime handle availability checks (these are the ONLY runtime conditions).
	// All other flags (config-based, cfg.Runtime-based) are already set by buildToolFlagsFromConfig.
	toolFlags.InvasionControlEnabled = cfg.InvasionControl.Enabled && runCfg.InvasionDB != nil
	toolFlags.RemoteControlEnabled = cfg.RemoteControl.Enabled && runCfg.RemoteHub != nil
	toolFlags.MediaRegistryEnabled = cfg.MediaRegistry.Enabled && runCfg.MediaRegistryDB != nil
	toolFlags.HomepageRegistryEnabled = cfg.Homepage.Enabled && runCfg.HomepageRegistryDB != nil
	toolFlags.ContactsEnabled = cfg.Tools.Contacts.Enabled && runCfg.ContactsDB != nil
	toolFlags.SQLConnectionsEnabled = cfg.SQLConnections.Enabled && runCfg.SQLConnectionsDB != nil && runCfg.SQLConnectionPool != nil
	toolFlags.MemoryAnalysisEnabled = resolveMemoryAnalysisSettings(cfg, runCfg.ShortTermMem).Enabled

	return resolvedToolFeatureState{
		ToolFlags:           toolFlags,
		WebDAVEnabled:       cfg.WebDAV.Enabled,
		PaperlessNGXEnabled: cfg.PaperlessNGX.Enabled,
		BraveSearchEnabled:  cfg.BraveSearch.Enabled,
		A2AEnabled:          cfg.A2A.Server.Enabled || cfg.A2A.Client.Enabled,
		TelnyxEnabled:       cfg.Telnyx.Enabled,
		UnifiedMemoryBlock:  resolveMemoryAnalysisSettings(cfg, runCfg.ShortTermMem).UnifiedMemoryBlock,
	}
}

func buildPromptContextFlags(runCfg RunConfig, policy ToolingPolicy, opts promptContextOptions) prompts.ContextFlags {
	cfg := runCfg.Config
	if cfg == nil {
		return prompts.ContextFlags{}
	}
	state := resolveToolFeatureState(runCfg, policy)
	flags := state.ToolFlags

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
		IsVoiceMode:              GetVoiceMode(),
		IsCoAgent:                runCfg.IsCoAgent || isCoAgentSession(runCfg.SessionID),
		DiscordEnabled:           flags.DiscordEnabled,
		EmailEnabled:             flags.EmailEnabled,
		DockerEnabled:            flags.DockerEnabled,
		HomeAssistantEnabled:     flags.HomeAssistantEnabled,
		WebDAVEnabled:            state.WebDAVEnabled,
		KoofrEnabled:             flags.KoofrEnabled,
		PaperlessNGXEnabled:      state.PaperlessNGXEnabled,
		ChromecastEnabled:        flags.ChromecastEnabled,
		CoAgentEnabled:           flags.CoAgentEnabled,
		GoogleWorkspaceEnabled:   flags.GoogleWorkspaceEnabled,
		OneDriveEnabled:          flags.OneDriveEnabled,
		JellyfinEnabled:          flags.JellyfinEnabled,
		ObsidianEnabled:          flags.ObsidianEnabled,
		TrueNASEnabled:           flags.TrueNASEnabled,
		ProxmoxEnabled:           flags.ProxmoxEnabled,
		OllamaEnabled:            flags.OllamaEnabled,
		TailscaleEnabled:         flags.TailscaleEnabled,
		CloudflareTunnelEnabled:  flags.CloudflareTunnelEnabled,
		AnsibleEnabled:           flags.AnsibleEnabled,
		InvasionControlEnabled:   flags.InvasionControlEnabled,
		GitHubEnabled:            flags.GitHubEnabled,
		MQTTEnabled:              flags.MQTTEnabled,
		AdGuardEnabled:           flags.AdGuardEnabled,
		MCPEnabled:               flags.MCPEnabled,
		SandboxEnabled:           flags.SandboxEnabled,
		MeshCentralEnabled:       flags.MeshCentralEnabled,
		HomepageEnabled:          flags.HomepageEnabled,
		HomepageAllowLocalServer: flags.HomepageAllowLocalServer,
		NetlifyEnabled:           flags.NetlifyEnabled,
		WebhooksEnabled:          flags.WebhooksEnabled,
		WebhooksDefinitions:      opts.WebhooksDefinitions,
		VirusTotalEnabled:        flags.VirusTotalEnabled,
		GolangciLintEnabled:      flags.GolangciLintEnabled,
		BraveSearchEnabled:       state.BraveSearchEnabled,
		MiniMaxTTSEnabled:        cfg.TTS.Provider == "minimax",
		VoiceOutputActive:        runCfg.VoiceOutputActive,
		ImageGenerationEnabled:   flags.ImageGenerationEnabled,
		MusicGenerationEnabled:   flags.MusicGenerationEnabled,
		RemoteControlEnabled:     flags.RemoteControlEnabled,
		MemoryEnabled:            flags.MemoryEnabled,
		KnowledgeGraphEnabled:    flags.KnowledgeGraphEnabled,
		SecretsVaultEnabled:      flags.SecretsVaultEnabled,
		SchedulerEnabled:         flags.SchedulerEnabled,
		NotesEnabled:             flags.NotesEnabled,
		JournalEnabled:           flags.JournalEnabled,
		MissionsEnabled:          flags.MissionsEnabled,
		StopProcessEnabled:       flags.StopProcessEnabled,
		InventoryEnabled:         flags.InventoryEnabled,
		MemoryMaintenanceEnabled: flags.MemoryMaintenanceEnabled,
		WOLEnabled:               flags.WOLEnabled,
		MediaRegistryEnabled:     flags.MediaRegistryEnabled,
		HomepageRegistryEnabled:  flags.HomepageRegistryEnabled,
		AllowShell:               flags.AllowShell,
		AllowPython:              flags.AllowPython,
		AllowFilesystemWrite:     flags.AllowFilesystemWrite,
		AllowNetworkRequests:     flags.AllowNetworkRequests,
		AllowRemoteShell:         flags.AllowRemoteShell,
		AllowSelfUpdate:          flags.AllowSelfUpdate,
		IsEgg:                    cfg.EggMode.Enabled,
		InternetExposed:          cfg.Server.HTTPS.Enabled,
		IsDocker:                 cfg.Runtime.IsDocker,
		DocumentCreatorEnabled:   flags.DocumentCreatorEnabled,
		WebCaptureEnabled:        flags.WebCaptureEnabled,
		NetworkPingEnabled:       flags.NetworkPingEnabled,
		WebScraperEnabled:        flags.WebScraperEnabled,
		S3Enabled:                flags.S3Enabled,
		NetworkScanEnabled:       flags.NetworkScanEnabled,
		FormAutomationEnabled:    flags.FormAutomationEnabled,
		UPnPScanEnabled:          flags.UPnPScanEnabled,
		FritzBoxSystemEnabled:    flags.FritzBoxSystemEnabled,
		FritzBoxNetworkEnabled:   flags.FritzBoxNetworkEnabled,
		FritzBoxTelephonyEnabled: flags.FritzBoxTelephonyEnabled,
		FritzBoxSmartHomeEnabled: flags.FritzBoxSmartHomeEnabled,
		FritzBoxStorageEnabled:   flags.FritzBoxStorageEnabled,
		FritzBoxTVEnabled:        flags.FritzBoxTVEnabled,
		A2AEnabled:               state.A2AEnabled,
		TelnyxEnabled:            state.TelnyxEnabled,
		AdditionalPrompt:         cfg.Agent.AdditionalPrompt,
		MessageSource:            runCfg.MessageSource,
		ToolsDir:                 cfg.Directories.ToolsDir,
		SkillsDir:                cfg.Directories.SkillsDir,
		UnifiedMemoryBlock:       state.UnifiedMemoryBlock,
		SpecialistsAvailable:     opts.SpecialistsAvailable,
		SpecialistsStatus:        opts.SpecialistsStatus,
		SpecialistsSuggestion:    opts.SpecialistsSuggestion,
		NativeToolsEnabled:       policy.UseNativeFunctions,
		IsTextModeModel:          !policy.UseNativeFunctions && policy.Capabilities.DisableNativeFunctionCalling,
	}
}

func buildToolFeatureFlags(runCfg RunConfig, policy ToolingPolicy) ToolFeatureFlags {
	return resolveToolFeatureState(runCfg, policy).ToolFlags
}

func isCoAgentSession(sessionID string) bool {
	return strings.HasPrefix(sessionID, "coagent-") || strings.HasPrefix(sessionID, "specialist-")
}
