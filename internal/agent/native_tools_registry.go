package agent

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"aurago/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

func buildExecuteSkillProps(ff ToolFeatureFlags) map[string]interface{} {
	execSkillProps := map[string]interface{}{
		"skill": prop("string", "Name of the skill to execute (e.g. 'ddg_search', 'web_scraper', 'pdf_extractor', 'virustotal_scan')"),
		"skill_args": map[string]interface{}{
			"type":        "object",
			"description": "Arguments to pass to the skill as key-value pairs",
		},
	}
	if ff.PythonSecretInjectionEnabled {
		execSkillProps["vault_keys"] = map[string]interface{}{
			"type":        "array",
			"description": "List of vault secret key names to inject as AURAGO_SECRET_<KEY> environment variables. Only user/agent-created secrets are accessible.",
			"items":       map[string]interface{}{"type": "string"},
		}
		execSkillProps["credential_ids"] = map[string]interface{}{
			"type":        "array",
			"description": "List of credential UUIDs to inject as AURAGO_CRED_<NAME>_USERNAME / _PASSWORD / _TOKEN environment variables. Only credentials with 'allow_python' enabled are accessible.",
			"items":       map[string]interface{}{"type": "string"},
		}
	}
	return execSkillProps
}

// builtinToolSchemas returns schemas for all built-in AuraGo tools.
// Optional feature tools (home_assistant, docker, co_agent) are only
// included when their corresponding feature is enabled in the config.
func builtinToolSchemas(ff ToolFeatureFlags) []openai.Tool {
	executePythonDesc := "Save and execute a Python script. Use for data processing, automation, calculations, and scripting tasks."
	if ff.SandboxEnabled {
		executePythonDesc = "Save and execute a Python script on the HOST system (unsandboxed). Use ONLY for persistent tools (save_tool), registered skills, or when execute_sandbox is unavailable. Prefer execute_sandbox for all other code execution."
	}
	tools := buildCoreToolSchemas(ff, buildExecuteSkillProps(ff))
	tools = appendExecutionToolSchemas(tools, ff, executePythonDesc)
	tools = appendMemoryToolSchemas(tools, ff)
	tools = appendPlannerToolSchemas(tools, ff)
	tools = appendIntegrationToolSchemas(tools, ff)
	tools = appendContentToolSchemas(tools, ff)
	tools = appendEdgeToolSchemas(tools, ff)
	return tools
}
func builtinToolSchemasCached(ff ToolFeatureFlags) []openai.Tool {
	if cached, ok := builtinToolSchemaCache.Load(ff.Key()); ok {
		return deepClone(cached.([]openai.Tool))
	}
	built := builtinToolSchemas(ff)
	// Store the original slice (no clone needed here — deepClone on load ensures
	// every caller gets its own independent copy to mutate safely).
	builtinToolSchemaCache.Store(ff.Key(), built)
	return built
}

// allBuiltinToolFeatureFlags returns a ToolFeatureFlags with every feature enabled.
// Used in tests to enumerate all possible tool schemas.
func allBuiltinToolFeatureFlags() ToolFeatureFlags {
	return ToolFeatureFlags{
		HomeAssistantEnabled: true, DockerEnabled: true, CoAgentEnabled: true, SudoEnabled: true,
		WebhooksEnabled: true, ProxmoxEnabled: true, OllamaEnabled: true, TailscaleEnabled: true,
		AnsibleEnabled: true, InvasionControlEnabled: true, GitHubEnabled: true, MQTTEnabled: true,
		AdGuardEnabled: true, MCPEnabled: true, SandboxEnabled: true, MeshCentralEnabled: true,
		HomepageEnabled: true, NetlifyEnabled: true, FirewallEnabled: true, EmailEnabled: true,
		CloudflareTunnelEnabled: true, GoogleWorkspaceEnabled: true, OneDriveEnabled: true,
		VirusTotalEnabled: true, ImageGenerationEnabled: true, MusicGenerationEnabled: true, RemoteControlEnabled: true,
		AllowShell: true, AllowPython: true, AllowFilesystemWrite: true, AllowNetworkRequests: true,
		AllowRemoteShell: true, AllowSelfUpdate: true, HomepageAllowLocalServer: true,
		MemoryEnabled: true, KnowledgeGraphEnabled: true, SecretsVaultEnabled: true,
		SchedulerEnabled: true, NotesEnabled: true, MissionsEnabled: true, StopProcessEnabled: true,
		InventoryEnabled: true, MemoryMaintenanceEnabled: true, WOLEnabled: true,
		MediaRegistryEnabled: true, HomepageRegistryEnabled: true, ContactsEnabled: true,
		PlannerEnabled: true, JournalEnabled: true, MemoryAnalysisEnabled: true, DocumentCreatorEnabled: true,
		WebCaptureEnabled: true, NetworkPingEnabled: true, WebScraperEnabled: true,
		S3Enabled: true, NetworkScanEnabled: true, FormAutomationEnabled: true, UPnPScanEnabled: true,
		JellyfinEnabled: true, ChromecastEnabled: true, DiscordEnabled: true, TrueNASEnabled: true,
		KoofrEnabled: true, FritzBoxSystemEnabled: true, FritzBoxNetworkEnabled: true,
		FritzBoxTelephonyEnabled: true, FritzBoxSmartHomeEnabled: true, FritzBoxStorageEnabled: true,
		FritzBoxTVEnabled: true, TelnyxSMSEnabled: true, TelnyxCallEnabled: true,
		SQLConnectionsEnabled: true, PythonSecretInjectionEnabled: true,
	}
}

// builtinToolNames returns the name of every tool schema produced with the given feature flags.
func builtinToolNames(ff ToolFeatureFlags) []string {
	var names []string
	for _, s := range builtinToolSchemas(ff) {
		if s.Function != nil && s.Function.Name != "" {
			names = append(names, s.Function.Name)
		}
	}
	return names
}

func (ff ToolFeatureFlags) Key() string {
	encoded, err := json.Marshal(ff)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func builtinToolNameSet(ff ToolFeatureFlags) map[string]struct{} {
	names := make(map[string]struct{})
	for _, name := range builtinToolNames(ff) {
		names[name] = struct{}{}
	}
	return names
}

func allBuiltinToolNameSet() map[string]struct{} {
	return builtinToolNameSet(allBuiltinToolFeatureFlags())
}

// ToolNamesFromConfig returns a best-effort list of built-in tool names
// derived solely from config (no runtime dependencies). Used by the mission
// preparation service to populate the available tools list.
func ToolNamesFromConfig(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	ff := ToolFeatureFlags{
		HomeAssistantEnabled:         cfg.HomeAssistant.Enabled,
		DockerEnabled:                cfg.Docker.Enabled,
		CoAgentEnabled:               cfg.CoAgents.Enabled,
		SudoEnabled:                  cfg.Agent.SudoEnabled,
		WebhooksEnabled:              cfg.Webhooks.Enabled,
		ProxmoxEnabled:               cfg.Proxmox.Enabled,
		OllamaEnabled:                cfg.Ollama.Enabled,
		TailscaleEnabled:             cfg.Tailscale.Enabled,
		AnsibleEnabled:               cfg.Ansible.Enabled,
		InvasionControlEnabled:       cfg.InvasionControl.Enabled,
		GitHubEnabled:                cfg.GitHub.Enabled,
		MQTTEnabled:                  cfg.MQTT.Enabled,
		AdGuardEnabled:               cfg.AdGuard.Enabled,
		MCPEnabled:                   cfg.MCP.Enabled && cfg.Agent.AllowMCP,
		SandboxEnabled:               cfg.Sandbox.Enabled,
		MeshCentralEnabled:           cfg.MeshCentral.Enabled,
		HomepageEnabled:              cfg.Homepage.Enabled,
		NetlifyEnabled:               cfg.Netlify.Enabled,
		FirewallEnabled:              cfg.Firewall.Enabled,
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
		MissionsEnabled:              cfg.Tools.Missions.Enabled,
		StopProcessEnabled:           cfg.Tools.StopProcess.Enabled,
		InventoryEnabled:             cfg.Tools.Inventory.Enabled,
		MemoryMaintenanceEnabled:     cfg.Tools.MemoryMaintenance.Enabled,
		WOLEnabled:                   cfg.Tools.WOL.Enabled,
		MediaRegistryEnabled:         cfg.MediaRegistry.Enabled,
		HomepageRegistryEnabled:      cfg.Homepage.Enabled,
		ContactsEnabled:              cfg.Tools.Contacts.Enabled,
		PlannerEnabled:               cfg.Tools.Planner.Enabled,
		JournalEnabled:               cfg.Tools.Journal.Enabled,
		MemoryAnalysisEnabled:        cfg.MemoryAnalysis.Enabled,
		DocumentCreatorEnabled:       cfg.Tools.DocumentCreator.Enabled,
		WebCaptureEnabled:            cfg.Tools.WebCapture.Enabled,
		NetworkPingEnabled:           cfg.Tools.NetworkPing.Enabled,
		WebScraperEnabled:            cfg.Tools.WebScraper.Enabled,
		S3Enabled:                    cfg.S3.Enabled,
		NetworkScanEnabled:           cfg.Tools.NetworkScan.Enabled,
		FormAutomationEnabled:        cfg.Tools.FormAutomation.Enabled,
		UPnPScanEnabled:              cfg.Tools.UPnPScan.Enabled,
		JellyfinEnabled:              cfg.Jellyfin.Enabled,
		ChromecastEnabled:            cfg.Chromecast.Enabled,
		DiscordEnabled:               cfg.Discord.Enabled,
		TrueNASEnabled:               cfg.TrueNAS.Enabled,
		KoofrEnabled:                 cfg.Koofr.Enabled,
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
	}
	return builtinToolNames(ff)
}

// ToolSummariesFromConfig returns tool names with short descriptions as
// "name: description" strings. Used by the mission preparation service
// so the LLM knows both the name and purpose of each available tool.
func ToolSummariesFromConfig(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	ff := ToolFeatureFlags{
		HomeAssistantEnabled:         cfg.HomeAssistant.Enabled,
		DockerEnabled:                cfg.Docker.Enabled,
		CoAgentEnabled:               cfg.CoAgents.Enabled,
		SudoEnabled:                  cfg.Agent.SudoEnabled,
		WebhooksEnabled:              cfg.Webhooks.Enabled,
		ProxmoxEnabled:               cfg.Proxmox.Enabled,
		OllamaEnabled:                cfg.Ollama.Enabled,
		TailscaleEnabled:             cfg.Tailscale.Enabled,
		AnsibleEnabled:               cfg.Ansible.Enabled,
		InvasionControlEnabled:       cfg.InvasionControl.Enabled,
		GitHubEnabled:                cfg.GitHub.Enabled,
		MQTTEnabled:                  cfg.MQTT.Enabled,
		AdGuardEnabled:               cfg.AdGuard.Enabled,
		MCPEnabled:                   cfg.MCP.Enabled && cfg.Agent.AllowMCP,
		SandboxEnabled:               cfg.Sandbox.Enabled,
		MeshCentralEnabled:           cfg.MeshCentral.Enabled,
		HomepageEnabled:              cfg.Homepage.Enabled,
		NetlifyEnabled:               cfg.Netlify.Enabled,
		FirewallEnabled:              cfg.Firewall.Enabled,
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
		MissionsEnabled:              cfg.Tools.Missions.Enabled,
		StopProcessEnabled:           cfg.Tools.StopProcess.Enabled,
		InventoryEnabled:             cfg.Tools.Inventory.Enabled,
		MemoryMaintenanceEnabled:     cfg.Tools.MemoryMaintenance.Enabled,
		WOLEnabled:                   cfg.Tools.WOL.Enabled,
		MediaRegistryEnabled:         cfg.MediaRegistry.Enabled,
		HomepageRegistryEnabled:      cfg.Homepage.Enabled,
		ContactsEnabled:              cfg.Tools.Contacts.Enabled,
		PlannerEnabled:               cfg.Tools.Planner.Enabled,
		JournalEnabled:               cfg.Tools.Journal.Enabled,
		MemoryAnalysisEnabled:        cfg.MemoryAnalysis.Enabled,
		DocumentCreatorEnabled:       cfg.Tools.DocumentCreator.Enabled,
		WebCaptureEnabled:            cfg.Tools.WebCapture.Enabled,
		NetworkPingEnabled:           cfg.Tools.NetworkPing.Enabled,
		WebScraperEnabled:            cfg.Tools.WebScraper.Enabled,
		S3Enabled:                    cfg.S3.Enabled,
		NetworkScanEnabled:           cfg.Tools.NetworkScan.Enabled,
		FormAutomationEnabled:        cfg.Tools.FormAutomation.Enabled,
		UPnPScanEnabled:              cfg.Tools.UPnPScan.Enabled,
		JellyfinEnabled:              cfg.Jellyfin.Enabled,
		ChromecastEnabled:            cfg.Chromecast.Enabled,
		DiscordEnabled:               cfg.Discord.Enabled,
		TrueNASEnabled:               cfg.TrueNAS.Enabled,
		KoofrEnabled:                 cfg.Koofr.Enabled,
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
	}
	schemas := builtinToolSchemas(ff)
	summaries := make([]string, 0, len(schemas))
	for _, s := range schemas {
		if s.Function == nil || s.Function.Name == "" {
			continue
		}
		desc := s.Function.Description
		if len(desc) > 120 {
			desc = desc[:117] + "..."
		}
		summaries = append(summaries, s.Function.Name+": "+desc)
	}
	return summaries
}

func customToolBuiltinCollisionName(name string, builtinNames map[string]struct{}) (string, bool) {
	candidates := []string{strings.TrimSpace(name)}
	if ext := filepath.Ext(name); ext != "" {
		candidates = append(candidates, strings.TrimSuffix(name, ext))
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := builtinNames[candidate]; ok {
			return candidate, true
		}
	}
	return "", false
}
