package agent

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/inventory"
	"aurago/internal/llm"
	loggerPkg "aurago/internal/logger"
	"aurago/internal/memory"
	"aurago/internal/meshcentral"
	"aurago/internal/prompts"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/tools"
	"aurago/internal/webhooks"

	"github.com/sashabaranov/go-openai"
)

// Agent encapsulates the agent's dependencies and state.
type Agent struct {
	Cfg          *config.Config
	Logger       *slog.Logger
	ShortTermMem *memory.SQLiteMemory
	LongTermMem  memory.VectorDB
	Vault        *security.Vault
	Registry     *tools.ProcessRegistry
	CronManager  *tools.CronManager
	KG           *memory.KnowledgeGraph
	InventoryDB  *sql.DB
	InvasionDB   *sql.DB
}

// NewAgent creates a new Agent instance.
func NewAgent(cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cron *tools.CronManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB) *Agent {
	return &Agent{
		Cfg:          cfg,
		Logger:       logger,
		ShortTermMem: stm,
		LongTermMem:  ltm,
		Vault:        vault,
		Registry:     registry,
		CronManager:  cron,
		KG:           kg,
		InventoryDB:  inventoryDB,
		InvasionDB:   invasionDB,
	}
}

// Shutdown ensures all agent resources are released properly.
func (a *Agent) Shutdown() error {
	a.Logger.Info("Agent shutdown initiated...")

	if a.ShortTermMem != nil {
		if err := a.ShortTermMem.Close(); err != nil {
			a.Logger.Error("Failed to close SQLite memory", "error", err)
		}
	}

	if a.LongTermMem != nil {
		if err := a.LongTermMem.Close(); err != nil {
			a.Logger.Error("Failed to close VectorDB", "error", err)
		}
	}

	if a.KG != nil {
		if err := a.KG.Close(); err != nil {
			a.Logger.Error("Failed to close Knowledge Graph", "error", err)
		}
	}

	a.Logger.Info("Agent shutdown completed.")
	return nil
}

// FeedbackBroker provides an abstraction for real-time status updates,
// allowing the reasoning loop to be used by multiple transports (SSE, Telegram, etc.)

var (
	GlobalTokenCount     int
	GlobalTokenEstimated bool
	muTokens             sync.Mutex

	sessionInterrupts = make(map[string]bool)
	muInterrupts      sync.Mutex

	debugModeEnabled bool
	muDebugMode      sync.Mutex
)

// SetDebugMode enables or disables the runtime debug mode for the agent.
// When enabled, the agent's system prompt includes an extra debugging instruction.
func SetDebugMode(enabled bool) {
	muDebugMode.Lock()
	defer muDebugMode.Unlock()
	debugModeEnabled = enabled
}

// GetDebugMode returns whether debug mode is currently active.
func GetDebugMode() bool {
	muDebugMode.Lock()
	defer muDebugMode.Unlock()
	return debugModeEnabled
}

// ToggleDebugMode flips the current debug mode state and returns the new value.
func ToggleDebugMode() bool {
	muDebugMode.Lock()
	defer muDebugMode.Unlock()
	debugModeEnabled = !debugModeEnabled
	return debugModeEnabled
}

// InterruptSession marks a specific session as interrupted.
func InterruptSession(sessionID string) {
	muInterrupts.Lock()
	defer muInterrupts.Unlock()
	sessionInterrupts[sessionID] = true
}

// checkAndClearInterrupt returns true if the session was interrupted and clears the flag.
func checkAndClearInterrupt(sessionID string) bool {
	muInterrupts.Lock()
	defer muInterrupts.Unlock()
	if sessionInterrupts[sessionID] {
		delete(sessionInterrupts, sessionID)
		return true
	}
	return false
}

// estimateTokens provides a rough character-based token count for when the API doesn't return one.
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// Rough heuristic: 1 token per 4 characters
	return len(text) / 4
}

// ── Recency-Boosted Re-ranking (Phase A3) ──────────────────────────

type FeedbackBroker interface {
	Send(event, message string)
	SendJSON(jsonStr string)
}

// NoopBroker is a silent fallback for transports that don't support real-time feedback
type NoopBroker struct{}

func (n NoopBroker) Send(event, message string) {}
func (n NoopBroker) SendJSON(jsonStr string)    {}

// ToolCall represents a parsed tool invocation from the LLM.
type ToolCall struct {
	Action             string                 `json:"action"`
	Code               string                 `json:"code"`
	Key                string                 `json:"key"`
	Value              string                 `json:"value"`
	Name               string                 `json:"name"`
	Description        string                 `json:"description"`
	Package            string                 `json:"package"`
	Args               interface{}            `json:"args"`
	Background         bool                   `json:"background"`
	PID                int                    `json:"pid"`
	IsTool             bool                   `json:"-"`
	RawCodeDetected    bool                   `json:"-"`
	RawJSON            string                 `json:"-"`
	NativeCallID       string                 `json:"-"` // Native API tool call ID for role=tool responses
	Operation          string                 `json:"operation"`
	Fact               string                 `json:"fact"`
	ID                 string                 `json:"id"`
	CronExpr           string                 `json:"cron_expr"`
	TaskPrompt         string                 `json:"task_prompt"`
	Skill              string                 `json:"skill"`
	SkillArgs          map[string]interface{} `json:"skill_args"`
	Content            string                 `json:"content"`
	Query              string                 `json:"query"` // Alias for content in query_memory
	Metadata           map[string]interface{} `json:"metadata"`
	FilePath           string                 `json:"file_path"`
	Path               string                 `json:"path"` // Alias for file_path
	Destination        string                 `json:"destination"`
	Dest               string                 `json:"dest"` // Alias for destination
	URL                string                 `json:"url"`
	Method             string                 `json:"method"`
	Headers            map[string]string      `json:"headers"`
	Params             map[string]interface{} `json:"params"`
	WebhookName        string                 `json:"webhook_name"`
	Parameters         interface{}            `json:"parameters"`
	PayloadType        string                 `json:"payload_type"`
	BodyTemplate       string                 `json:"body_template"`
	Tag                string                 `json:"tag"`
	Hostname           string                 `json:"hostname"`
	ServerID           string                 `json:"server_id"`
	MemoryKey          string                 `json:"memory_key"`   // Synonym for fact
	MemoryValue        string                 `json:"memory_value"` // Synonym for fact/content
	NotifyOnCompletion bool                   `json:"notify_on_completion"`
	Body               string                 `json:"body"`
	Source             string                 `json:"source"`
	Target             string                 `json:"target"`
	Relation           string                 `json:"relation"`
	Properties         map[string]string      `json:"properties"`
	Preview            bool                   `json:"preview"`
	Port               int                    `json:"port"`
	Username           string                 `json:"username"`
	Password           string                 `json:"password"`
	Owner              string                 `json:"owner"`
	PrivateKeyPath     string                 `json:"private_key_path"`
	Tags               string                 `json:"tags"`
	Direction          string                 `json:"direction"`
	LocalPath          string                 `json:"local_path"`
	RemotePath         string                 `json:"remote_path"`
	ToolName           string                 `json:"tool_name"`
	Tool               string                 `json:"tool"`         // Hallucination fallback
	Arguments          interface{}            `json:"arguments"`    // Hallucination fallback
	ActionInput        map[string]interface{} `json:"action_input"` // LangChain-style nested params
	Label              string                 `json:"label"`
	Command            string                 `json:"command"`
	ThresholdLow       int                    `json:"threshold_low"`
	ThresholdMedium    int                    `json:"threshold_medium"`
	Pinned             bool                   `json:"pinned"`
	Locked             bool                   `json:"locked"`
	IPAddress          string                 `json:"ip_address"`
	To                 string                 `json:"to"`
	CC                 string                 `json:"cc"`
	Subject            string                 `json:"subject"`
	Folder             string                 `json:"folder"`
	Limit              int                    `json:"limit"`
	Account            string                 `json:"account"` // email account ID (multi-account)
	ChannelID          string                 `json:"channel_id"`
	Message            string                 `json:"message"`
	// Notes / To-Do fields
	Title    string `json:"title"`
	Priority int    `json:"priority"`
	DueDate  string `json:"due_date"`
	Category string `json:"category"`
	Done     int    `json:"done"` // -1=all, 0=open, 1=done (filter for list)
	// Inventory / Device fields
	DeviceType string `json:"device_type,omitempty"`
	MACAddress string `json:"mac_address,omitempty"` // Optional MAC for Wake-on-LAN
	NoteID     int64  `json:"note_id"`
	// Google Workspace fields
	DocumentID string `json:"document_id"`
	MaxResults int    `json:"max_results"`
	Append     bool   `json:"append"`
	// Vision / STT fields
	Prompt string `json:"prompt"`
	// Home Assistant fields
	EntityID    string                 `json:"entity_id"`
	Domain      string                 `json:"domain"`
	Service     string                 `json:"service"`
	ServiceData map[string]interface{} `json:"service_data"`
	// Docker fields
	ContainerID string            `json:"container_id"`
	Image       string            `json:"image"`
	Env         []string          `json:"env"`
	Ports       map[string]string `json:"ports"`
	Volumes     []string          `json:"volumes"`
	Restart     string            `json:"restart"`
	Force       bool              `json:"force"`
	Tail        int               `json:"tail"`
	All         bool              `json:"all"`
	Network     string            `json:"network"`
	Driver      string            `json:"driver"`
	User        string            `json:"user"`
	File        string            `json:"file"`
	// Co-Agent fields
	CoAgentID    string   `json:"co_agent_id"`
	Task         string   `json:"task"`
	ContextHints []string `json:"context_hints"`
	// TTS / Chromecast fields
	Text        string  `json:"text"`
	DeviceAddr  string  `json:"device_addr"`
	DeviceName  string  `json:"device_name"`
	DevicePort  int     `json:"device_port"`
	Volume      float64 `json:"volume"`
	ContentType string  `json:"content_type"`
	Language    string  `json:"language"`
	// MDNS fields
	ServiceType string `json:"service_type"`
	Timeout     int    `json:"timeout"`
	// MeshCentral fields
	MeshID      string `json:"mesh_id"`
	NodeID      string `json:"node_id"`
	PowerAction int    `json:"power_action"`
	// Notification fields
	Channel string `json:"channel"`
	// Webhook fields
	Slug    string `json:"slug"`
	TokenID string `json:"token_id"`
	Enabled bool   `json:"enabled"`
	// Proxmox fields
	VMID         string `json:"vmid"`
	VMType       string `json:"vm_type"`
	ResourceType string `json:"resource_type"`
	UPID         string `json:"upid"`
	// Ollama fields
	Model string `json:"model"`
	// Ansible fields
	Module    string `json:"module"`     // ansible module name for adhoc (e.g. "ping", "shell", "copy")
	HostLimit string `json:"host_limit"` // ansible --limit: restrict playbook/adhoc to subset of hosts
	SkipTags  string `json:"skip_tags"`  // ansible --skip-tags
	Inventory string `json:"inventory"`  // inventory path override (defaults to sidecar default)
	// Invasion Control fields
	NestID   string `json:"nest_id"`   // invasion nest ID for nest_status/assign_egg
	NestName string `json:"nest_name"` // invasion nest name (alternative lookup)
	EggID    string `json:"egg_id"`    // invasion egg ID for assign_egg
	// Image sending
	Caption string `json:"caption"`
	// MQTT fields
	Topic   string `json:"topic"`
	Payload string `json:"payload"`
	Retain  bool   `json:"retain"`
	QoS     int    `json:"qos"`
	// MCP fields
	Server  string                 `json:"server"`
	MCPArgs map[string]interface{} `json:"mcp_args"`
	// Sandbox fields
	SandboxLang string   `json:"sandbox_lang"` // language for execute_sandbox (python, javascript, go, etc.)
	Libraries   []string `json:"libraries"`    // optional packages to install before running sandbox code
	// Homepage fields
	Framework  string   `json:"framework"`   // web framework: next, vite, astro, svelte, vue, html
	Viewport   string   `json:"viewport"`    // screenshot viewport: "1280x720"
	Packages   []string `json:"packages"`    // npm packages to install
	ProjectDir string   `json:"project_dir"` // subdirectory within /workspace
	BuildDir   string   `json:"build_dir"`   // build output directory (auto-detected if empty)
	// Circuit Breaker Override - ermöglicht temporäre Erhöhung des Limits für komplexe Operationen
	CircuitBreakerOverride int `json:"circuit_breaker_override,omitempty"`
	// Netlify fields
	SiteID       string `json:"site_id"`       // Netlify site ID
	DeployID     string `json:"deploy_id"`     // Netlify deploy ID
	FormID       string `json:"form_id"`       // Netlify form ID
	HookID       string `json:"hook_id"`       // Netlify hook ID
	EnvKey       string `json:"env_key"`       // environment variable key
	EnvValue     string `json:"env_value"`     // environment variable value
	EnvContext   string `json:"env_context"`   // env var context: all, production, deploy-preview, branch-deploy, dev
	SiteName     string `json:"site_name"`     // site subdomain name (for create)
	Draft        bool   `json:"draft"`         // deploy as draft
	HookType     string `json:"hook_type"`     // hook type: url, email, slack
	HookEvent    string `json:"hook_event"`    // hook event: deploy_created, deploy_building, deploy_failed, etc.
	CustomDomain string `json:"custom_domain"` // custom domain for site
}

// GetArgs returns Args as a string slice, handling various input types (slice of strings or interface).
func (tc ToolCall) GetArgs() []string {
	if tc.Args == nil {
		return nil
	}
	if slice, ok := tc.Args.([]string); ok {
		return slice
	}
	if slice, ok := tc.Args.([]interface{}); ok {
		var res []string
		for _, v := range slice {
			if s, ok := v.(string); ok {
				res = append(res, s)
			} else {
				res = append(res, fmt.Sprintf("%v", v))
			}
		}
		return res
	}
	return nil
}

// RunConfig holds all the dependencies required to run the agent loop,
// consolidating the parameter list that was previously over 20 items long.
type RunConfig struct {
	Config          *config.Config
	Logger          *slog.Logger
	LLMClient       llm.ChatClient
	ShortTermMem    *memory.SQLiteMemory
	HistoryManager  *memory.HistoryManager
	LongTermMem     memory.VectorDB
	KG              *memory.KnowledgeGraph
	InventoryDB     *sql.DB
	InvasionDB      *sql.DB
	Vault           *security.Vault
	Registry        *tools.ProcessRegistry
	Manifest        *tools.Manifest
	CronManager     *tools.CronManager
	MissionManager  *tools.MissionManager
	CoAgentRegistry *CoAgentRegistry
	BudgetTracker   *budget.Tracker
	SessionID       string
	IsMaintenance   bool
	SurgeryPlan     string
}

// ExecuteAgentLoop executes the multi-turn reasoning and tool execution loop.
// It supports both synchronous returns and asynchronous streaming via the broker.
func ExecuteAgentLoop(ctx context.Context, req openai.ChatCompletionRequest, runCfg RunConfig, stream bool, broker FeedbackBroker) (openai.ChatCompletionResponse, error) {
	cfg := runCfg.Config
	logger := runCfg.Logger
	client := runCfg.LLMClient
	shortTermMem := runCfg.ShortTermMem
	historyManager := runCfg.HistoryManager
	longTermMem := runCfg.LongTermMem
	kg := runCfg.KG
	inventoryDB := runCfg.InventoryDB
	invasionDB := runCfg.InvasionDB
	vault := runCfg.Vault
	registry := runCfg.Registry
	manifest := runCfg.Manifest
	cronManager := runCfg.CronManager
	missionManager := runCfg.MissionManager
	coAgentRegistry := runCfg.CoAgentRegistry
	budgetTracker := runCfg.BudgetTracker
	sessionID := runCfg.SessionID
	isMaintenance := runCfg.IsMaintenance
	surgeryPlan := runCfg.SurgeryPlan
	var webhooksDef strings.Builder
	if cfg.Webhooks.Enabled && len(cfg.Webhooks.Outgoing) > 0 {
		for _, w := range cfg.Webhooks.Outgoing {
			webhooksDef.WriteString(fmt.Sprintf("- **%s**: %s\n", w.Name, w.Description))
			if len(w.Parameters) > 0 {
				webhooksDef.WriteString("  Parameters:\n")
				for _, p := range w.Parameters {
					reqStr := ""
					if p.Required {
						reqStr = " (required)"
					}
					webhooksDef.WriteString(fmt.Sprintf("    - `%s` [%s]%s: %s\n", p.Name, p.Type, reqStr, p.Description))
				}
			}
		}
	}

	flags := prompts.ContextFlags{
		IsErrorState:             false,
		RequiresCoding:           false,
		SystemLanguage:           cfg.Agent.SystemLanguage,
		LifeboatEnabled:          cfg.Maintenance.LifeboatEnabled,
		IsMaintenanceMode:        isMaintenance,
		SurgeryPlan:              surgeryPlan,
		CorePersonality:          cfg.Agent.CorePersonality,
		TokenBudget:              cfg.Agent.SystemPromptTokenBudget,
		IsDebugMode:              cfg.Agent.DebugMode || GetDebugMode(),
		IsCoAgent:                strings.HasPrefix(sessionID, "coagent-"),
		DiscordEnabled:           cfg.Discord.Enabled,
		EmailEnabled:             cfg.Email.Enabled,
		DockerEnabled:            cfg.Docker.Enabled,
		HomeAssistantEnabled:     cfg.HomeAssistant.Enabled,
		WebDAVEnabled:            cfg.WebDAV.Enabled,
		KoofrEnabled:             cfg.Koofr.Enabled,
		ChromecastEnabled:        cfg.Chromecast.Enabled,
		CoAgentEnabled:           cfg.CoAgents.Enabled,
		GoogleWorkspaceEnabled:   cfg.Agent.EnableGoogleWorkspace,
		ProxmoxEnabled:           cfg.Proxmox.Enabled,
		OllamaEnabled:            cfg.Ollama.Enabled,
		TailscaleEnabled:         cfg.Tailscale.Enabled,
		AnsibleEnabled:           cfg.Ansible.Enabled,
		InvasionControlEnabled:   cfg.InvasionControl.Enabled && invasionDB != nil,
		GitHubEnabled:            cfg.GitHub.Enabled,
		MQTTEnabled:              cfg.MQTT.Enabled,
		MCPEnabled:               cfg.MCP.Enabled && cfg.Agent.AllowMCP,
		SandboxEnabled:           cfg.Sandbox.Enabled,
		MeshCentralEnabled:       cfg.MeshCentral.Enabled,
		HomepageEnabled:          cfg.Homepage.Enabled && cfg.Docker.Enabled,
		NetlifyEnabled:           cfg.Netlify.Enabled,
		WebhooksEnabled:          cfg.Webhooks.Enabled,
		WebhooksDefinitions:      webhooksDef.String(),
		VirusTotalEnabled:        cfg.VirusTotal.Enabled,
		BraveSearchEnabled:       cfg.BraveSearch.Enabled,
		MemoryEnabled:            cfg.Tools.Memory.Enabled,
		KnowledgeGraphEnabled:    cfg.Tools.KnowledgeGraph.Enabled,
		SecretsVaultEnabled:      cfg.Tools.SecretsVault.Enabled,
		SchedulerEnabled:         cfg.Tools.Scheduler.Enabled,
		NotesEnabled:             cfg.Tools.Notes.Enabled,
		MissionsEnabled:          cfg.Tools.Missions.Enabled,
		StopProcessEnabled:       cfg.Tools.StopProcess.Enabled,
		InventoryEnabled:         cfg.Tools.Inventory.Enabled,
		MemoryMaintenanceEnabled: cfg.Tools.MemoryMaintenance.Enabled,
		WOLEnabled:               cfg.Tools.WOL.Enabled,
		AllowShell:               cfg.Agent.AllowShell,
		AllowPython:              cfg.Agent.AllowPython,
		AllowFilesystemWrite:     cfg.Agent.AllowFilesystemWrite,
		AllowNetworkRequests:     cfg.Agent.AllowNetworkRequests,
		AllowRemoteShell:         cfg.Agent.AllowRemoteShell,
		AllowSelfUpdate:          cfg.Agent.AllowSelfUpdate,
		IsEgg:                    cfg.EggMode.Enabled,
		AdditionalPrompt:         cfg.Agent.AdditionalPrompt,
	}
	toolCallCount := 0
	rawCodeCount := 0
	missedToolCount := 0
	announcementCount := 0
	sessionTokens := 0
	emptyRetried := false // Prevents infinite retry on persistent empty responses
	stepsSinceLastFeedback := 0
	lastToolError := ""        // Tracks the last tool error string for consecutive-error detection
	consecutiveErrorCount := 0 // Incremented each time the same tool error repeats back-to-back

	// Guardian: prompt injection defense
	guardian := security.NewGuardian(logger)

	var currentLogger *slog.Logger = logger
	lastActivity := time.Now()
	lastTool := ""
	recentTools := make([]string, 0, 5) // Track last 5 tools for lazy schema injection
	explicitTools := make([]string, 0)  // Explicit tool guides requested via <workflow_plan> tag
	workflowPlanCount := 0              // Prevent infinite workflow_plan loops
	lastResponseWasTool := false        // True when the previous iteration was a tool call; suppresses announcement detector on completion messages
	pendingTCs := make([]ToolCall, 0)   // Queued tool calls from multi-tool responses (processed without a new LLM call)

	// Core memory cache: read once, invalidate on manage_memory calls
	coreMemCache := ""
	coreMemDirty := true // Force initial load

	// Phase D: Personality Engine (opt-in)
	personalityEnabled := cfg.Agent.PersonalityEngine
	if personalityEnabled && shortTermMem != nil {
		if err := shortTermMem.InitPersonalityTables(); err != nil {
			logger.Error("[Personality] Failed to init tables, disabling", "error", err)
			personalityEnabled = false
		}
	}

	// Native function calling: build tool schemas once and attach to request
	toolGuidesDir := filepath.Join(cfg.Directories.PromptsDir, "tools_manuals")

	// Auto-detect DeepSeek and enable native function calling
	useNativeFunctions := cfg.LLM.UseNativeFunctions
	if strings.Contains(strings.ToLower(cfg.LLM.Model), "deepseek") && !useNativeFunctions {
		useNativeFunctions = true
		logger.Info("[NativeTools] DeepSeek detected, auto-enabling native function calling")
	}

	if useNativeFunctions {
		ff := ToolFeatureFlags{
			HomeAssistantEnabled:     cfg.HomeAssistant.Enabled,
			DockerEnabled:            cfg.Docker.Enabled,
			CoAgentEnabled:           cfg.CoAgents.Enabled,
			SudoEnabled:              cfg.Agent.SudoEnabled,
			WebhooksEnabled:          cfg.Webhooks.Enabled,
			ProxmoxEnabled:           cfg.Proxmox.Enabled,
			OllamaEnabled:            cfg.Ollama.Enabled,
			TailscaleEnabled:         cfg.Tailscale.Enabled,
			AnsibleEnabled:           cfg.Ansible.Enabled,
			InvasionControlEnabled:   cfg.InvasionControl.Enabled && invasionDB != nil,
			GitHubEnabled:            cfg.GitHub.Enabled,
			MQTTEnabled:              cfg.MQTT.Enabled,
			MCPEnabled:               cfg.MCP.Enabled && cfg.Agent.AllowMCP,
			SandboxEnabled:           cfg.Sandbox.Enabled,
			MeshCentralEnabled:       cfg.MeshCentral.Enabled,
			HomepageEnabled:          cfg.Homepage.Enabled && cfg.Docker.Enabled,
			NetlifyEnabled:           cfg.Netlify.Enabled,
			MemoryEnabled:            cfg.Tools.Memory.Enabled,
			KnowledgeGraphEnabled:    cfg.Tools.KnowledgeGraph.Enabled,
			SecretsVaultEnabled:      cfg.Tools.SecretsVault.Enabled,
			SchedulerEnabled:         cfg.Tools.Scheduler.Enabled,
			NotesEnabled:             cfg.Tools.Notes.Enabled,
			MissionsEnabled:          cfg.Tools.Missions.Enabled,
			StopProcessEnabled:       cfg.Tools.StopProcess.Enabled,
			InventoryEnabled:         cfg.Tools.Inventory.Enabled,
			MemoryMaintenanceEnabled: cfg.Tools.MemoryMaintenance.Enabled,
			WOLEnabled:               cfg.Tools.WOL.Enabled,
		}
		ntSchemas := BuildNativeToolSchemas(cfg.Directories.SkillsDir, manifest, cfg.Agent.EnableGoogleWorkspace, ff, logger)
		// Structured Outputs: set Strict=true on every tool definition so the
		// provider uses constrained decoding for tool-call arguments.
		// Only enable this for models that support structured outputs (e.g. GPT-4o,
		// some OpenRouter models). Ollama does not support strict mode.
		isOllama := strings.EqualFold(cfg.LLM.ProviderType, "ollama")
		if cfg.LLM.StructuredOutputs && !isOllama {
			for i := range ntSchemas {
				if ntSchemas[i].Function != nil {
					ntSchemas[i].Function.Strict = true
				}
			}
			logger.Info("[NativeTools] Structured outputs enabled (strict mode)")
		} else if cfg.LLM.StructuredOutputs && isOllama {
			logger.Warn("[NativeTools] Structured outputs not supported by Ollama, ignoring")
		}
		req.Tools = ntSchemas
		req.ToolChoice = "auto"
		// Ollama does not support parallel_tool_calls — only set for compatible providers
		if !isOllama {
			req.ParallelToolCalls = true
		}
		logger.Info("[NativeTools] Native function calling enabled", "tool_count", len(ntSchemas), "parallel", !isOllama)
	}

	for {
		// Check for user interrupt
		if checkAndClearInterrupt(sessionID) {
			currentLogger.Warn("[Sync] User interrupted the agent")
			interruptMsg := "the user has interrupted your work. ask what is wrong"
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: interruptMsg})
			// Reset error states to focus on the new interrupt msg
			flags.IsErrorState = false
			broker.Send("thinking", "User interrupted. Asking for instructions...")
			continue
		}

		// Revive logic: If idle in lifeboat for too long, poke the agent
		if isMaintenance && time.Since(lastActivity) > time.Duration(cfg.CircuitBreaker.MaintenanceTimeoutMinutes)*time.Minute {
			currentLogger.Warn("[Sync] Lifeboat idle for too long, injecting revive prompt", "minutes", cfg.CircuitBreaker.MaintenanceTimeoutMinutes)
			reviveMsg := "You are idle in the lifeboat. finish your tasks or change back to the supervisor."
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: reviveMsg})
			lastActivity = time.Now() // Reset timer
		}

		// Refresh maintenance status to account for mid-loop handovers
		isMaintenance = isMaintenance || tools.IsBusy()
		flags.IsMaintenanceMode = isMaintenance

		// Caching the logger to avoid opening file on every iteration (leaking FDs)
		if isMaintenance && currentLogger == nil {
			logPath := filepath.Join(cfg.Logging.LogDir, "lifeboat.log")
			if l, err := loggerPkg.SetupWithFile(true, logPath, true); err == nil {
				currentLogger = l.Logger
			}
		}
		if currentLogger == nil {
			currentLogger = logger
		}

		currentLogger.Debug("[Sync] Agent loop iteration starting", "is_maintenance", isMaintenance, "lock_exists", tools.IsBusy())

		// Process queued tool calls from multi-tool responses (skip LLM for these)
		if len(pendingTCs) > 0 {
			ptc := pendingTCs[0]
			pendingTCs = pendingTCs[1:]
			toolCallCount++
			broker.Send("thinking", fmt.Sprintf("[%d] Running %s...", toolCallCount, ptc.Action))
			ptcJSON := ptc.RawJSON
			if ptcJSON == "" {
				ptcJSON = fmt.Sprintf(`{"action":"%s"}`, ptc.Action)
			}
			id, idErr := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, ptcJSON, false, true)
			if idErr != nil {
				currentLogger.Error("Failed to persist queued tool-call message", "error", idErr)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleAssistant, ptcJSON, id, false, true)
			}
			broker.Send("tool_call", ptcJSON)
			broker.Send("tool_start", ptc.Action)
			pResultContent := DispatchToolCall(ctx, ptc, cfg, currentLogger, client, vault, registry, manifest, cronManager, missionManager, longTermMem, shortTermMem, kg, inventoryDB, invasionDB, historyManager, tools.IsBusy(), surgeryPlan, guardian, sessionID, coAgentRegistry, budgetTracker)
			broker.Send("tool_output", pResultContent)
			if ptc.Action == "send_image" {
				var imgRes struct {
					Status  string `json:"status"`
					WebPath string `json:"web_path"`
					Caption string `json:"caption"`
				}
				raw := strings.TrimPrefix(pResultContent, "[Tool Output]\n")
				raw = strings.TrimPrefix(raw, "Tool Output: ")
				if json.Unmarshal([]byte(raw), &imgRes) == nil && imgRes.Status == "success" {
					evtPayload, _ := json.Marshal(map[string]string{"path": imgRes.WebPath, "caption": imgRes.Caption})
					broker.Send("image", string(evtPayload))
				}
			}
			broker.Send("tool_end", ptc.Action)
			lastActivity = time.Now()
			if ptc.Action == "manage_memory" || ptc.Action == "core_memory" {
				coreMemDirty = true
			}
			id, idErr = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, pResultContent, false, true)
			if idErr != nil {
				currentLogger.Error("Failed to persist queued tool-result message", "error", idErr)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleUser, pResultContent, id, false, true)
			}
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: ptcJSON})
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: pResultContent})
			lastResponseWasTool = true
			continue
		}

		// Load Personality Meta
		var meta memory.PersonalityMeta
		if personalityEnabled {
			meta = prompts.GetCorePersonalityMeta(cfg.Directories.PromptsDir, flags.CorePersonality)
		}

		// Circuit breaker - berechne Basis-Limit (Tool-spezifische Anpassungen erfolgen später wenn tc bekannt ist)
		effectiveMaxCalls := calculateEffectiveMaxCalls(cfg, ToolCall{}, personalityEnabled, shortTermMem, currentLogger)

		if toolCallCount >= effectiveMaxCalls {
			currentLogger.Warn("[Sync] Circuit breaker triggered", "count", toolCallCount, "limit", effectiveMaxCalls)
			breakerMsg := fmt.Sprintf("CIRCUIT BREAKER: You have reached the maximum of %d consecutive tool calls. You MUST now summarize your progress and respond to the user with a final answer.", effectiveMaxCalls)
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: breakerMsg})
		}

		flags.ActiveProcesses = GetActiveProcessStatus(registry)

		// Load Core Memory (cached, invalidated when manage_memory is called)
		if coreMemDirty {
			if shortTermMem != nil {
				coreMemCache = shortTermMem.ReadCoreMemory()
			}
			coreMemDirty = false
		}

		// Extract explicit workflow tools if present (populated from previous iteration's <workflow_plan> tag)
		// explicitTools is persistent across loop iterations

		// Prepare Dynamic Tool Guides
		lastUserMsg := ""
		if len(req.Messages) > 0 && req.Messages[len(req.Messages)-1].Role == openai.ChatMessageRoleUser {
			lastUserMsg = req.Messages[len(req.Messages)-1].Content
		}

		// Get the mood trigger context from the message history
		triggerValue := getMoodTrigger(req.Messages, lastUserMsg)
		moodTrigger := func() string { return triggerValue }

		// Note: The call to PrepareDynamicGuides will happen after the response is received
		// We initialize flags.PredictedGuides now with empty explicit tools to satisfy builder.go for the first prompt
		flags.PredictedGuides = prompts.PrepareDynamicGuides(longTermMem, shortTermMem, lastUserMsg, lastTool, toolGuidesDir, recentTools, explicitTools, currentLogger)

		// Automatic RAG: retrieve relevant long-term memories for the current user message
		// Phase A3: Over-fetch and re-rank with recency boost from memory_meta
		flags.RetrievedMemories = ""
		flags.PredictedMemories = ""
		if lastUserMsg != "" && longTermMem != nil {
			// Over-fetch 6 candidates, then re-rank to keep best 3
			memories, docIDs, err := longTermMem.SearchSimilar(lastUserMsg, 6)
			if err == nil && len(memories) > 0 {
				ranked := rerankWithRecency(memories, docIDs, shortTermMem, currentLogger)
				for _, r := range ranked {
					_ = shortTermMem.UpdateMemoryAccess(r.docID)
				}
				if len(ranked) > 3 {
					ranked = ranked[:3]
				}
				var topMemories []string
				for _, r := range ranked {
					topMemories = append(topMemories, r.text)
				}
				flags.RetrievedMemories = strings.Join(topMemories, "\n---\n")
				currentLogger.Debug("[Sync] RAG: Retrieved memories (recency-boosted)", "count", len(ranked))
			}

			// Phase A4: Record interaction pattern for temporal learning
			if shortTermMem != nil {
				topic := lastUserMsg
				if len(topic) > 80 {
					topic = topic[:80]
				}
				_ = shortTermMem.RecordInteraction(topic)
			}

			// Phase B: Predictive pre-fetch based on temporal patterns + tool transitions
			if shortTermMem != nil {
				now := time.Now()
				predictions, err := shortTermMem.PredictNextQuery(lastTool, now.Hour(), int(now.Weekday()), 2)
				if err == nil && len(predictions) > 0 {
					var predictedResults []string
					for _, pred := range predictions {
						// Use SearchMemoriesOnly: predictive pre-fetch needs only user memories,
						// not tool_guides/documentation — avoids 2 full extra search cycles per request.
						pMem, _, pErr := longTermMem.SearchMemoriesOnly(pred, 1)
						if pErr == nil && len(pMem) > 0 {
							predictedResults = append(predictedResults, pMem[0])
						}
					}
					if len(predictedResults) > 0 {
						flags.PredictedMemories = strings.Join(predictedResults, "\n---\n")
						currentLogger.Debug("[Sync] Predictive RAG: Pre-fetched memories", "count", len(predictedResults), "predictions", predictions)
					}
				}
			}
		}

		// Phase D: Inject personality line before building system prompt
		if personalityEnabled && shortTermMem != nil {
			if cfg.Agent.PersonalityEngineV2 {
				// V2 Feature: Narrative Events based on Milestones & Loneliness
				processBehavioralEvents(shortTermMem, &req.Messages, sessionID, meta, currentLogger)
			}
			flags.PersonalityLine = shortTermMem.GetPersonalityLine(cfg.Agent.PersonalityEngineV2)
		}

		// User Profile: inject compact summary if profiling is enabled
		if cfg.Agent.UserProfiling && cfg.Agent.PersonalityEngineV2 && shortTermMem != nil {
			flags.UserProfileSummary = shortTermMem.GetUserProfileSummary(cfg.Agent.UserProfilingThreshold)
		}

		// Adaptive tier: adjust prompt complexity based on conversation length
		flags.MessageCount = len(req.Messages)
		flags.Tier = prompts.DetermineTier(flags.MessageCount)
		flags.RecentlyUsedTools = recentTools
		flags.IsDebugMode = cfg.Agent.DebugMode || GetDebugMode() // re-check each iteration (toggleable at runtime)

		sysPrompt := prompts.BuildSystemPrompt(cfg.Directories.PromptsDir, flags, coreMemCache, currentLogger)

		// Inject budget hint into system prompt when threshold is crossed
		if budgetTracker != nil {
			if hint := budgetTracker.GetPromptHint(); hint != "" {
				sysPrompt += "\n\n" + hint
			}
		}

		currentLogger.Debug("[Sync] System prompt rebuilt", "length", len(sysPrompt), "tier", flags.Tier, "tokens", prompts.CountTokens(sysPrompt), "error_state", flags.IsErrorState, "coding_mode", flags.RequiresCoding, "active_daemons", flags.ActiveProcesses)

		if len(req.Messages) > 0 && req.Messages[0].Role == openai.ChatMessageRoleSystem {
			req.Messages[0].Content = sysPrompt
		} else {
			req.Messages = append([]openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
			}, req.Messages...)
		}

		// ── Context window guard ──
		// Count total tokens across all messages and trim old history if we would
		// exceed the model's context window. We keep the system prompt (index 0) and
		// always preserve the final user message so the model has something to answer.
		// A 4096-token margin is reserved for the model's completion output.
		ctxWindow := cfg.Agent.ContextWindow
		if ctxWindow <= 0 {
			ctxWindow = 163840 // sensible default matching common 160k-context models
		}
		completionMargin := 4096
		maxHistoryTokens := ctxWindow - completionMargin
		if maxHistoryTokens < 4096 {
			maxHistoryTokens = 4096
		}
		totalMsgTokens := 0
		for _, m := range req.Messages {
			totalMsgTokens += prompts.CountTokens(m.Content) + 4 // ~4 tokens overhead per message
		}
		if totalMsgTokens > maxHistoryTokens && len(req.Messages) > 2 {
			currentLogger.Warn("[ContextGuard] Token limit exceeded before LLM call — trimming history",
				"tokens", totalMsgTokens, "limit", maxHistoryTokens, "messages", len(req.Messages))
			sysMsg := req.Messages[0]
			lastMsg := req.Messages[len(req.Messages)-1]
			// Drop messages from index 1 onward (oldest first) until we fit.
			// Always keep system (0) and the latest message.
			mid := req.Messages[1 : len(req.Messages)-1]
			for totalMsgTokens > maxHistoryTokens && len(mid) > 0 {
				dropped := mid[0]
				mid = mid[1:]
				totalMsgTokens -= prompts.CountTokens(dropped.Content) + 4
			}
			req.Messages = append([]openai.ChatCompletionMessage{sysMsg}, append(mid, lastMsg)...)
			currentLogger.Info("[ContextGuard] History trimmed",
				"remaining_messages", len(req.Messages), "estimated_tokens", totalMsgTokens)
		}

		// Verbose Logging of LLM Request
		if len(req.Messages) > 0 {
			lastMsg := req.Messages[len(req.Messages)-1]
			// Keep conversation logs in the original logger (stdout) to avoid pollution of technical log
			logger.Info("[LLM Request]", "role", lastMsg.Role, "content_len", len(lastMsg.Content), "preview", Truncate(lastMsg.Content, 200))
			currentLogger.Info("[LLM Request Redirected]", "role", lastMsg.Role, "content_len", len(lastMsg.Content))
			currentLogger.Debug("[LLM Full History]", "messages_count", len(req.Messages))
		}

		broker.Send("thinking", "")

		// ── Temperature: base from config + personality modulation ──
		baseTemp := cfg.LLM.Temperature
		if baseTemp <= 0 {
			baseTemp = 0.7
		}
		tempDelta := 0.0
		if personalityEnabled && shortTermMem != nil {
			tempDelta = shortTermMem.GetTemperatureDelta()
		}
		effectiveTemp := baseTemp + tempDelta
		// Clamp to safe range [0.05, 1.5] — never fully deterministic, never too wild
		if effectiveTemp < 0.05 {
			effectiveTemp = 0.05
		}
		if effectiveTemp > 1.5 {
			effectiveTemp = 1.5
		}
		req.Temperature = float32(effectiveTemp)
		if tempDelta != 0 {
			currentLogger.Debug("[Temperature] Personality modulation applied", "base", baseTemp, "delta", tempDelta, "effective", effectiveTemp)
		}

		// Budget check: block if daily budget exceeded and enforcement = full
		if budgetTracker != nil && budgetTracker.IsBlocked("chat") {
			broker.Send("budget_blocked", "Daily budget exceeded. All LLM calls blocked until reset.")
			return openai.ChatCompletionResponse{}, fmt.Errorf("budget exceeded (enforcement=full)")
		}

		// Configurable timeout for each individual LLM call to prevent infinite hangs
		llmCtx, cancelResp := context.WithTimeout(ctx, time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds)*time.Second)

		var resp openai.ChatCompletionResponse
		var content string
		var err error
		var promptTokens, completionTokens, totalTokens int

		if stream {
			stm, streamErr := llm.ExecuteStreamWithRetry(llmCtx, client, req, currentLogger, broker)
			if streamErr != nil {
				cancelResp()
				// Same 422 recovery as the sync path: trim malformed history and retry.
				if strings.Contains(streamErr.Error(), "422") || strings.Contains(strings.ToLower(streamErr.Error()), "unprocessable") {
					currentLogger.Warn("[Stream] 422 Unprocessable from provider — trimming malformed history", "error", streamErr)
					broker.Send("thinking", "Context error recovered — retrying...")
					var trimmed []openai.ChatCompletionMessage
					for _, m := range req.Messages {
						if m.Role != openai.ChatMessageRoleTool {
							trimmed = append(trimmed, m)
						}
					}
					if len(trimmed) > 7 {
						trimmed = append(trimmed[:1], trimmed[len(trimmed)-6:]...)
					}
					trimmed = append(trimmed, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleSystem,
						Content: "SYSTEM: The previous tool call history was trimmed due to a provider error. Summarise the situation for the user and explain what you were doing and what went wrong.",
					})
					req.Messages = trimmed
					currentLogger.Info("[Stream] Context trimmed after 422, retrying", "new_messages_count", len(req.Messages))
					continue
				}
				return openai.ChatCompletionResponse{}, streamErr
			}

			var assembledResponse strings.Builder
			// Collect streamed tool calls (native function calling via streaming).
			// The API sends partial tool call data across multiple chunks that must
			// be reassembled: each chunk carries an Index identifying the call and
			// incremental Function.Name / Function.Arguments fragments.
			streamToolCalls := map[int]*openai.ToolCall{}
			for {
				chunk, rErr := stm.Recv()
				if rErr != nil {
					if rErr.Error() != "EOF" {
						currentLogger.Error("Stream error", "error", rErr)
					}
					break
				}
				if len(chunk.Choices) > 0 {
					delta := chunk.Choices[0].Delta
					if delta.Content != "" {
						assembledResponse.WriteString(delta.Content)
						// Proxy the JSON chunk to the broker if it supports dynamic passthrough (SSE)
						// We'll marshal it so we can push it cleanly
						if chunkData, mErr := json.Marshal(chunk); mErr == nil {
							broker.SendJSON(fmt.Sprintf("data: %s\n\n", string(chunkData)))
						}
					}
					// Accumulate streamed tool call fragments
					for _, tc := range delta.ToolCalls {
						idx := 0
						if tc.Index != nil {
							idx = *tc.Index
						}
						existing, ok := streamToolCalls[idx]
						if !ok {
							clone := openai.ToolCall{
								Index: tc.Index,
								ID:    tc.ID,
								Type:  tc.Type,
								Function: openai.FunctionCall{
									Name:      tc.Function.Name,
									Arguments: tc.Function.Arguments,
								},
							}
							streamToolCalls[idx] = &clone
						} else {
							if tc.ID != "" {
								existing.ID = tc.ID
							}
							if tc.Function.Name != "" {
								existing.Function.Name += tc.Function.Name
							}
							existing.Function.Arguments += tc.Function.Arguments
						}
					}
				}
			}
			stm.Close()
			content = assembledResponse.String()

			// Build sorted slice of assembled tool calls
			var assembledToolCalls []openai.ToolCall
			if len(streamToolCalls) > 0 {
				assembledToolCalls = make([]openai.ToolCall, 0, len(streamToolCalls))
				for i := 0; i < len(streamToolCalls); i++ {
					if tc, ok := streamToolCalls[i]; ok {
						assembledToolCalls = append(assembledToolCalls, *tc)
					}
				}
				currentLogger.Info("[Stream] Assembled streamed tool calls", "count", len(assembledToolCalls))
			}

			// Estimate streaming tokens
			completionTokens = estimateTokens(content)
			for _, m := range req.Messages {
				promptTokens += estimateTokens(m.Content)
			}
			totalTokens = promptTokens + completionTokens

			// Mock a response object for remaining loop logic
			resp = openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{
					{Message: openai.ChatCompletionMessage{
						Role:      openai.ChatMessageRoleAssistant,
						Content:   content,
						ToolCalls: assembledToolCalls,
					}},
				},
				Usage: openai.Usage{
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TotalTokens:      totalTokens,
				},
			}
		} else {
			resp, err = llm.ExecuteWithRetry(llmCtx, client, req, currentLogger, broker)
			if err != nil {
				cancelResp()
				// 422 Unprocessable Entity: the provider rejected the message sequence —
				// this happens when repeated identical tool responses have grown the history
				// into an invalid state (e.g. tool messages without matching tool_calls).
				// Instead of killing the session, strip role=tool messages, trim history,
				// inject an explanatory system note, and retry.
				if strings.Contains(err.Error(), "422") || strings.Contains(strings.ToLower(err.Error()), "unprocessable") {
					currentLogger.Warn("[Sync] 422 Unprocessable from provider — trimming malformed history", "error", err)
					broker.Send("thinking", "Context error recovered — retrying...")
					// Remove all role=tool messages (they need matching tool_call_ids which may
					// have been invalidated by trimming), keep system + user/assistant only.
					var trimmed []openai.ChatCompletionMessage
					for _, m := range req.Messages {
						if m.Role != openai.ChatMessageRoleTool {
							trimmed = append(trimmed, m)
						}
					}
					// Keep system prompt + last 6 messages to avoid re-triggering 422.
					if len(trimmed) > 7 {
						trimmed = append(trimmed[:1], trimmed[len(trimmed)-6:]...)
					}
					trimmed = append(trimmed, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleSystem,
						Content: "SYSTEM: The previous tool call history was trimmed due to a provider error. Summarise the situation for the user and explain what you were doing and what went wrong.",
					})
					req.Messages = trimmed
					currentLogger.Info("[Sync] Context trimmed after 422, retrying", "new_messages_count", len(req.Messages))
					continue
				}
				return openai.ChatCompletionResponse{}, err
			}
			if len(resp.Choices) == 0 {
				cancelResp()
				return openai.ChatCompletionResponse{}, fmt.Errorf("no choices returned from LLM")
			}
			content = resp.Choices[0].Message.Content
		}

		cancelResp()

		// Empty response recovery: if the LLM returns nothing, trim history and retry once.
		// This typically happens when the total context exceeds the model's window.
		if strings.TrimSpace(content) == "" && len(resp.Choices[0].Message.ToolCalls) == 0 && len(req.Messages) > 4 && !emptyRetried {
			emptyRetried = true
			currentLogger.Warn("[Sync] Empty LLM response detected, trimming history and retrying", "messages_count", len(req.Messages))
			broker.Send("thinking", "Context too large, retrimming...")
			// Keep system prompt (index 0) + optional summary (index 1 if system) + last 4 messages
			var trimmed []openai.ChatCompletionMessage
			trimmed = append(trimmed, req.Messages[0]) // system prompt
			// Keep second message if it's a system summary
			startIdx := 1
			if len(req.Messages) > 1 && req.Messages[1].Role == openai.ChatMessageRoleSystem {
				trimmed = append(trimmed, req.Messages[1])
				startIdx = 2
			}
			// Keep last 4 messages from history
			historyMsgs := req.Messages[startIdx:]
			if len(historyMsgs) > 4 {
				historyMsgs = historyMsgs[len(historyMsgs)-4:]
			}
			trimmed = append(trimmed, historyMsgs...)
			req.Messages = trimmed
			currentLogger.Info("[Sync] Retrying with trimmed context", "new_messages_count", len(req.Messages))
			continue
		}

		// Safety Check: Strip "RECAP" hallucinations if the model is still stuck in the old pattern
		content = strings.TrimPrefix(content, "[RECAP OF PREVIOUS DISCUSSIONS]:")
		content = strings.TrimPrefix(content, "[RECAP OF PREVIOUS DISCUSSIONS]:\n")
		content = strings.TrimPrefix(content, "[CONTEXT_RECAP]:")
		content = strings.TrimPrefix(content, "[CONTEXT_RECAP]:\n")
		content = strings.TrimSpace(content)

		// Conversation log to stdout
		logger.Info("[LLM Response]", "content_len", len(content), "preview", Truncate(content, 200))
		// Activity log to file
		currentLogger.Info("[LLM Response Received]", "content_len", len(content))
		lastActivity = time.Now() // LLM activity

		// Detect tool call: native API-level ToolCalls (use_native_functions=true) or text-based JSON
		var tc ToolCall
		useNativePath := false
		nativeAssistantMsg := resp.Choices[0].Message // snapshot for role=tool continuation

		if len(resp.Choices[0].Message.ToolCalls) > 0 {
			nativeCall := resp.Choices[0].Message.ToolCalls[0]
			// Primary native path: parse directly from API-level ToolCall object
			// We now take this path if UseNativeFunctions is true OR if the model sent them anyway
			tc = NativeToolCallToToolCall(nativeCall, currentLogger)
			useNativePath = true
			currentLogger.Info("[Sync] Native tool call detected", "function", tc.Action, "id", nativeCall.ID, "forced", !cfg.LLM.UseNativeFunctions)

			// Queue additional native tool calls for batch execution.
			// The OpenAI API requires a role=tool response for each tool_call in the
			// assistant message, so these are processed inline (not in the regular pendingTCs loop).
			if len(resp.Choices[0].Message.ToolCalls) > 1 {
				for _, extra := range resp.Choices[0].Message.ToolCalls[1:] {
					extraTC := NativeToolCallToToolCall(extra, currentLogger)
					pendingTCs = append(pendingTCs, extraTC)
				}
				currentLogger.Info("[MultiTool] Queued additional native tool calls from response", "count", len(resp.Choices[0].Message.ToolCalls)-1)
			}
		}

		// Text-based fallback: parse JSON from content string if native path not taken
		if !useNativePath {
			tc = ParseToolCall(content)
			// If the response contains multiple tool calls (e.g. two manage_memory adds),
			// queue the extras so they execute in subsequent iterations without a new LLM call.
			if tc.IsTool {
				extras := extractExtraToolCalls(content, tc.RawJSON)
				if len(extras) > 0 {
					currentLogger.Info("[MultiTool] Queued additional tool calls from response", "count", len(extras))
					pendingTCs = append(pendingTCs, extras...)
				}
			}
		}

		// Obsolete: we now send it later when histContent is fully assembled.
		if !stream {
			promptTokens = resp.Usage.PromptTokens
			completionTokens = resp.Usage.CompletionTokens
			totalTokens = resp.Usage.TotalTokens
		}

		if totalTokens == 0 {
			// Estimate tokens if usage is missing
			muTokens.Lock()
			GlobalTokenEstimated = true
			muTokens.Unlock()

			// Estimate prompt tokens from all messages in request
			for _, m := range req.Messages {
				promptTokens += estimateTokens(m.Content)
			}
			// Estimate completion tokens from response content
			completionTokens = estimateTokens(content)
			totalTokens = promptTokens + completionTokens
		}

		sessionTokens += totalTokens
		muTokens.Lock()
		GlobalTokenCount += totalTokens
		localGlobalTotal := GlobalTokenCount
		localIsEstimated := GlobalTokenEstimated
		muTokens.Unlock()

		broker.SendJSON(fmt.Sprintf(`{"event":"tokens","prompt":%d,"completion":%d,"total":%d,"session_total":%d,"global_total":%d,"is_estimated":%t}`,
			promptTokens, completionTokens, totalTokens, sessionTokens, localGlobalTotal, localIsEstimated))

		// Budget tracking: record cost and send status to UI
		if budgetTracker != nil {
			actualModel := resp.Model
			if actualModel == "" {
				actualModel = req.Model
			}
			crossedWarning := budgetTracker.Record(actualModel, promptTokens, completionTokens)
			budgetJSON := budgetTracker.GetStatusJSON()
			if budgetJSON != "" {
				broker.SendJSON(budgetJSON)
			}
			if crossedWarning {
				bs := budgetTracker.GetStatus()
				warnMsg := fmt.Sprintf("\u26a0\ufe0f Budget warning: %.0f%% used ($%.4f / $%.2f)", bs.Percentage*100, bs.SpentUSD, bs.DailyLimit)
				broker.Send("budget_warning", warnMsg)
			}
			if budgetTracker.IsExceeded() {
				bs := budgetTracker.GetStatus()
				exMsg := fmt.Sprintf("\u26d4 Budget exceeded! $%.4f / $%.2f (enforcement: %s)", bs.SpentUSD, bs.DailyLimit, bs.Enforcement)
				broker.Send("budget_blocked", exMsg)
			}
		}

		currentLogger.Debug("[Sync] Tool detection", "is_tool", tc.IsTool, "action", tc.Action, "raw_code", tc.RawCodeDetected)

		// Clear explicit tools after they've been consumed (they were injected this iteration)
		if len(explicitTools) > 0 {
			explicitTools = explicitTools[:0]
		}

		// Detect <workflow_plan>["tool1","tool2"]</workflow_plan> in the response
		if workflowPlanCount < 3 {
			if parsed, stripped := parseWorkflowPlan(content); len(parsed) > 0 {
				workflowPlanCount++
				explicitTools = parsed
				currentLogger.Info("[Sync] Workflow plan detected, loading tool guides", "tools", parsed, "attempt", workflowPlanCount)
				broker.Send("workflow_plan", strings.Join(parsed, ", "))

				// Store the stripped content as assistant message
				strippedContent := strings.TrimSpace(stripped)
				if strippedContent != "" {
					id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, strippedContent, false, false)
					if err != nil {
						currentLogger.Error("Failed to persist workflow plan message", "error", err)
					}
					if sessionID == "default" {
						historyManager.Add(openai.ChatMessageRoleAssistant, strippedContent, id, false, false)
					}
					req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: strippedContent})
				}

				// Inject a system nudge so the agent knows the guides are available
				nudge := fmt.Sprintf("Tool manuals loaded for: %s. Proceed with your plan.", strings.Join(parsed, ", "))
				req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: nudge})
				continue
			}
		}

		if tc.RawCodeDetected && rawCodeCount < 2 {
			rawCodeCount++
			currentLogger.Warn("[Sync] Raw code detected, sending corrective feedback", "attempt", rawCodeCount)
			broker.Send("error_recovery", "Raw code detected, requesting JSON format...")

			id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, content, false, true)
			if err != nil {
				currentLogger.Error("Failed to persist assistant message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleAssistant, content, id, false, true)
			}

			feedbackMsg := "ERROR: You sent raw Python code instead of a JSON tool call. My supervisor only understands JSON tool calls. Please wrap your code in a valid JSON object: {\"action\": \"save_tool\", \"name\": \"script.py\", \"description\": \"...\", \"code\": \"<your python code with \\n escaped>\"}."
			id, err = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, feedbackMsg, false, false)
			if err != nil {
				currentLogger.Error("Failed to persist feedback message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleUser, feedbackMsg, id, false, false)
			}

			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: feedbackMsg})
			continue
		}

		// Recovery: model sent an announcement/preamble instead of a tool call
		// Triggered when: no tool, short response, contains action-intent phrases
		announcementPhrases := []string{
			"lass mich", "ich starte", "ich werde", "ich führe", "ich teste",
			"let me", "i will", "i'll", "i am going to", "i'm going to",
			"let's start", "starting", "launching", "i'll start", "i'll run",
			"alles klar", "okay, let", "sure, let", "sure, i",
			"ich suche nach", "ich schaue nach", "ich prüfe", "ich überprüfe",
			"ich sehe mir", "lass mich sehen", "ich werde nachschauen",
			"i'll check", "let me check", "checking", "searching", "looking",
			"i am looking", "i will look", "i'll search", "i will search",
			"ich frage ab", "ich lade", "i'll load", "i am loading",
		}
		isAnnouncement := func() bool {
			if tc.IsTool || useNativePath || tc.RawCodeDetected || len(content) > 1000 {
				return false
			}
			// A response ending with '?' is a conversational reply, not an action announcement
			if strings.HasSuffix(strings.TrimRight(strings.TrimSpace(content), "\"'"), "?") {
				return false
			}
			// If the LLM just completed a tool call, a text response is a completion confirmation, not an announcement
			if lastResponseWasTool {
				return false
			}
			lc := strings.ToLower(content)
			for _, phrase := range announcementPhrases {
				if strings.Contains(lc, phrase) {
					return true
				}
			}
			return false
		}()
		if isAnnouncement && announcementCount < 2 {
			announcementCount++
			currentLogger.Warn("[Sync] Announcement-only response detected, requesting immediate tool call", "attempt", announcementCount, "content_preview", Truncate(content, 120))
			broker.Send("error_recovery", "Announcement without action detected, requesting tool call...")

			id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, content, false, true)
			if err != nil {
				currentLogger.Error("Failed to persist assistant message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleAssistant, content, id, false, true)
			}

			feedbackMsg := "ERROR: You announced what you were going to do but did not output a tool call. When executing a task, your ENTIRE response must be ONLY the raw JSON tool call — no explanation before it. Output the JSON tool call NOW."
			id, err = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, feedbackMsg, false, false)
			if err != nil {
				currentLogger.Error("Failed to persist feedback message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleUser, feedbackMsg, id, false, false)
			}

			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: feedbackMsg})
			continue
		}

		// Recovery: model wrapped tool call in markdown fence instead of bare JSON
		if !tc.IsTool && !tc.RawCodeDetected && missedToolCount < 2 &&
			(strings.Contains(content, "```") || strings.Contains(content, "{")) &&
			(strings.Contains(content, `"action"`) || strings.Contains(content, `'action'`)) {
			missedToolCount++
			currentLogger.Warn("[Sync] Missed tool call in fence, sending corrective feedback", "attempt", missedToolCount, "content_preview", Truncate(content, 150))
			broker.Send("error_recovery", "Tool call wrapped in fence, requesting raw JSON...")

			id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, content, false, true)
			if err != nil {
				currentLogger.Error("Failed to persist assistant message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleAssistant, content, id, false, true)
			}

			feedbackMsg := "ERROR: Your response contained explanation text and/or markdown fences (```json). Tool calls MUST be a raw JSON object ONLY - no explanation before or after, no markdown, no fences. Output ONLY the JSON object, starting with { and ending with }. Example: {\"action\": \"co_agent\", \"operation\": \"spawn\", \"task\": \"...\"}"
			id, err = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, feedbackMsg, false, false)
			if err != nil {
				currentLogger.Error("Failed to persist feedback message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleUser, feedbackMsg, id, false, false)
			}

			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: feedbackMsg})
			continue
		}

		// Berechne effektives Limit neu mit bekanntem tc (für Tool-spezifische Anpassungen)
		effectiveMaxCallsWithTool := calculateEffectiveMaxCalls(cfg, tc, personalityEnabled, shortTermMem, currentLogger)

		if tc.IsTool && toolCallCount < effectiveMaxCallsWithTool {
			toolCallCount++
			broker.Send("thinking", fmt.Sprintf("[%d] Running %s...", toolCallCount, tc.Action))

			// Persist tool call to history: native path synthesizes a text representation
			histContent := content

			// Decide if this message should be hidden from the UI history endpoint.
			// Hide it if it's purely a synthetic JSON string (e.g. no text, only tool call),
			// but show it if the LLM provided conversational text.
			isMsgInternal := true
			if strings.TrimSpace(content) != "" && !strings.HasPrefix(strings.TrimSpace(content), "{") {
				isMsgInternal = false
			}

			if useNativePath && histContent == "" && len(nativeAssistantMsg.ToolCalls) > 0 {
				nc := nativeAssistantMsg.ToolCalls[0]
				histContent = fmt.Sprintf("{\"action\": \"%s\"}", nc.Function.Name)
				if nc.Function.Arguments != "" && len(nc.Function.Arguments) > 2 {
					args := strings.TrimSpace(nc.Function.Arguments)
					if strings.HasPrefix(args, "{") && strings.HasSuffix(args, "}") {
						inner := args[1 : len(args)-1]
						if inner != "" {
							histContent = fmt.Sprintf("{\"action\": \"%s\", %s}", nc.Function.Name, inner)
						}
					}
				}
			}
			id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, histContent, false, isMsgInternal)
			if err != nil {
				currentLogger.Error("Failed to persist tool-call message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleAssistant, histContent, id, false, isMsgInternal)
			}

			// For SSE: send only the preamble text (without the raw JSON) to prevent
			// nested-JSON regex failures that leave stray `}` characters in the chat UI.
			sseToolContent := histContent
			if !useNativePath && tc.RawJSON != "" {
				sseToolContent = strings.TrimSpace(strings.Replace(sseToolContent, tc.RawJSON, "", 1))
			}
			broker.Send("tool_call", sseToolContent)
			broker.Send("tool_start", tc.Action)

			if tc.Action == "execute_python" {
				flags.RequiresCoding = true
				broker.Send("coding", "Executing Python script...")
			}

			// Co-agent spawn: send a dedicated status event with a task preview
			if (tc.Action == "co_agent" || tc.Action == "co_agents") &&
				(tc.Operation == "spawn" || tc.Operation == "start" || tc.Operation == "create") {
				taskPreview := tc.Task
				if taskPreview == "" {
					taskPreview = tc.Content
				}
				if len(taskPreview) > 80 {
					taskPreview = taskPreview[:80] + "…"
				}
				broker.Send("co_agent_spawn", taskPreview)
			}

			resultContent := DispatchToolCall(ctx, tc, cfg, currentLogger, client, vault, registry, manifest, cronManager, missionManager, longTermMem, shortTermMem, kg, inventoryDB, invasionDB, historyManager, tools.IsBusy(), surgeryPlan, guardian, sessionID, coAgentRegistry, budgetTracker)
			broker.Send("tool_output", resultContent)

			// Emit SSE image event so the Web UI shows the image immediately (before LLM responds)
			if tc.Action == "send_image" {
				var imgRes struct {
					Status  string `json:"status"`
					WebPath string `json:"web_path"`
					Caption string `json:"caption"`
				}
				raw := strings.TrimPrefix(resultContent, "[Tool Output]\n")
				raw = strings.TrimPrefix(raw, "Tool Output: ")
				if json.Unmarshal([]byte(raw), &imgRes) == nil && imgRes.Status == "success" {
					evtPayload, _ := json.Marshal(map[string]string{
						"path":    imgRes.WebPath,
						"caption": imgRes.Caption,
					})
					broker.Send("image", string(evtPayload))
				}
			}

			broker.Send("tool_end", tc.Action)
			lastActivity = time.Now() // Tool activity

			// Invalidate core memory cache when it was modified
			if tc.Action == "manage_memory" {
				coreMemDirty = true
			}

			// Record transition
			if lastTool != "" {
				_ = shortTermMem.RecordToolTransition(lastTool, tc.Action)
			}
			lastTool = tc.Action
			// Track recent tools for lazy schema injection (keep last 5, dedup)
			found := false
			for _, rt := range recentTools {
				if rt == tc.Action {
					found = true
					break
				}
			}
			if !found {
				recentTools = append(recentTools, tc.Action)
				if len(recentTools) > 5 {
					recentTools = recentTools[len(recentTools)-5:]
				}
			}

			// Proactive Workflow Feedback (Phase: Keep the user engaged during long chains)
			if cfg.Agent.WorkflowFeedback && !flags.IsCoAgent && sessionID == "default" {
				stepsSinceLastFeedback++
				if stepsSinceLastFeedback >= 4 {
					stepsSinceLastFeedback = 0
					feedbackPhrases := []string{
						"Ich brauche noch einen Moment, bin aber dran...",
						"Die Analyse läuft noch, einen Augenblick bitte...",
						"Ich suche noch nach weiteren Informationen...",
						"Bin gleich fertig mit der Bearbeitung...",
						"Das dauert einen Moment länger als erwartet, bleib dran...",
						"Ich verarbeite die Daten noch...",
					}
					// Simple pseudo-random selection based on time
					phrase := feedbackPhrases[time.Now().Unix()%int64(len(feedbackPhrases))]
					broker.Send("progress", phrase)
				}
			}

			// Phase D: Mood detection after each tool call
			if personalityEnabled && shortTermMem != nil {
				triggerInfo := moodTrigger()
				if strings.Contains(resultContent, "ERROR") || strings.Contains(resultContent, "error") {
					triggerInfo = moodTrigger() + " [tool error]"
				}

				if cfg.Agent.PersonalityEngineV2 {
					// ── V2: Asynchronous LLM-Based Mood Analysis ──
					// Extract recent context (e.g. last 5 messages) for the analyzer
					recentMsgs := req.Messages
					if len(recentMsgs) > 5 {
						recentMsgs = recentMsgs[len(recentMsgs)-5:]
					}
					var historyBuilder strings.Builder
					var userHistoryBuilder strings.Builder
					for _, m := range recentMsgs {
						// Skip system messages — they contain the full agent prompt (tool guides, identity,
						// rules, etc.) and must not be fed to the mood/profile analyzer. Including them
						// causes the LLM to attribute every mentioned technology to the user's profile
						// even when the user never mentioned it.
						if m.Role == openai.ChatMessageRoleSystem {
							continue
						}
						historyBuilder.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
						// Build a user-only history to avoid attributing agent/tool content to the user profile
						if m.Role == openai.ChatMessageRoleUser {
							userHistoryBuilder.WriteString(fmt.Sprintf("user: %s\n", m.Content))
						}
					}
					historyBuilder.WriteString(fmt.Sprintf("Tool Result: %s\n", resultContent))
					// Note: Tool Results are intentionally excluded from userHistory

					var v2Client memory.PersonalityAnalyzerClient = client
					if cfg.Agent.PersonalityV2URL != "" {
						key := cfg.Agent.PersonalityV2APIKey
						if key == "" {
							key = "dummy" // Ollama sometimes requires a non-empty string
						}
						v2Cfg := openai.DefaultConfig(key)
						v2Cfg.BaseURL = cfg.Agent.PersonalityV2URL
						v2Client = openai.NewClientWithConfig(v2Cfg)
					}

					go func(contextHistory string, userHistory string, tInfo string, modelName string, analyzerClient memory.PersonalityAnalyzerClient, m memory.PersonalityMeta, profilingEnabled bool) {
						v2Ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Agent.PersonalityV2TimeoutSecs)*time.Second)
						defer cancel()

						mood, affDelta, traitDeltas, profileUpdates, err := shortTermMem.AnalyzeMoodV2(v2Ctx, analyzerClient, modelName, contextHistory, userHistory, m, profilingEnabled)
						if err != nil {
							currentLogger.Warn("[Personality V2] Failed to analyze mood", "error", err)
							return
						}

						_ = shortTermMem.LogMood(mood, tInfo)
						for trait, delta := range traitDeltas {
							_ = shortTermMem.UpdateTrait(trait, delta)
						}
						_ = shortTermMem.UpdateTrait(memory.TraitAffinity, affDelta)

						// User Profiling: persist observed profile attributes
						if profilingEnabled && len(profileUpdates) > 0 {
							validCategories := map[string]bool{"tech": true, "prefs": true, "interests": true, "context": true, "comm": true}
							count := 0
							for _, pu := range profileUpdates {
								if count >= 2 {
									break // Hard limit (matches prompt)
								}
								if validCategories[pu.Category] && pu.Key != "" && pu.Value != "" {
									if err := shortTermMem.UpsertProfileEntry(pu.Category, pu.Key, pu.Value, "v2"); err != nil {
										currentLogger.Warn("[User Profiling] Failed to upsert profile entry", "key", pu.Key, "error", err)
									}
									count++
								}
							}
							_ = shortTermMem.EnforceProfileSizeLimit(50)
							if del, down, err := shortTermMem.PruneStaleProfileEntries(); err == nil && (del > 0 || down > 0) {
								currentLogger.Debug("[User Profiling] Pruned stale entries", "deleted", del, "downgraded", down)
							}
							currentLogger.Debug("[User Profiling] Profile updates applied", "count", count)
						}

						currentLogger.Debug("[Personality V2] Asynchronous mood analysis complete", "mood", mood, "affinity_delta", affDelta)
					}(historyBuilder.String(), userHistoryBuilder.String(), triggerInfo, cfg.Agent.PersonalityV2Model, v2Client, meta, cfg.Agent.UserProfiling)

				} else {
					// ── V1: Synchronous Heuristic-Based Mood Analysis ──
					mood, traitDeltas := memory.DetectMood(lastUserMsg, resultContent, meta)
					_ = shortTermMem.LogMood(mood, triggerInfo)
					for trait, delta := range traitDeltas {
						_ = shortTermMem.UpdateTrait(trait, delta)
					}
				}
				flags.PersonalityLine = shortTermMem.GetPersonalityLine(cfg.Agent.PersonalityEngineV2)
			}

			if tc.NotifyOnCompletion {
				resultContent = fmt.Sprintf(
					"[TOOL COMPLETION NOTIFICATION]\nAction: %s\nStatus: Completed\nTimestamp: %s\nOutput:\n%s",
					tc.Action,
					time.Now().Format(time.RFC3339),
					resultContent,
				)
			}
			// Make sure errors from execute_python trigger recovery mode
			if tc.Action == "execute_python" {
				if strings.Contains(resultContent, "[EXECUTION ERROR]") || strings.Contains(resultContent, "TIMEOUT") {
					flags.IsErrorState = true
					broker.Send("error_recovery", "Script error detected, retrying...")
				} else {
					flags.IsErrorState = false
				}
			}
			id, err = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleSystem, resultContent, false, true)
			if err != nil {
				currentLogger.Error("Failed to persist tool-result message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleSystem, resultContent, id, false, true)
			}

			// Phase 72: Broadcast the supervisor's result to the UI (shown only in debug mode)
			broker.Send("tool_output", resultContent)

			// Phase 1: Lifecycle Handover check
			if strings.Contains(resultContent, "Maintenance Mode activated") {
				currentLogger.Info("Handover sentinel detected, Sidecar taking over...")
				// We return the response so the user sees the handover message,
				// and the loop terminates. The process stays alive in "busy" mode
				// until the sidecar triggers a reload.
				id, err := shortTermMem.InsertMessage(sessionID, resp.Choices[0].Message.Role, content, false, false)
				if err != nil {
					currentLogger.Error("Failed to persist handover message to SQLite", "error", err)
				}
				if sessionID == "default" {
					historyManager.Add(resp.Choices[0].Message.Role, content, id, false, false)
				}
				return resp, nil
			}

			if useNativePath {
				// Native path: use proper role=tool format so the LLM gets structured multi-turn context
				req.Messages = append(req.Messages, nativeAssistantMsg)
				req.Messages = append(req.Messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    resultContent,
					ToolCallID: tc.NativeCallID,
				})

				// Execute batched native tool calls inline.
				// The OpenAI API requires ALL role=tool responses to be present before
				// the next API call when the assistant message contains multiple tool_calls.
				for len(pendingTCs) > 0 && pendingTCs[0].NativeCallID != "" {
					btc := pendingTCs[0]
					pendingTCs = pendingTCs[1:]
					toolCallCount++
					broker.Send("thinking", fmt.Sprintf("[%d] Running %s (batched)...", toolCallCount, btc.Action))
					broker.Send("tool_start", btc.Action)

					bResult := DispatchToolCall(ctx, btc, cfg, currentLogger, client, vault, registry, manifest, cronManager, missionManager, longTermMem, shortTermMem, kg, inventoryDB, invasionDB, historyManager, tools.IsBusy(), surgeryPlan, guardian, sessionID, coAgentRegistry, budgetTracker)
					broker.Send("tool_output", bResult)
					broker.Send("tool_end", btc.Action)
					lastActivity = time.Now()

					if btc.Action == "manage_memory" || btc.Action == "core_memory" {
						coreMemDirty = true
					}

					// Persist batched call to history
					bHistContent := fmt.Sprintf(`{"action": "%s"}`, btc.Action)
					bID, bErr := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, bHistContent, false, true)
					if bErr != nil {
						currentLogger.Error("Failed to persist batched tool-call message", "error", bErr)
					}
					if sessionID == "default" {
						historyManager.Add(openai.ChatMessageRoleAssistant, bHistContent, bID, false, true)
					}
					bID, bErr = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleSystem, bResult, false, true)
					if bErr != nil {
						currentLogger.Error("Failed to persist batched tool-result message", "error", bErr)
					}
					if sessionID == "default" {
						historyManager.Add(openai.ChatMessageRoleSystem, bResult, bID, false, true)
					}

					req.Messages = append(req.Messages, openai.ChatCompletionMessage{
						Role:       openai.ChatMessageRoleTool,
						Content:    bResult,
						ToolCallID: btc.NativeCallID,
					})
				}
			} else {
				req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
				req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: resultContent})
			}

			// Support early exit for Lifeboat
			if strings.Contains(resultContent, "[LIFEBOAT_EXIT_SIGNAL]") {
				currentLogger.Info("[Sync] Early exit signal received, stopping loop.")
				return resp, nil
			}

			// Consecutive identical error circuit breaker:
			// If the agent keeps retrying the exact same failing tool call, stop it before
			// it exhausts MaxToolCalls and wastes the entire budget on pointless retries.
			// Also catches sandbox/shell failures reported as a non-zero exit_code.
			hasSandboxFailure := strings.Contains(resultContent, `"exit_code":`) &&
				!strings.Contains(resultContent, `"exit_code": 0`) &&
				!strings.Contains(resultContent, `"exit_code":0`)
			isToolError := strings.Contains(resultContent, `"status": "error"`) ||
				strings.Contains(resultContent, `"status":"error"`) ||
				strings.Contains(resultContent, `[EXECUTION ERROR]`) ||
				hasSandboxFailure
			if isToolError {
				if resultContent == lastToolError {
					consecutiveErrorCount++
					if consecutiveErrorCount >= 3 {
						currentLogger.Warn("[Sync] Consecutive identical error — circuit breaker triggered",
							"action", tc.Action, "count", consecutiveErrorCount)
						abortMsg := fmt.Sprintf(
							"CIRCUIT BREAKER: The tool '%s' returned the same error %d times in a row. "+
								"You MUST stop retrying it — calling it again will produce the exact same result. "+
								"Do NOT call '%s' again this session. "+
								"Instead: inform the user about the error, explain what likely needs to be fixed "+
								"(e.g. wrong URL, missing credentials, service unavailable), and wait for their input.",
							tc.Action, consecutiveErrorCount, tc.Action)
						req.Messages = append(req.Messages,
							openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: abortMsg})
						consecutiveErrorCount = 0
						lastToolError = ""
					}
				} else {
					consecutiveErrorCount = 1
				}
				lastToolError = resultContent
			} else {
				consecutiveErrorCount = 0
				lastToolError = ""
			}

			// 429 Mitigation: Add a delay between turns to respect rate limits (controlled by config)
			select {
			case <-time.After(time.Duration(cfg.Agent.StepDelaySeconds) * time.Second):
				// Continue to next turn
			case <-ctx.Done():
				return resp, ctx.Err()
			}
			lastResponseWasTool = true
			continue
		}

		// Final answer
		if content == "" {
			content = "[Empty Response]"
		}
		currentLogger.Debug("[Sync] Final answer", "content_len", len(content), "content_preview", Truncate(content, 200))
		broker.Send("done", "Response complete.")

		// Don't persist [Empty Response] as a real message — it pollutes future context
		isEmpty := content == "[Empty Response]"
		if !isEmpty {
			id, err := shortTermMem.InsertMessage(sessionID, resp.Choices[0].Message.Role, content, false, false)
			if err != nil {
				currentLogger.Error("Failed to persist final-answer message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(resp.Choices[0].Message.Role, content, id, false, false)
			}
		} else {
			currentLogger.Warn("[Sync] Skipping history persistence for empty response")
		}

		// Phase D: Final mood + trait update + milestone check at session end
		if personalityEnabled && shortTermMem != nil {
			mood, traitDeltas := memory.DetectMood(lastUserMsg, "", meta)
			_ = shortTermMem.LogMood(mood, moodTrigger())
			for trait, delta := range traitDeltas {
				_ = shortTermMem.UpdateTrait(trait, delta)
			}
			// Milestone check
			traits, tErr := shortTermMem.GetTraits()
			if tErr == nil {
				for _, m := range memory.CheckMilestones(traits) {
					has, err := shortTermMem.HasMilestone(m.Label)
					if err != nil {
						continue // skip on DB error
					}
					if !has {
						trigger := shortTermMem.GetLastMoodTrigger()
						details := fmt.Sprintf("%s %s %.2f", m.Trait, m.Direction, m.Threshold)
						if trigger != "" {
							details = fmt.Sprintf("%s (Trigger: %q)", details, trigger)
						}
						_ = shortTermMem.AddMilestone(m.Label, details)
					}
				}
			}
		}

		return resp, nil
	}
}

// splitCSV splits a comma-separated value string into a trimmed, non-empty slice.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func dispatchInner(ctx context.Context, tc ToolCall, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, missionManager *tools.MissionManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB, historyMgr *memory.HistoryManager, isMaintenance bool, surgeryPlan string, guardian *security.Guardian, sessionID string, coAgentRegistry *CoAgentRegistry, budgetTracker *budget.Tracker) string {
	// Co-Agent blacklist: co-agents (identified by sessionID prefix) cannot modify memory, notes, KG, or spawn sub-agents
	isCoAgent := strings.HasPrefix(sessionID, "coagent-")
	if isCoAgent {
		switch tc.Action {
		case "manage_memory":
			if tc.Operation != "read" && tc.Operation != "query" && tc.Operation != "" {
				return `Tool Output: {"status": "error", "message": "Co-Agents cannot modify memory. Only read/query operations are allowed."}`
			}
		case "knowledge_graph":
			if tc.Operation != "query" && tc.Operation != "search" && tc.Operation != "get" && tc.Operation != "" {
				return `Tool Output: {"status": "error", "message": "Co-Agents cannot modify the knowledge graph. Only read operations are allowed."}`
			}
		case "manage_notes":
			if tc.Operation != "list" {
				return `Tool Output: {"status": "error", "message": "Co-Agents cannot modify notes. Only 'list' is allowed."}`
			}
		case "co_agent", "co_agents":
			return `Tool Output: {"status": "error", "message": "Co-Agents cannot spawn sub-agents."}`
		case "follow_up":
			return `Tool Output: {"status": "error", "message": "Co-Agents cannot schedule follow-ups."}`
		case "cron_scheduler":
			return `Tool Output: {"status": "error", "message": "Co-Agents cannot manage cron jobs."}`
		}
	}

	switch tc.Action {
	case "execute_sandbox":
		if !cfg.Sandbox.Enabled {
			return "Tool Output: [PERMISSION DENIED] execute_sandbox is disabled (sandbox.enabled: false)."
		}
		if tc.Code == "" {
			return "Tool Output: [EXECUTION ERROR] 'code' field is empty. Provide the source code to execute."
		}
		lang := tc.SandboxLang
		if lang == "" {
			lang = tc.Language
		}
		if lang == "" {
			lang = "python"
		}
		logger.Info("LLM requested sandbox execution", "language", lang, "code_len", len(tc.Code), "libraries", len(tc.Libraries))
		result, err := tools.SandboxExecuteCode(tc.Code, lang, tc.Libraries, cfg.Sandbox.TimeoutSeconds, logger)
		if err != nil {
			// Fall back to execute_python if sandbox not ready and Python is allowed
			if cfg.Agent.AllowPython && lang == "python" {
				logger.Warn("Sandbox execution failed, falling back to execute_python", "error", err)
				stdout, stderr, pyErr := tools.ExecutePython(tc.Code, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir)
				var sb strings.Builder
				sb.WriteString("Tool Output (sandbox unavailable — ran via local Python):\n")
				if stdout != "" {
					sb.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdout))
				}
				if stderr != "" {
					sb.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderr))
				}
				if pyErr != nil {
					sb.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %v\n", pyErr))
				}
				return sb.String()
			}
			return fmt.Sprintf("Tool Output: [EXECUTION ERROR] sandbox: %v", err)
		}
		return "Tool Output:\n" + result

	case "execute_python":
		if !cfg.Agent.AllowPython {
			return "Tool Output: [PERMISSION DENIED] execute_python is disabled in Danger Zone settings (agent.allow_python: false)."
		}
		logger.Info("LLM requested python execution", "code_len", len(tc.Code), "background", tc.Background)
		if tc.Code == "" {
			return "Tool Output: [EXECUTION ERROR] 'code' field is empty. You MUST provide Python source code in the 'code' field. Do NOT use execute_python for SSH or remote tasks — use query_inventory / execute_remote_shell instead."
		}
		if tc.Background {
			logger.Info("LLM requested background Python execution", "code_len", len(tc.Code))
			pid, err := tools.ExecutePythonBackground(tc.Code, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir, registry)
			if err != nil {
				return fmt.Sprintf("Tool Output: [EXECUTION ERROR] starting background process: %v", err)
			}
			return fmt.Sprintf("Tool Output: Process started in background. PID=%d. Use {\"action\": \"read_process_logs\", \"pid\": %d} to check output.", pid, pid)
		}
		logger.Debug("Executing Python (foreground)", "code_preview", Truncate(tc.Code, 300))
		logger.Info("LLM requested python execution", "code_len", len(tc.Code))
		stdout, stderr, err := tools.ExecutePython(tc.Code, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir)

		var sb strings.Builder
		sb.WriteString("Tool Output:\n")
		if stdout != "" {
			sb.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdout))
		}
		if stderr != "" {
			sb.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderr))
		}
		if err != nil {
			sb.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %v\n", err))
		}
		return sb.String()

	case "execute_shell":
		if !cfg.Agent.AllowShell {
			return "Tool Output: [PERMISSION DENIED] execute_shell is disabled in Danger Zone settings (agent.allow_shell: false)."
		}
		logger.Info("LLM requested shell execution", "command", tc.Command, "background", tc.Background)
		if tc.Background {
			pid, err := tools.ExecuteShellBackground(tc.Command, cfg.Directories.WorkspaceDir, registry)
			if err != nil {
				return fmt.Sprintf("Tool Output: [EXECUTION ERROR] starting background shell process: %v", err)
			}
			return fmt.Sprintf("Tool Output: Shell process started in background. PID=%d. Use {\"action\": \"read_process_logs\", \"pid\": %d} to check output.", pid, pid)
		}
		stdout, stderr, err := tools.ExecuteShell(tc.Command, cfg.Directories.WorkspaceDir)

		var sb strings.Builder
		sb.WriteString("Tool Output:\n")
		if stdout != "" {
			sb.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdout))
		}
		if stderr != "" {
			sb.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderr))
		}
		if err != nil {
			sb.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %v\n", err))
		}
		return sb.String()

	case "execute_sudo":
		if !cfg.Agent.SudoEnabled {
			return "Tool Output: [PERMISSION DENIED] execute_sudo is not enabled in config. Set agent.sudo_enabled: true and store the sudo password in the vault as 'sudo_password'."
		}
		if tc.Command == "" {
			return "Tool Output: [EXECUTION ERROR] 'command' is required for execute_sudo"
		}
		sudoPass, vaultErr := vault.ReadSecret("sudo_password")
		if vaultErr != nil || sudoPass == "" {
			return "Tool Output: [PERMISSION DENIED] sudo password not found in vault. Store it first: {\"action\": \"secrets_vault\", \"operation\": \"store\", \"key\": \"sudo_password\", \"value\": \"<password>\"}"
		}
		logger.Info("LLM requested sudo execution", "command", tc.Command)
		stdoutS, stderrS, errS := tools.ExecuteSudo(tc.Command, cfg.Directories.WorkspaceDir, sudoPass)

		var sbSudo strings.Builder
		sbSudo.WriteString("Tool Output:\n")
		if stdoutS != "" {
			sbSudo.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdoutS))
		}
		if stderrS != "" {
			sbSudo.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderrS))
		}
		if errS != nil {
			sbSudo.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %v\n", errS))
		}
		return sbSudo.String()

	case "install_package":
		if !cfg.Agent.AllowShell {
			return "Tool Output: [PERMISSION DENIED] install_package is disabled in Danger Zone settings (agent.allow_shell: false)."
		}
		logger.Info("LLM requested package installation", "package", tc.Package)
		if tc.Package == "" {
			return "Tool Output: [EXECUTION ERROR] 'package' is required for install_package"
		}
		stdout, stderr, err := tools.InstallPackage(tc.Package, cfg.Directories.WorkspaceDir)

		var sb strings.Builder
		sb.WriteString("Tool Output:\n")
		if stdout != "" {
			sb.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdout))
		}
		if stderr != "" {
			sb.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderr))
		}
		if err != nil {
			sb.WriteString(fmt.Sprintf("[EXECUTION ERROR]: %v\n", err))
		}
		return sb.String()

	case "save_tool":
		if !cfg.Agent.AllowPython {
			return "Tool Output: [PERMISSION DENIED] save_tool is disabled in Danger Zone settings (agent.allow_python: false)."
		}
		logger.Info("LLM requested tool persistence", "name", tc.Name)
		if tc.Name == "" || tc.Code == "" {
			return "Tool Output: ERROR 'name' and 'code' are required for save_tool"
		}
		if err := manifest.SaveTool(cfg.Directories.ToolsDir, tc.Name, tc.Description, tc.Code); err != nil {
			return fmt.Sprintf("Tool Output: ERROR saving tool: %v", err)
		}
		return fmt.Sprintf("Tool Output: Tool '%s' saved and registered successfully.", tc.Name)

	case "list_tools":
		logger.Info("LLM requested to list tools")
		loaded, err := manifest.Load()
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR loading tool manifest: %v", err)
		}
		var sb strings.Builder
		if len(loaded) == 0 {
			sb.WriteString("Tool Output: No custom Python tools saved yet. Use 'save_tool' to create them.\n")
		} else {
			sb.WriteString("Tool Output: Saved Reusable Tools (Python):\n")
			for k, v := range loaded {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
			}
		}

		sb.WriteString("\n[NOTE] Core capabilities like 'filesystem', 'execute_python', 'core_memory', 'query_memory', 'execute_surgery' (Maintenance only) are built-in and always available. See your system prompt and 'get_tool_manual' for details.")
		return sb.String()

	case "run_tool":
		if !cfg.Agent.AllowPython {
			return "Tool Output: [PERMISSION DENIED] run_tool is disabled in Danger Zone settings (agent.allow_python: false)."
		}
		// Intercept LLM confusing Skills for Tools
		toolPath := filepath.Join(cfg.Directories.ToolsDir, tc.Name)
		if _, err := os.Stat(toolPath); os.IsNotExist(err) {
			skillCheckName := tc.Name
			if !strings.HasSuffix(skillCheckName, ".py") {
				skillCheckName += ".py"
			}
			skillPath := filepath.Join(cfg.Directories.SkillsDir, skillCheckName)
			if _, err2 := os.Stat(skillPath); err2 == nil {
				skillBase := strings.TrimSuffix(skillCheckName, ".py")
				return fmt.Sprintf("Tool Output: ERROR '%s' is a registered SKILL, not a generic tool. You MUST use {\"action\": \"execute_skill\", \"skill\": \"%s\", \"skill_args\": {\"arg1\": \"val1\"}} (JSON object) instead.", tc.Name, skillBase)
			}
		}

		if tc.Background {
			logger.Info("LLM requested background tool execution", "name", tc.Name)
			pid, err := tools.RunToolBackground(tc.Name, tc.GetArgs(), cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir, registry)
			if err != nil {
				return fmt.Sprintf("Tool Output: ERROR starting background tool: %v", err)
			}
			return fmt.Sprintf("Tool Output: Tool started in background. PID=%d. Use {\"action\": \"read_process_logs\", \"pid\": %d} to check output.", pid, pid)
		}
		logger.Info("LLM requested tool execution", "name", tc.Name)
		stdout, stderr, err := tools.RunTool(tc.Name, tc.GetArgs(), cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir)
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		return fmt.Sprintf("Tool Output:\nSTDOUT:\n%s\nSTDERR:\n%s\nERROR:\n%s\n", stdout, stderr, errStr)

	case "list_processes":
		logger.Info("LLM requested process list")
		list := registry.List()
		if len(list) == 0 {
			return "Tool Output: No active background processes."
		}
		var sb strings.Builder
		sb.WriteString("Tool Output: Active processes:\n")
		for _, p := range list {
			pid, _ := p["pid"].(int)
			started, _ := p["started"].(string)
			sb.WriteString(fmt.Sprintf("- PID: %d, Started: %s\n", pid, started))
		}
		return sb.String()

	case "stop_process":
		if !cfg.Tools.StopProcess.Enabled {
			return `Tool Output: {"status":"error","message":"stop_process is disabled. Set tools.stop_process.enabled=true in config.yaml."}`
		}
		logger.Info("LLM requested process stop", "pid", tc.PID)
		if err := registry.Terminate(tc.PID); err != nil {
			return fmt.Sprintf("Tool Output: ERROR stopping process %d: %v", tc.PID, err)
		}
		return fmt.Sprintf("Tool Output: Process %d stopped.", tc.PID)

	case "read_process_logs":
		logger.Info("LLM requested process logs", "pid", tc.PID)
		proc, ok := registry.Get(tc.PID)
		if !ok {
			return fmt.Sprintf("Tool Output: ERROR process %d not found", tc.PID)
		}
		return fmt.Sprintf("Tool Output: [LOGS for PID %d]\n%s", tc.PID, proc.ReadOutput())

	case "query_memory":
		if !cfg.Tools.Memory.Enabled {
			return `Tool Output: {"status":"error","message":"Memory tools are disabled. Set tools.memory.enabled=true in config.yaml."}`
		}
		searchContent := tc.Content
		if searchContent == "" {
			searchContent = tc.Query
		}
		logger.Info("LLM requested memory search", "content", searchContent)
		if searchContent == "" {
			return `Tool Output: {"status": "error", "message": "'content' or 'query' (search query) is required"}`
		}
		// Phase 69: Implement semantic query against the VectorDB
		results, _, err := longTermMem.SearchSimilar(searchContent, 5)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "VectorDB search failed: %v"}`, err)
		}
		if len(results) == 0 {
			return `Tool Output: {"status": "success", "message": "No matching long-term memories found."}`
		}
		b, err := json.Marshal(results)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to serialize results: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "data": %s}`, string(b))
	case "manage_updates":
		if !cfg.Agent.AllowSelfUpdate {
			return "Tool Output: [PERMISSION DENIED] manage_updates is disabled in Danger Zone settings (agent.allow_self_update: false)."
		}
		logger.Info("LLM requested update management", "operation", tc.Operation)
		switch tc.Operation {
		case "check":
			installDir := filepath.Dir(cfg.ConfigPath)

			// Binary-only install: no .git directory → use GitHub Releases API
			if _, gitErr := os.Stat(filepath.Join(installDir, ".git")); os.IsNotExist(gitErr) {
				// Read installed version from .version file
				currentVer := "unknown"
				if vb, err := os.ReadFile(filepath.Join(installDir, ".version")); err == nil {
					currentVer = strings.TrimSpace(string(vb))
				}
				// Fetch latest release from GitHub
				type ghRelease struct {
					TagName string `json:"tag_name"`
				}
				httpClient := &http.Client{Timeout: 10 * time.Second}
				req, reqErr := http.NewRequest("GET", "https://api.github.com/repos/antibyte/AuraGo/releases/latest", nil)
				if reqErr != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to build request: %v"}`, reqErr)
				}
				req.Header.Set("User-Agent", "AuraGo-Agent/1.0")
				resp, fetchErr := httpClient.Do(req)
				if fetchErr != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to reach GitHub: %v"}`, fetchErr)
				}
				defer resp.Body.Close()
				var rel ghRelease
				if decErr := json.NewDecoder(resp.Body).Decode(&rel); decErr != nil {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to parse GitHub response: %v"}`, decErr)
				}
				if currentVer != "unknown" && currentVer == rel.TagName {
					return fmt.Sprintf(`Tool Output: {"status":"success","update_available":false,"current_version":%q,"latest_version":%q,"message":"AuraGo is up to date."}`, currentVer, rel.TagName)
				}
				return fmt.Sprintf(`Tool Output: {"status":"success","update_available":true,"current_version":%q,"latest_version":%q,"message":"Update available."}`, currentVer, rel.TagName)
			}

			// Git-based install
			_, err := runGitCommand(filepath.Dir(cfg.ConfigPath), "fetch", "origin", "main", "--quiet")
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to fetch updates: %v"}`, err)
			}

			countOut, err := runGitCommand(filepath.Dir(cfg.ConfigPath), "rev-list", "HEAD..origin/main", "--count")
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to check update count: %v"}`, err)
			}
			countStr := strings.TrimSpace(string(countOut))
			count, _ := strconv.Atoi(countStr)

			if count == 0 {
				return `Tool Output: {"status": "success", "update_available": false, "message": "AuraGo is up to date."}`
			}

			logOut, _ := runGitCommand(filepath.Dir(cfg.ConfigPath), "log", "HEAD..origin/main", "--oneline", "-n", "10")

			return fmt.Sprintf(`Tool Output: {"status": "success", "update_available": true, "count": %d, "changelog": %q}`, count, string(logOut))

		case "install":
			logger.Warn("LLM requested update installation")
			updateScript := filepath.Join(filepath.Dir(cfg.ConfigPath), "update.sh")
			if _, err := os.Stat(updateScript); err != nil {
				return `Tool Output: {"status": "error", "message": "update.sh not found in application directory"}`
			}

			// Run ./update.sh --yes
			updateCmd := exec.Command("/bin/bash", "./update.sh", "--yes")
			updateCmd.Dir = filepath.Dir(cfg.ConfigPath)
			// Ensure environment is passed for update script too
			home, _ := os.UserHomeDir()
			if home != "" {
				updateCmd.Env = append(os.Environ(), "HOME="+home)
			}
			// Start update script. It will handle the rest, potentially killing this process.
			if err := updateCmd.Start(); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to start update script: %v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Update initiated. The system will restart and apply changes shortly."}`

		default:
			return `Tool Output: {"status": "error", "message": "Invalid operation. Use 'check' or 'install'."}`
		}

	case "archive_memory":
		if !cfg.Tools.MemoryMaintenance.Enabled {
			return `Tool Output: {"status":"error","message":"Memory maintenance is disabled. Set tools.memory_maintenance.enabled=true in config.yaml."}`
		}
		logger.Info("LLM requested memory archival", "id", tc.ID)
		return "Tool Output: " + runMemoryOrchestrator(tc, cfg, logger, llmClient, longTermMem, shortTermMem, kg)

	case "optimize_memory":
		if !cfg.Tools.MemoryMaintenance.Enabled {
			return `Tool Output: {"status":"error","message":"Memory maintenance is disabled. Set tools.memory_maintenance.enabled=true in config.yaml."}`
		}
		logger.Info("LLM requested memory optimization")
		return "Tool Output: " + runMemoryOrchestrator(tc, cfg, logger, llmClient, longTermMem, shortTermMem, kg)

	case "manage_knowledge", "knowledge_graph":
		if !cfg.Tools.KnowledgeGraph.Enabled {
			return `Tool Output: {"status":"error","message":"Knowledge graph is disabled. Set tools.knowledge_graph.enabled=true in config.yaml."}`
		}
		if cfg.Tools.KnowledgeGraph.ReadOnly {
			switch tc.Operation {
			case "add_node", "add_edge", "delete_node", "delete_edge", "optimize":
				return `Tool Output: {"status":"error","message":"Knowledge graph is in read-only mode. Disable tools.knowledge_graph.read_only to allow changes."}`
			}
		}
		logger.Info("LLM requested knowledge graph operation", "op", tc.Operation)
		// Phase 69: Route to actual KnowledgeGraph implementation
		switch tc.Operation {
		case "add_node":
			err := kg.AddNode(tc.ID, tc.Label, tc.Properties)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Node added to graph"}`
		case "add_edge":
			err := kg.AddEdge(tc.Source, tc.Target, tc.Relation, tc.Properties)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Edge added to graph"}`
		case "delete_node":
			err := kg.DeleteNode(tc.ID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Node deleted"}`
		case "delete_edge":
			err := kg.DeleteEdge(tc.Source, tc.Target, tc.Relation)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Edge deleted"}`
		case "search":
			res := kg.Search(tc.Content)
			return fmt.Sprintf("Tool Output: %s", res)
		case "optimize":
			res := runMemoryOrchestrator(tc, cfg, logger, llmClient, longTermMem, shortTermMem, kg)
			return fmt.Sprintf("Tool Output: %s", res)

		default:
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown graph operation: %s"}`, tc.Operation)
		}

	case "manage_memory", "core_memory":
		if !cfg.Tools.Memory.Enabled {
			return `Tool Output: {"status":"error","message":"Memory tools are disabled. Set tools.memory.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Memory.ReadOnly {
			switch tc.Operation {
			case "add", "update", "delete", "reset_profile", "delete_profile_entry":
				return `Tool Output: {"status":"error","message":"Memory is in read-only mode. Disable tools.memory.read_only to allow changes."}`
			}
		}
		// Handle synonyms for 'fact'
		fact := tc.Fact
		if fact == "" {
			if tc.MemoryValue != "" {
				fact = tc.MemoryValue
			} else if tc.MemoryKey != "" {
				fact = tc.MemoryKey
			} else if tc.Value != "" {
				fact = tc.Value
			} else if tc.Content != "" {
				fact = tc.Content
			}
		}
		// When LLM uses separate key+value fields, combine into a meaningful fact (e.g. "agent_name: Nova")
		// Only for add/update, and only when key is a descriptive word (not a numeric ID)
		{
			op := strings.ToLower(tc.Operation)
			keyField := tc.Key
			if keyField == "" {
				keyField = tc.MemoryKey
			}
			if (op == "add" || op == "update") && keyField != "" && fact != "" && fact != keyField {
				if _, parseErr := strconv.ParseInt(keyField, 10, 64); parseErr != nil {
					// Key is not a numeric ID — prefix fact with key for context
					if !strings.HasPrefix(strings.ToLower(fact), strings.ToLower(keyField)+":") &&
						!strings.HasPrefix(strings.ToLower(fact), strings.ToLower(keyField)+" ") {
						fact = keyField + ": " + fact
					}
				}
			}
		}

		logger.Info("LLM requested core memory management", "op", tc.Operation, "fact", fact)
		if tc.Operation == "" {
			return `Tool Output: {"status": "error", "message": "'operation' is required for manage_memory"}`
		}

		// User Profile operations (sub-ops of manage_memory)
		switch tc.Operation {
		case "view_profile":
			entries, err := shortTermMem.GetProfileEntries("")
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			if len(entries) == 0 {
				return `Tool Output: {"status": "success", "message": "No user profile data collected yet.", "entries": []}`
			}
			var sb strings.Builder
			sb.WriteString(`{"status":"success","entries":[`)
			for i, e := range entries {
				if i > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(fmt.Sprintf(`{"category":%q,"key":%q,"value":%q,"confidence":%d}`, e.Category, e.Key, e.Value, e.Confidence))
			}
			sb.WriteString(`]}`)
			return fmt.Sprintf("Tool Output: %s", sb.String())
		case "reset_profile":
			if err := shortTermMem.ResetUserProfile(); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "User profile has been completely reset."}`
		case "delete_profile_entry":
			cat := tc.Key
			key := tc.Value
			if cat == "" || key == "" {
				return `Tool Output: {"status": "error", "message": "'key' (category) and 'value' (key name) are required for delete_profile_entry"}`
			}
			if err := shortTermMem.DeleteProfileEntry(cat, key); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Profile entry %s/%s deleted."}`, cat, key)
		}

		var memID int64
		fmt.Sscanf(tc.ID, "%d", &memID)
		result, err := tools.ManageCoreMemory(tc.Operation, fact, memID, shortTermMem, cfg.Agent.CoreMemoryMaxEntries, cfg.Agent.CoreMemoryCapMode)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
		}
		return fmt.Sprintf("Tool Output: %s", result)

	case "get_secret", "secrets_vault":
		if !cfg.Tools.SecretsVault.Enabled {
			return `Tool Output: {"status":"error","message":"Secrets vault is disabled. Set tools.secrets_vault.enabled=true in config.yaml."}`
		}
		op := strings.TrimSpace(strings.ToLower(tc.Operation))
		if cfg.Tools.SecretsVault.ReadOnly && (op == "store" || op == "set" || tc.Action == "set_secret") {
			return `Tool Output: {"status":"error","message":"Secrets vault is in read-only mode. Disable tools.secrets_vault.read_only to allow changes."}`
		}
		if op == "store" || op == "set" || (tc.Action == "set_secret") {
			logger.Info("LLM requested secret storage", "key", tc.Key)
			if tc.Key == "" || tc.Value == "" {
				return `Tool Output: {"status": "error", "message": "'key' and 'value' are required for set_secret/store"}`
			}
			err := vault.WriteSecret(tc.Key, tc.Value)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Secret '%s' stored safely."}`, tc.Key)
		}

		// Default: read/list
		logger.Info("LLM requested secret retrieval", "key", tc.Key)
		if tc.Key == "" {
			// List available secret keys when no key is specified
			keys, err := vault.ListKeys()
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			b, mErr := json.Marshal(keys)
			if mErr != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to serialize keys: %v"}`, mErr)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Stored secret keys (use get_secret with 'key' to retrieve a value)", "keys": %s}`, string(b))
		}
		secret, err := vault.ReadSecret(tc.Key)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
		}
		// JSON-encode the secret value to prevent injection from special characters
		safeVal, _ := json.Marshal(secret)
		return fmt.Sprintf(`Tool Output: {"status": "success", "key": "%s", "value": %s}`, tc.Key, string(safeVal))

	case "set_secret":
		if !cfg.Tools.SecretsVault.Enabled {
			return `Tool Output: {"status":"error","message":"Secrets vault is disabled. Set tools.secrets_vault.enabled=true in config.yaml."}`
		}
		if cfg.Tools.SecretsVault.ReadOnly {
			return `Tool Output: {"status":"error","message":"Secrets vault is in read-only mode. Disable tools.secrets_vault.read_only to allow changes."}`
		}
		logger.Info("LLM requested secret storage", "key", tc.Key)
		if tc.Key == "" || tc.Value == "" {
			return `Tool Output: {"status": "error", "message": "'key' and 'value' are required for set_secret"}`
		}
		err := vault.WriteSecret(tc.Key, tc.Value)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Secret '%s' stored safely."}`, tc.Key)

	case "filesystem", "filesystem_op":
		// Parameter robustness: handle 'path' and 'dest' aliases frequently hallucinated by LLMs
		fpath := tc.FilePath
		if fpath == "" {
			fpath = tc.Path
		}
		fdest := tc.Destination
		if fdest == "" {
			fdest = tc.Dest
		}

		op := strings.TrimSpace(strings.ToLower(tc.Operation))
		if op == "list" || op == "ls" {
			op = "list_dir"
		}
		if !cfg.Agent.AllowFilesystemWrite {
			writeOps := map[string]bool{"write": true, "write_file": true, "append": true, "delete": true, "remove": true, "move": true, "rename": true, "mkdir": true, "create_dir": true, "create": true}
			if writeOps[op] {
				return "Tool Output: [PERMISSION DENIED] filesystem write operations are disabled in Danger Zone settings (agent.allow_filesystem_write: false)."
			}
		}
		logger.Info("LLM requested filesystem operation", "op", op, "path", fpath, "dest", fdest)
		return tools.ExecuteFilesystem(op, fpath, fdest, tc.Content, cfg.Directories.WorkspaceDir)

	case "api_request":
		if !cfg.Agent.AllowNetworkRequests {
			return "Tool Output: [PERMISSION DENIED] api_request is disabled in Danger Zone settings (agent.allow_network_requests: false)."
		}
		logger.Info("LLM requested generic API request", "url", tc.URL)
		return tools.ExecuteAPIRequest(tc.Method, tc.URL, tc.Body, tc.Headers)

	case "koofr", "koofr_api", "koofr_op":
		if !cfg.Koofr.Enabled {
			return `Tool Output: {"status": "error", "message": "Koofr integration is not enabled. Set koofr.enabled=true in config.yaml."}`
		}
		if cfg.Koofr.ReadOnly {
			switch tc.Operation {
			case "write", "put", "upload", "mkdir", "delete", "rm", "move", "rename", "mv":
				return `Tool Output: {"status":"error","message":"Koofr is in read-only mode. Disable koofr.read_only to allow changes."}`
			}
		}
		fpath := tc.FilePath
		if fpath == "" {
			fpath = tc.Path
		}
		fdest := tc.Destination
		if fdest == "" {
			fdest = tc.Dest
		}
		logger.Info("LLM requested koofr operation", "op", tc.Operation, "path", fpath, "dest", fdest)
		koofrCfg := tools.KoofrConfig{
			BaseURL:     cfg.Koofr.BaseURL,
			Username:    cfg.Koofr.Username,
			AppPassword: cfg.Koofr.AppPassword,
		}
		return tools.ExecuteKoofr(koofrCfg, tc.Operation, fpath, fdest, tc.Content)

	case "google_workspace", "gworkspace":
		if !cfg.Agent.EnableGoogleWorkspace {
			return `Tool Output: {"status": "error", "message": "Google Workspace is not enabled. Set agent.enable_google_workspace=true in config.yaml."}`
		}
		op := tc.Operation
		if op == "" {
			op = tc.Action // Fallback if LLM puts it in action
		}
		logger.Info("LLM requested google_workspace operation", "op", op, "doc_id", tc.DocumentID)
		gConfig := tools.GoogleWorkspaceConfig{
			Action:     op,
			MaxResults: tc.MaxResults,
			DocumentID: tc.DocumentID,
			Title:      tc.Title,
			Text:       tc.Text,
			Append:     tc.Append,
		}
		res, err := tools.ExecuteGoogleWorkspace(vault, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir, gConfig)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
		}
		return res

	case "query_inventory":
		queryTag := tc.Tag
		if queryTag == "" {
			queryTag = tc.Tags
		}
		logger.Info("LLM requested inventory query", "tag", queryTag, "name", tc.Hostname)
		devices, err := inventory.QueryDevices(inventoryDB, queryTag, tc.DeviceType, tc.Hostname)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to query inventory: %v"}`, err)
		}
		b, mErr := json.Marshal(devices)
		if mErr != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to serialize devices: %v"}`, mErr)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "tag": "%s", "device_type": "%s", "name_match": "%s", "devices": %s}`, tc.Tag, tc.DeviceType, tc.Hostname, string(b))

	case "execute_remote_shell", "remote_execution":
		if !cfg.Agent.AllowRemoteShell {
			return "Tool Output: [PERMISSION DENIED] execute_remote_shell is disabled in Danger Zone settings (agent.allow_remote_shell: false)."
		}
		logger.Info("LLM requested remote shell execution", "server_id", tc.ServerID, "command", tc.Command)
		if tc.ServerID == "" || tc.Command == "" {
			return `Tool Output: {"status": "error", "message": "'server_id' and 'command' are required"}`
		}
		device, err := inventory.GetDeviceByID(inventoryDB, tc.ServerID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Device not found: %v"}`, err)
		}
		secret, err := vault.ReadSecret(device.VaultSecretID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to fetch secret: %v"}`, err)
		}
		rCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		output, err := remote.ExecuteRemoteCommand(rCtx, device.Name, device.Port, device.Username, []byte(secret), tc.Command)
		if err != nil {
			safeOutput, mErr := json.Marshal(output)
			if mErr != nil {
				safeOutput = []byte(`""`)
			}
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Remote execution failed", "output": %s, "error": "%v"}`, string(safeOutput), err)
		}
		safeOutput, mErr := json.Marshal(output)
		if mErr != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to serialize output: %v"}`, mErr)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "output": %s}`, string(safeOutput))

	case "transfer_remote_file":
		if !cfg.Agent.AllowRemoteShell {
			return "Tool Output: [PERMISSION DENIED] transfer_remote_file is disabled in Danger Zone settings (agent.allow_remote_shell: false)."
		}
		logger.Info("LLM requested remote file transfer", "server_id", tc.ServerID, "direction", tc.Direction)
		if tc.ServerID == "" || tc.Direction == "" || tc.LocalPath == "" || tc.RemotePath == "" {
			return `Tool Output: {"status": "error", "message": "'server_id', 'direction', 'local_path', and 'remote_path' are required"}`
		}
		// Sanitize and restrict local path
		absLocal, err := filepath.Abs(tc.LocalPath)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Invalid local path: %v"}`, err)
		}
		workspaceWorkdir := filepath.Join(cfg.Directories.WorkspaceDir, "workdir")
		if !strings.HasPrefix(strings.ToLower(absLocal), strings.ToLower(workspaceWorkdir)) {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Permission denied: local_path must be within %s"}`, workspaceWorkdir)
		}

		device, err := inventory.GetDeviceByID(inventoryDB, tc.ServerID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Device not found: %v"}`, err)
		}
		secret, err := vault.ReadSecret(device.VaultSecretID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to fetch secret: %v"}`, err)
		}
		rCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		err = remote.TransferFile(rCtx, device.Name, device.Port, device.Username, []byte(secret), absLocal, tc.RemotePath, tc.Direction)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "File transfer failed: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "File %s successfully"}`, tc.Direction)

	case "manage_schedule", "cron_scheduler":
		if !cfg.Tools.Scheduler.Enabled {
			return `Tool Output: {"status":"error","message":"Scheduler is disabled. Set tools.scheduler.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Scheduler.ReadOnly {
			switch tc.Operation {
			case "add", "remove":
				return `Tool Output: {"status":"error","message":"Scheduler is in read-only mode. Disable tools.scheduler.read_only to allow changes."}`
			}
		}
		logger.Info("LLM requested cron management", "operation", tc.Operation)
		result, err := cronManager.ManageSchedule(tc.Operation, tc.ID, tc.CronExpr, tc.TaskPrompt)
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR in manage_schedule: %v", err)
		}
		return result

	case "schedule_cron":
		if !cfg.Tools.Scheduler.Enabled {
			return `Tool Output: {"status":"error","message":"Scheduler is disabled. Set tools.scheduler.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Scheduler.ReadOnly {
			return `Tool Output: {"status":"error","message":"Scheduler is in read-only mode. Disable tools.scheduler.read_only to allow changes."}`
		}
		logger.Info("LLM requested cron scheduling", "expr", tc.CronExpr)
		result, err := cronManager.ManageSchedule("add", "", tc.CronExpr, tc.TaskPrompt)
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR scheduling cron: %v", err)
		}
		return result

	case "list_cron_jobs":
		if !cfg.Tools.Scheduler.Enabled {
			return `Tool Output: {"status":"error","message":"Scheduler is disabled. Set tools.scheduler.enabled=true in config.yaml."}`
		}
		logger.Info("LLM requested cron job list")
		result, _ := cronManager.ManageSchedule("list", "", "", "")
		return result

	case "remove_cron_job":
		if !cfg.Tools.Scheduler.Enabled {
			return `Tool Output: {"status":"error","message":"Scheduler is disabled. Set tools.scheduler.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Scheduler.ReadOnly {
			return `Tool Output: {"status":"error","message":"Scheduler is in read-only mode. Disable tools.scheduler.read_only to allow changes."}`
		}
		logger.Info("LLM requested cron job removal", "id", tc.ID)
		result, _ := cronManager.ManageSchedule("remove", tc.ID, "", "")
		return result

	case "call_webhook":
		if !cfg.Webhooks.Enabled {
			return `Tool Output: {"status":"error","message":"Webhooks are disabled in the config. Set webhooks.enabled=true."}`
		}
		logger.Info("LLM requested webhook execution", "webhook_name", tc.WebhookName)

		// Find the webhook by name
		var targetHook *config.OutgoingWebhook
		for _, w := range cfg.Webhooks.Outgoing {
			if strings.EqualFold(w.Name, tc.WebhookName) {
				targetHook = &w
				break
			}
		}

		if targetHook == nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Webhook '%s' not found. Check the exact name of the webhook from your System Context."}`, tc.WebhookName)
		}

		// Map parameters
		paramMap := make(map[string]interface{})
		if pm, ok := tc.Parameters.(map[string]interface{}); ok {
			for k, v := range pm {
				paramMap[k] = v
			}
		}

		out, statusCode, err := tools.ExecuteOutgoingWebhook(ctx, *targetHook, paramMap)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to execute webhook: %v"}`, err)
		}

		// Provide simple response
		return fmt.Sprintf(`Tool Output: {"status":"success", "http_status_code": %d, "response": %q}`, statusCode, out)

	case "manage_outgoing_webhooks":
		if !cfg.Webhooks.Enabled {
			return `Tool Output: {"status":"error","message":"Webhooks are disabled in the config. Set webhooks.enabled=true."}`
		}
		if cfg.Webhooks.ReadOnly && tc.Operation != "list" {
			return `Tool Output: {"status":"error","message":"Webhooks tool is set to Read-Only mode. Cannot modify."}`
		}

		var rawParams []interface{}
		if rp, ok := tc.Parameters.([]interface{}); ok {
			rawParams = rp
		}

		return tools.ManageOutgoingWebhooks(tc.Operation, tc.ID, tc.Name, tc.Description, tc.Method, tc.URL, tc.PayloadType, tc.BodyTemplate, tc.Headers, rawParams, cfg)

	case "list_skills":
		logger.Info("LLM requested to list skills")
		skills, err := tools.ListSkills(cfg.Directories.SkillsDir, cfg.Agent.EnableGoogleWorkspace)
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR listing skills: %v", err)
		}
		if len(skills) == 0 {
			return "Tool Output: No internal skills found."
		}
		b, err := json.MarshalIndent(skills, "", "  ")
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR serializing skills list: %v", err)
		}
		return fmt.Sprintf("Tool Output: Internal Skills Configuration:\n%s", string(b))

	case "execute_skill":
		if !cfg.Agent.AllowPython {
			return "Tool Output: [PERMISSION DENIED] execute_skill is disabled in Danger Zone settings (agent.allow_python: false)."
		}
		logger.Info("LLM requested skill execution", "skill", tc.Skill, "args", tc.SkillArgs, "params", tc.Params)
		// Robust argument lookup: handle both 'skill_args' and 'params'
		args := tc.SkillArgs
		if args == nil {
			args = tc.Params
		}

		skillName := tc.Skill
		if skillName == "" && args != nil {
			// Aggressive recovery: Check if LLM nested the skill name inside arguments
			for _, key := range []string{"skill", "skill_name", "name", "tool"} {
				if s, ok := args[key].(string); ok && s != "" {
					skillName = s
					logger.Info("[Recovery] Found nested skill name in arguments", "key", key, "skill", skillName)
					break
				}
			}
		}

		if skillName == "" {
			return "Tool Output: ERROR 'skill' name is required. Use {\"action\": \"execute_skill\", \"skill\": \"name\", \"params\": {...}}"
		}

		// Unwrap skill_args if the LLM nested the actual parameters under that key.
		// e.g. {"skill_name": "ddg_search", "skill_args": {"query": "..."}} → {"query": "..."}
		if innerArgs, ok := args["skill_args"].(map[string]interface{}); ok && len(innerArgs) > 0 {
			args = innerArgs
		} else {
			// Clean up metadata keys that aren't real skill parameters
			cleanArgs := make(map[string]interface{}, len(args))
			metaKeys := map[string]bool{"skill_name": true, "skill": true, "name": true, "tool": true, "action": true}
			for k, v := range args {
				if !metaKeys[k] {
					cleanArgs[k] = v
				}
			}
			args = cleanArgs
		}

		cleanSkillName := strings.TrimSuffix(skillName, ".py")
		switch cleanSkillName {
		case "web_scraper":
			if !cfg.Tools.WebScraper.Enabled {
				return "Tool Output: [PERMISSION DENIED] web_scraper is disabled in settings (tools.web_scraper.enabled: false)."
			}
			urlStr, _ := args["url"].(string)
			scraped := tools.ExecuteWebScraper(urlStr)

			// Summary mode: send scraped content to a separate LLM for
			// summarisation so the agent only receives a concise summary.
			// This saves tokens in the main model and prevents prompt
			// injection from external web content.
			if cfg.Tools.WebScraper.SummaryMode {
				searchQuery, _ := args["search_query"].(string)
				if searchQuery == "" {
					searchQuery = "general summary of the page content"
				}
				summary, err := tools.SummariseScrapedContent(ctx, cfg, logger, scraped, searchQuery)
				if err != nil {
					logger.Warn("web_scraper summary failed, returning raw content", "error", err)
				} else {
					scraped = summary
				}
			}
			return scraped
		case "wikipedia_search":
			queryStr, _ := args["query"].(string)
			langStr, _ := args["language"].(string)
			return tools.ExecuteWikipediaSearch(queryStr, langStr)
		case "ddg_search":
			queryStr, _ := args["query"].(string)
			maxRes, ok := args["max_results"].(float64)
			if !ok {
				maxRes = 5
			}
			return tools.ExecuteDDGSearch(queryStr, int(maxRes))
		case "virustotal_scan":
			if !cfg.VirusTotal.Enabled {
				return `Tool Output: {"status": "error", "message": "VirusTotal integration is not enabled. Set virustotal.enabled=true in config.yaml."}`
			}
			resource, _ := args["resource"].(string)
			return tools.ExecuteVirusTotalScan(cfg.VirusTotal.APIKey, resource)
		case "brave_search":
			if !cfg.BraveSearch.Enabled {
				return `Tool Output: {"status": "error", "message": "Brave Search integration is not enabled. Set brave_search.enabled=true in config.yaml."}`
			}
			queryStr, _ := args["query"].(string)
			count, ok := args["count"].(float64)
			if !ok {
				count = 10
			}
			country, _ := args["country"].(string)
			if country == "" {
				country = cfg.BraveSearch.Country
			}
			lang, _ := args["lang"].(string)
			if lang == "" {
				lang = cfg.BraveSearch.Lang
			}
			return tools.ExecuteBraveSearch(cfg.BraveSearch.APIKey, queryStr, int(count), country, lang)
		case "git_backup_restore":
			reqJSON, _ := json.Marshal(args)
			var req tools.GitBackupRequest
			json.Unmarshal(reqJSON, &req)
			return tools.ExecuteGit(cfg.Directories.WorkspaceDir, req)
		case "google_workspace":
			op, _ := args["operation"].(string)
			limit, _ := args["limit"].(float64)
			docID, _ := args["document_id"].(string)
			title, _ := args["title"].(string)
			text, _ := args["text"].(string)
			appendMode, _ := args["append"].(bool)
			gConfig := tools.GoogleWorkspaceConfig{
				Action:     op,
				MaxResults: int(limit),
				DocumentID: docID,
				Title:      title,
				Text:       text,
				Append:     appendMode,
			}
			res, err := tools.ExecuteGoogleWorkspace(vault, cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir, gConfig)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return res
		}

		res, err := tools.ExecuteSkill(cfg.Directories.SkillsDir, cfg.Directories.WorkspaceDir, skillName, args, cfg.Agent.EnableGoogleWorkspace)
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR executing skill: %v\nOutput: %s", err, res)
		}
		return fmt.Sprintf("Tool Output: %s", res)

	case "follow_up":
		logger.Info("LLM requested follow-up", "prompt", tc.TaskPrompt)
		if tc.TaskPrompt == "" {
			return "Tool Output: ERROR 'task_prompt' is required for follow_up"
		}

		// Guard: follow_up must describe work for the agent to do autonomously.
		// It must NEVER be used to relay a question back to the user — that causes
		// an infinite loop where each invocation re-asks the same question.
		trimmedPrompt := strings.TrimSpace(tc.TaskPrompt)
		if isFollowUpQuestion(trimmedPrompt) {
			logger.Warn("[follow_up] Blocked: task_prompt looks like a question directed at the user", "prompt", trimmedPrompt)
			return `Tool Output: [ERROR] follow_up must not be used to ask the user for information. ` +
				`If you need input from the user, respond directly with your question in plain text. ` +
				`follow_up is only for scheduling autonomous background work you will perform yourself.`
		}

		// Trigger background follow-up request
		go func(prompt string, port int) {
			time.Sleep(2 * time.Second) // Let current response finish
			url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", port)

			payload := map[string]interface{}{
				"model":  "aurago",
				"stream": false,
				"messages": []map[string]string{
					{"role": "user", "content": prompt},
				},
			}

			body, _ := json.Marshal(payload)
			req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
			if err != nil {
				logger.Error("Failed to create follow-up request", "error", err)
				return
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Internal-FollowUp", "true")

			client := &http.Client{Timeout: 10 * time.Minute}
			resp, err := client.Do(req)
			if err != nil {
				logger.Error("Follow-up request failed", "error", err)
				return
			}
			defer resp.Body.Close()
			logger.Info("Follow-up triggered successfully", "status", resp.Status)
		}(tc.TaskPrompt, cfg.Server.Port)

		return "Tool Output: Follow-up scheduled. I will continue in the background immediately after this message."

	case "get_tool_manual":
		logger.Info("LLM requested tool manual", "name", tc.ToolName)
		if tc.ToolName == "" {
			return "Tool Output: ERROR 'tool_name' is required"
		}

		// Fallback for LLMs getting creative with the manual name
		cleanName := strings.TrimSuffix(tc.ToolName, ".md")
		cleanName = strings.TrimSuffix(cleanName, "_tool_manual")
		manualPath := filepath.Join(cfg.Directories.PromptsDir, "tools_manuals", cleanName+".md")
		data, err := os.ReadFile(manualPath)
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR could not read manual for '%s': %v", tc.ToolName, err)
		}
		return fmt.Sprintf("Tool Output: [MANUAL FOR %s]\n%s", tc.ToolName, string(data))

	case "execute_surgery":
		if !isMaintenance {
			return "Tool Output: ERROR 'execute_surgery' can ONLY be used when in Maintenance mode (Lifeboat). You are currently in Supervisor mode. You MUST use 'initiate_handover' first to propose a plan and switch to Maintenance mode for complex code changes."
		}

		// Robustness: handle both 'task_prompt' and 'content' for the plan
		plan := tc.TaskPrompt
		if plan == "" {
			plan = tc.Content
		}

		logger.Info("LLM requested surgery via Gemini CLI", "plan_len", len(plan), "prompt_preview", Truncate(plan, 100))
		if plan == "" {
			return "Tool Output: ERROR surgery plan is required (via 'task_prompt' or 'content')"
		}
		// Using external Gemini CLI via the surgery tool
		res, err := tools.ExecuteSurgery(plan, cfg.Directories.WorkspaceDir, logger)
		if err != nil {
			return fmt.Sprintf("Tool Output: ERROR surgery failed: %v\nOutput: %s", err, res)
		}
		return fmt.Sprintf("Tool Output: Surgery successful.\nDetails:\n%s", res)

	case "exit_lifeboat":
		if !isMaintenance {
			return "Tool Output: ERROR 'exit_lifeboat' can only be used when already in maintenance mode. You are currently in the standard Supervisor mode."
		}
		logger.Info("LLM requested to exit lifeboat")
		tools.SetBusy(false)
		return "Tool Output: [LIFEBOAT_EXIT_SIGNAL] Maintenance complete. Attempting to return to main supervisor."

	case "initiate_handover":
		if isMaintenance {
			return "Tool Output: ERROR You are already in Lifeboat mode. Maintenance is active. Use 'exit_lifeboat' to return to the supervisor or 'execute_surgery' for code changes."
		}
		logger.Info("LLM requested lifeboat handover", "plan_len", len(tc.TaskPrompt))
		return tools.InitiateLifeboatHandover(tc.TaskPrompt, cfg)

	case "get_system_metrics", "system_metrics":
		logger.Info("LLM requested system metrics")
		return "Tool Output: " + tools.GetSystemMetrics()

	case "send_notification", "notification_center", "send_push_notification", "web_push":
		if tc.ToolName == "send_push_notification" || tc.ToolName == "web_push" {
			tc.Channel = "push"
		}
		logger.Info("LLM requested notification", "channel", tc.Channel, "title", tc.Title)
		// Use discord bridge (tools.DiscordSend) to avoid import cycle
		var discordSend tools.DiscordSendFunc
		if cfg.Discord.Enabled {
			discordSend = func(channelID, content string) error {
				return tools.DiscordSend(channelID, content, logger)
			}
		}
		priority := tc.Tag // reuse existing Tag field for priority
		return "Tool Output: " + tools.SendNotification(cfg, logger, tc.Channel, tc.Title, tc.Message, priority, discordSend)

	case "send_image":
		logger.Info("LLM requested image send", "path", tc.Path, "caption", tc.Caption)
		return handleSendImage(tc, cfg, logger)

	case "manage_processes", "process_management":
		logger.Info("LLM requested process management", "op", tc.Operation)
		return "Tool Output: " + tools.ManageProcesses(tc.Operation, int32(tc.PID))

	case "register_device", "register_server":
		logger.Info("LLM requested device registration", "name", tc.Hostname)
		tags := services.ParseTags(tc.Tags)
		deviceType := tc.DeviceType
		if deviceType == "" {
			deviceType = "server"
		}

		// If LLM hallucinated, putting IP in Hostname and leaving IPAddress empty:
		if tc.IPAddress == "" && net.ParseIP(tc.Hostname) != nil {
			tc.IPAddress = tc.Hostname
		}

		id, err := services.RegisterDevice(inventoryDB, vault, tc.Hostname, deviceType, tc.IPAddress, tc.Port, tc.Username, tc.Password, tc.PrivateKeyPath, tc.Description, tags, tc.MACAddress)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to register device: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Device registered successfully", "id": "%s"}`, id)

	case "wake_on_lan", "wake_device", "wol":
		if !cfg.Tools.WOL.Enabled {
			return `Tool Output: {"status": "error", "message": "Wake-on-LAN is disabled. Enable it via tools.wol.enabled in config.yaml."}`
		}
		logger.Info("LLM requested Wake-on-LAN", "server_id", tc.ServerID, "mac", tc.MACAddress)

		mac := tc.MACAddress
		if mac == "" && tc.ServerID != "" && inventoryDB != nil {
			// Look up MAC from inventory
			device, err := inventory.GetDeviceByID(inventoryDB, tc.ServerID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Device not found: %v"}`, err)
			}
			mac = device.MACAddress
		}
		if mac == "" {
			return `Tool Output: {"status": "error", "message": "No MAC address available. Provide 'mac_address' or a 'server_id' with a registered MAC address."}`
		}

		if err := tools.SendWakeOnLAN(mac, tc.IPAddress); err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to send WOL packet: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Wake-on-LAN magic packet sent to %s"}`, mac)

	case "pin_message":
		logger.Info("LLM requested message pinning", "id", tc.ID, "pinned", tc.Pinned)
		if tc.ID == "" {
			return `Tool Output: {"status": "error", "message": "'id' is required for pin_message"}`
		}
		// Try to parse ID as int64
		var msgID int64
		fmt.Sscanf(tc.ID, "%d", &msgID)
		if msgID == 0 {
			return `Tool Output: {"status": "error", "message": "Invalid 'id' format"}`
		}

		err := shortTermMem.SetMessagePinned(msgID, tc.Pinned)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to update SQLite: %v"}`, err)
		}
		if historyMgr != nil {
			_ = historyMgr.SetPinned(msgID, tc.Pinned)
		}
		status := "pinned"
		if !tc.Pinned {
			status = "unpinned"
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Message %d %s successfully."}`, msgID, status)

	case "fetch_email", "check_email":
		if !cfg.Email.Enabled && len(cfg.EmailAccounts) == 0 {
			return `Tool Output: {"status": "error", "message": "Email is not enabled. Configure the email section in config.yaml or add email_accounts."}`
		}
		// Resolve email account
		var acct *config.EmailAccount
		if tc.Account != "" {
			acct = cfg.FindEmailAccount(tc.Account)
			if acct == nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' not found. Use list_email_accounts to see available accounts."}`, tc.Account)
			}
		} else {
			acct = cfg.DefaultEmailAccount()
		}
		if acct == nil {
			return `Tool Output: {"status": "error", "message": "No email account configured."}`
		}
		logger.Info("LLM requested email fetch", "account", acct.ID, "folder", tc.Folder)
		folder := tc.Folder
		if folder == "" {
			folder = acct.WatchFolder
		}
		limit := tc.Limit
		if limit <= 0 {
			limit = 10
		}
		messages, err := tools.FetchEmails(
			acct.IMAPHost, acct.IMAPPort,
			acct.Username, acct.Password,
			folder, limit, logger,
		)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "IMAP fetch failed (%s): %v"}`, acct.ID, err)
		}
		// Guardian: scan each message body for injection attempts
		if guardian != nil {
			for i := range messages {
				combined := messages[i].From + " " + messages[i].Subject + " " + messages[i].Body
				scanRes := guardian.ScanForInjection(combined)
				if scanRes.Level >= security.ThreatHigh {
					logger.Warn("[Email] Guardian HIGH threat in message", "uid", messages[i].UID, "from", messages[i].From, "threat", scanRes.Level.String())
					messages[i].Body = "[REDACTED by Guardian — injection attempt detected]"
					messages[i].Subject = "[SANITIZED] " + messages[i].Subject
					messages[i].Snippet = "[REDACTED]"
				} else {
					messages[i].Body = guardian.SanitizeToolOutput("email", messages[i].Body)
				}
			}
		}
		result := tools.EmailResult{Status: "success", Count: len(messages), Data: messages, Message: fmt.Sprintf("Account: %s", acct.ID)}
		return "Tool Output: " + tools.EncodeEmailResult(result)

	case "send_email":
		if !cfg.Email.Enabled && len(cfg.EmailAccounts) == 0 {
			return `Tool Output: {"status": "error", "message": "Email is not enabled. Configure the email section in config.yaml or add email_accounts."}`
		}
		if cfg.Email.ReadOnly {
			return `Tool Output: {"status":"error","message":"Email is in read-only mode. Disable email.read_only to allow sending."}`
		}
		// Resolve email account
		var acct *config.EmailAccount
		if tc.Account != "" {
			acct = cfg.FindEmailAccount(tc.Account)
			if acct == nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' not found. Use list_email_accounts to see available accounts."}`, tc.Account)
			}
		} else {
			acct = cfg.DefaultEmailAccount()
		}
		if acct == nil {
			return `Tool Output: {"status": "error", "message": "No email account configured."}`
		}
		to := tc.To
		if to == "" {
			return `Tool Output: {"status": "error", "message": "'to' (recipient address) is required"}`
		}
		subject := tc.Subject
		if subject == "" {
			subject = "(no subject)"
		}
		body := tc.Body
		if body == "" {
			body = tc.Content
		}
		logger.Info("LLM requested email send", "account", acct.ID, "to", to, "subject", subject)
		var sendErr error
		if acct.SMTPPort == 465 {
			sendErr = tools.SendEmailTLS(acct.SMTPHost, acct.SMTPPort, acct.Username, acct.Password, acct.FromAddress, to, subject, body, logger)
		} else {
			sendErr = tools.SendEmail(acct.SMTPHost, acct.SMTPPort, acct.Username, acct.Password, acct.FromAddress, to, subject, body, logger)
		}
		if sendErr != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "SMTP send failed (%s): %v"}`, acct.ID, sendErr)
		}
		result := tools.EmailResult{Status: "success", Message: fmt.Sprintf("Email sent to %s via account %s", to, acct.ID)}
		return "Tool Output: " + tools.EncodeEmailResult(result)

	case "list_email_accounts":
		if len(cfg.EmailAccounts) == 0 {
			return `Tool Output: {"status": "success", "count": 0, "data": [], "message": "No email accounts configured."}`
		}
		type acctInfo struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Email   string `json:"email"`
			IMAP    string `json:"imap"`
			SMTP    string `json:"smtp"`
			Watcher bool   `json:"watcher"`
		}
		var accts []acctInfo
		for _, a := range cfg.EmailAccounts {
			accts = append(accts, acctInfo{
				ID:      a.ID,
				Name:    a.Name,
				Email:   a.FromAddress,
				IMAP:    fmt.Sprintf("%s:%d", a.IMAPHost, a.IMAPPort),
				SMTP:    fmt.Sprintf("%s:%d", a.SMTPHost, a.SMTPPort),
				Watcher: a.WatchEnabled,
			})
		}
		result := tools.EmailResult{Status: "success", Count: len(accts), Data: accts}
		return "Tool Output: " + tools.EncodeEmailResult(result)

	case "send_discord":
		if !cfg.Discord.Enabled {
			return `Tool Output: {"status": "error", "message": "Discord is not enabled. Configure the discord section in config.yaml."}`
		}
		if cfg.Discord.ReadOnly {
			return `Tool Output: {"status":"error","message":"Discord is in read-only mode. Disable discord.read_only to allow changes."}`
		}
		channelID := tc.ChannelID
		if channelID == "" {
			channelID = cfg.Discord.DefaultChannelID
		}
		if channelID == "" {
			return `Tool Output: {"status": "error", "message": "'channel_id' is required (or set default_channel_id in config)"}`
		}
		message := tc.Message
		if message == "" {
			message = tc.Content
		}
		if message == "" {
			message = tc.Body
		}
		if message == "" {
			return `Tool Output: {"status": "error", "message": "'message' (or 'content') is required"}`
		}
		logger.Info("LLM requested Discord send", "channel", channelID)
		if err := tools.DiscordSend(channelID, message, logger); err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Discord send failed: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Message sent to Discord channel %s"}`, channelID)

	case "fetch_discord":
		if !cfg.Discord.Enabled {
			return `Tool Output: {"status": "error", "message": "Discord is not enabled. Configure the discord section in config.yaml."}`
		}
		channelID := tc.ChannelID
		if channelID == "" {
			channelID = cfg.Discord.DefaultChannelID
		}
		if channelID == "" {
			return `Tool Output: {"status": "error", "message": "'channel_id' is required (or set default_channel_id in config)"}`
		}
		limit := tc.Limit
		if limit <= 0 {
			limit = 10
		}
		logger.Info("LLM requested Discord message fetch", "channel", channelID, "limit", limit)
		msgs, err := tools.DiscordFetch(channelID, limit, logger)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Discord fetch failed: %v"}`, err)
		}
		// Guardian-sanitize external content
		if guardian != nil {
			for i := range msgs {
				scanRes := guardian.ScanForInjection(msgs[i].Author + " " + msgs[i].Content)
				if scanRes.Level >= security.ThreatHigh {
					logger.Warn("[Discord] Guardian HIGH threat in message", "author", msgs[i].Author, "threat", scanRes.Level.String())
					msgs[i].Content = "[REDACTED by Guardian — injection attempt detected]"
				} else {
					msgs[i].Content = guardian.SanitizeToolOutput("discord", msgs[i].Content)
				}
			}
		}
		data, _ := json.Marshal(map[string]interface{}{
			"status": "success",
			"count":  len(msgs),
			"data":   msgs,
		})
		return "Tool Output: " + string(data)

	case "list_discord_channels":
		if !cfg.Discord.Enabled {
			return `Tool Output: {"status": "error", "message": "Discord is not enabled."}`
		}
		guildID := cfg.Discord.GuildID
		if guildID == "" {
			return `Tool Output: {"status": "error", "message": "'guild_id' must be set in config.yaml"}`
		}
		logger.Info("LLM requested Discord channel list", "guild", guildID)
		channels, err := tools.DiscordListChannels(guildID, logger)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Channel list failed: %v"}`, err)
		}
		data, _ := json.Marshal(map[string]interface{}{
			"status": "success",
			"count":  len(channels),
			"data":   channels,
		})
		return "Tool Output: " + string(data)

	case "manage_missions":
		if !cfg.Tools.Missions.Enabled {
			return `Tool Output: {"status":"error","message":"Missions are disabled. Set tools.missions.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Missions.ReadOnly {
			switch tc.Operation {
			case "create", "add", "update", "edit", "delete", "remove", "run", "run_now", "execute":
				return `Tool Output: {"status":"error","message":"Missions are in read-only mode. Disable tools.missions.read_only to allow changes."}`
			}
		}
		logger.Info("LLM requested mission management", "op", tc.Operation)
		if missionManager == nil {
			return `Tool Output: {"status": "error", "message": "Mission control storage not available"}`
		}

		switch tc.Operation {
		case "list":
			missions := missionManager.List()
			b, _ := json.Marshal(map[string]interface{}{"status": "success", "data": missions})
			return "Tool Output: " + string(b)

		case "create", "add":
			if tc.Title == "" || tc.Command == "" {
				return `Tool Output: {"status": "error", "message": "'title' (name) and 'command' (prompt) are required for create"}`
			}
			priorityStr := "medium"
			if tc.Priority == 1 {
				priorityStr = "low"
			} else if tc.Priority == 3 {
				priorityStr = "high"
			}
			m := tools.Mission{
				Name:     tc.Title,
				Prompt:   tc.Command,
				Schedule: tc.CronExpr,
				Priority: priorityStr,
				Locked:   tc.Locked,
			}
			err := missionManager.Create(m)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			b, _ := json.Marshal(map[string]interface{}{"status": "success", "message": "Mission created"})
			return "Tool Output: " + string(b)

		case "update", "edit":
			if tc.ID == "" {
				return `Tool Output: {"status": "error", "message": "'id' is required for update"}`
			}
			existing, ok := missionManager.Get(tc.ID)
			if !ok {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Mission %s not found"}`, tc.ID)
			}

			if tc.Title != "" {
				existing.Name = tc.Title
			}
			if tc.Command != "" {
				existing.Prompt = tc.Command
			}
			if tc.CronExpr != "" {
				existing.Schedule = tc.CronExpr
			}
			if tc.Priority > 0 {
				if tc.Priority == 1 {
					existing.Priority = "low"
				} else if tc.Priority == 3 {
					existing.Priority = "high"
				} else {
					existing.Priority = "medium"
				}
			}
			// Only apply lock state changes if the LLM explicitly provides it,
			// though typical struct fields default to false if omitted.
			// Since we want to allow keeping existing state, we should check if it was provided in raw json
			if strings.Contains(tc.RawJSON, `"locked"`) {
				existing.Locked = tc.Locked
			}

			err := missionManager.Update(tc.ID, existing)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Mission updated"}`

		case "delete", "remove":
			if tc.ID == "" {
				return `Tool Output: {"status": "error", "message": "'id' is required for delete"}`
			}
			err := missionManager.Delete(tc.ID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Mission deleted"}`

		case "run", "run_now":
			if tc.ID == "" {
				return `Tool Output: {"status": "error", "message": "'id' is required for run"}`
			}
			err := missionManager.RunNow(tc.ID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Mission scheduled for immediate execution by the background task queue"}`

		default:
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown operation: %s"}`, tc.Operation)
		}

	case "manage_notes", "notes", "todo":
		if !cfg.Tools.Notes.Enabled {
			return `Tool Output: {"status":"error","message":"Notes are disabled. Set tools.notes.enabled=true in config.yaml."}`
		}
		if cfg.Tools.Notes.ReadOnly {
			switch tc.Operation {
			case "add", "update", "toggle", "delete":
				return `Tool Output: {"status":"error","message":"Notes are in read-only mode. Disable tools.notes.read_only to allow changes."}`
			}
		}
		logger.Info("LLM requested notes/todo management", "op", tc.Operation)
		if shortTermMem == nil {
			return `Tool Output: {"status": "error", "message": "Notes storage not available"}`
		}
		switch tc.Operation {
		case "add":
			if tc.Title == "" {
				return `Tool Output: {"status": "error", "message": "'title' is required for add"}`
			}
			id, err := shortTermMem.AddNote(tc.Category, tc.Title, tc.Content, tc.Priority, tc.DueDate)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Note created", "id": %d}`, id)
		case "list":
			notes, err := shortTermMem.ListNotes(tc.Category, tc.Done)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "count": %d, "notes": %s}`, len(notes), memory.FormatNotesJSON(notes))
		case "update":
			if tc.NoteID <= 0 {
				return `Tool Output: {"status": "error", "message": "'note_id' is required for update"}`
			}
			err := shortTermMem.UpdateNote(tc.NoteID, tc.Title, tc.Content, tc.Category, tc.Priority, tc.DueDate)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Note %d updated"}`, tc.NoteID)
		case "toggle":
			if tc.NoteID <= 0 {
				return `Tool Output: {"status": "error", "message": "'note_id' is required for toggle"}`
			}
			newState, err := shortTermMem.ToggleNoteDone(tc.NoteID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "note_id": %d, "done": %t}`, tc.NoteID, newState)
		case "delete":
			if tc.NoteID <= 0 {
				return `Tool Output: {"status": "error", "message": "'note_id' is required for delete"}`
			}
			err := shortTermMem.DeleteNote(tc.NoteID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Note %d deleted"}`, tc.NoteID)
		default:
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown notes operation: %s. Use add, list, update, toggle, or delete"}`, tc.Operation)
		}

	case "analyze_image", "vision":
		if budgetTracker != nil && budgetTracker.IsBlocked("vision") {
			return `Tool Output: {"status": "error", "message": "Vision blocked: daily budget exceeded. Try again tomorrow."}`
		}
		logger.Info("LLM requested image analysis", "file_path", tc.FilePath)
		fpath := tc.FilePath
		if fpath == "" {
			fpath = tc.Path
		}
		if fpath == "" {
			return `Tool Output: {"status": "error", "message": "'file_path' is required for analyze_image"}`
		}
		if strings.Contains(fpath, "..") {
			return `Tool Output: {"status": "error", "message": "path traversal sequences ('..') are not allowed"}`
		}
		prompt := tc.Prompt
		if prompt == "" {
			prompt = "Describe this image in detail. What do you see? If there is text, transcribe it. If there are people, describe their actions."
		}
		result, err := tools.AnalyzeImageWithPrompt(fpath, prompt, cfg)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Vision analysis failed: %v"}`, err)
		}
		return fmt.Sprintf("Tool Output: %s", result)

	case "transcribe_audio", "speech_to_text":
		if budgetTracker != nil && budgetTracker.IsBlocked("stt") {
			return `Tool Output: {"status": "error", "message": "Speech-to-text blocked: daily budget exceeded. Try again tomorrow."}`
		}
		logger.Info("LLM requested audio transcription", "file_path", tc.FilePath)
		fpath := tc.FilePath
		if fpath == "" {
			fpath = tc.Path
		}
		if fpath == "" {
			return `Tool Output: {"status": "error", "message": "'file_path' is required for transcribe_audio"}`
		}
		if strings.Contains(fpath, "..") {
			return `Tool Output: {"status": "error", "message": "path traversal sequences ('..') are not allowed"}`
		}
		result, err := tools.TranscribeAudioFile(fpath, cfg)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Transcription failed: %v"}`, err)
		}
		return fmt.Sprintf("Tool Output: %s", result)

	case "meshcentral":
		if !cfg.MeshCentral.Enabled {
			return `Tool Output: {"status": "error", "message": "MeshCentral integration is not enabled in config.yaml."}`
		}

		logger.Info("LLM requested MeshCentral operation", "op", tc.Operation)

		normalizeMeshCentralOp := func(op string) string {
			switch strings.ToLower(strings.TrimSpace(op)) {
			case "meshes":
				return "list_groups"
			case "nodes":
				return "list_devices"
			case "wakeonlan":
				return "wake"
			default:
				return strings.ToLower(strings.TrimSpace(op))
			}
		}

		op := normalizeMeshCentralOp(tc.Operation)

		if cfg.MeshCentral.ReadOnly {
			switch op {
			case "list_groups", "list_devices":
				// allowed in read-only mode
			default:
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MeshCentral operation '%s' blocked: meshcentral.readonly is enabled."}`, tc.Operation)
			}
		}

		for _, blocked := range cfg.MeshCentral.BlockedOperations {
			if normalizeMeshCentralOp(blocked) == op && op != "" {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MeshCentral operation '%s' blocked by policy (meshcentral.blocked_operations)."}`, tc.Operation)
			}
		}

		// Attempt to resolve password/token from vault if missing
		token := cfg.MeshCentral.LoginToken
		pass := cfg.MeshCentral.Password
		if token == "" {
			vToken, _ := vault.ReadSecret("meshcentral_token")
			if vToken != "" {
				token = vToken
			}
		}
		if pass == "" {
			vPass, _ := vault.ReadSecret("meshcentral_password")
			if vPass != "" {
				pass = vPass
			}
		}
		if pass == "" && token == "" && cfg.MeshCentral.Username != "" {
			return `Tool Output: {"status": "error", "message": "No password or token found. Please set 'meshcentral_password' or 'meshcentral_token' in the vault."}`
		}

		mcClient := meshcentral.NewClient(cfg.MeshCentral.URL, cfg.MeshCentral.Username, pass, token, true)
		if err := mcClient.Connect(); err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to connect to MeshCentral: %v"}`, err)
		}
		defer mcClient.Close()

		switch op {
		case "list_groups":
			meshes, err := mcClient.ListDeviceGroups()
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to list device groups: %v"}`, err)
			}
			b, _ := json.Marshal(meshes)
			return fmt.Sprintf(`Tool Output: {"status": "success", "groups": %s}`, string(b))

		case "list_devices":
			nodes, err := mcClient.ListDevices(tc.MeshID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to list devices: %v"}`, err)
			}
			b, _ := json.Marshal(nodes)
			return fmt.Sprintf(`Tool Output: {"status": "success", "devices": %s}`, string(b))

		case "wake":
			if tc.NodeID == "" {
				return `Tool Output: {"status": "error", "message": "'node_id' is required for wake"}`
			}
			if err := mcClient.WakeOnLan([]string{tc.NodeID}); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to send wake magic packet: %v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Wake-on-LAN packet sent"}`

		case "power_action":
			if tc.NodeID == "" {
				return `Tool Output: {"status": "error", "message": "'node_id' is required for power_action"}`
			}
			if tc.PowerAction < 1 || tc.PowerAction > 4 {
				return `Tool Output: {"status": "error", "message": "Invalid power action. 1=Sleep, 2=Hibernate, 3=PowerOff, 4=Reset"}`
			}
			if err := mcClient.PowerAction([]string{tc.NodeID}, tc.PowerAction); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to send power action: %v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Power action sent"}`

		case "run_command":
			if tc.NodeID == "" || tc.Command == "" {
				return `Tool Output: {"status": "error", "message": "'node_id' and 'command' are required for run_command"}`
			}
			if err := mcClient.RunCommand(tc.NodeID, tc.Command); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to run command: %v"}`, err)
			}
			return `Tool Output: {"status": "success", "message": "Command dispatched to MeshAgent"}`

		default:
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Unknown operation: %s"}`, tc.Operation)
		}

	case "docker", "docker_management":
		if !cfg.Docker.Enabled {
			return `Tool Output: {"status": "error", "message": "Docker integration is not enabled. Set docker.enabled=true in config.yaml."}`
		}
		if cfg.Docker.ReadOnly {
			switch tc.Operation {
			case "start", "stop", "restart", "pause", "unpause", "remove", "rm", "create", "create_container", "run", "pull_image", "pull", "remove_image", "rmi":
				return `Tool Output: {"status":"error","message":"Docker is in read-only mode. Disable docker.read_only to allow changes."}`
			}
		}
		dockerCfg := tools.DockerConfig{Host: cfg.Docker.Host}
		containerID := tc.ContainerID
		if containerID == "" {
			containerID = tc.Name
		}
		switch tc.Operation {
		case "list_containers", "ps":
			logger.Info("LLM requested Docker list_containers", "all", tc.All)
			return "Tool Output: " + tools.DockerListContainers(dockerCfg, tc.All)
		case "inspect", "inspect_container":
			logger.Info("LLM requested Docker inspect", "container_id", containerID)
			return "Tool Output: " + tools.DockerInspectContainer(dockerCfg, containerID)
		case "start":
			logger.Info("LLM requested Docker start", "container_id", containerID)
			return "Tool Output: " + tools.DockerContainerAction(dockerCfg, containerID, "start", false)
		case "stop":
			logger.Info("LLM requested Docker stop", "container_id", containerID)
			return "Tool Output: " + tools.DockerContainerAction(dockerCfg, containerID, "stop", false)
		case "restart":
			logger.Info("LLM requested Docker restart", "container_id", containerID)
			return "Tool Output: " + tools.DockerContainerAction(dockerCfg, containerID, "restart", false)
		case "pause":
			logger.Info("LLM requested Docker pause", "container_id", containerID)
			return "Tool Output: " + tools.DockerContainerAction(dockerCfg, containerID, "pause", false)
		case "unpause":
			logger.Info("LLM requested Docker unpause", "container_id", containerID)
			return "Tool Output: " + tools.DockerContainerAction(dockerCfg, containerID, "unpause", false)
		case "remove", "rm":
			logger.Info("LLM requested Docker remove", "container_id", containerID, "force", tc.Force)
			return "Tool Output: " + tools.DockerContainerAction(dockerCfg, containerID, "remove", tc.Force)
		case "logs":
			logger.Info("LLM requested Docker logs", "container_id", containerID, "tail", tc.Tail)
			return "Tool Output: " + tools.DockerContainerLogs(dockerCfg, containerID, tc.Tail)
		case "create", "create_container", "run":
			logger.Info("LLM requested Docker create", "image", tc.Image, "name", tc.Name)
			var cmd []string
			if tc.Command != "" {
				cmd = strings.Fields(tc.Command)
			}
			restart := tc.Restart
			if restart == "" {
				restart = "no"
			}
			result := tools.DockerCreateContainer(dockerCfg, tc.Name, tc.Image, tc.Env, tc.Ports, tc.Volumes, cmd, restart)
			// Auto-start if operation was "run"
			if tc.Operation == "run" {
				var created map[string]interface{}
				if json.Unmarshal([]byte(result), &created) == nil {
					if id, ok := created["id"].(string); ok && id != "" {
						tools.DockerContainerAction(dockerCfg, id, "start", false)
						created["message"] = "Container created and started"
						updated, _ := json.Marshal(created)
						result = string(updated)
					}
				}
			}
			return "Tool Output: " + result
		case "list_images", "images":
			logger.Info("LLM requested Docker list_images")
			return "Tool Output: " + tools.DockerListImages(dockerCfg)
		case "pull_image", "pull":
			logger.Info("LLM requested Docker pull", "image", tc.Image)
			return "Tool Output: " + tools.DockerPullImage(dockerCfg, tc.Image)
		case "remove_image", "rmi":
			logger.Info("LLM requested Docker remove_image", "image", tc.Image, "force", tc.Force)
			return "Tool Output: " + tools.DockerRemoveImage(dockerCfg, tc.Image, tc.Force)
		case "list_networks", "networks":
			logger.Info("LLM requested Docker list_networks")
			return "Tool Output: " + tools.DockerListNetworks(dockerCfg)
		case "list_volumes", "volumes":
			logger.Info("LLM requested Docker list_volumes")
			return "Tool Output: " + tools.DockerListVolumes(dockerCfg)
		case "info", "system_info":
			logger.Info("LLM requested Docker system_info")
			return "Tool Output: " + tools.DockerSystemInfo(dockerCfg)
		case "exec":
			logger.Info("LLM requested Docker exec", "container_id", containerID, "cmd", tc.Command)
			return "Tool Output: " + tools.DockerExec(dockerCfg, containerID, tc.Command, tc.User)
		case "stats":
			logger.Info("LLM requested Docker stats", "container_id", containerID)
			return "Tool Output: " + tools.DockerStats(dockerCfg, containerID)
		case "top":
			logger.Info("LLM requested Docker top", "container_id", containerID)
			return "Tool Output: " + tools.DockerTop(dockerCfg, containerID)
		case "port":
			logger.Info("LLM requested Docker port", "container_id", containerID)
			return "Tool Output: " + tools.DockerPort(dockerCfg, containerID)
		case "cp", "copy":
			logger.Info("LLM requested Docker cp", "container_id", containerID, "src", tc.Source, "dest", tc.Destination, "direction", tc.Direction)
			return "Tool Output: " + tools.DockerCopy(dockerCfg, containerID, tc.Source, tc.Destination, tc.Direction)
		case "create_network":
			logger.Info("LLM requested Docker create_network", "name", tc.Name, "driver", tc.Driver)
			return "Tool Output: " + tools.DockerCreateNetwork(dockerCfg, tc.Name, tc.Driver)
		case "remove_network":
			logger.Info("LLM requested Docker remove_network", "name", tc.Name)
			return "Tool Output: " + tools.DockerRemoveNetwork(dockerCfg, tc.Name)
		case "connect":
			logger.Info("LLM requested Docker connect", "container_id", containerID, "network", tc.Network)
			return "Tool Output: " + tools.DockerConnectNetwork(dockerCfg, containerID, tc.Network)
		case "disconnect":
			logger.Info("LLM requested Docker disconnect", "container_id", containerID, "network", tc.Network)
			return "Tool Output: " + tools.DockerDisconnectNetwork(dockerCfg, containerID, tc.Network)
		case "create_volume":
			logger.Info("LLM requested Docker create_volume", "name", tc.Name, "driver", tc.Driver)
			return "Tool Output: " + tools.DockerCreateVolume(dockerCfg, tc.Name, tc.Driver)
		case "remove_volume":
			logger.Info("LLM requested Docker remove_volume", "name", tc.Name, "force", tc.Force)
			return "Tool Output: " + tools.DockerRemoveVolume(dockerCfg, tc.Name, tc.Force)
		case "compose":
			logger.Info("LLM requested Docker compose", "file", tc.File, "cmd", tc.Command)
			return "Tool Output: " + tools.DockerCompose(dockerCfg, tc.File, tc.Command)
		default:
			return `Tool Output: {"status": "error", "message": "Unknown docker operation. Use: list_containers, inspect, start, stop, restart, pause, unpause, remove, logs, create, run, list_images, pull, remove_image, list_networks, create_network, remove_network, connect, disconnect, list_volumes, create_volume, remove_volume, exec, stats, top, port, cp, compose, info"}`
		}

	case "homepage", "homepage_tool":
		if !cfg.Docker.Enabled {
			return `Tool Output: {"status": "error", "message": "Homepage tool requires Docker. Set docker.enabled=true in config.yaml."}`
		}
		if !cfg.Homepage.Enabled {
			return `Tool Output: {"status": "error", "message": "Homepage tool is not enabled. Set homepage.enabled=true in config.yaml."}`
		}
		homepageCfg := tools.HomepageConfig{
			DockerHost:      cfg.Docker.Host,
			WorkspacePath:   cfg.Homepage.WorkspacePath,
			WebServerPort:   cfg.Homepage.WebServerPort,
			WebServerDomain: cfg.Homepage.WebServerDomain,
		}
		if homepageCfg.WorkspacePath == "" {
			homepageCfg.WorkspacePath = filepath.Join(cfg.Directories.DataDir, "homepage")
		}
		deployCfg := tools.HomepageDeployConfig{
			Host:     cfg.Homepage.DeployHost,
			Port:     cfg.Homepage.DeployPort,
			User:     cfg.Homepage.DeployUser,
			Password: cfg.Homepage.DeployPassword,
			Key:      cfg.Homepage.DeployKey,
			Path:     cfg.Homepage.DeployPath,
			Method:   cfg.Homepage.DeployMethod,
		}

		// Permission checks for restricted operations
		switch tc.Operation {
		case "deploy", "test_connection":
			if !cfg.Homepage.AllowDeploy {
				return `Tool Output: {"status":"error","message":"Deployment is disabled. Enable homepage.allow_deploy in config."}`
			}
		case "init", "start", "stop", "rebuild", "destroy", "webserver_start", "webserver_stop":
			if !cfg.Homepage.AllowContainerManagement {
				return `Tool Output: {"status":"error","message":"Container management is disabled. Enable homepage.allow_container_management in config."}`
			}
		}

		switch tc.Operation {
		case "init":
			logger.Info("LLM requested homepage init")
			return "Tool Output: " + tools.HomepageInit(homepageCfg, logger)
		case "start":
			logger.Info("LLM requested homepage start")
			return "Tool Output: " + tools.HomepageStart(homepageCfg, logger)
		case "stop":
			logger.Info("LLM requested homepage stop")
			return "Tool Output: " + tools.HomepageStop(homepageCfg, logger)
		case "status":
			logger.Info("LLM requested homepage status")
			return "Tool Output: " + tools.HomepageStatus(homepageCfg, logger)
		case "rebuild":
			logger.Info("LLM requested homepage rebuild")
			return "Tool Output: " + tools.HomepageRebuild(homepageCfg, logger)
		case "destroy":
			logger.Info("LLM requested homepage destroy")
			return "Tool Output: " + tools.HomepageDestroy(homepageCfg, logger)
		case "exec":
			logger.Info("LLM requested homepage exec", "cmd", tc.Command)
			return "Tool Output: " + tools.HomepageExec(homepageCfg, tc.Command, logger)
		case "init_project":
			logger.Info("LLM requested homepage init_project", "framework", tc.Framework, "name", tc.Name)
			return "Tool Output: " + tools.HomepageInitProject(homepageCfg, tc.Framework, tc.Name, logger)
		case "build":
			logger.Info("LLM requested homepage build", "dir", tc.ProjectDir)
			return "Tool Output: " + tools.HomepageBuild(homepageCfg, tc.ProjectDir, logger)
		case "install_deps":
			logger.Info("LLM requested homepage install_deps", "packages", tc.Packages)
			return "Tool Output: " + tools.HomepageInstallDeps(homepageCfg, tc.ProjectDir, tc.Packages, logger)
		case "lighthouse":
			logger.Info("LLM requested homepage lighthouse", "url", tc.URL)
			return "Tool Output: " + tools.HomepageLighthouse(homepageCfg, tc.URL, logger)
		case "screenshot":
			logger.Info("LLM requested homepage screenshot", "url", tc.URL, "viewport", tc.Viewport)
			return "Tool Output: " + tools.HomepageScreenshot(homepageCfg, tc.URL, tc.Viewport, logger)
		case "lint":
			logger.Info("LLM requested homepage lint", "dir", tc.ProjectDir)
			return "Tool Output: " + tools.HomepageLint(homepageCfg, tc.ProjectDir, logger)
		case "list_files":
			logger.Info("LLM requested homepage list_files", "path", tc.Path)
			return "Tool Output: " + tools.HomepageListFiles(homepageCfg, tc.Path, logger)
		case "read_file":
			logger.Info("LLM requested homepage read_file", "path", tc.Path)
			return "Tool Output: " + tools.HomepageReadFile(homepageCfg, tc.Path, logger)
		case "write_file":
			logger.Info("LLM requested homepage write_file", "path", tc.Path)
			return "Tool Output: " + tools.HomepageWriteFile(homepageCfg, tc.Path, tc.Content, logger)
		case "optimize_images":
			logger.Info("LLM requested homepage optimize_images", "dir", tc.ProjectDir)
			return "Tool Output: " + tools.HomepageOptimizeImages(homepageCfg, tc.ProjectDir, logger)
		case "dev":
			logger.Info("LLM requested homepage dev server", "dir", tc.ProjectDir)
			return "Tool Output: " + tools.HomepageDev(homepageCfg, tc.ProjectDir, 3000, logger)
		case "deploy":
			logger.Info("LLM requested homepage deploy", "host", deployCfg.Host)
			return "Tool Output: " + tools.HomepageDeploy(homepageCfg, deployCfg, tc.ProjectDir, tc.BuildDir, logger)
		case "test_connection":
			logger.Info("LLM requested homepage test_connection")
			return "Tool Output: " + tools.HomepageTestConnection(deployCfg, logger)
		case "webserver_start":
			logger.Info("LLM requested homepage webserver_start")
			return "Tool Output: " + tools.HomepageWebServerStart(homepageCfg, tc.ProjectDir, tc.BuildDir, logger)
		case "webserver_stop":
			logger.Info("LLM requested homepage webserver_stop")
			return "Tool Output: " + tools.HomepageWebServerStop(homepageCfg, logger)
		case "webserver_status":
			logger.Info("LLM requested homepage webserver_status")
			return "Tool Output: " + tools.HomepageWebServerStatus(homepageCfg, logger)
		case "publish_local":
			logger.Info("LLM requested homepage publish_local")
			return "Tool Output: " + tools.HomepagePublishToLocal(homepageCfg, tc.ProjectDir, logger)
		default:
			return `Tool Output: {"status":"error","message":"Unknown homepage operation. Use: init, start, stop, status, rebuild, destroy, exec, init_project, build, install_deps, lighthouse, screenshot, lint, list_files, read_file, write_file, optimize_images, dev, deploy, test_connection, webserver_start, webserver_stop, webserver_status, publish_local"}`
		}

	case "webdav", "webdav_storage":
		if !cfg.WebDAV.Enabled {
			return `Tool Output: {"status": "error", "message": "WebDAV integration is not enabled. Set webdav.enabled=true in config.yaml."}`
		}
		if cfg.WebDAV.ReadOnly {
			switch tc.Operation {
			case "write", "put", "upload", "mkdir", "create_dir", "delete", "rm", "move", "rename", "mv":
				return `Tool Output: {"status":"error","message":"WebDAV is in read-only mode. Disable webdav.read_only to allow changes."}`
			}
		}
		davCfg := tools.WebDAVConfig{
			URL:      cfg.WebDAV.URL,
			Username: cfg.WebDAV.Username,
			Password: cfg.WebDAV.Password,
		}
		path := tc.Path
		if path == "" {
			path = tc.RemotePath
		}
		if path == "" {
			path = tc.FilePath
		}
		switch tc.Operation {
		case "list", "ls":
			logger.Info("LLM requested WebDAV list", "path", path)
			return "Tool Output: " + tools.WebDAVList(davCfg, path)
		case "read", "get", "download":
			logger.Info("LLM requested WebDAV read", "path", path)
			return "Tool Output: " + tools.WebDAVRead(davCfg, path)
		case "write", "put", "upload":
			logger.Info("LLM requested WebDAV write", "path", path)
			content := tc.Content
			if content == "" {
				content = tc.Body
			}
			return "Tool Output: " + tools.WebDAVWrite(davCfg, path, content)
		case "mkdir", "create_dir":
			logger.Info("LLM requested WebDAV mkdir", "path", path)
			return "Tool Output: " + tools.WebDAVMkdir(davCfg, path)
		case "delete", "rm":
			logger.Info("LLM requested WebDAV delete", "path", path)
			return "Tool Output: " + tools.WebDAVDelete(davCfg, path)
		case "move", "rename", "mv":
			logger.Info("LLM requested WebDAV move", "path", path, "destination", tc.Destination)
			dst := tc.Destination
			if dst == "" {
				dst = tc.Dest
			}
			return "Tool Output: " + tools.WebDAVMove(davCfg, path, dst)
		case "info", "stat":
			logger.Info("LLM requested WebDAV info", "path", path)
			return "Tool Output: " + tools.WebDAVInfo(davCfg, path)
		default:
			return `Tool Output: {"status": "error", "message": "Unknown webdav operation. Use: list, read, write, mkdir, delete, move, info"}`
		}

	case "home_assistant", "homeassistant", "ha":
		if !cfg.HomeAssistant.Enabled {
			return `Tool Output: {"status": "error", "message": "Home Assistant integration is not enabled. Set home_assistant.enabled=true in config.yaml."}`
		}
		if cfg.HomeAssistant.ReadOnly {
			switch tc.Operation {
			case "call_service", "service":
				return `Tool Output: {"status":"error","message":"Home Assistant is in read-only mode. Disable home_assistant.read_only to allow changes."}`
			}
		}
		haCfg := tools.HAConfig{
			URL:         cfg.HomeAssistant.URL,
			AccessToken: cfg.HomeAssistant.AccessToken,
		}
		// Merge service_data from Params if ServiceData is nil
		serviceData := tc.ServiceData
		if serviceData == nil && tc.Params != nil {
			if sd, ok := tc.Params["service_data"].(map[string]interface{}); ok {
				serviceData = sd
			}
		}
		switch tc.Operation {
		case "get_states", "list_states", "states":
			logger.Info("LLM requested HA get_states", "domain", tc.Domain)
			return "Tool Output: " + tools.HAGetStates(haCfg, tc.Domain)
		case "get_state", "state":
			logger.Info("LLM requested HA get_state", "entity_id", tc.EntityID)
			return "Tool Output: " + tools.HAGetState(haCfg, tc.EntityID)
		case "call_service", "service":
			logger.Info("LLM requested HA call_service", "domain", tc.Domain, "service", tc.Service, "entity_id", tc.EntityID)
			return "Tool Output: " + tools.HACallService(haCfg, tc.Domain, tc.Service, tc.EntityID, serviceData)
		case "list_services", "services":
			logger.Info("LLM requested HA list_services", "domain", tc.Domain)
			return "Tool Output: " + tools.HAListServices(haCfg, tc.Domain)
		default:
			return `Tool Output: {"status": "error", "message": "Unknown home_assistant operation. Use: get_states, get_state, call_service, list_services"}`
		}

	case "co_agent", "co_agents":
		if budgetTracker != nil && budgetTracker.IsBlocked("coagent") {
			return `Tool Output: {"status": "error", "message": "Co-Agent spawn blocked: daily budget exceeded. Try again tomorrow."}`
		}
		if !cfg.CoAgents.Enabled {
			return `Tool Output: {"status": "error", "message": "Co-Agent system is not enabled. Set co_agents.enabled=true in config.yaml."}`
		}
		if coAgentRegistry == nil {
			return `Tool Output: {"status": "error", "message": "Co-Agent registry not initialized."}`
		}
		switch tc.Operation {
		case "spawn", "start", "create":
			task := tc.Task
			if task == "" {
				task = tc.Content
			}
			if task == "" {
				return `Tool Output: {"status": "error", "message": "'task' is required to spawn a co-agent."}`
			}
			coReq := CoAgentRequest{
				Task:         task,
				ContextHints: tc.ContextHints,
			}
			id, err := SpawnCoAgent(cfg, ctx, logger, coAgentRegistry,
				shortTermMem, longTermMem, vault, registry, manifest, kg, inventoryDB, coReq, budgetTracker)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			slots := coAgentRegistry.AvailableSlots()
			return fmt.Sprintf(`Tool Output: {"status": "ok", "co_agent_id": "%s", "available_slots": %d, "message": "Co-Agent started. Use operation 'list' to check status and 'get_result' when completed."}`, id, slots)

		case "list", "status":
			list := coAgentRegistry.List()
			data, _ := json.Marshal(map[string]interface{}{
				"status":          "ok",
				"available_slots": coAgentRegistry.AvailableSlots(),
				"max_slots":       cfg.CoAgents.MaxConcurrent,
				"co_agents":       list,
			})
			return "Tool Output: " + string(data)

		case "get_result", "result":
			coID := tc.CoAgentID
			if coID == "" {
				coID = tc.ID
			}
			if coID == "" {
				return `Tool Output: {"status": "error", "message": "'co_agent_id' is required."}`
			}
			result, err := coAgentRegistry.GetResult(coID)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			out, _ := json.Marshal(map[string]interface{}{
				"status":      "ok",
				"co_agent_id": coID,
				"result":      result,
			})
			return "Tool Output: " + string(out)

		case "stop", "cancel", "kill":
			coID := tc.CoAgentID
			if coID == "" {
				coID = tc.ID
			}
			if coID == "" {
				return `Tool Output: {"status": "error", "message": "'co_agent_id' is required."}`
			}
			if err := coAgentRegistry.Stop(coID); err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "ok", "message": "Co-Agent '%s' stopped."}`, coID)

		case "stop_all", "cancel_all":
			n := coAgentRegistry.StopAll()
			return fmt.Sprintf(`Tool Output: {"status": "ok", "message": "Stopped %d co-agent(s)."}`, n)

		default:
			return `Tool Output: {"status": "error", "message": "Unknown co_agent operation. Use: spawn, list, get_result, stop, stop_all"}`
		}

	case "mdns_scan":
		logger.Info("LLM requested mdns_scan", "service_type", tc.ServiceType, "timeout", tc.Timeout)
		return "Tool Output: " + tools.MDNSScan(logger, tc.ServiceType, tc.Timeout)

	case "tts":
		if !cfg.Chromecast.Enabled && cfg.TTS.Provider == "" {
			return `Tool Output: {"status": "error", "message": "TTS is not configured. Set tts.provider in config.yaml."}`
		}
		text := tc.Text
		if text == "" {
			text = tc.Content
		}
		ttsCfg := tools.TTSConfig{
			Provider: cfg.TTS.Provider,
			Language: tc.Language,
			DataDir:  cfg.Directories.DataDir,
		}
		if ttsCfg.Language == "" {
			ttsCfg.Language = cfg.TTS.Language
		}
		ttsCfg.ElevenLabs.APIKey = cfg.TTS.ElevenLabs.APIKey
		ttsCfg.ElevenLabs.VoiceID = cfg.TTS.ElevenLabs.VoiceID
		ttsCfg.ElevenLabs.ModelID = cfg.TTS.ElevenLabs.ModelID
		filename, err := tools.TTSSynthesize(ttsCfg, text)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "TTS failed: %v"}`, err)
		}
		ttsPort := cfg.Chromecast.TTSPort
		if ttsPort == 0 {
			ttsPort = cfg.Server.Port // Fallback if chromecast integration is disabled
		}
		audioURL := fmt.Sprintf("http://%s:%d/tts/%s", getLocalIP(cfg), ttsPort, filename)
		return fmt.Sprintf(`Tool Output: {"status": "success", "file": "%s", "url": "%s"}`, filename, audioURL)

	case "chromecast":
		if !cfg.Chromecast.Enabled {
			return `Tool Output: {"status": "error", "message": "Chromecast is disabled. Set chromecast.enabled=true in config.yaml."}`
		}
		// Resolve device_name → device_addr via inventory if device_addr is empty
		if tc.DeviceAddr == "" && tc.DeviceName != "" && inventoryDB != nil {
			devices, err := inventory.QueryDevices(inventoryDB, "", "chromecast", tc.DeviceName)
			if err == nil && len(devices) > 0 {
				tc.DeviceAddr = devices[0].IPAddress
				if tc.DevicePort == 0 && devices[0].Port > 0 {
					tc.DevicePort = devices[0].Port
				}
				logger.Info("Resolved chromecast device name", "name", tc.DeviceName, "addr", tc.DeviceAddr, "port", tc.DevicePort)
			} else {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Could not find chromecast device named '%s' in the device registry."}`, tc.DeviceName)
			}
		}
		op := tc.Operation
		switch op {
		case "discover":
			return "Tool Output: " + tools.ChromecastDiscover(logger)
		case "play":
			url := tc.URL
			ct := tc.ContentType
			return "Tool Output: " + tools.ChromecastPlay(tc.DeviceAddr, tc.DevicePort, url, ct, logger)
		case "speak":
			text := tc.Text
			if text == "" {
				text = tc.Content
			}
			ttsCfg := tools.TTSConfig{
				Provider: cfg.TTS.Provider,
				Language: tc.Language,
				DataDir:  cfg.Directories.DataDir,
			}
			if ttsCfg.Language == "" {
				ttsCfg.Language = cfg.TTS.Language
			}
			ttsCfg.ElevenLabs.APIKey = cfg.TTS.ElevenLabs.APIKey
			ttsCfg.ElevenLabs.VoiceID = cfg.TTS.ElevenLabs.VoiceID
			ttsCfg.ElevenLabs.ModelID = cfg.TTS.ElevenLabs.ModelID
			ccCfg := tools.ChromecastConfig{
				ServerHost: cfg.Server.Host,
				ServerPort: cfg.Chromecast.TTSPort,
			}
			return "Tool Output: " + tools.ChromecastSpeak(tc.DeviceAddr, tc.DevicePort, text, ttsCfg, ccCfg, logger)
		case "stop":
			return "Tool Output: " + tools.ChromecastStop(tc.DeviceAddr, tc.DevicePort, logger)
		case "volume":
			return "Tool Output: " + tools.ChromecastVolume(tc.DeviceAddr, tc.DevicePort, tc.Volume, logger)
		case "status":
			return "Tool Output: " + tools.ChromecastStatus(tc.DeviceAddr, tc.DevicePort, logger)
		default:
			return `Tool Output: {"status": "error", "message": "Unknown chromecast operation. Use: discover, play, speak, stop, volume, status"}`
		}

	case "manage_webhooks":
		if !cfg.Webhooks.Enabled {
			return `Tool Output: {"status":"error","message":"Webhooks are not enabled. Set webhooks.enabled: true in config."}`
		}
		if cfg.Webhooks.ReadOnly {
			switch tc.Operation {
			case "create", "update", "delete":
				return `Tool Output: {"status":"error","message":"Webhooks are in read-only mode. Disable webhooks.read_only to allow changes."}`
			}
		}
		whFilePath := filepath.Join(cfg.Directories.DataDir, "webhooks.json")
		whLogPath := filepath.Join(cfg.Directories.DataDir, "webhook_log.json")
		whMgr, whErr := webhooks.NewManager(whFilePath, whLogPath)
		if whErr != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to load webhook manager: %s"}`, whErr)
		}
		return handleWebhookToolCall(tc, whMgr, logger)

	case "proxmox", "proxmox_ve":
		if !cfg.Proxmox.Enabled {
			return `Tool Output: {"status":"error","message":"Proxmox integration is not enabled. Set proxmox.enabled=true in config.yaml."}`
		}
		if cfg.Proxmox.ReadOnly {
			switch tc.Operation {
			case "start", "stop", "shutdown", "reboot", "suspend", "resume", "reset", "create_snapshot", "snapshot":
				return `Tool Output: {"status":"error","message":"Proxmox is in read-only mode. Disable proxmox.read_only to allow changes."}`
			}
		}
		pxCfg := tools.ProxmoxConfig{
			URL:      cfg.Proxmox.URL,
			TokenID:  cfg.Proxmox.TokenID,
			Secret:   cfg.Proxmox.Secret,
			Node:     cfg.Proxmox.Node,
			Insecure: cfg.Proxmox.Insecure,
		}
		node := tc.Hostname
		if node == "" {
			node = tc.Name
		}
		vmid := tc.VMID
		if vmid == "" {
			vmid = tc.ID
		}
		vmType := tc.VMType
		switch tc.Operation {
		case "list_nodes":
			logger.Info("LLM requested Proxmox list_nodes")
			return "Tool Output: " + tools.ProxmoxListNodes(pxCfg)
		case "list_vms", "vms":
			logger.Info("LLM requested Proxmox list_vms", "node", node)
			return "Tool Output: " + tools.ProxmoxListVMs(pxCfg, node)
		case "list_containers", "lxc":
			logger.Info("LLM requested Proxmox list_containers", "node", node)
			return "Tool Output: " + tools.ProxmoxListContainers(pxCfg, node)
		case "status":
			logger.Info("LLM requested Proxmox status", "vmid", vmid, "type", vmType)
			return "Tool Output: " + tools.ProxmoxGetStatus(pxCfg, node, vmType, vmid)
		case "start", "stop", "shutdown", "reboot", "suspend", "resume", "reset":
			logger.Info("LLM requested Proxmox action", "action", tc.Operation, "vmid", vmid)
			return "Tool Output: " + tools.ProxmoxVMAction(pxCfg, node, vmType, vmid, tc.Operation)
		case "node_status":
			logger.Info("LLM requested Proxmox node_status", "node", node)
			return "Tool Output: " + tools.ProxmoxNodeStatus(pxCfg, node)
		case "cluster_resources", "resources":
			resType := tc.ResourceType
			logger.Info("LLM requested Proxmox cluster_resources", "type", resType)
			return "Tool Output: " + tools.ProxmoxClusterResources(pxCfg, resType)
		case "storage":
			logger.Info("LLM requested Proxmox storage", "node", node)
			return "Tool Output: " + tools.ProxmoxGetStorage(pxCfg, node)
		case "create_snapshot", "snapshot":
			logger.Info("LLM requested Proxmox create_snapshot", "vmid", vmid, "name", tc.Name)
			return "Tool Output: " + tools.ProxmoxCreateSnapshot(pxCfg, node, vmType, vmid, tc.Name, tc.Description)
		case "list_snapshots", "snapshots":
			logger.Info("LLM requested Proxmox list_snapshots", "vmid", vmid)
			return "Tool Output: " + tools.ProxmoxListSnapshots(pxCfg, node, vmType, vmid)
		case "task_log":
			upid := tc.UPID
			if upid == "" {
				upid = tc.ID
			}
			logger.Info("LLM requested Proxmox task_log", "upid", upid)
			return "Tool Output: " + tools.ProxmoxGetTaskLog(pxCfg, node, upid)
		default:
			return `Tool Output: {"status":"error","message":"Unknown proxmox operation. Use: list_nodes, list_vms, list_containers, status, start, stop, shutdown, reboot, node_status, cluster_resources, storage, create_snapshot, list_snapshots, task_log"}`
		}

	case "ollama", "ollama_management":
		if !cfg.Ollama.Enabled {
			return `Tool Output: {"status":"error","message":"Ollama integration is not enabled. Set ollama.enabled=true in config.yaml."}`
		}
		if cfg.Ollama.ReadOnly {
			switch tc.Operation {
			case "pull", "download", "delete", "remove", "copy", "load", "unload":
				return `Tool Output: {"status":"error","message":"Ollama is in read-only mode. Disable ollama.read_only to allow changes."}`
			}
		}
		olCfg := tools.OllamaConfig{URL: cfg.Ollama.URL}
		modelName := tc.Model
		if modelName == "" {
			modelName = tc.Name
		}
		switch tc.Operation {
		case "list", "list_models":
			logger.Info("LLM requested Ollama list models")
			return "Tool Output: " + tools.OllamaListModels(olCfg)
		case "running", "ps":
			logger.Info("LLM requested Ollama list running")
			return "Tool Output: " + tools.OllamaListRunning(olCfg)
		case "show", "info":
			logger.Info("LLM requested Ollama show model", "model", modelName)
			return "Tool Output: " + tools.OllamaShowModel(olCfg, modelName)
		case "pull", "download":
			logger.Info("LLM requested Ollama pull model", "model", modelName)
			return "Tool Output: " + tools.OllamaPullModel(olCfg, modelName)
		case "delete", "remove":
			logger.Info("LLM requested Ollama delete model", "model", modelName)
			return "Tool Output: " + tools.OllamaDeleteModel(olCfg, modelName)
		case "copy":
			src := tc.Source
			dst := tc.Destination
			if dst == "" {
				dst = tc.Dest
			}
			logger.Info("LLM requested Ollama copy model", "source", src, "destination", dst)
			return "Tool Output: " + tools.OllamaCopyModel(olCfg, src, dst)
		case "load":
			logger.Info("LLM requested Ollama load model", "model", modelName)
			return "Tool Output: " + tools.OllamaLoadModel(olCfg, modelName)
		case "unload":
			logger.Info("LLM requested Ollama unload model", "model", modelName)
			return "Tool Output: " + tools.OllamaUnloadModel(olCfg, modelName)
		default:
			return `Tool Output: {"status":"error","message":"Unknown ollama operation. Use: list, running, show, pull, delete, copy, load, unload"}`
		}

	case "tailscale":
		if !cfg.Tailscale.Enabled {
			return `Tool Output: {"status":"error","message":"Tailscale integration is not enabled. Set tailscale.enabled=true in config.yaml."}`
		}
		if cfg.Tailscale.ReadOnly {
			switch tc.Operation {
			case "enable_routes", "disable_routes":
				return `Tool Output: {"status":"error","message":"Tailscale is in read-only mode. Disable tailscale.read_only to allow changes."}`
			}
		}
		tsCfg := tools.TailscaleConfig{APIKey: cfg.Tailscale.APIKey, Tailnet: cfg.Tailscale.Tailnet}
		// query is hostname, IP, or node ID for device-specific operations
		query := tc.Query
		if query == "" {
			query = tc.Hostname
		}
		if query == "" {
			query = tc.ID
		}
		if query == "" {
			query = tc.Name
		}
		switch tc.Operation {
		case "devices", "list", "list_devices":
			logger.Info("LLM requested Tailscale list devices")
			return "Tool Output: " + tools.TailscaleListDevices(tsCfg)
		case "device", "get", "get_device":
			logger.Info("LLM requested Tailscale get device", "query", query)
			return "Tool Output: " + tools.TailscaleGetDevice(tsCfg, query)
		case "routes", "get_routes":
			logger.Info("LLM requested Tailscale get routes", "query", query)
			return "Tool Output: " + tools.TailscaleGetRoutes(tsCfg, query)
		case "enable_routes":
			routes := splitCSV(tc.Value)
			logger.Info("LLM requested Tailscale enable routes", "query", query, "routes", routes)
			return "Tool Output: " + tools.TailscaleSetRoutes(tsCfg, query, routes, true)
		case "disable_routes":
			routes := splitCSV(tc.Value)
			logger.Info("LLM requested Tailscale disable routes", "query", query, "routes", routes)
			return "Tool Output: " + tools.TailscaleSetRoutes(tsCfg, query, routes, false)
		case "dns", "get_dns":
			logger.Info("LLM requested Tailscale DNS config")
			return "Tool Output: " + tools.TailscaleGetDNS(tsCfg)
		case "acl", "get_acl":
			logger.Info("LLM requested Tailscale ACL policy")
			return "Tool Output: " + tools.TailscaleGetACL(tsCfg)
		case "local_status", "status":
			logger.Info("LLM requested Tailscale local status")
			return "Tool Output: " + tools.TailscaleLocalStatus()
		default:
			return `Tool Output: {"status":"error","message":"Unknown tailscale operation. Use: devices, device, routes, enable_routes, disable_routes, dns, acl, local_status"}`
		}

	case "ansible":
		if !cfg.Ansible.Enabled {
			return `Tool Output: {"status":"error","message":"Ansible integration is not enabled. Set ansible.enabled=true in config.yaml."}`
		}
		if cfg.Ansible.ReadOnly {
			switch tc.Operation {
			case "adhoc", "command", "run_module", "playbook", "run", "run_playbook":
				return `Tool Output: {"status":"error","message":"Ansible is in read-only mode. Disable ansible.read_only to allow changes."}`
			}
		}
		// Resolve host pattern (hosts for ad-hoc / limit for playbooks)
		hosts := tc.Hostname
		if hosts == "" {
			hosts = tc.HostLimit
		}
		if hosts == "" {
			hosts = tc.Query
		}
		inventoryPath := tc.Inventory
		// Parse extra_vars from tc.Body (JSON string → map)
		var extraVars map[string]interface{}
		if tc.Body != "" {
			_ = json.Unmarshal([]byte(tc.Body), &extraVars)
		}

		isLocal := cfg.Ansible.Mode == "local"

		if isLocal {
			// ── Local CLI mode ──────────────────────────────────────────────────────
			localCfg := tools.AnsibleLocalConfig{
				PlaybooksDir:     cfg.Ansible.PlaybooksDir,
				DefaultInventory: cfg.Ansible.DefaultInventory,
				Timeout:          cfg.Ansible.Timeout,
			}
			switch tc.Operation {
			case "status", "health":
				logger.Info("LLM requested Ansible status (local)")
				return "Tool Output: " + tools.AnsibleLocalStatus(localCfg)
			case "list_playbooks", "playbooks":
				logger.Info("LLM requested Ansible list playbooks (local)")
				return "Tool Output: " + tools.AnsibleLocalListPlaybooks(localCfg)
			case "inventory", "list_inventory":
				logger.Info("LLM requested Ansible inventory (local)", "path", inventoryPath)
				return "Tool Output: " + tools.AnsibleLocalListInventory(localCfg, inventoryPath)
			case "ping":
				logger.Info("LLM requested Ansible ping (local)", "hosts", hosts)
				return "Tool Output: " + tools.AnsibleLocalPing(localCfg, hosts, inventoryPath)
			case "adhoc", "command", "run_module":
				module := tc.Module
				if module == "" {
					module = tc.Package
				}
				moduleArgs := tc.Command
				logger.Info("LLM requested Ansible adhoc (local)", "hosts", hosts, "module", module)
				return "Tool Output: " + tools.AnsibleLocalAdhoc(localCfg, hosts, module, moduleArgs, inventoryPath, extraVars)
			case "playbook", "run", "run_playbook":
				playbook := tc.Name
				if playbook == "" {
					return `Tool Output: {"status":"error","message":"'name' (playbook filename) is required for operation=playbook"}`
				}
				logger.Info("LLM requested Ansible playbook (local)", "playbook", playbook, "limit", tc.HostLimit)
				return "Tool Output: " + tools.AnsibleLocalRunPlaybook(localCfg, playbook, inventoryPath, tc.HostLimit, tc.Tags, tc.SkipTags, extraVars, tc.Preview, false)
			case "check", "dry_run":
				playbook := tc.Name
				if playbook == "" {
					return `Tool Output: {"status":"error","message":"'name' (playbook filename) is required for operation=check"}`
				}
				logger.Info("LLM requested Ansible playbook dry-run (local)", "playbook", playbook)
				return "Tool Output: " + tools.AnsibleLocalRunPlaybook(localCfg, playbook, inventoryPath, tc.HostLimit, tc.Tags, tc.SkipTags, extraVars, true, true)
			case "facts", "gather_facts":
				logger.Info("LLM requested Ansible gather facts (local)", "hosts", hosts)
				return "Tool Output: " + tools.AnsibleLocalGatherFacts(localCfg, hosts, inventoryPath)
			default:
				return `Tool Output: {"status":"error","message":"Unknown ansible operation. Use: status, list_playbooks, inventory, ping, adhoc, playbook, check, facts"}`
			}
		}

		// ── Sidecar mode (default) ──────────────────────────────────────────────
		ansCfg := tools.AnsibleConfig{
			URL:     cfg.Ansible.URL,
			Token:   cfg.Ansible.Token,
			Timeout: cfg.Ansible.Timeout,
		}
		switch tc.Operation {
		case "status", "health":
			logger.Info("LLM requested Ansible status")
			return "Tool Output: " + tools.AnsibleStatus(ansCfg)
		case "list_playbooks", "playbooks":
			logger.Info("LLM requested Ansible list playbooks")
			return "Tool Output: " + tools.AnsibleListPlaybooks(ansCfg)
		case "inventory", "list_inventory":
			logger.Info("LLM requested Ansible inventory", "path", inventoryPath)
			return "Tool Output: " + tools.AnsibleListInventory(ansCfg, inventoryPath)
		case "ping":
			logger.Info("LLM requested Ansible ping", "hosts", hosts)
			return "Tool Output: " + tools.AnsiblePing(ansCfg, hosts, inventoryPath)
		case "adhoc", "command", "run_module":
			module := tc.Module
			if module == "" {
				module = tc.Package
			}
			moduleArgs := tc.Command
			logger.Info("LLM requested Ansible adhoc", "hosts", hosts, "module", module)
			return "Tool Output: " + tools.AnsibleAdhoc(ansCfg, hosts, module, moduleArgs, inventoryPath, extraVars)
		case "playbook", "run", "run_playbook":
			playbook := tc.Name
			if playbook == "" {
				return `Tool Output: {"status":"error","message":"'name' (playbook filename) is required for operation=playbook"}`
			}
			logger.Info("LLM requested Ansible playbook", "playbook", playbook, "limit", tc.HostLimit, "check", tc.Preview)
			return "Tool Output: " + tools.AnsibleRunPlaybook(ansCfg, playbook, inventoryPath, tc.HostLimit, tc.Tags, tc.SkipTags, extraVars, tc.Preview, false)
		case "check", "dry_run":
			playbook := tc.Name
			if playbook == "" {
				return `Tool Output: {"status":"error","message":"'name' (playbook filename) is required for operation=check"}`
			}
			logger.Info("LLM requested Ansible playbook dry-run", "playbook", playbook)
			return "Tool Output: " + tools.AnsibleRunPlaybook(ansCfg, playbook, inventoryPath, tc.HostLimit, tc.Tags, tc.SkipTags, extraVars, true, true)
		case "facts", "gather_facts":
			logger.Info("LLM requested Ansible gather facts", "hosts", hosts)
			return "Tool Output: " + tools.AnsibleGatherFacts(ansCfg, hosts, inventoryPath)
		default:
			return `Tool Output: {"status":"error","message":"Unknown ansible operation. Use: status, list_playbooks, inventory, ping, adhoc, playbook, check, facts"}`
		}

	case "invasion_control":
		return handleInvasionControl(tc, cfg, invasionDB, vault, logger)

	case "github":
		if !cfg.GitHub.Enabled {
			return `Tool Output: {"status":"error","message":"GitHub integration is not enabled. Set github.enabled=true in config.yaml."}`
		}
		if cfg.GitHub.ReadOnly {
			switch tc.Operation {
			case "create_repo", "delete_repo", "create_issue", "close_issue", "create_or_update_file", "track_project", "untrack_project":
				return `Tool Output: {"status":"error","message":"GitHub is in read-only mode. Disable github.read_only to allow changes."}`
			}
		}
		token, err := vault.ReadSecret("github_token")
		if err != nil || token == "" {
			return `Tool Output: {"status":"error","message":"GitHub token not found in vault. Store it with key 'github_token' via the vault API."}`
		}
		ghCfg := tools.GitHubConfig{
			Token:          token,
			Owner:          cfg.GitHub.Owner,
			BaseURL:        cfg.GitHub.BaseURL,
			DefaultPrivate: cfg.GitHub.DefaultPrivate,
		}
		owner := tc.Owner
		if owner == "" {
			owner = cfg.GitHub.Owner
		}
		repo := tc.Name
		switch tc.Operation {
		case "list_repos":
			logger.Info("LLM requested GitHub list repos", "owner", owner)
			return "Tool Output: " + tools.GitHubListRepos(ghCfg, owner)
		case "create_repo":
			logger.Info("LLM requested GitHub create repo", "name", repo, "desc", tc.Description)
			return "Tool Output: " + tools.GitHubCreateRepo(ghCfg, repo, tc.Description, nil)
		case "delete_repo":
			logger.Info("LLM requested GitHub delete repo", "owner", owner, "repo", repo)
			return "Tool Output: " + tools.GitHubDeleteRepo(ghCfg, owner, repo)
		case "get_repo":
			logger.Info("LLM requested GitHub get repo", "owner", owner, "repo", repo)
			return "Tool Output: " + tools.GitHubGetRepo(ghCfg, owner, repo)
		case "list_issues":
			state := tc.Value
			if state == "" {
				state = "open"
			}
			logger.Info("LLM requested GitHub list issues", "repo", repo, "state", state)
			return "Tool Output: " + tools.GitHubListIssues(ghCfg, owner, repo, state)
		case "create_issue":
			var labels []string
			if tc.Label != "" {
				labels = splitCSV(tc.Label)
			}
			logger.Info("LLM requested GitHub create issue", "repo", repo, "title", tc.Title)
			return "Tool Output: " + tools.GitHubCreateIssue(ghCfg, owner, repo, tc.Title, tc.Body, labels)
		case "close_issue":
			issueNum := 0
			if tc.ID != "" {
				fmt.Sscanf(tc.ID, "%d", &issueNum)
			}
			logger.Info("LLM requested GitHub close issue", "repo", repo, "number", issueNum)
			return "Tool Output: " + tools.GitHubCloseIssue(ghCfg, owner, repo, issueNum)
		case "list_pull_requests":
			state := tc.Value
			if state == "" {
				state = "open"
			}
			logger.Info("LLM requested GitHub list PRs", "repo", repo, "state", state)
			return "Tool Output: " + tools.GitHubListPullRequests(ghCfg, owner, repo, state)
		case "list_branches":
			logger.Info("LLM requested GitHub list branches", "repo", repo)
			return "Tool Output: " + tools.GitHubListBranches(ghCfg, owner, repo)
		case "get_file":
			branch := tc.Query
			logger.Info("LLM requested GitHub get file", "repo", repo, "path", tc.Path, "branch", branch)
			return "Tool Output: " + tools.GitHubGetFileContent(ghCfg, owner, repo, tc.Path, branch)
		case "create_or_update_file":
			logger.Info("LLM requested GitHub create/update file", "repo", repo, "path", tc.Path)
			return "Tool Output: " + tools.GitHubCreateOrUpdateFile(ghCfg, owner, repo, tc.Path, tc.Content, tc.Body, tc.Value, tc.Query)
		case "list_commits":
			branch := tc.Query
			limit := tc.Limit
			if limit <= 0 {
				limit = 20
			}
			logger.Info("LLM requested GitHub list commits", "repo", repo, "branch", branch)
			return "Tool Output: " + tools.GitHubListCommits(ghCfg, owner, repo, branch, limit)
		case "list_workflow_runs":
			limit := tc.Limit
			if limit <= 0 {
				limit = 10
			}
			logger.Info("LLM requested GitHub list workflow runs", "repo", repo)
			return "Tool Output: " + tools.GitHubListWorkflowRuns(ghCfg, owner, repo, limit)
		case "search_repos":
			limit := tc.Limit
			if limit <= 0 {
				limit = 10
			}
			logger.Info("LLM requested GitHub search repos", "query", tc.Query)
			return "Tool Output: " + tools.GitHubSearchRepos(ghCfg, tc.Query, limit)
		case "list_projects":
			logger.Info("LLM requested GitHub list tracked projects")
			return "Tool Output: " + tools.GitHubListProjects(cfg.Directories.WorkspaceDir)
		case "track_project":
			purpose := tc.Content
			if purpose == "" {
				purpose = tc.Description
			}
			logger.Info("LLM requested GitHub track project", "name", repo, "purpose", purpose)
			return "Tool Output: " + tools.GitHubTrackProject(cfg.Directories.WorkspaceDir, repo, purpose, "", "", owner, cfg.GitHub.DefaultPrivate)
		case "untrack_project":
			logger.Info("LLM requested GitHub untrack project", "name", repo)
			return "Tool Output: " + tools.GitHubUntrackProject(cfg.Directories.WorkspaceDir, repo)
		default:
			return `Tool Output: {"status":"error","message":"Unknown github operation. Use: list_repos, create_repo, delete_repo, get_repo, list_issues, create_issue, close_issue, list_pull_requests, list_branches, get_file, create_or_update_file, list_commits, list_workflow_runs, search_repos, list_projects, track_project, untrack_project"}`
		}

	case "netlify":
		if !cfg.Netlify.Enabled {
			return `Tool Output: {"status":"error","message":"Netlify integration is not enabled. Set netlify.enabled=true in config.yaml."}`
		}
		token, tokenErr := vault.ReadSecret("netlify_token")
		if tokenErr != nil || token == "" {
			return `Tool Output: {"status":"error","message":"Netlify token not found in vault. Store it with key 'netlify_token' via the vault API."}`
		}
		nfCfg := tools.NetlifyConfig{
			Token:         token,
			DefaultSiteID: cfg.Netlify.DefaultSiteID,
			TeamSlug:      cfg.Netlify.TeamSlug,
		}
		// Read-only mode: block all mutating operations
		if cfg.Netlify.ReadOnly {
			switch tc.Operation {
			case "create_site", "update_site", "delete_site",
				"deploy_zip", "deploy_draft", "rollback", "cancel_deploy",
				"set_env", "delete_env",
				"create_hook", "delete_hook",
				"provision_ssl":
				return `Tool Output: {"status":"error","message":"Netlify is in read-only mode. Disable netlify.readonly to allow changes."}`
			}
		}
		// Granular permission checks
		if !cfg.Netlify.AllowDeploy {
			switch tc.Operation {
			case "deploy_zip", "deploy_draft", "rollback", "cancel_deploy":
				return `Tool Output: {"status":"error","message":"Netlify deploy is not allowed. Set netlify.allow_deploy=true in config.yaml."}`
			}
		}
		if !cfg.Netlify.AllowSiteManagement {
			switch tc.Operation {
			case "create_site", "update_site", "delete_site":
				return `Tool Output: {"status":"error","message":"Netlify site management is not allowed. Set netlify.allow_site_management=true in config.yaml."}`
			}
		}
		if !cfg.Netlify.AllowEnvManagement {
			switch tc.Operation {
			case "set_env", "delete_env":
				return `Tool Output: {"status":"error","message":"Netlify env var management is not allowed. Set netlify.allow_env_management=true in config.yaml."}`
			}
		}
		switch tc.Operation {
		// ── Sites ──
		case "list_sites":
			logger.Info("LLM requested Netlify list sites")
			return "Tool Output: " + tools.NetlifyListSites(nfCfg)
		case "get_site":
			logger.Info("LLM requested Netlify get site", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyGetSite(nfCfg, tc.SiteID)
		case "create_site":
			logger.Info("LLM requested Netlify create site", "name", tc.SiteName, "custom_domain", tc.CustomDomain)
			return "Tool Output: " + tools.NetlifyCreateSite(nfCfg, tc.SiteName, tc.CustomDomain)
		case "update_site":
			logger.Info("LLM requested Netlify update site", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyUpdateSite(nfCfg, tc.SiteID, tc.SiteName, tc.CustomDomain)
		case "delete_site":
			logger.Info("LLM requested Netlify delete site", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyDeleteSite(nfCfg, tc.SiteID)
		// ── Deploys ──
		case "list_deploys":
			logger.Info("LLM requested Netlify list deploys", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyListDeploys(nfCfg, tc.SiteID)
		case "get_deploy":
			logger.Info("LLM requested Netlify get deploy", "deploy_id", tc.DeployID)
			return "Tool Output: " + tools.NetlifyGetDeploy(nfCfg, tc.DeployID)
		case "deploy_zip":
			logger.Info("LLM requested Netlify deploy ZIP", "site_id", tc.SiteID, "draft", tc.Draft)
			// ZIP deploy requires building and zipping from the homepage tool first.
			// The agent must pass the ZIP data via tc.Content (base64 encoded).
			if tc.Content == "" {
				return `Tool Output: {"status":"error","message":"content (base64 ZIP) is required for deploy_zip. Build with the homepage tool first, then zip and base64-encode the output."}`
			}
			zipData, decErr := decodeBase64(tc.Content)
			if decErr != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to decode base64 ZIP: %v"}`, decErr)
			}
			return "Tool Output: " + tools.NetlifyDeployZip(nfCfg, tc.SiteID, tc.Title, tc.Draft, zipData)
		case "deploy_draft":
			logger.Info("LLM requested Netlify draft deploy", "site_id", tc.SiteID)
			if tc.Content == "" {
				return `Tool Output: {"status":"error","message":"content (base64 ZIP) is required for deploy_draft."}`
			}
			zipData, decErr := decodeBase64(tc.Content)
			if decErr != nil {
				return fmt.Sprintf(`Tool Output: {"status":"error","message":"Failed to decode base64 ZIP: %v"}`, decErr)
			}
			return "Tool Output: " + tools.NetlifyDeployZip(nfCfg, tc.SiteID, tc.Title, true, zipData)
		case "rollback":
			logger.Info("LLM requested Netlify rollback", "site_id", tc.SiteID, "deploy_id", tc.DeployID)
			return "Tool Output: " + tools.NetlifyRollback(nfCfg, tc.SiteID, tc.DeployID)
		case "cancel_deploy":
			logger.Info("LLM requested Netlify cancel deploy", "deploy_id", tc.DeployID)
			return "Tool Output: " + tools.NetlifyCancelDeploy(nfCfg, tc.DeployID)
		// ── Environment Variables ──
		case "list_env":
			logger.Info("LLM requested Netlify list env vars", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyListEnvVars(nfCfg, tc.SiteID)
		case "get_env":
			logger.Info("LLM requested Netlify get env var", "site_id", tc.SiteID, "key", tc.EnvKey)
			return "Tool Output: " + tools.NetlifyGetEnvVar(nfCfg, tc.SiteID, tc.EnvKey)
		case "set_env":
			logger.Info("LLM requested Netlify set env var", "site_id", tc.SiteID, "key", tc.EnvKey)
			return "Tool Output: " + tools.NetlifySetEnvVar(nfCfg, tc.SiteID, tc.EnvKey, tc.EnvValue, tc.EnvContext)
		case "delete_env":
			logger.Info("LLM requested Netlify delete env var", "site_id", tc.SiteID, "key", tc.EnvKey)
			return "Tool Output: " + tools.NetlifyDeleteEnvVar(nfCfg, tc.SiteID, tc.EnvKey)
		// ── Files ──
		case "list_files":
			logger.Info("LLM requested Netlify list files", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyListFiles(nfCfg, tc.SiteID)
		// ── Forms ──
		case "list_forms":
			logger.Info("LLM requested Netlify list forms", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyListForms(nfCfg, tc.SiteID)
		case "get_submissions":
			logger.Info("LLM requested Netlify get form submissions", "form_id", tc.FormID)
			return "Tool Output: " + tools.NetlifyGetFormSubmissions(nfCfg, tc.FormID)
		// ── Hooks ──
		case "list_hooks":
			logger.Info("LLM requested Netlify list hooks", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyListHooks(nfCfg, tc.SiteID)
		case "create_hook":
			logger.Info("LLM requested Netlify create hook", "site_id", tc.SiteID, "type", tc.HookType, "event", tc.HookEvent)
			hookData := map[string]interface{}{}
			if tc.URL != "" {
				hookData["url"] = tc.URL
			}
			if tc.Value != "" {
				hookData["email"] = tc.Value
			}
			return "Tool Output: " + tools.NetlifyCreateHook(nfCfg, tc.SiteID, tc.HookType, tc.HookEvent, hookData)
		case "delete_hook":
			logger.Info("LLM requested Netlify delete hook", "hook_id", tc.HookID)
			return "Tool Output: " + tools.NetlifyDeleteHook(nfCfg, tc.HookID)
		// ── SSL ──
		case "provision_ssl":
			logger.Info("LLM requested Netlify provision SSL", "site_id", tc.SiteID)
			return "Tool Output: " + tools.NetlifyProvisionSSL(nfCfg, tc.SiteID)
		default:
			return `Tool Output: {"status":"error","message":"Unknown netlify operation. Use: list_sites, get_site, create_site, update_site, delete_site, list_deploys, get_deploy, deploy_zip, deploy_draft, rollback, cancel_deploy, list_env, get_env, set_env, delete_env, list_files, list_forms, get_submissions, list_hooks, create_hook, delete_hook, provision_ssl"}`
		}

	case "mqtt_publish":
		if !cfg.MQTT.Enabled {
			return `Tool Output: {"status": "error", "message": "MQTT is not enabled. Configure the mqtt section in config.yaml."}`
		}
		if cfg.MQTT.ReadOnly {
			return `Tool Output: {"status":"error","message":"MQTT is in read-only mode. Disable mqtt.read_only to allow changes."}`
		}
		topic := tc.Topic
		if topic == "" {
			return `Tool Output: {"status": "error", "message": "'topic' is required"}`
		}
		payload := tc.Payload
		if payload == "" {
			payload = tc.Message
		}
		if payload == "" {
			payload = tc.Content
		}
		qos := tc.QoS
		if qos < 0 || qos > 2 {
			qos = cfg.MQTT.QoS
		}
		logger.Info("LLM requested MQTT publish", "topic", topic, "retain", tc.Retain, "payload_len", len(payload))
		if err := tools.MQTTPublish(topic, payload, qos, tc.Retain, logger); err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MQTT publish failed: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Published to topic '%s'"}`, topic)

	case "mqtt_subscribe":
		if !cfg.MQTT.Enabled {
			return `Tool Output: {"status": "error", "message": "MQTT is not enabled. Configure the mqtt section in config.yaml."}`
		}
		topic := tc.Topic
		if topic == "" {
			return `Tool Output: {"status": "error", "message": "'topic' is required"}`
		}
		qos := tc.QoS
		if qos < 0 || qos > 2 {
			qos = cfg.MQTT.QoS
		}
		logger.Info("LLM requested MQTT subscribe", "topic", topic, "qos", qos)
		if err := tools.MQTTSubscribe(topic, qos, logger); err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MQTT subscribe failed: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Subscribed to topic '%s' with QoS %d"}`, topic, qos)

	case "mqtt_unsubscribe":
		if !cfg.MQTT.Enabled {
			return `Tool Output: {"status": "error", "message": "MQTT is not enabled. Configure the mqtt section in config.yaml."}`
		}
		topic := tc.Topic
		if topic == "" {
			return `Tool Output: {"status": "error", "message": "'topic' is required"}`
		}
		logger.Info("LLM requested MQTT unsubscribe", "topic", topic)
		if err := tools.MQTTUnsubscribe(topic, logger); err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MQTT unsubscribe failed: %v"}`, err)
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Unsubscribed from topic '%s'"}`, topic)

	case "mqtt_get_messages":
		if !cfg.MQTT.Enabled {
			return `Tool Output: {"status": "error", "message": "MQTT is not enabled. Configure the mqtt section in config.yaml."}`
		}
		topic := tc.Topic // empty = all topics
		limit := tc.Limit
		if limit <= 0 {
			limit = 20
		}
		logger.Info("LLM requested MQTT get messages", "topic", topic, "limit", limit)
		msgs, err := tools.MQTTGetMessages(topic, limit, logger)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MQTT get messages failed: %v"}`, err)
		}
		data, _ := json.Marshal(map[string]interface{}{
			"status": "success",
			"count":  len(msgs),
			"data":   msgs,
		})
		return "Tool Output: " + string(data)

	case "mcp_call":
		// Two-gate security: allow_mcp (Danger Zone) AND mcp.enabled
		if !cfg.Agent.AllowMCP {
			return `Tool Output: [PERMISSION DENIED] MCP is disabled in Danger Zone settings (agent.allow_mcp: false).`
		}
		if !cfg.MCP.Enabled {
			return `Tool Output: [PERMISSION DENIED] MCP is disabled (mcp.enabled: false).`
		}

		op := strings.ToLower(strings.TrimSpace(tc.Operation))
		switch op {
		case "list_servers":
			servers, err := tools.MCPListServers(logger)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MCP list servers failed: %v"}`, err)
			}
			data, _ := json.Marshal(map[string]interface{}{"status": "success", "servers": servers})
			return "Tool Output: " + string(data)

		case "list_tools":
			mcpTools, err := tools.MCPListTools(tc.Server, logger)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MCP list tools failed: %v"}`, err)
			}
			data, _ := json.Marshal(map[string]interface{}{"status": "success", "tools": mcpTools})
			return "Tool Output: " + string(data)

		case "call_tool", "call":
			if tc.Server == "" || tc.ToolName == "" {
				return `Tool Output: {"status": "error", "message": "mcp_call with operation=call requires 'server' and 'tool_name'"}`
			}
			args := tc.MCPArgs
			if args == nil {
				args = map[string]interface{}{}
			}
			result, err := tools.MCPCallTool(tc.Server, tc.ToolName, args, logger)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "MCP call failed: %v"}`, err)
			}
			return "Tool Output: " + result

		default:
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "unknown mcp_call operation '%s'. Use list_servers, list_tools, or call_tool."}`, op)
		}

	default:

		logger.Warn("LLM requested unknown action", "action", tc.Action)
		return fmt.Sprintf("Tool Output: ERROR unknown action '%s'", tc.Action)
	}
}

// DispatchToolCall executes the appropriate tool based on the parsed ToolCall.
// It automatically handles Redaction, Guardian sanitization, and ensures the output
// is correctly prefixed with "[Tool Output]\n" unless it's a known error marker.
func DispatchToolCall(ctx context.Context, tc ToolCall, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, missionManager *tools.MissionManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB, historyMgr *memory.HistoryManager, isMaintenance bool, surgeryPlan string, guardian *security.Guardian, sessionID string, coAgentRegistry *CoAgentRegistry, budgetTracker *budget.Tracker) string {

	rawResult := dispatchInner(ctx, tc, cfg, logger, llmClient, vault, registry, manifest, cronManager, missionManager, longTermMem, shortTermMem, kg, inventoryDB, invasionDB, historyMgr, isMaintenance, surgeryPlan, guardian, sessionID, coAgentRegistry, budgetTracker)

	// Apply redaction to tool output
	sanitized := security.RedactSensitiveInfo(rawResult)

	// Guardian: Sanitize tool output (isolation + role-marker stripping)
	if guardian != nil {
		sanitized = guardian.SanitizeToolOutput(tc.Action, sanitized)
	}

	// Make sure errors from execute_python are preserved for context
	if tc.Action == "execute_python" {
		if strings.Contains(sanitized, "[EXECUTION ERROR]") || strings.Contains(sanitized, "TIMEOUT") {
			// handled outside in isErrorState flags if necessary, but we preserve the string here
		}
	}

	// Prefix to clearly identify it as tool output
	if !strings.HasPrefix(sanitized, "[TOOL ") && !strings.HasPrefix(sanitized, "[Tool ") {
		sanitized = "[Tool Output]\n" + sanitized
	}

	return sanitized
}

// getLocalIP returns a LAN-reachable IP address for the TTS audio server.
func getLocalIP(cfg *config.Config) string {
	host := cfg.Server.Host
	if host == "" || host == "127.0.0.1" || host == "0.0.0.0" {
		conn, err := net.Dial("udp", "8.8.8.8:80")
		if err == nil {
			defer conn.Close()
			return conn.LocalAddr().(*net.UDPAddr).IP.String()
		}
		return "127.0.0.1"
	}
	return host
}

// runMemoryOrchestrator handles the Priority-Based Forgetting System across both RAG and Knowledge Graph.
func runMemoryOrchestrator(tc ToolCall, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph) string {
	thresholdLow := tc.ThresholdLow
	if thresholdLow == 0 {
		thresholdLow = 1
	}
	thresholdMedium := tc.ThresholdMedium
	if thresholdMedium == 0 {
		thresholdMedium = 3
	}

	metas, err := shortTermMem.GetAllMemoryMeta()
	if err != nil {
		logger.Error("Failed to fetch memory tracking metadata", "error", err)
		return fmt.Sprintf(`{"status": "error", "message": "Failed to fetch metadata: %v"}`, err)
	}

	highCount, mediumCount, lowCount := 0, 0, 0
	var lowDocs []string
	var mediumDocs []string

	for _, meta := range metas {
		if meta.Protected || meta.KeepForever {
			highCount++
			continue
		}

		lastA, err := time.Parse(time.RFC3339, strings.Replace(meta.LastAccessed, " ", "T", 1)+"Z")
		daysSince := 0
		if err == nil {
			daysSince = int(time.Since(lastA).Hours() / 24)
		}

		priority := meta.AccessCount - daysSince

		if priority < thresholdLow {
			lowCount++
			lowDocs = append(lowDocs, meta.DocID)
		} else if priority < thresholdMedium {
			mediumCount++
			mediumDocs = append(mediumDocs, meta.DocID)
		} else {
			highCount++
		}
	}

	graphRemoved := 0
	if !tc.Preview {
		// 1. Process VectorDB Low Priority
		for _, docID := range lowDocs {
			_ = longTermMem.DeleteDocument(docID)
			_ = shortTermMem.DeleteMemoryMeta(docID)
		}

		// 2. Process VectorDB Medium Priority (Compression)
		for _, docID := range mediumDocs {
			content, err := longTermMem.GetByID(docID)
			if err != nil || len(content) < 300 {
				continue
			}

			// Compress via LLM
			resp, err := llm.ExecuteWithRetry(
				context.Background(),
				client,
				openai.ChatCompletionRequest{
					Model: cfg.LLM.Model,
					Messages: []openai.ChatCompletionMessage{
						{Role: openai.ChatMessageRoleSystem, Content: "You are an AI compressing old memories. Summarize the following RAG memory into a dense, concise bullet-point list containing only core facts. Lose the verbose narrative immediately."},
						{Role: openai.ChatMessageRoleUser, Content: content},
					},
					MaxTokens: 500,
				},
				logger,
				nil,
			)
			if err == nil && len(resp.Choices) > 0 {
				compressed := resp.Choices[0].Message.Content

				parts := strings.SplitN(content, "\n\n", 2)
				concept := "Compressed Memory"
				if len(parts) == 2 {
					concept = parts[0]
				}

				newIDs, err2 := longTermMem.StoreDocument(concept, compressed)
				if err2 == nil {
					_ = longTermMem.DeleteDocument(docID)
					_ = shortTermMem.DeleteMemoryMeta(docID)
					for _, newID := range newIDs {
						_ = shortTermMem.UpsertMemoryMeta(newID)
					}
				}
			}
		}

		// 3. Process Graph Low Priority
		graphRemoved, _ = kg.OptimizeGraph(thresholdLow)
	}

	return fmt.Sprintf(
		`{"status": "success", "preview": %v, "memory_rag": {"high_kept": %d, "medium_compressed": %d, "low_archived": %d}, "graph_nodes_archived": %d}`,
		tc.Preview, highCount, mediumCount, lowCount, graphRemoved,
	)
}

// parseWorkflowPlan extracts tool names from a <workflow_plan>["t1","t2"]</workflow_plan> tag.
// Returns the parsed tool list and the content with the tag removed.
// If no tag is found, returns nil and the original content unchanged.
func parseWorkflowPlan(content string) ([]string, string) {
	const openTag = "<workflow_plan>"
	const closeTag = "</workflow_plan>"

	startIdx := strings.Index(content, openTag)
	if startIdx < 0 {
		return nil, content
	}
	endIdx := strings.Index(content[startIdx:], closeTag)
	if endIdx < 0 {
		return nil, content
	}
	endIdx += startIdx // absolute position

	inner := strings.TrimSpace(content[startIdx+len(openTag) : endIdx])
	if inner == "" {
		return nil, content
	}

	// Parse the JSON array of tool names
	var tools []string
	if err := json.Unmarshal([]byte(inner), &tools); err != nil {
		// Fallback: try comma-separated without JSON
		inner = strings.Trim(inner, "[]")
		for _, t := range strings.Split(inner, ",") {
			t = strings.Trim(strings.TrimSpace(t), "\"'")
			if t != "" {
				tools = append(tools, t)
			}
		}
	}

	if len(tools) == 0 {
		return nil, content
	}

	// Cap at 5 to prevent abuse
	if len(tools) > 5 {
		tools = tools[:5]
	}

	// Strip the tag from the content
	stripped := content[:startIdx] + content[endIdx+len(closeTag):]
	return tools, stripped
}

// extractExtraToolCalls scans content for additional valid JSON tool calls beyond the first
// one already parsed (identified by firstRawJSON). Used to handle LLM responses that contain
// multiple sequential tool calls in one message (e.g. two manage_memory adds).
func extractExtraToolCalls(content, firstRawJSON string) []ToolCall {
	var results []ToolCall
	// Skip past the already-extracted JSON blob so we don't re-parse it
	remaining := content
	if firstRawJSON != "" {
		idx := strings.Index(remaining, firstRawJSON)
		if idx >= 0 {
			remaining = remaining[idx+len(firstRawJSON):]
		}
	}
	// Extract all remaining valid JSON tool calls
	for {
		start := strings.Index(remaining, "{")
		if start == -1 {
			break
		}
		bStr := remaining[start:]
		found := false
		for j := strings.LastIndex(bStr, "}"); j > 0; {
			candidate := bStr[:j+1]
			var tmp ToolCall
			if json.Unmarshal([]byte(candidate), &tmp) == nil && tmp.Action != "" {
				tmp.IsTool = true
				tmp.RawJSON = candidate
				results = append(results, tmp)
				remaining = bStr[j+1:]
				found = true
				break
			}
			j = strings.LastIndex(bStr[:j], "}")
			if j < 0 {
				break
			}
		}
		if !found {
			break
		}
	}
	return results
}

func ParseToolCall(content string) ToolCall {
	var tc ToolCall
	lowerContent := strings.ToLower(content)

	// Stepfun / OpenRouter <tool_call> fallback
	// Format 1: <function=name> ... </function>
	// Format 2: <tool_calls><invoke name="..."> ... </invoke></tool_calls>
	if start := strings.Index(lowerContent, "<tool_calls>"); start != -1 {
		tc.IsTool = true
		// Extract first invoke
		if invStart := strings.Index(lowerContent[start:], "<invoke name="); invStart != -1 {
			invStart += start
			invNameStart := invStart + 13
			invEndChar := strings.Index(lowerContent[invNameStart:], ">")
			if invEndChar != -1 {
				tc.Action = strings.Trim(strings.TrimSpace(content[invNameStart:invNameStart+invEndChar]), "\"'")

				// Extract params
				bodyStart := invNameStart + invEndChar + 1
				bodyEnd := strings.Index(lowerContent[bodyStart:], "</invoke>")
				if bodyEnd != -1 {
					paramSearch := content[bodyStart : bodyStart+bodyEnd]
					parseXMLParams(&tc, paramSearch)
				}
			}
		}
		return tc
	}

	if start := strings.Index(lowerContent, "<function="); start != -1 {
		end := strings.Index(lowerContent[start:], ">")
		if end != -1 {
			actionName := content[start+10 : start+end]
			actionName = strings.Trim(strings.TrimSpace(actionName), "\"'")
			tc.IsTool = true
			tc.Action = actionName

			// Extract any JSON arguments inside <function=...>{...}</function> if present
			funcBodyStart := start + end + 1
			funcBodyEnd := strings.Index(lowerContent[funcBodyStart:], "</function>")
			if funcBodyEnd != -1 {
				jsonBody := content[funcBodyStart : funcBodyStart+funcBodyEnd]
				parseXMLParams(&tc, jsonBody)
			}

			// AGGRESSIVE RECOVERY for LLMs placing the python block OUTSIDE the JSON
			if (tc.Action == "execute_python" || tc.Action == "save_tool") && tc.Code == "" {
				if blockStart := strings.Index(content, "```python"); blockStart != -1 {
					if blockEnd := strings.Index(content[blockStart+9:], "```"); blockEnd != -1 {
						tc.Code = strings.TrimSpace(content[blockStart+9 : blockStart+9+blockEnd])
					}
				}
			}
			return tc
		}
	}

	// Also allow pure operation-only JSON through (e.g. native function call arguments leaked as plain text
	// by models that emit {"operation":"list_devices"} instead of using the structured tool_calls API).
	if (strings.Contains(lowerContent, "\"action\"") || strings.Contains(lowerContent, "'action'") || strings.Contains(lowerContent, "\"tool\"") || strings.Contains(lowerContent, "\"command\"") || strings.Contains(lowerContent, "\"operation\"") || (strings.Contains(lowerContent, "\"name\"") && strings.Contains(lowerContent, "\"arguments\""))) && (strings.Contains(lowerContent, "{") || strings.Contains(lowerContent, "```")) {
		extractedFromFence := false

		// Try all common fence variants: ```json, ``` json, ```JSON, plain ```
		fenceVariants := []string{"```json\n", "```json\r\n", "```json ", "```json", "``` json", "```JSON"}
		for _, fv := range fenceVariants {
			if start := strings.Index(content, fv); start != -1 {
				after := content[start+len(fv):]
				// Trim any leading whitespace/newline after the fence marker
				after = strings.TrimLeft(after, " \t\r\n")
				// Find closing ```
				if end := strings.Index(after, "```"); end != -1 {
					candidate := strings.TrimSpace(after[:end])
					if strings.HasPrefix(candidate, "{") {
						var tmp ToolCall
						if json.Unmarshal([]byte(candidate), &tmp) == nil && (tmp.Action != "" || tmp.Operation != "" || tmp.Name != "" || tmp.Tool != "" || tmp.Command != "") {
							tc = tmp
							extractedFromFence = true
							tc.RawJSON = candidate
							break
						}
					}
				}
			}
		}

		if !extractedFromFence {
			// No fence or fence extraction failed — try raw brace extraction from content.
			// Try all '{' positions as potential JSON starts.
			for i := 0; i < len(content); i++ {
				if content[i] == '{' {
					bStr := content[i:]
					// Search from the end for the furthest '}' that yields a valid ToolCall
					for j := strings.LastIndex(bStr, "}"); j != -1; j = strings.LastIndex(bStr[:j], "}") {
						candidate := bStr[:j+1]
						var tmp ToolCall
						if json.Unmarshal([]byte(candidate), &tmp) == nil && (tmp.Action != "" || tmp.Operation != "" || tmp.Name != "" || tmp.Tool != "" || tmp.Command != "") {
							tc = tmp
							extractedFromFence = true
							tc.RawJSON = candidate
							break
						}
					}
				}
				if extractedFromFence {
					break
				}
			}
		}
		if tc.Action != "" || tc.Operation != "" || tc.Name != "" || tc.Tool != "" || tc.Command != "" {
			tc.IsTool = true

			// AGGRESSIVE RECOVERY: Handle wrappers like {"action": "execute_tool", "tool": "name", "args": {...}}
			if (tc.Action == "execute_tool" || tc.Action == "run_tool" || tc.Action == "execute_tool_call") && tc.Tool != "" {
				tc.Action = tc.Tool
			}

			// Fallback: LLM used "tool" key instead of "action"
			if tc.Action == "" && tc.Tool != "" {
				tc.Action = tc.Tool
			}

			// Fallback: LLM sent only "command" — treat as execute_shell
			if tc.Action == "" && tc.Command != "" {
				tc.Action = "execute_shell"
			}

			// Fallback: MeshCentral LLM hallucinated operation as action or omitted action entirely.
			// This covers the case where the model emits bare native-function-call arguments as plain
			// text (e.g. {"operation":"list_devices"} without an "action" wrapper).
			if tc.Action == "" && tc.Operation != "" {
				switch strings.ToLower(tc.Operation) {
				case "list_groups", "list_devices", "nodes", "meshes", "wake", "power_action", "run_command":
					// These operation values are unique to the meshcentral tool.
					tc.Action = "meshcentral"
				}
				// Generic heuristic: JSON body or struct fields hint at meshcentral.
				if tc.Action == "" {
					if strings.Contains(tc.RawJSON, "\"meshcentral\"") || tc.MeshID != "" || tc.NodeID != "" || tc.PowerAction != 0 {
						tc.Action = "meshcentral"
					}
				}
			}

			// Fallback: OpenAI native function_call format {"name": "tool", "arguments": {...}}
			if tc.Action == "" && tc.Name != "" {
				tc.Action = tc.Name
			}

			// If LLM uses 'arguments' (hallucination)
			if tc.Arguments != nil {
				if tc.Params == nil {
					tc.Params = make(map[string]interface{})
				}
				switch v := tc.Arguments.(type) {
				case map[string]interface{}:
					for k, val := range v {
						tc.Params[k] = val
					}
				case string:
					// Robust recovery: the LLM sometimes JSON-encodes the arguments into a string
					var argMap map[string]interface{}
					if err := json.Unmarshal([]byte(v), &argMap); err == nil {
						for k, val := range argMap {
							tc.Params[k] = val
						}
					}
				}
			}

			// Recovery for map-based 'args' which fails to unmarshal into tc.Args ([]string)
			if argsMap, ok := tc.Args.(map[string]interface{}); ok {
				if tc.Params == nil {
					tc.Params = make(map[string]interface{})
				}
				for k, v := range argsMap {
					tc.Params[k] = v
				}
			}

			// Flatten action_input (LangChain-style nested params) into Params
			if tc.ActionInput != nil {
				if tc.Params == nil {
					tc.Params = make(map[string]interface{})
				}
				for k, v := range tc.ActionInput {
					tc.Params[k] = v
				}
			}

			// Final parameter promotion: Ensure specific fields are populated from Params if missing
			if tc.Params != nil {
				promoteString := func(target *string, keys ...string) {
					if *target != "" {
						return
					}
					for _, k := range keys {
						if v, ok := tc.Params[k].(string); ok && v != "" {
							*target = v
							return
						}
					}
				}
				promoteInt := func(target *int, keys ...string) {
					if *target != 0 {
						return
					}
					for _, k := range keys {
						if v, ok := tc.Params[k].(float64); ok && v != 0 {
							*target = int(v)
							return
						}
					}
				}

				promoteString(&tc.Hostname, "hostname", "host", "server_id")
				promoteString(&tc.IPAddress, "ip_address", "ip", "address")
				promoteString(&tc.Username, "username", "user")
				promoteString(&tc.Password, "password", "pass")
				promoteString(&tc.Tags, "tags", "tag")
				promoteString(&tc.PrivateKeyPath, "private_key_path", "key_path", "private_key")
				promoteString(&tc.ServerID, "server_id", "serverId", "id", "hostname", "host")
				promoteString(&tc.Command, "command", "cmd")
				promoteString(&tc.Tag, "tag", "tags")
				promoteString(&tc.LocalPath, "local_path", "localPath", "source")
				promoteString(&tc.RemotePath, "remote_path", "remotePath", "destination", "dest")
				promoteString(&tc.Direction, "direction")
				promoteString(&tc.Operation, "operation", "op")
				promoteString(&tc.FilePath, "file_path", "path", "filepath", "filename", "file")
				if tc.FilePath != "" && tc.Path == "" {
					tc.Path = tc.FilePath
				}
				promoteString(&tc.Destination, "destination", "dest", "target")
				if tc.Destination != "" && tc.Dest == "" {
					tc.Dest = tc.Destination
				}
				promoteString(&tc.Content, "content", "query")
				promoteString(&tc.Query, "query", "content")
				promoteString(&tc.Name, "name")
				promoteString(&tc.Description, "description")
				promoteString(&tc.Code, "code", "script")
				promoteString(&tc.Package, "package", "package_name")
				promoteString(&tc.ToolName, "tool_name", "toolName")
				promoteString(&tc.Label, "label")
				promoteString(&tc.TaskPrompt, "task_prompt")
				promoteString(&tc.Prompt, "prompt")
				// Notes / Vision / STT fields
				promoteString(&tc.Title, "title")
				promoteString(&tc.Category, "category")
				promoteString(&tc.DueDate, "due_date", "dueDate")
				// Home Assistant fields
				promoteString(&tc.EntityID, "entity_id", "entityId", "entity")
				promoteString(&tc.Domain, "domain")
				promoteString(&tc.Service, "service")
				// Docker fields
				promoteString(&tc.ContainerID, "container_id", "containerId", "container")
				promoteString(&tc.Image, "image")
				promoteString(&tc.Restart, "restart", "restart_policy")
				// Co-Agent fields
				promoteString(&tc.CoAgentID, "co_agent_id", "coAgentId", "coagent_id", "agent_id", "agentId")
				promoteString(&tc.Task, "task")
				// context_hints is []string — promote manually
				if len(tc.ContextHints) == 0 {
					for _, k := range []string{"context_hints", "contextHints", "hints"} {
						if arr, ok := tc.Params[k].([]interface{}); ok && len(arr) > 0 {
							for _, v := range arr {
								if s, ok := v.(string); ok {
									tc.ContextHints = append(tc.ContextHints, s)
								}
							}
							break
						}
					}
				}

				promoteInt(&tc.Port, "port")
				promoteInt(&tc.PID, "pid")
				promoteInt(&tc.Priority, "priority")
				promoteInt(&tc.Done, "done")
				// NoteID is int64 — promote manually
				if tc.NoteID == 0 {
					for _, k := range []string{"note_id", "noteId", "id"} {
						if v, ok := tc.Params[k].(float64); ok && v != 0 {
							tc.NoteID = int64(v)
							break
						}
					}
				}
			}

			// AGGRESSIVE RECOVERY for LLMs placing the python block OUTSIDE the JSON
			if (tc.Action == "execute_python" || tc.Action == "save_tool") && tc.Code == "" {
				if blockStart := strings.Index(content, "```python"); blockStart != -1 {
					if blockEnd := strings.Index(content[blockStart+9:], "```"); blockEnd != -1 {
						tc.Code = strings.TrimSpace(content[blockStart+9 : blockStart+9+blockEnd])
					}
				}
			}
			return tc
		}
	}

	// ── Native-function bare-args fallback ───────────────────────────────────
	// Some providers (e.g. InceptionLabs Mercury) emit raw function arguments in
	// message content instead of proper ToolCalls, without an "action" field.
	// Infer the action from the unique field combination present in the JSON.
	if strings.Contains(lowerContent, "{") {
		for i := 0; i < len(content); i++ {
			if content[i] == '{' {
				bStr := content[i:]
				for j := strings.LastIndex(bStr, "}"); j != -1; j = strings.LastIndex(bStr[:j], "}") {
					candidate := bStr[:j+1]
					var tmp ToolCall
					if json.Unmarshal([]byte(candidate), &tmp) == nil && tmp.Action == "" {
						switch {
						case tmp.Path != "" && tmp.Caption != "":
							// send_image: {"path": "...", "caption": "..."}
							tmp.Action = "send_image"
						case tmp.Skill != "" && tmp.SkillArgs != nil:
							// execute_skill: {"skill": "...", "skill_args": {...}}
							tmp.Action = "execute_skill"
						case tmp.Content != "" && tmp.Fact == "" && tmp.Command == "":
							// query_memory / store_memory: ambiguous without action, skip
						default:
							continue
						}
						if tmp.Action != "" {
							tmp.IsTool = true
							tmp.RawJSON = candidate
							return tmp
						}
					}
				}
				break
			}
		}
	}

	if strings.HasPrefix(lowerContent, "import ") ||
		strings.HasPrefix(lowerContent, "def ") ||
		strings.HasPrefix(lowerContent, "print(") ||
		strings.HasPrefix(lowerContent, "# ") ||
		strings.Contains(lowerContent, "```python") {
		return ToolCall{RawCodeDetected: true}
	}

	return ToolCall{}
}

func parseXMLParams(tc *ToolCall, body string) {
	hasXMLParams := false
	lowerBody := strings.ToLower(body)
	paramSearch := lowerBody

	for {
		// Support <parameter=name> and <parameter name="...">
		pStart := strings.Index(paramSearch, "<parameter")
		if pStart == -1 {
			break
		}
		pAttrEnd := strings.Index(paramSearch[pStart:], ">")
		if pAttrEnd == -1 {
			break
		}
		pAttrEnd += pStart

		attrStr := body[pStart : pAttrEnd+1]
		paramName := ""
		if strings.Contains(attrStr, "=") {
			// <parameter=name>
			eqIdx := strings.Index(attrStr, "=")
			paramName = strings.Trim(strings.TrimSpace(attrStr[eqIdx+1:len(attrStr)-1]), "\"' ")
		} else if strings.Contains(attrStr, "name=") {
			// <parameter name="name">
			nameIdx := strings.Index(attrStr, "name=")
			paramName = strings.Trim(strings.TrimSpace(attrStr[nameIdx+5:len(attrStr)-1]), "\"' ")
		}

		vStart := pAttrEnd + 1
		vEndOffset := strings.Index(paramSearch[vStart:], "</parameter>")
		if vEndOffset == -1 {
			break
		}

		paramVal := strings.TrimSpace(body[vStart : vStart+vEndOffset])
		hasXMLParams = true

		switch paramName {
		case "code":
			tc.Code = paramVal
		case "name":
			tc.Name = strings.Trim(paramVal, "\"'")
		case "tool_name":
			tc.ToolName = strings.Trim(paramVal, "\"'")
		case "package":
			tc.Package = strings.Trim(paramVal, "\"'")
		case "key":
			tc.Key = strings.Trim(paramVal, "\"'")
		case "value":
			tc.Value = strings.Trim(paramVal, "\"'")
		case "skill":
			tc.Skill = strings.Trim(paramVal, "\"'")
		case "skill_args", "params":
			_ = json.Unmarshal([]byte(paramVal), &tc.Params)
			tc.SkillArgs = tc.Params
		case "operation":
			tc.Operation = strings.Trim(paramVal, "\"'")
		case "file_path", "path":
			tc.FilePath = strings.Trim(paramVal, "\"'")
			tc.Path = tc.FilePath
		case "destination", "dest":
			tc.Destination = strings.Trim(paramVal, "\"'")
			tc.Dest = tc.Destination
		case "content":
			tc.Content = paramVal
		case "query":
			tc.Query = paramVal
		case "task_prompt", "plan", "description":
			tc.TaskPrompt = paramVal
		case "prompt":
			tc.Prompt = paramVal
		case "title":
			tc.Title = strings.Trim(paramVal, "\"'")
		case "category":
			tc.Category = strings.Trim(paramVal, "\"'")
		case "priority":
			if v, err := strconv.Atoi(strings.TrimSpace(paramVal)); err == nil {
				tc.Priority = v
			}
		case "due_date":
			tc.DueDate = strings.Trim(paramVal, "\"'")
		case "note_id":
			if v, err := strconv.ParseInt(strings.TrimSpace(paramVal), 10, 64); err == nil {
				tc.NoteID = v
			}
		case "done":
			if v, err := strconv.Atoi(strings.TrimSpace(paramVal)); err == nil {
				tc.Done = v
			}
		case "args":
			_ = json.Unmarshal([]byte(paramVal), &tc.Args)
		}

		// advance
		advance := vStart + vEndOffset + 12
		if advance >= len(paramSearch) {
			break
		}
		paramSearch = paramSearch[advance:]
		body = body[advance:]
	}

	// 2. If no XML parameters were found, fallback to parsing as JSON
	if !hasXMLParams {
		jsonBody := strings.TrimSpace(body)
		// Strip markdown markdown block strings
		if strings.HasPrefix(jsonBody, "```json") {
			jsonBody = strings.TrimPrefix(jsonBody, "```json")
		} else if strings.HasPrefix(jsonBody, "```") {
			jsonBody = strings.TrimPrefix(jsonBody, "```")
		}
		jsonBody = strings.TrimSuffix(jsonBody, "```")
		jsonBody = strings.TrimSpace(jsonBody)

		if strings.HasPrefix(jsonBody, "{") {
			_ = json.Unmarshal([]byte(jsonBody), tc)
		}
	}
}

func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// isFollowUpQuestion returns true when a follow_up task_prompt looks like a
// question directed at the user rather than a self-contained task for the agent.
// Using follow_up to ask for user input causes infinite recursion because each
// invocation re-triggers the same unanswerable question.
func isFollowUpQuestion(prompt string) bool {
	// Ends with a question mark → almost certainly a user-facing question
	if strings.HasSuffix(prompt, "?") {
		return true
	}

	// Common German/English phrases that introduce a request for user input
	lower := strings.ToLower(prompt)
	questionPhrases := []string{
		"bitte gib",
		"bitte teile",
		"bitte sag",
		"bitte nenne",
		"bitte geben sie",
		"please provide",
		"please tell me",
		"please give",
		"please specify",
		"please enter",
		"köntest du",
		"könntest du",
		"kannst du mir",
		"what is the",
		"what path",
		"which path",
		"what interval",
	}
	for _, phrase := range questionPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// GetActiveProcessStatus returns a comma-separated string of PIDs for the manifest sysprompt.
func GetActiveProcessStatus(registry *tools.ProcessRegistry) string {
	list := registry.List()
	if len(list) == 0 {
		return "None"
	}
	var names []string
	for _, p := range list {
		alive, _ := p["alive"].(bool)
		if alive {
			pid, _ := p["pid"].(int)
			names = append(names, fmt.Sprintf("PID:%d", pid))
		}
	}
	if len(names) == 0 {
		return "None"
	}
	return strings.Join(names, ", ")
}

// runGitCommand helper runs a git command with enforced environment and safe.directory config.
func runGitCommand(dir string, args ...string) ([]byte, error) {
	// Add safe.directory to bypass ownership warnings when running as root in user dirs
	fullArgs := append([]string{"-c", "safe.directory=" + dir}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Dir = dir

	// Ensure HOME is set, otherwise git may fail with exit status 128
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/root" // Default for root-run services
	}
	cmd.Env = append(os.Environ(), "HOME="+home)

	return cmd.CombinedOutput()
}

// handleWebhookToolCall processes manage_webhooks tool calls.
func handleWebhookToolCall(tc ToolCall, mgr *webhooks.Manager, logger *slog.Logger) string {
	switch tc.Operation {
	case "list":
		list := mgr.List()
		type summary struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Slug    string `json:"slug"`
			Enabled bool   `json:"enabled"`
			URL     string `json:"url"`
		}
		out := make([]summary, len(list))
		for i, w := range list {
			out[i] = summary{ID: w.ID, Name: w.Name, Slug: w.Slug, Enabled: w.Enabled, URL: "/webhook/" + w.Slug}
		}
		data, _ := json.Marshal(map[string]any{"status": "ok", "webhooks": out})
		return "Tool Output: " + string(data)

	case "get":
		if tc.ID == "" {
			return `Tool Output: {"status":"error","message":"id is required"}`
		}
		w, err := mgr.Get(tc.ID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err)
		}
		data, _ := json.Marshal(map[string]any{"status": "ok", "webhook": w})
		return "Tool Output: " + string(data)

	case "create":
		w := webhooks.Webhook{
			Name:    tc.Name,
			Slug:    tc.Slug,
			Enabled: true,
		}
		if tc.TokenID != "" {
			w.TokenID = tc.TokenID
		}
		created, err := mgr.Create(w)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err)
		}
		logger.Info("Webhook created via tool", "id", created.ID, "slug", created.Slug)
		data, _ := json.Marshal(map[string]any{"status": "ok", "webhook_id": created.ID, "slug": created.Slug, "url": "/webhook/" + created.Slug})
		return "Tool Output: " + string(data)

	case "update":
		if tc.ID == "" {
			return `Tool Output: {"status":"error","message":"id is required"}`
		}
		patch := webhooks.Webhook{Name: tc.Name, Slug: tc.Slug, Enabled: tc.Enabled}
		if tc.TokenID != "" {
			patch.TokenID = tc.TokenID
		}
		updated, err := mgr.Update(tc.ID, patch)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err)
		}
		logger.Info("Webhook updated via tool", "id", updated.ID)
		data, _ := json.Marshal(map[string]any{"status": "ok", "webhook_id": updated.ID})
		return "Tool Output: " + string(data)

	case "delete":
		if tc.ID == "" {
			return `Tool Output: {"status":"error","message":"id is required"}`
		}
		err := mgr.Delete(tc.ID)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s"}`, err)
		}
		logger.Info("Webhook deleted via tool", "id", tc.ID)
		return `Tool Output: {"status":"ok","message":"webhook deleted"}`

	case "logs":
		whLog := mgr.GetLog()
		n := 20
		var entries []webhooks.LogEntry
		if tc.ID != "" {
			entries = whLog.ForWebhook(tc.ID, n)
		} else {
			entries = whLog.Recent(n)
		}
		data, _ := json.Marshal(map[string]any{"status": "ok", "entries": entries})
		return "Tool Output: " + string(data)

	default:
		return `Tool Output: {"status":"error","message":"Unknown operation. Use: list, get, create, update, delete, logs"}`
	}
}

// decodeBase64 decodes a standard or URL-safe base64 string.
func decodeBase64(s string) ([]byte, error) {
	// Try standard encoding first, then URL-safe
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(s)
	}
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(s)
	}
	return data, err
}

// calculateEffectiveMaxCalls berechnet das effektive Circuit Breaker Limit
// basierend auf Personality Traits, Homepage-Multiplier und explizitem Override.
// Wenn tc leer ist (ToolCall{}), werden nur die Basis-Anpassungen berechnet (Personality).
// Tool-spezifische Anpassungen erfolgen später wenn tc bekannt ist.
func calculateEffectiveMaxCalls(cfg *config.Config, tc ToolCall, personalityEnabled bool, shortTermMem *memory.SQLiteMemory, logger *slog.Logger) int {
	effectiveMaxCalls := cfg.CircuitBreaker.MaxToolCalls

	// 1. Personality Engine V2: Thoroughness Trait
	if personalityEnabled && cfg.Agent.PersonalityEngineV2 && shortTermMem != nil {
		if traits, err := shortTermMem.GetTraits(); err == nil {
			if thoroughness, ok := traits[memory.TraitThoroughness]; ok && thoroughness > 0.8 {
				effectiveMaxCalls = int(float64(effectiveMaxCalls) * 1.5)
				logger.Debug("[Behavioral Tool Calling] Increased MaxToolCalls due to high Thoroughness", "new_max", effectiveMaxCalls)
			}
		}
	}

	// 2. Homepage Tool: Multiplier für komplexe Web-Workflows
	// Nur anwenden wenn tc bekannt ist (nicht leer)
	if tc.Tool != "" && tc.Tool == "homepage" && cfg.Homepage.Enabled {
		multiplier := cfg.Homepage.CircuitBreakerMultiplier
		if multiplier > 0 {
			// Cap bei 5x
			if multiplier > 5.0 {
				multiplier = 5.0
			}
			newLimit := int(float64(effectiveMaxCalls) * multiplier)
			logger.Debug("[Circuit Breaker] Homepage multiplier applied", "base_limit", effectiveMaxCalls, "multiplier", multiplier, "new_limit", newLimit)
			effectiveMaxCalls = newLimit
		}
	}

	// 3. Expliziter Override im ToolCall (höchste Priorität)
	// Nur anwenden wenn tc bekannt ist
	if tc.Tool != "" && tc.CircuitBreakerOverride > 0 {
		// Max 3x Standard-Limit für Sicherheit
		maxAllowed := cfg.CircuitBreaker.MaxToolCalls * 3
		if tc.CircuitBreakerOverride > maxAllowed {
			logger.Warn("[Circuit Breaker] Override exceeds maximum allowed, capping", "requested", tc.CircuitBreakerOverride, "max_allowed", maxAllowed)
			effectiveMaxCalls = maxAllowed
		} else {
			logger.Debug("[Circuit Breaker] Explicit override applied", "override", tc.CircuitBreakerOverride)
			effectiveMaxCalls = tc.CircuitBreakerOverride
		}
	}

	return effectiveMaxCalls
}
