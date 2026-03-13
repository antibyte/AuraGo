package agent

import (
	"context"
	"database/sql"
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
	"aurago/internal/memory"
	"aurago/internal/meshcentral"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/tools"
	"aurago/internal/webhooks"
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
	// AdGuard Home fields
	Answer   string   `json:"answer"`   // DNS rewrite answer (IP or CNAME)
	Rules    string   `json:"rules"`    // custom filtering rules (newline-separated)
	Services []string `json:"services"` // blocked service IDs / upstream DNS servers
	MAC      string   `json:"mac"`      // MAC address for DHCP leases
	IP       string   `json:"ip"`       // IP address for DHCP leases
	Offset   int      `json:"offset"`   // pagination offset
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
			if isSystemSecret(tc.Key) {
				logger.Warn("LLM attempted to overwrite system-managed secret — access denied", "key", tc.Key)
				return `Tool Output: {"status": "error", "message": "Access denied: this secret is managed by a system component and cannot be overwritten via secrets_vault."}`
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
			// Filter out system-managed keys — the agent must not know they exist
			keys, err := vault.ListKeys()
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "%v"}`, err)
			}
			visibleKeys := keys[:0]
			for _, k := range keys {
				if !isSystemSecret(k) {
					visibleKeys = append(visibleKeys, k)
				}
			}
			b, mErr := json.Marshal(visibleKeys)
			if mErr != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to serialize keys: %v"}`, mErr)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Stored secret keys (use get_secret with 'key' to retrieve a value)", "keys": %s}`, string(b))
		}
		// Block access to system-managed secrets
		if isSystemSecret(tc.Key) {
			logger.Warn("LLM attempted to read system-managed secret — access denied", "key", tc.Key)
			return `Tool Output: {"status": "error", "message": "Access denied: this secret is managed by a system component and cannot be retrieved via secrets_vault."}`
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
		if isSystemSecret(tc.Key) {
			logger.Warn("LLM attempted to overwrite system-managed secret — access denied", "key", tc.Key)
			return `Tool Output: {"status": "error", "message": "Access denied: this secret is managed by a system component and cannot be overwritten via secrets_vault."}`
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

		// Block access to system-sensitive files (config, vault, databases, .env)
		wsDir := cfg.Directories.WorkspaceDir
		for _, checkPath := range []string{fpath, fdest} {
			if isProtectedSystemPath(checkPath, wsDir, cfg) {
				logger.Warn("LLM attempted filesystem access to protected system file — blocked",
					"op", op, "path", checkPath)
				return "Tool Output: [PERMISSION DENIED] Access to this file is not allowed. System configuration, database and credential files are off-limits."
			}
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
		device, err := inventory.GetDeviceByIDOrName(inventoryDB, tc.ServerID)
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

		device, err := inventory.GetDeviceByIDOrName(inventoryDB, tc.ServerID)
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
			if !cfg.Agent.AllowPython {
				return "Tool Output: [PERMISSION DENIED] google_workspace skill requires Python (agent.allow_python: false)."
			}
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

		// Generic Python skill fallback — gate on AllowPython
		if !cfg.Agent.AllowPython {
			return fmt.Sprintf("Tool Output: [PERMISSION DENIED] Skill '%s' requires Python execution which is disabled (agent.allow_python: false).", skillName)
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
			device, err := inventory.GetDeviceByIDOrName(inventoryDB, tc.ServerID)
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
			return `Tool Output: {"status": "error", "message": "No active email account configured. Enable an account in Settings > Email."}`
		}
		if acct.Disabled {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' is disabled. Enable it in Settings > Email."}`, acct.ID)
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
			return `Tool Output: {"status": "error", "message": "No active email account configured. Enable an account in Settings > Email."}`
		}
		if acct.Disabled {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' is disabled. Enable it in Settings > Email."}`, acct.ID)
		}
		if acct.ReadOnly {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Email account '%s' is read-only. Enable sending in Settings > Email."}`, acct.ID)
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
			ID        string `json:"id"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			IMAP      string `json:"imap"`
			SMTP      string `json:"smtp"`
			Watcher   bool   `json:"watcher"`
			Enabled   bool   `json:"enabled"`
			AllowSend bool   `json:"allow_sending"`
		}
		var accts []acctInfo
		for _, a := range cfg.EmailAccounts {
			accts = append(accts, acctInfo{
				ID:        a.ID,
				Name:      a.Name,
				Email:     a.FromAddress,
				IMAP:      fmt.Sprintf("%s:%d", a.IMAPHost, a.IMAPPort),
				SMTP:      fmt.Sprintf("%s:%d", a.SMTPHost, a.SMTPPort),
				Watcher:   a.WatchEnabled,
				Enabled:   !a.Disabled,
				AllowSend: !a.ReadOnly,
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

		mcClient := meshcentral.NewClient(cfg.MeshCentral.URL, cfg.MeshCentral.Username, pass, token, cfg.MeshCentral.Insecure)
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
			result, err := mcClient.WakeOnLan([]string{tc.NodeID})
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to send wake magic packet: %v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "%s"}`, result)

		case "power_action":
			if tc.NodeID == "" {
				return `Tool Output: {"status": "error", "message": "'node_id' is required for power_action"}`
			}
			if tc.PowerAction < 1 || tc.PowerAction > 4 {
				return `Tool Output: {"status": "error", "message": "Invalid power action. 1=Sleep, 2=Hibernate, 3=PowerOff, 4=Reset"}`
			}
			result, err := mcClient.PowerAction([]string{tc.NodeID}, tc.PowerAction)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to send power action: %v"}`, err)
			}
			return fmt.Sprintf(`Tool Output: {"status": "success", "message": "%s"}`, result)

		case "run_command":
			if tc.NodeID == "" || tc.Command == "" {
				return `Tool Output: {"status": "error", "message": "'node_id' and 'command' are required for run_command"}`
			}
			result, err := mcClient.RunCommand(tc.NodeID, tc.Command)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to run command: %v"}`, err)
			}
			// Format result for display
			resultJSON, _ := json.Marshal(result)
			return fmt.Sprintf(`Tool Output: {"status": "success", "data": %s}`, string(resultJSON))

		case "shell":
			if tc.NodeID == "" || tc.Command == "" {
				return `Tool Output: {"status": "error", "message": "'node_id' and 'command' are required for shell"}`
			}
			result, err := mcClient.Shell(tc.NodeID, tc.Command)
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Failed to execute shell command: %v"}`, err)
			}
			// Format result for display
			resultJSON, _ := json.Marshal(result)
			return fmt.Sprintf(`Tool Output: {"status": "success", "data": %s}`, string(resultJSON))

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
			DockerHost:       cfg.Docker.Host,
			WorkspacePath:    cfg.Homepage.WorkspacePath,
			WebServerPort:    cfg.Homepage.WebServerPort,
			WebServerDomain:  cfg.Homepage.WebServerDomain,
			AllowLocalServer: cfg.Homepage.AllowLocalServer,
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
		case "deploy_netlify":
			if !cfg.Homepage.AllowDeploy {
				return `Tool Output: {"status":"error","message":"Deployment is disabled. Enable homepage.allow_deploy in config."}`
			}
			if !cfg.Netlify.Enabled {
				return `Tool Output: {"status":"error","message":"Netlify integration is not enabled. Set netlify.enabled=true in config.yaml."}`
			}
			nfToken, nfErr := vault.ReadSecret("netlify_token")
			if nfErr != nil || nfToken == "" {
				return `Tool Output: {"status":"error","message":"Netlify token not found in vault. Store it with key 'netlify_token' via the Config UI."}`
			}
			nfCfg := tools.NetlifyConfig{
				Token:         nfToken,
				DefaultSiteID: cfg.Netlify.DefaultSiteID,
				TeamSlug:      cfg.Netlify.TeamSlug,
			}
			logger.Info("LLM requested homepage deploy_netlify", "project", tc.ProjectDir, "build_dir", tc.BuildDir, "site_id", tc.SiteID, "draft", tc.Draft)
			return "Tool Output: " + tools.HomepageDeployNetlify(homepageCfg, nfCfg, tc.ProjectDir, tc.BuildDir, tc.SiteID, tc.Title, tc.Draft, logger)
		default:
			return `Tool Output: {"status":"error","message":"Unknown homepage operation. Use: init, start, stop, status, rebuild, destroy, exec, init_project, build, install_deps, lighthouse, screenshot, lint, list_files, read_file, write_file, optimize_images, dev, deploy, deploy_netlify, test_connection, webserver_start, webserver_stop, webserver_status, publish_local"}`
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

		// Allowed-repos enforcement: if a list is configured the agent may only access
		// repos that are explicitly allowed OR repos it created itself (tracked projects).
		if len(cfg.GitHub.AllowedRepos) > 0 {
			repoArg := tc.Name
			repoOpsNeedCheck := map[string]bool{
				"delete_repo": true, "get_repo": true, "list_issues": true,
				"create_issue": true, "close_issue": true, "list_pull_requests": true,
				"list_branches": true, "get_file": true, "create_or_update_file": true,
				"list_commits": true, "list_workflow_runs": true,
			}
			if repoArg != "" && repoOpsNeedCheck[tc.Operation] {
				allowedMap := map[string]bool{}
				for _, r := range cfg.GitHub.AllowedRepos {
					allowedMap[r] = true
				}
				// Agent-created repos (tracked in workspace) are always permitted
				isTracked := false
				trackedRaw := tools.GitHubListProjects(cfg.Directories.WorkspaceDir)
				var trackedResult map[string]interface{}
				if jsonErr := json.Unmarshal([]byte(trackedRaw), &trackedResult); jsonErr == nil {
					if projects, ok := trackedResult["projects"].([]interface{}); ok {
						for _, p := range projects {
							if pm, ok := p.(map[string]interface{}); ok {
								if name, _ := pm["name"].(string); name == repoArg {
									isTracked = true
									break
								}
							}
						}
					}
				}
				if !allowedMap[repoArg] && !isTracked {
					return fmt.Sprintf(`Tool Output: {"status":"error","message":"Repo '%s' is not in the allowed repos list. Add it in Settings → GitHub to grant access."}`, repoArg)
				}
			}
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
			if tc.Content == "" {
				return `Tool Output: {"status":"error","message":"content (base64 ZIP) is required for deploy_zip."}`
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

	case "adguard", "adguard_home":
		if !cfg.AdGuard.Enabled {
			return `Tool Output: {"status":"error","message":"AdGuard Home is not enabled. Configure the adguard section in config.yaml."}`
		}
		adgCfg := tools.AdGuardConfig{
			URL:      cfg.AdGuard.URL,
			Username: cfg.AdGuard.Username,
			Password: cfg.AdGuard.Password,
		}
		op := strings.ToLower(strings.TrimSpace(tc.Operation))

		// Read-only operations
		switch op {
		case "status":
			logger.Info("LLM requested AdGuard status")
			return "Tool Output: " + tools.AdGuardStatus(adgCfg)
		case "stats":
			logger.Info("LLM requested AdGuard stats")
			return "Tool Output: " + tools.AdGuardStats(adgCfg)
		case "stats_top":
			logger.Info("LLM requested AdGuard top stats")
			return "Tool Output: " + tools.AdGuardStatsTop(adgCfg)
		case "query_log":
			logger.Info("LLM requested AdGuard query log", "search", tc.Query, "limit", tc.Limit)
			return "Tool Output: " + tools.AdGuardQueryLog(adgCfg, tc.Query, tc.Limit, tc.Offset)
		case "filtering_status":
			logger.Info("LLM requested AdGuard filtering status")
			return "Tool Output: " + tools.AdGuardFilteringStatus(adgCfg)
		case "rewrite_list":
			logger.Info("LLM requested AdGuard rewrite list")
			return "Tool Output: " + tools.AdGuardRewriteList(adgCfg)
		case "blocked_services_list":
			logger.Info("LLM requested AdGuard blocked services list")
			return "Tool Output: " + tools.AdGuardBlockedServicesList(adgCfg)
		case "safebrowsing_status":
			logger.Info("LLM requested AdGuard safe browsing status")
			return "Tool Output: " + tools.AdGuardSafeBrowsingStatus(adgCfg)
		case "parental_status":
			logger.Info("LLM requested AdGuard parental status")
			return "Tool Output: " + tools.AdGuardParentalStatus(adgCfg)
		case "dhcp_status":
			logger.Info("LLM requested AdGuard DHCP status")
			return "Tool Output: " + tools.AdGuardDHCPStatus(adgCfg)
		case "clients":
			logger.Info("LLM requested AdGuard clients")
			return "Tool Output: " + tools.AdGuardClients(adgCfg)
		case "dns_info":
			logger.Info("LLM requested AdGuard DNS info")
			return "Tool Output: " + tools.AdGuardDNSInfo(adgCfg)
		case "test_upstream":
			logger.Info("LLM requested AdGuard test upstream", "servers", tc.Services)
			return "Tool Output: " + tools.AdGuardTestUpstream(adgCfg, tc.Services)
		}

		// Write operations — check readonly
		if cfg.AdGuard.ReadOnly {
			return `Tool Output: {"status":"error","message":"AdGuard Home is in read-only mode. Disable adguard.readonly to allow changes."}`
		}
		switch op {
		case "query_log_clear":
			logger.Info("LLM requested AdGuard query log clear")
			return "Tool Output: " + tools.AdGuardQueryLogClear(adgCfg)
		case "filtering_toggle":
			logger.Info("LLM requested AdGuard filtering toggle", "enabled", tc.Enabled)
			return "Tool Output: " + tools.AdGuardFilteringToggle(adgCfg, tc.Enabled)
		case "filtering_add_url":
			logger.Info("LLM requested AdGuard add filter URL", "url", tc.URL)
			return "Tool Output: " + tools.AdGuardFilteringAddURL(adgCfg, tc.Name, tc.URL)
		case "filtering_remove_url":
			logger.Info("LLM requested AdGuard remove filter URL", "url", tc.URL)
			return "Tool Output: " + tools.AdGuardFilteringRemoveURL(adgCfg, tc.URL)
		case "filtering_refresh":
			logger.Info("LLM requested AdGuard filtering refresh")
			return "Tool Output: " + tools.AdGuardFilteringRefresh(adgCfg)
		case "filtering_set_rules":
			logger.Info("LLM requested AdGuard set filtering rules")
			return "Tool Output: " + tools.AdGuardFilteringSetRules(adgCfg, tc.Rules)
		case "rewrite_add":
			logger.Info("LLM requested AdGuard add rewrite", "domain", tc.Domain, "answer", tc.Answer)
			return "Tool Output: " + tools.AdGuardRewriteAdd(adgCfg, tc.Domain, tc.Answer)
		case "rewrite_delete":
			logger.Info("LLM requested AdGuard delete rewrite", "domain", tc.Domain, "answer", tc.Answer)
			return "Tool Output: " + tools.AdGuardRewriteDelete(adgCfg, tc.Domain, tc.Answer)
		case "blocked_services_set":
			logger.Info("LLM requested AdGuard set blocked services", "services", tc.Services)
			return "Tool Output: " + tools.AdGuardBlockedServicesSet(adgCfg, tc.Services)
		case "safebrowsing_toggle":
			logger.Info("LLM requested AdGuard safe browsing toggle", "enabled", tc.Enabled)
			return "Tool Output: " + tools.AdGuardSafeBrowsingToggle(adgCfg, tc.Enabled)
		case "parental_toggle":
			logger.Info("LLM requested AdGuard parental toggle", "enabled", tc.Enabled)
			return "Tool Output: " + tools.AdGuardParentalToggle(adgCfg, tc.Enabled)
		case "dhcp_set_config":
			logger.Info("LLM requested AdGuard DHCP set config")
			return "Tool Output: " + tools.AdGuardDHCPSetConfig(adgCfg, tc.Content)
		case "dhcp_add_lease":
			logger.Info("LLM requested AdGuard DHCP add lease", "mac", tc.MAC, "ip", tc.IP)
			return "Tool Output: " + tools.AdGuardDHCPAddLease(adgCfg, tc.MAC, tc.IP, tc.Hostname)
		case "dhcp_remove_lease":
			logger.Info("LLM requested AdGuard DHCP remove lease", "mac", tc.MAC, "ip", tc.IP)
			return "Tool Output: " + tools.AdGuardDHCPRemoveLease(adgCfg, tc.MAC, tc.IP, tc.Hostname)
		case "client_add":
			logger.Info("LLM requested AdGuard client add")
			return "Tool Output: " + tools.AdGuardClientAdd(adgCfg, tc.Content)
		case "client_update":
			logger.Info("LLM requested AdGuard client update")
			return "Tool Output: " + tools.AdGuardClientUpdate(adgCfg, tc.Content)
		case "client_delete":
			logger.Info("LLM requested AdGuard client delete", "name", tc.Name)
			return "Tool Output: " + tools.AdGuardClientDelete(adgCfg, tc.Name)
		case "dns_config":
			logger.Info("LLM requested AdGuard DNS config update")
			return "Tool Output: " + tools.AdGuardDNSConfig(adgCfg, tc.Content)
		default:
			return fmt.Sprintf(`Tool Output: {"status":"error","message":"Unknown adguard operation '%s'. Use: status, stats, stats_top, query_log, query_log_clear, filtering_status, filtering_toggle, filtering_add_url, filtering_remove_url, filtering_refresh, filtering_set_rules, rewrite_list, rewrite_add, rewrite_delete, blocked_services_list, blocked_services_set, safebrowsing_status, safebrowsing_toggle, parental_status, parental_toggle, dhcp_status, dhcp_set_config, dhcp_add_lease, dhcp_remove_lease, clients, client_add, client_update, client_delete, dns_info, dns_config, test_upstream"}`, op)
		}

	default:

		logger.Warn("LLM requested unknown action", "action", tc.Action)
		hint := ""
		switch tc.Action {
		case "firewall", "firewall_rules", "iptables":
			hint = " For firewall rules, use execute_shell with iptables commands (e.g. sudo iptables -L -n)."
		}
		return fmt.Sprintf("Tool Output: ERROR unknown action '%s'.%s Available actions are listed in the tool schema.", tc.Action, hint)
	}
}
