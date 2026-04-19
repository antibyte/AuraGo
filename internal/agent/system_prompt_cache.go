package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"aurago/internal/prompts"
)

type systemPromptCacheKey struct {
	PromptsDir             string   `json:"prompts_dir"`
	CoreMemory             string   `json:"core_memory"`
	BudgetHint             string   `json:"budget_hint"`
	EnabledTools           []string `json:"enabled_tools"`
	FeatureToggles         []string `json:"feature_toggles"`
	Tier                   string   `json:"tier"`
	TokenBudget            int      `json:"token_budget"`
	IsMission              bool     `json:"is_mission"`
	IsErrorState           bool     `json:"is_error_state"`
	RequiresCoding         bool     `json:"requires_coding"`
	SystemLanguage         string   `json:"system_language"`
	CorePersonality        string   `json:"core_personality"`
	AdditionalPrompt       string   `json:"additional_prompt"`
	InnerVoice             string   `json:"inner_voice"`
	PredictedGuidesHash    string   `json:"predicted_guides_hash"`
	HighPriorityNotes      string   `json:"high_priority_notes"`
	PlannerContext         string   `json:"planner_context"`
	DailyTodoReminder      string   `json:"daily_todo_reminder"`
	SessionTodoItems       string   `json:"session_todo_items"`
	WebhooksDefinitions    string   `json:"webhooks_definitions"`
	RetrievedMemories      string   `json:"retrieved_memories"`
	RecentActivityOverview string   `json:"recent_activity_overview"`
	PredictedMemories      string   `json:"predicted_memories"`
	ActiveProcesses        string   `json:"active_processes"`
	IsVoiceMode            bool     `json:"is_voice_mode"`
	SpecialistsStatus      string   `json:"specialists_status"`
	KnowledgeContext       string   `json:"knowledge_context"`
	ErrorPatternContext    string   `json:"error_pattern_context"`
	ReuseContext           string   `json:"reuse_context"`
	EmotionDescription     string   `json:"emotion_description"`
	UserProfileSummary     string   `json:"user_profile_summary"`
	MessageSource          string   `json:"message_source"`
	Model                  string   `json:"model"`
	IsTextModeModel        bool     `json:"is_text_mode_model"`
	PersonalityLine        string   `json:"personality_line"`
}

func buildSystemPromptCacheKey(promptsDir string, flags *prompts.ContextFlags, coreMemory, budgetHint string) (string, error) {
	enabledTools := collectEnabledTools(flags)
	featureToggles := collectFeatureToggles(flags)

	// Hash PredictedGuides to include guide content in cache key without blowing up key size
	predictedGuidesHash := ""
	if len(flags.PredictedGuides) > 0 {
		h := sha256.New()
		for _, g := range flags.PredictedGuides {
			h.Write([]byte(g))
		}
		predictedGuidesHash = hex.EncodeToString(h.Sum(nil))
	}

	key := systemPromptCacheKey{
		PromptsDir:             promptsDir,
		CoreMemory:             coreMemory,
		BudgetHint:             budgetHint,
		EnabledTools:           enabledTools,
		FeatureToggles:         featureToggles,
		Tier:                   flags.Tier,
		TokenBudget:            flags.TokenBudget,
		IsMission:              flags.IsMission,
		IsErrorState:           flags.IsErrorState,
		RequiresCoding:         flags.RequiresCoding,
		SystemLanguage:         flags.SystemLanguage,
		CorePersonality:        flags.CorePersonality,
		AdditionalPrompt:       flags.AdditionalPrompt,
		InnerVoice:             flags.InnerVoice,
		PredictedGuidesHash:    predictedGuidesHash,
		HighPriorityNotes:      flags.HighPriorityNotes,
		PlannerContext:         flags.PlannerContext,
		DailyTodoReminder:      flags.DailyTodoReminder,
		SessionTodoItems:       flags.SessionTodoItems,
		WebhooksDefinitions:    flags.WebhooksDefinitions,
		RetrievedMemories:      flags.RetrievedMemories,
		RecentActivityOverview: flags.RecentActivityOverview,
		PredictedMemories:      flags.PredictedMemories,
		ActiveProcesses:        flags.ActiveProcesses,
		IsVoiceMode:            flags.IsVoiceMode,
		SpecialistsStatus:      flags.SpecialistsStatus,
		KnowledgeContext:       flags.KnowledgeContext,
		ErrorPatternContext:    flags.ErrorPatternContext,
		ReuseContext:           flags.ReuseContext,
		EmotionDescription:     flags.EmotionDescription,
		UserProfileSummary:     flags.UserProfileSummary,
		MessageSource:          flags.MessageSource,
		Model:                  flags.Model,
		IsTextModeModel:        flags.IsTextModeModel,
		PersonalityLine:        flags.PersonalityLine,
	}
	b, err := json.Marshal(key)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func collectEnabledTools(flags *prompts.ContextFlags) []string {
	var tools []string
	if flags.DockerEnabled {
		tools = append(tools, "docker")
	}
	if flags.HomeAssistantEnabled {
		tools = append(tools, "homeassistant")
	}
	if flags.DiscordEnabled {
		tools = append(tools, "discord")
	}
	if flags.EmailEnabled {
		tools = append(tools, "email")
	}
	if flags.WebhooksEnabled {
		tools = append(tools, "webhooks")
	}
	if flags.ProxmoxEnabled {
		tools = append(tools, "proxmox")
	}
	if flags.OllamaEnabled {
		tools = append(tools, "ollama")
	}
	if flags.TailscaleEnabled {
		tools = append(tools, "tailscale")
	}
	if flags.AnsibleEnabled {
		tools = append(tools, "ansible")
	}
	if flags.InvasionControlEnabled {
		tools = append(tools, "invasioncontrol")
	}
	if flags.GitHubEnabled {
		tools = append(tools, "github")
	}
	if flags.MQTTEnabled {
		tools = append(tools, "mqtt")
	}
	if flags.AdGuardEnabled {
		tools = append(tools, "adguard")
	}
	if flags.UptimeKumaEnabled {
		tools = append(tools, "uptimekuma")
	}
	if flags.MCPEnabled {
		tools = append(tools, "mcp")
	}
	if flags.SandboxEnabled {
		tools = append(tools, "sandbox")
	}
	if flags.MeshCentralEnabled {
		tools = append(tools, "meshcentral")
	}
	if flags.HomepageEnabled {
		tools = append(tools, "homepage")
	}
	if flags.NetlifyEnabled {
		tools = append(tools, "netlify")
	}
	if flags.VercelEnabled {
		tools = append(tools, "vercel")
	}
	if flags.GoogleWorkspaceEnabled {
		tools = append(tools, "googleworkspace")
	}
	if flags.OneDriveEnabled {
		tools = append(tools, "onedrive")
	}
	if flags.JellyfinEnabled {
		tools = append(tools, "jellyfin")
	}
	if flags.TrueNASEnabled {
		tools = append(tools, "truenas")
	}
	if flags.KoofrEnabled {
		tools = append(tools, "koofr")
	}
	if flags.ChromecastEnabled {
		tools = append(tools, "chromecast")
	}
	if flags.WebDAVEnabled {
		tools = append(tools, "webdav")
	}
	if flags.PaperlessNGXEnabled {
		tools = append(tools, "paperlessngx")
	}
	if flags.VirusTotalEnabled {
		tools = append(tools, "virustotal")
	}
	if flags.GolangciLintEnabled {
		tools = append(tools, "golangcilint")
	}
	if flags.BraveSearchEnabled {
		tools = append(tools, "bravsearch")
	}
	if flags.ImageGenerationEnabled {
		tools = append(tools, "imagegeneration")
	}
	if flags.MusicGenerationEnabled {
		tools = append(tools, "musicgeneration")
	}
	if flags.RemoteControlEnabled {
		tools = append(tools, "remotecontrol")
	}
	if flags.MemoryEnabled {
		tools = append(tools, "memory")
	}
	if flags.KnowledgeGraphEnabled {
		tools = append(tools, "knowledgegraph")
	}
	if flags.SecretsVaultEnabled {
		tools = append(tools, "secretsvault")
	}
	if flags.SchedulerEnabled {
		tools = append(tools, "scheduler")
	}
	if flags.NotesEnabled {
		tools = append(tools, "notes")
	}
	if flags.JournalEnabled {
		tools = append(tools, "journal")
	}
	if flags.MissionsEnabled {
		tools = append(tools, "missions")
	}
	if flags.StopProcessEnabled {
		tools = append(tools, "stopprocess")
	}
	if flags.InventoryEnabled {
		tools = append(tools, "inventory")
	}
	if flags.MemoryMaintenanceEnabled {
		tools = append(tools, "memorymaintenance")
	}
	if flags.WOLEnabled {
		tools = append(tools, "wol")
	}
	if flags.MediaRegistryEnabled {
		tools = append(tools, "mediaregistry")
	}
	if flags.HomepageRegistryEnabled {
		tools = append(tools, "homepageregistry")
	}
	if flags.DocumentCreatorEnabled {
		tools = append(tools, "documentcreator")
	}
	if flags.WebCaptureEnabled {
		tools = append(tools, "webcapture")
	}
	if flags.NetworkPingEnabled {
		tools = append(tools, "networkping")
	}
	if flags.WebScraperEnabled {
		tools = append(tools, "webscraper")
	}
	if flags.S3Enabled {
		tools = append(tools, "s3")
	}
	if flags.NetworkScanEnabled {
		tools = append(tools, "networkscan")
	}
	if flags.FormAutomationEnabled {
		tools = append(tools, "formautomation")
	}
	if flags.UPnPScanEnabled {
		tools = append(tools, "upnpscan")
	}
	if flags.FritzBoxSystemEnabled {
		tools = append(tools, "fritzboxsystem")
	}
	if flags.FritzBoxNetworkEnabled {
		tools = append(tools, "fritzboxnetwork")
	}
	if flags.FritzBoxTelephonyEnabled {
		tools = append(tools, "fritzboxtelephony")
	}
	if flags.FritzBoxSmartHomeEnabled {
		tools = append(tools, "fritzboxsmarthome")
	}
	if flags.FritzBoxStorageEnabled {
		tools = append(tools, "fritzboxstorage")
	}
	if flags.FritzBoxTVEnabled {
		tools = append(tools, "fritzboxtv")
	}
	if flags.TelnyxEnabled {
		tools = append(tools, "telnyx")
	}
	if flags.A2AEnabled {
		tools = append(tools, "a2a")
	}
	if flags.MiniMaxTTSEnabled {
		tools = append(tools, "minimaxtts")
	}
	if flags.CoAgentEnabled {
		tools = append(tools, "coagent")
	}
	sort.Strings(tools)
	return tools
}

func collectFeatureToggles(flags *prompts.ContextFlags) []string {
	var toggles []string
	if flags.AllowShell {
		toggles = append(toggles, "allow_shell")
	}
	if flags.AllowPython {
		toggles = append(toggles, "allow_python")
	}
	if flags.AllowFilesystemWrite {
		toggles = append(toggles, "allow_filesystem_write")
	}
	if flags.AllowNetworkRequests {
		toggles = append(toggles, "allow_network_requests")
	}
	if flags.AllowRemoteShell {
		toggles = append(toggles, "allow_remote_shell")
	}
	if flags.AllowSelfUpdate {
		toggles = append(toggles, "allow_self_update")
	}
	if flags.InternetExposed {
		toggles = append(toggles, "internet_exposed")
	}
	if flags.IsDocker {
		toggles = append(toggles, "is_docker")
	}
	if flags.NativeToolsEnabled {
		toggles = append(toggles, "native_tools")
	}
	if flags.VoiceOutputActive {
		toggles = append(toggles, "voice_output")
	}
	if flags.IsDebugMode {
		toggles = append(toggles, "debug_mode")
	}
	if flags.IsMaintenanceMode {
		toggles = append(toggles, "maintenance_mode")
	}
	if flags.LifeboatEnabled {
		toggles = append(toggles, "lifeboat")
	}
	if flags.MemoryEnabled {
		toggles = append(toggles, "memory")
	}
	if flags.KnowledgeGraphEnabled {
		toggles = append(toggles, "knowledge_graph")
	}
	if flags.SecretsVaultEnabled {
		toggles = append(toggles, "secrets_vault")
	}
	if flags.SchedulerEnabled {
		toggles = append(toggles, "scheduler")
	}
	if flags.NotesEnabled {
		toggles = append(toggles, "notes")
	}
	if flags.JournalEnabled {
		toggles = append(toggles, "journal")
	}
	if flags.MissionsEnabled {
		toggles = append(toggles, "missions")
	}
	if flags.MemoryMaintenanceEnabled {
		toggles = append(toggles, "memory_maintenance")
	}
	if flags.UnifiedMemoryBlock {
		toggles = append(toggles, "unified_memory_block")
	}
	if flags.UserProfilingEnabled {
		toggles = append(toggles, "user_profiling")
	}
	if flags.SpecialistsAvailable {
		toggles = append(toggles, "specialists")
	}
	sort.Strings(toggles)
	return toggles
}
