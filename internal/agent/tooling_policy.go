package agent

import (
	"fmt"
	"sort"
	"strings"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/prompts"
	"aurago/internal/tools"
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
	ProviderToolCalling       bool
	ProviderStructuredOutputs bool
	ProviderMultimodal        bool
	CapabilitySource          string
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
	VercelEnabled              bool
	WOLEnabled                 bool
	EffectiveMaxToolGuides     int
	ProviderToolProfile        string
	EffectiveMaxAdaptiveTools  int
	EffectiveMaxTotalTools     int
	EffectiveHeaderTimeoutSec  int
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
	providerCaps := llm.ResolveConfigProviderCapabilities(cfg)

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
		ProviderToolCalling:          providerCaps.ToolCalling,
		ProviderStructuredOutputs:    providerCaps.StructuredOutputs,
		ProviderMultimodal:           providerCaps.Multimodal,
		CapabilitySource:             providerCaps.Source,
		AutoEnableNativeFunctions:    providerCaps.ToolCalling,
		SupportsStructuredOutputs:    providerCaps.StructuredOutputs && !isNoStrictStructuredOutputs,
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

	useNativeFunctions := caps.ProviderToolCalling
	// Force JSON text mode for models known to emit tool calls in text content rather
	// than proper API tool_calls (e.g. GLM/Zhipu, MiniMax). This ensures the prompt-based
	// JSON extraction path is used regardless of what the config says.
	if caps.DisableNativeFunctionCalling {
		useNativeFunctions = false
	}
	autoEnabled := !cfg.LLM.UseNativeFunctions && useNativeFunctions
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
	providerToolProfile, effectiveMaxAdaptiveTools, effectiveMaxTotalTools, effectiveHeaderTimeoutSec :=
		resolveProviderToolProfile(cfg, caps, cfg.Agent.AdaptiveTools.MaxTools, cfg.Agent.AdaptiveTools.MaxTotalTools)

	return ToolingPolicy{
		Capabilities:               caps,
		TelemetryScope:             scope,
		TelemetryProfile:           telemetryProfile,
		TelemetrySnapshot:          scopedTelemetry,
		IntentFamily:               intentFamily,
		FamilyTelemetry:            familyTelemetry,
		UseNativeFunctions:         useNativeFunctions,
		AutoEnabledNativeFunctions: autoEnabled && useNativeFunctions, // only true if it actually was auto-enabled
		StructuredOutputsRequested: caps.ProviderStructuredOutputs,
		StructuredOutputsEnabled:   caps.ProviderStructuredOutputs && caps.SupportsStructuredOutputs,
		ParallelToolCallsEnabled:   caps.SupportsParallelToolCalls,
		DockerEnabled:              dockerEnabled,
		SandboxEnabled:             sandboxEnabled,
		HomepageEnabled:            homepageEnabled,
		HomepageAllowLocalServer:   cfg.Homepage.AllowLocalServer,
		VercelEnabled:              cfg.Vercel.Enabled,
		WOLEnabled:                 wolEnabled,
		EffectiveMaxToolGuides:     effectiveMaxToolGuides,
		ProviderToolProfile:        providerToolProfile,
		EffectiveMaxAdaptiveTools:  effectiveMaxAdaptiveTools,
		EffectiveMaxTotalTools:     effectiveMaxTotalTools,
		EffectiveHeaderTimeoutSec:  effectiveHeaderTimeoutSec,
		EffectiveGuideStrategy:     guideStrategy,
	}
}

func resolveProviderToolProfile(cfg *config.Config, caps ModelCapabilities, maxAdaptiveTools, maxTotalTools int) (string, int, int, int) {
	headerTimeoutSec := 30
	if cfg == nil || !cfg.Agent.AdaptiveTools.ProviderProfilesEnabled {
		return "default", maxAdaptiveTools, maxTotalTools, headerTimeoutSec
	}
	lowerProvider := strings.ToLower(strings.TrimSpace(caps.ProviderType))
	lowerModel := strings.ToLower(strings.TrimSpace(caps.Model))
	isMiniMax := lowerProvider == "minimax" || strings.Contains(lowerModel, "minimax") || strings.HasPrefix(lowerModel, "abab")
	isGLM := strings.Contains(lowerModel, "glm-") || strings.Contains(lowerModel, "/glm-") || strings.Contains(lowerModel, "zhipuai/")
	if isMiniMax {
		return "minimax_stability", minPositive(maxAdaptiveTools, 12), minPositive(maxTotalTools, 24), 90
	}
	if isGLM {
		return "glm_stability", minPositive(maxAdaptiveTools, 12), minPositive(maxTotalTools, 24), 60
	}
	if caps.IsOllama {
		return "ollama_local", maxAdaptiveTools, maxTotalTools, 30
	}
	return "default", maxAdaptiveTools, maxTotalTools, headerTimeoutSec
}

func minPositive(value, cap int) int {
	if value <= 0 {
		return cap
	}
	if cap <= 0 || value < cap {
		return value
	}
	return cap
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
	packageManagerEnabled := cfg.Agent.AllowPackageManager && cfg.PackageManager.Enabled && (!cfg.Runtime.IsDocker || cfg.Agent.SudoEnabled)

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
		FrigateEnabled:               cfg.Frigate.Enabled,
		ThreeDPrinterEnabled:         cfg.ThreeDPrinters.Enabled,
		OllamaEnabled:                cfg.Ollama.Enabled,
		TailscaleEnabled:             cfg.Tailscale.Enabled,
		AnsibleEnabled:               cfg.Ansible.Enabled,
		InvasionControlEnabled:       cfg.InvasionControl.Enabled,
		GitHubEnabled:                cfg.GitHub.Enabled,
		HuggingFaceEnabled:           cfg.HuggingFace.Enabled,
		MQTTEnabled:                  cfg.MQTT.Enabled,
		AdGuardEnabled:               cfg.AdGuard.Enabled,
		UptimeKumaEnabled:            cfg.UptimeKuma.Enabled,
		GrafanaEnabled:               cfg.Grafana.Enabled,
		MCPEnabled:                   cfg.MCP.Enabled && cfg.Agent.AllowMCP,
		ComposioEnabled:              cfg.Composio.Enabled && strings.TrimSpace(cfg.Composio.APIKey) != "",
		EvomapEnabled:                cfg.Evomap.Enabled,
		SandboxEnabled:               sandboxEnabled,
		MeshCentralEnabled:           cfg.MeshCentral.Enabled,
		HomepageEnabled:              homepageEnabled,
		HomepageAllowLocalServer:     cfg.Homepage.AllowLocalServer,
		NetlifyEnabled:               cfg.Netlify.Enabled,
		VercelEnabled:                cfg.Vercel.Enabled,
		FirewallEnabled:              cfg.Firewall.Enabled && (cfg.Runtime.FirewallAccessOK || (cfg.Agent.SudoEnabled && !cfg.Runtime.IsDocker)),
		EmailEnabled:                 cfg.Email.Enabled || len(cfg.EmailAccounts) > 0,
		AgentMailEnabled:             cfg.AgentMail.Enabled,
		CloudflareTunnelEnabled:      cfg.CloudflareTunnel.Enabled,
		GoogleWorkspaceEnabled:       cfg.GoogleWorkspace.Enabled,
		OneDriveEnabled:              cfg.OneDrive.Enabled,
		WebDAVEnabled:                cfg.WebDAV.Enabled,
		VirusTotalEnabled:            cfg.VirusTotal.Enabled,
		GolangciLintEnabled:          cfg.GolangciLint.Enabled,
		ImageGenerationEnabled:       cfg.ImageGeneration.Enabled,
		MusicGenerationEnabled:       cfg.MusicGeneration.Enabled,
		VideoGenerationEnabled:       cfg.VideoGeneration.Enabled,
		TTSEnabled:                   isTTSConfigured(cfg),
		RemoteControlEnabled:         cfg.RemoteControl.Enabled,
		PackageManagerEnabled:        packageManagerEnabled,
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
		PlannerEnabled:               cfg.Tools.Planner.Enabled,
		MemoryAnalysisEnabled:        cfg.MemoryAnalysis.Enabled,
		DocumentCreatorEnabled:       cfg.Tools.DocumentCreator.Enabled,
		MediaConversionEnabled:       cfg.Tools.MediaConversion.Enabled,
		VideoDownloadEnabled:         cfg.Tools.VideoDownload.Enabled,
		VideoDownloadAllowDownload:   cfg.Tools.VideoDownload.AllowDownload && !cfg.Tools.VideoDownload.ReadOnly,
		VideoDownloadAllowTranscribe: cfg.Tools.VideoDownload.AllowTranscribe && !cfg.Tools.VideoDownload.ReadOnly,
		WorkspaceSearchEnabled:       cfg.WorkspaceSearch.Enabled,
		SendYouTubeVideoEnabled:      cfg.Tools.SendYouTubeVideo.Enabled,
		WebCaptureEnabled:            cfg.Tools.WebCapture.Enabled,
		BrowserAutomationEnabled:     cfg.BrowserAutomation.Enabled && cfg.Tools.BrowserAutomation.Enabled,
		SpaceAgentEnabled:            cfg.SpaceAgent.Enabled,
		VirtualDesktopEnabled:        cfg.VirtualDesktop.Enabled && cfg.VirtualDesktop.AllowAgentControl && cfg.Tools.VirtualDesktop.Enabled,
		VirtualComputersEnabled:      cfg.VirtualComputers.Enabled && cfg.Tools.VirtualComputers.Enabled,
		OpenSCADEnabled:              cfg.VirtualDesktop.Enabled && cfg.VirtualDesktop.AllowAgentControl && cfg.VirtualDesktop.OpenSCAD.Enabled,
		OfficeDocumentEnabled:        cfg.VirtualDesktop.Enabled && cfg.VirtualDesktop.AllowAgentControl && cfg.Tools.OfficeDocument.Enabled,
		OfficeWorkbookEnabled:        cfg.VirtualDesktop.Enabled && cfg.VirtualDesktop.AllowAgentControl && cfg.Tools.OfficeWorkbook.Enabled,
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
		PaperlessNGXEnabled:          cfg.PaperlessNGX.Enabled,
		YepAPIEnabled:                cfg.YepAPI.Enabled,
		YepAPISEOEnabled:             cfg.YepAPI.Enabled && cfg.YepAPI.SEO.Enabled,
		YepAPISERPEnabled:            cfg.YepAPI.Enabled && cfg.YepAPI.SERP.Enabled,
		YepAPIScrapingEnabled:        cfg.YepAPI.Enabled && cfg.YepAPI.Scraping.Enabled,
		YepAPIYouTubeEnabled:         cfg.YepAPI.Enabled && cfg.YepAPI.YouTube.Enabled,
		YepAPITikTokEnabled:          cfg.YepAPI.Enabled && cfg.YepAPI.TikTok.Enabled,
		YepAPIInstagramEnabled:       cfg.YepAPI.Enabled && cfg.YepAPI.Instagram.Enabled,
		YepAPIAmazonEnabled:          cfg.YepAPI.Enabled && cfg.YepAPI.Amazon.Enabled,
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
	toolFlags.AgoDeskChatEnabled = hasConnectedAgoDeskChatDevice(runCfg.RemoteHub)
	toolFlags.MediaRegistryEnabled = cfg.MediaRegistry.Enabled && runCfg.MediaRegistryDB != nil
	toolFlags.HomepageRegistryEnabled = cfg.Homepage.Enabled && runCfg.HomepageRegistryDB != nil
	toolFlags.ContactsEnabled = cfg.Tools.Contacts.Enabled && runCfg.ContactsDB != nil
	toolFlags.PlannerEnabled = cfg.Tools.Planner.Enabled && runCfg.PlannerDB != nil
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
		IsMaintenanceMode:        opts.IsMaintenanceMode,
		CorePersonality:          cfg.Personality.CorePersonality,
		TokenBudget:              config.CalculateAdaptiveSystemPromptTokenBudget(cfg),
		IsDebugMode:              cfg.Agent.DebugMode || GetDebugMode(),
		IsVoiceMode:              GetVoiceMode() && !isAutonomousAgentRun(runCfg, runCfg.SessionID) && !runCfg.IsMission,
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
		TelegramEnabled:          flags.TelegramEnabled,
		JellyfinEnabled:          flags.JellyfinEnabled,
		ObsidianEnabled:          flags.ObsidianEnabled,
		TrueNASEnabled:           flags.TrueNASEnabled,
		ProxmoxEnabled:           flags.ProxmoxEnabled,
		FrigateEnabled:           flags.FrigateEnabled,
		ThreeDPrinterEnabled:     flags.ThreeDPrinterEnabled,
		OllamaEnabled:            flags.OllamaEnabled,
		TailscaleEnabled:         flags.TailscaleEnabled,
		CloudflareTunnelEnabled:  flags.CloudflareTunnelEnabled,
		AnsibleEnabled:           flags.AnsibleEnabled,
		InvasionControlEnabled:   flags.InvasionControlEnabled,
		GitHubEnabled:            flags.GitHubEnabled,
		MQTTEnabled:              flags.MQTTEnabled,
		AdGuardEnabled:           flags.AdGuardEnabled,
		UptimeKumaEnabled:        flags.UptimeKumaEnabled,
		GrafanaEnabled:           flags.GrafanaEnabled,
		MCPEnabled:               flags.MCPEnabled,
		SandboxEnabled:           flags.SandboxEnabled,
		MeshCentralEnabled:       flags.MeshCentralEnabled,
		HomepageEnabled:          flags.HomepageEnabled,
		HomepageAllowLocalServer: flags.HomepageAllowLocalServer,
		NetlifyEnabled:           flags.NetlifyEnabled,
		VercelEnabled:            flags.VercelEnabled,
		WebhooksEnabled:          flags.WebhooksEnabled,
		WebhooksDefinitions:      opts.WebhooksDefinitions,
		VirusTotalEnabled:        flags.VirusTotalEnabled,
		GolangciLintEnabled:      flags.GolangciLintEnabled,
		BraveSearchEnabled:       state.BraveSearchEnabled,
		MiniMaxTTSEnabled:        isTTSConfigured(cfg) && strings.EqualFold(cfg.TTS.Provider, "minimax"),
		VoiceOutputActive:        runCfg.VoiceOutputActive && isTTSConfigured(cfg) && !isAutonomousAgentRun(runCfg, runCfg.SessionID) && !runCfg.IsMission,
		ImageGenerationEnabled:   flags.ImageGenerationEnabled,
		MusicGenerationEnabled:   flags.MusicGenerationEnabled,
		VideoGenerationEnabled:   flags.VideoGenerationEnabled,
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
		SudoEnabled:              flags.SudoEnabled,
		PackageManagerEnabled:    flags.PackageManagerEnabled,
		IsEgg:                    cfg.EggMode.Enabled,
		InternetExposed:          cfg.Server.HTTPS.Enabled,
		IsDocker:                 cfg.Runtime.IsDocker,
		DocumentCreatorEnabled:   flags.DocumentCreatorEnabled,
		MediaConversionEnabled:   flags.MediaConversionEnabled,
		VideoDownloadEnabled:     flags.VideoDownloadEnabled,
		WebCaptureEnabled:        flags.WebCaptureEnabled,
		BrowserAutomationEnabled: flags.BrowserAutomationEnabled,
		SpaceAgentEnabled:        flags.SpaceAgentEnabled,
		SpaceAgentPublicURL:      strings.TrimSpace(cfg.SpaceAgent.PublicURL),
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
		ReuseContext:             buildManagedSitesPromptContext(runCfg),
		ChatChannelsContext:      buildReachableChatChannelsContext(runCfg),
		ComposioServicesContext:  buildComposioServicesPromptContext(cfg),
		ToolsDir:                 "",
		SkillsDir:                "",
		UnifiedMemoryBlock:       state.UnifiedMemoryBlock,
		SpecialistsAvailable:     false,
		SpecialistsStatus:        "",
		SpecialistsSuggestion:    "",
		NativeToolsEnabled:       policy.UseNativeFunctions,
		IsTextModeModel:          !policy.UseNativeFunctions && policy.Capabilities.DisableNativeFunctionCalling,
	}
}

func buildComposioServicesPromptContext(cfg *config.Config) string {
	if cfg == nil || !cfg.Composio.Enabled || strings.TrimSpace(cfg.Composio.APIKey) == "" {
		return ""
	}
	type serviceLine struct {
		slug string
		line string
	}
	seen := make(map[string]bool)
	lines := make([]serviceLine, 0, len(cfg.Composio.Toolkits))
	for _, tk := range cfg.Composio.Toolkits {
		slug := strings.ToLower(strings.TrimSpace(tk.Slug))
		if slug == "" || !tk.Enabled || seen[slug] {
			continue
		}
		seen[slug] = true
		readOnly := cfg.Composio.ReadOnly
		if tk.ReadOnly != nil {
			readOnly = *tk.ReadOnly
		}
		allowDestructive := cfg.Composio.AllowDestructive
		if tk.AllowDestructive != nil {
			allowDestructive = *tk.AllowDestructive
		}
		allowNL := cfg.Composio.AllowNaturalLanguageInput
		if tk.AllowNaturalLanguageInput != nil {
			allowNL = *tk.AllowNaturalLanguageInput
		}
		line := fmt.Sprintf("- %s: route=composio_call toolkit_slug=%q read_only=%t allow_destructive=%t natural_language_input=%t next=capabilities,search_tools,get_tool,execute_tool",
			slug, slug, readOnly, allowDestructive, allowNL)
		lines = append(lines, serviceLine{slug: slug, line: line})
	}
	if len(lines) == 0 {
		return ""
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].slug < lines[j].slug })
	out := make([]string, 0, len(lines)+1)
	out = append(out, "Use operation=capabilities with toolkit_slug to get live connection status and a small tool_preview. Then use search_tools, get_tool, and execute_tool through composio_call; do not reason from local tool counts.")
	for _, item := range lines {
		out = append(out, item.line)
	}
	return strings.Join(out, "\n")
}

func buildManagedSitesPromptContext(runCfg RunConfig) string {
	if runCfg.Config == nil || !runCfg.Config.Homepage.Enabled || runCfg.HomepageRegistryDB == nil {
		return ""
	}
	sites, err := tools.ListHomepageManagedSites(runCfg.HomepageRegistryDB)
	if err != nil || len(sites) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# MANAGED WEBSITES\n")
	limit := len(sites)
	if limit > 8 {
		limit = 8
	}
	for i := 0; i < limit; i++ {
		site := sites[i]
		if detailed, detailErr := tools.GetHomepageManagedSite(runCfg.HomepageRegistryDB, site.ID); detailErr == nil {
			site = detailed
		}
		line := fmt.Sprintf("- %s (`%s`): drift=%s", site.Name, site.ProjectDir, site.DriftStatus)
		if site.LocalRoot != "" {
			line += fmt.Sprintf(", local=%s", site.LocalRoot)
		}
		if site.CurrentRevisionID > 0 {
			line += fmt.Sprintf(", revision=%d", site.CurrentRevisionID)
		}
		if site.LastDeployURL != "" {
			line += fmt.Sprintf(", last_deploy=%s", site.LastDeployURL)
		}
		if site.GitSHA != "" {
			short := site.GitSHA
			if len(short) > 12 {
				short = short[:12]
			}
			line += fmt.Sprintf(", git=%s", short)
		}
		if site.DriftMessage != "" {
			line += fmt.Sprintf(", note=%s", site.DriftMessage)
		}
		if len(site.DeployTargets) > 0 {
			var targets []string
			for _, target := range site.DeployTargets {
				value := target.URL
				if value == "" {
					value = target.RemotePath
				}
				if value == "" {
					value = target.ProviderTargetID
				}
				if value != "" {
					targets = append(targets, fmt.Sprintf("%s=%s", target.Provider, value))
				} else {
					targets = append(targets, target.Provider)
				}
			}
			line += ", targets=" + strings.Join(targets, "|")
		}
		if len(site.RemoteObservations) > 0 {
			obs := site.RemoteObservations[0]
			line += fmt.Sprintf(", remote=%s/%s", obs.Provider, obs.Status)
			if obs.ObservedAt != "" {
				line += fmt.Sprintf("@%s", obs.ObservedAt)
			}
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if len(sites) > limit {
		b.WriteString(fmt.Sprintf("- ...and %d more managed website(s)\n", len(sites)-limit))
	}
	b.WriteString("Before editing a managed website, use the listed project_dir and keep its ledger current through homepage tools.\n")
	return strings.TrimSpace(b.String())
}

func buildToolFeatureFlags(runCfg RunConfig, policy ToolingPolicy) ToolFeatureFlags {
	return resolveToolFeatureState(runCfg, policy).ToolFlags
}

func isCoAgentSession(sessionID string) bool {
	return strings.HasPrefix(sessionID, "coagent-") ||
		strings.HasPrefix(sessionID, "specialist-") ||
		strings.HasPrefix(sessionID, "a2a-")
}
