package agent

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// Agent encapsulates the agent's dependencies and state.
type Agent struct {
	Cfg                *config.Config
	Logger             *slog.Logger
	ShortTermMem       *memory.SQLiteMemory
	LongTermMem        memory.VectorDB
	Vault              *security.Vault
	Registry           *tools.ProcessRegistry
	CronManager        *tools.CronManager
	KG                 *memory.KnowledgeGraph
	InventoryDB        *sql.DB
	InvasionDB         *sql.DB
	CheatsheetDB       *sql.DB
	ImageGalleryDB     *sql.DB
	MediaRegistryDB    *sql.DB
	HomepageRegistryDB *sql.DB
}

// NewAgent creates a new Agent instance.
func NewAgent(cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cron *tools.CronManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB, cheatsheetDB *sql.DB, imageGalleryDB *sql.DB, mediaRegistryDB *sql.DB, homepageRegistryDB *sql.DB) *Agent {
	return &Agent{
		Cfg:                cfg,
		Logger:             logger,
		ShortTermMem:       stm,
		LongTermMem:        ltm,
		Vault:              vault,
		Registry:           registry,
		CronManager:        cron,
		KG:                 kg,
		InventoryDB:        inventoryDB,
		InvasionDB:         invasionDB,
		CheatsheetDB:       cheatsheetDB,
		ImageGalleryDB:     imageGalleryDB,
		MediaRegistryDB:    mediaRegistryDB,
		HomepageRegistryDB: homepageRegistryDB,
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
	return estimateTokensForModel(text, "")
}

// estimateTokensForModel estimates the token count with model-aware character ratios.
// Different model families use different tokenizers with meaningfully different ratios.
func estimateTokensForModel(text string, model string) int {
	if text == "" {
		return 0
	}
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "gpt-4") || strings.Contains(lower, "gpt-3.5") ||
		strings.Contains(lower, "o1") || strings.Contains(lower, "o3"):
		// OpenAI cl100k_base tokenizer: ~3.3 chars/token for English
		return int(float64(len(text)) / 3.3)
	case strings.Contains(lower, "claude"):
		// Anthropic tokenizer: slightly larger tokens on average
		return int(float64(len(text)) / 3.5)
	case strings.Contains(lower, "gemini"):
		// Google SentencePiece / BPE: similar to cl100k
		return int(float64(len(text)) / 3.4)
	case strings.Contains(lower, "llama") || strings.Contains(lower, "mistral") ||
		strings.Contains(lower, "qwen") || strings.Contains(lower, "deepseek"):
		// LLaMA/Mistral-family BPE: ~3.5-4.0 chars/token
		return int(float64(len(text)) / 3.7)
	default:
		// Conservative fallback: 1 token per 4 characters
		return len(text) / 4
	}
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
	NativeCallID       string                 `json:"-"`               // Native API tool call ID for role=tool responses
	Todo               string                 `json:"_todo,omitempty"` // Session-scoped task list piggybacked on every tool call
	Operation          string                 `json:"operation"`
	Fact               string                 `json:"fact"`
	ID                 string                 `json:"id"`
	CronExpr           string                 `json:"cron_expr"`
	TaskPrompt         string                 `json:"task_prompt"`
	Skill              string                 `json:"skill"`
	SkillArgs          map[string]interface{} `json:"skill_args"`
	Content            string                 `json:"content"`
	Query              string                 `json:"query"`   // Alias for content in query_memory
	Sources            []string               `json:"sources"` // Memory sources filter for query_memory (vector_db, knowledge_graph, journal, notes, core_memory)
	Scope              string                 `json:"scope"`   // Scope for memory_reflect (recent, monthly, full)
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
	// Journal fields
	EntryType  string `json:"entry_type"`
	Importance int    `json:"importance"`
	FromDate   string `json:"from_date"`
	ToDate     string `json:"to_date"`
	EntryID    int64  `json:"entry_id"`
	// Inventory / Device fields
	DeviceType string `json:"device_type,omitempty"`
	MACAddress string `json:"mac_address,omitempty"` // Optional MAC for Wake-on-LAN
	NoteID     int64  `json:"note_id"`
	// Google Workspace fields
	DocumentID   string          `json:"document_id"`
	MaxResults   int             `json:"max_results"`
	Append       bool            `json:"append"`
	MessageID    string          `json:"message_id"`
	AddLabels    []string        `json:"add_labels"`
	RemoveLabels []string        `json:"remove_labels"`
	EventID      string          `json:"event_id"`
	StartTime    string          `json:"start_time"`
	EndTime      string          `json:"end_time"`
	FileID       string          `json:"file_id"`
	Range        string          `json:"range"`
	Values       [][]interface{} `json:"values"`
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
	// Remote Control fields
	DeviceID  string `json:"device_id"`
	Recursive bool   `json:"recursive"`
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
	// Image generation fields
	SourceImage   string `json:"source_image"`
	EnhancePrompt *bool  `json:"enhance_prompt,omitempty"` // pointer: nil = not provided, true/false = explicit
	Size          string `json:"size"`
	Quality       string `json:"quality"`
	Style         string `json:"style"`
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
	CircuitBreakerOverride int    `json:"circuit_breaker_override,omitempty"`
	GuardianJustification  string `json:"_guardian_justification,omitempty"` // agent explains why a blocked tool call is needed
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
	// AdGuard Home fields
	Answer   string   `json:"answer"`   // DNS rewrite answer (IP or CNAME)
	Rules    string   `json:"rules"`    // custom filtering rules (newline-separated)
	Services []string `json:"services"` // blocked service IDs / upstream DNS servers
	MAC      string   `json:"mac"`      // MAC address for DHCP leases
	IP       string   `json:"ip"`       // IP address for DHCP leases
	Offset   int      `json:"offset"`   // pagination offset
	// Cheat Sheet fields
	Active *bool `json:"active,omitempty"` // pointer so nil = not provided vs false = explicitly inactive
	// Media Registry / Homepage Registry fields
	MediaType string `json:"media_type,omitempty"` // image, tts, audio, music
	TagMode   string `json:"tag_mode,omitempty"`   // add, remove, set (for media_registry tag op)
	Reason    string `json:"reason,omitempty"`     // edit reason (homepage_registry log_edit)
	Problem   string `json:"problem,omitempty"`    // problem description (homepage_registry log_problem/resolve_problem)
	Status    string `json:"status,omitempty"`     // project status: active, archived, maintenance
	Notes     string `json:"notes,omitempty"`      // additional notes
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
	Config             *config.Config
	Logger             *slog.Logger
	LLMClient          llm.ChatClient
	ShortTermMem       *memory.SQLiteMemory
	HistoryManager     *memory.HistoryManager
	LongTermMem        memory.VectorDB
	KG                 *memory.KnowledgeGraph
	InventoryDB        *sql.DB
	InvasionDB         *sql.DB
	CheatsheetDB       *sql.DB
	ImageGalleryDB     *sql.DB
	MediaRegistryDB    *sql.DB
	HomepageRegistryDB *sql.DB
	RemoteHub          *remote.RemoteHub
	Vault              *security.Vault
	Registry           *tools.ProcessRegistry
	Manifest           *tools.Manifest
	CronManager        *tools.CronManager
	MissionManager     *tools.MissionManager
	CoAgentRegistry    *CoAgentRegistry
	BudgetTracker      *budget.Tracker
	SessionID          string
	IsMaintenance      bool
	SurgeryPlan        string
}

func dispatchInner(ctx context.Context, tc ToolCall, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, missionManager *tools.MissionManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, invasionDB *sql.DB, cheatsheetDB *sql.DB, imageGalleryDB *sql.DB, mediaRegistryDB *sql.DB, homepageRegistryDB *sql.DB, remoteHub *remote.RemoteHub, historyMgr *memory.HistoryManager, isMaintenance bool, surgeryPlan string, guardian *security.Guardian, llmGuardian *security.LLMGuardian, sessionID string, coAgentRegistry *CoAgentRegistry, budgetTracker *budget.Tracker) string {
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
		case "manage_journal":
			if tc.Operation != "list" && tc.Operation != "search" && tc.Operation != "get_summary" {
				return `Tool Output: {"status": "error", "message": "Co-Agents cannot modify journal entries. Only list, search, and get_summary are allowed."}`
			}
		case "co_agent", "co_agents":
			return `Tool Output: {"status": "error", "message": "Co-Agents cannot spawn sub-agents."}`
		case "follow_up":
			return `Tool Output: {"status": "error", "message": "Co-Agents cannot schedule follow-ups."}`
		case "cron_scheduler":
			return `Tool Output: {"status": "error", "message": "Co-Agents cannot manage cron jobs."}`
		}
	}

	// Route to sub-dispatchers
	if result := dispatchExec(ctx, tc, cfg, logger, llmClient, vault, registry, manifest, cronManager, missionManager, longTermMem, shortTermMem, kg, inventoryDB, invasionDB, cheatsheetDB, imageGalleryDB, mediaRegistryDB, homepageRegistryDB, remoteHub, historyMgr, isMaintenance, surgeryPlan, guardian, sessionID, coAgentRegistry, budgetTracker); result != dispatchNotHandled {
		return result
	}
	if result := dispatchComm(ctx, tc, cfg, logger, llmClient, vault, registry, manifest, cronManager, missionManager, longTermMem, shortTermMem, kg, inventoryDB, invasionDB, cheatsheetDB, imageGalleryDB, mediaRegistryDB, homepageRegistryDB, remoteHub, historyMgr, isMaintenance, surgeryPlan, guardian, llmGuardian, sessionID, coAgentRegistry, budgetTracker); result != dispatchNotHandled {
		return result
	}
	if result := dispatchServices(ctx, tc, cfg, logger, llmClient, vault, registry, manifest, cronManager, missionManager, longTermMem, shortTermMem, kg, inventoryDB, invasionDB, cheatsheetDB, imageGalleryDB, mediaRegistryDB, homepageRegistryDB, remoteHub, historyMgr, isMaintenance, surgeryPlan, guardian, sessionID, coAgentRegistry, budgetTracker); result != dispatchNotHandled {
		return result
	}
	if result := dispatchInfra(ctx, tc, cfg, logger, llmClient, vault, registry, manifest, cronManager, missionManager, longTermMem, shortTermMem, kg, inventoryDB, invasionDB, cheatsheetDB, imageGalleryDB, mediaRegistryDB, homepageRegistryDB, remoteHub, historyMgr, isMaintenance, surgeryPlan, guardian, sessionID, coAgentRegistry, budgetTracker); result != dispatchNotHandled {
		return result
	}

	logger.Warn("LLM requested unknown action", "action", tc.Action)
	hint := ""
	switch tc.Action {
	case "firewall", "firewall_rules", "iptables":
		hint = " For firewall rules, use execute_shell with iptables commands (e.g. sudo iptables -L -n)."
	}
	return fmt.Sprintf("Tool Output: ERROR unknown action '%s'.%s Available actions are listed in the tool schema.", tc.Action, hint)
}
