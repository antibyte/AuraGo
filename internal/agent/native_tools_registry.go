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
	builtinToolSchemaCache.Store(ff.Key(), deepClone(built))
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
		JellyfinEnabled: true, ChromecastEnabled: true, DiscordEnabled: true, TelegramEnabled: true, TrueNASEnabled: true,
		KoofrEnabled: true, FritzBoxSystemEnabled: true, FritzBoxNetworkEnabled: true,
		FritzBoxTelephonyEnabled: true, FritzBoxSmartHomeEnabled: true, FritzBoxStorageEnabled: true,
		FritzBoxTVEnabled: true, TelnyxSMSEnabled: true, TelnyxCallEnabled: true,
		SQLConnectionsEnabled: true, PythonSecretInjectionEnabled: true,
		LDAPEnabled: true,
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
	// Use the shared helper so this stays in sync with resolveToolFeatureState.
	// buildToolFlagsFromConfig already uses cfg.Runtime for environment-aware decisions.
	ff := buildToolFlagsFromConfig(cfg)
	return builtinToolNames(ff)
}


// ToolSummariesFromConfig returns tool names with short descriptions as
// "name: description" strings. Used by the mission preparation service
// so the LLM knows both the name and purpose of each available tool.
func ToolSummariesFromConfig(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	// Use the shared helper so this stays in sync with resolveToolFeatureState.
	// buildToolFlagsFromConfig already uses cfg.Runtime for environment-aware decisions.
	ff := buildToolFlagsFromConfig(cfg)
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
