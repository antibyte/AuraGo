package agent

import (
	"context"
	"database/sql"
	"encoding/json"
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
	"aurago/internal/sqlconnections"
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

	voiceModeEnabled bool
	muVoiceMode      sync.Mutex
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

// SetVoiceMode enables or disables voice output mode (TTS auto-play / voice notes).
func SetVoiceMode(enabled bool) {
	muVoiceMode.Lock()
	defer muVoiceMode.Unlock()
	voiceModeEnabled = enabled
}

// GetVoiceMode returns whether voice output mode is currently active.
func GetVoiceMode() bool {
	muVoiceMode.Lock()
	defer muVoiceMode.Unlock()
	return voiceModeEnabled
}

// ToggleVoiceMode flips the current voice mode state and returns the new value.
func ToggleVoiceMode() bool {
	muVoiceMode.Lock()
	defer muVoiceMode.Unlock()
	voiceModeEnabled = !voiceModeEnabled
	return voiceModeEnabled
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

// StringOrArray is a ToolCall field type that accepts both a JSON string
// and a JSON array (the LLM sometimes sends _todo as ["item1","item2",...]
// instead of a plain string, which would cause json.Unmarshal to fail and
// break tool-call detection entirely).
type StringOrArray string

func (s *StringOrArray) UnmarshalJSON(data []byte) error {
	// Try plain string first.
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = StringOrArray(str)
		return nil
	}
	// Fall back to array-of-anything and join to a newline-separated string.
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		parts := make([]string, 0, len(arr))
		for _, v := range arr {
			parts = append(parts, fmt.Sprintf("%v", v))
		}
		*s = StringOrArray(strings.Join(parts, "\n"))
		return nil
	}
	// Last resort: store raw JSON.
	*s = StringOrArray(string(data))
	return nil
}

// ToolCall represents a parsed tool invocation from the LLM.
type ToolCall struct {
	Action              string                   `json:"action"`
	SubOperation        string                   `json:"sub_operation"`
	Code                string                   `json:"code"`
	Key                 string                   `json:"key"`
	Value               string                   `json:"value"`
	Name                string                   `json:"name"`
	Description         string                   `json:"description"`
	Package             string                   `json:"package"`
	Args                interface{}              `json:"args"`
	Background          bool                     `json:"background"`
	PID                 int                      `json:"pid"`
	IsTool              bool                     `json:"-"`
	RawCodeDetected     bool                     `json:"-"`
	XMLFallbackDetected bool                     `json:"-"` // Model used proprietary XML (e.g. minimax:tool_call) instead of native API
	RawJSON             string                   `json:"-"`
	NativeCallID        string                   `json:"-"` // Native API tool call ID for role=tool responses
	NativeArgsMalformed bool                     `json:"-"`
	NativeArgsError     string                   `json:"-"`
	NativeArgsRaw       string                   `json:"-"`
	Todo                StringOrArray            `json:"_todo,omitempty"` // Session-scoped task list piggybacked on every tool call
	Operation           string                   `json:"operation"`
	Fact                string                   `json:"fact"`
	ID                  string                   `json:"id"`
	CronExpr            string                   `json:"cron_expr"`
	TaskPrompt          string                   `json:"task_prompt"`
	EventType           string                   `json:"event_type"`
	Skill               string                   `json:"skill"`
	SkillID             string                   `json:"skill_id"`
	SkillArgs           map[string]interface{}   `json:"skill_args"`
	Content             string                   `json:"content"`
	Query               string                   `json:"query"` // Alias for content in query_memory
	Resource            string                   `json:"resource"`
	Mode                string                   `json:"mode"`
	Sources             []string                 `json:"sources"`         // Memory sources filter for query_memory/context_memory
	Scope               string                   `json:"scope"`           // Scope for memory_reflect
	ContextDepth        string                   `json:"context_depth"`   // Depth for context_memory (shallow, normal, deep)
	TimeRange           string                   `json:"time_range"`      // Temporal window for context_memory
	IncludeRelated      bool                     `json:"include_related"` // Expand contextual relationships in context_memory
	Metadata            map[string]interface{}   `json:"metadata"`
	FilePath            string                   `json:"file_path"`
	Path                string                   `json:"path"` // Alias for file_path
	Destination         string                   `json:"destination"`
	Dest                string                   `json:"dest"` // Alias for destination
	Items               []map[string]interface{} `json:"items"`
	URL                 string                   `json:"url"`
	Method              string                   `json:"method"`
	Headers             map[string]string        `json:"headers"`
	Params              map[string]interface{}   `json:"params"`
	WebhookName         string                   `json:"webhook_name"`
	Parameters          interface{}              `json:"parameters"`
	PayloadType         string                   `json:"payload_type"`
	BodyTemplate        string                   `json:"body_template"`
	Tag                 string                   `json:"tag"`
	Hostname            string                   `json:"hostname"`
	Host                string                   `json:"host"`       // generic target host (ping, etc.)
	Selector            string                   `json:"selector"`   // CSS selector (web_capture)
	OutputDir           string                   `json:"output_dir"` // output directory override
	ServerID            string                   `json:"server_id"`
	MemoryKey           string                   `json:"memory_key"`   // Synonym for fact
	MemoryValue         string                   `json:"memory_value"` // Synonym for fact/content
	NotifyOnCompletion  bool                     `json:"notify_on_completion"`
	Body                string                   `json:"body"`
	Source              string                   `json:"source"`
	Target              string                   `json:"target"`
	Relation            string                   `json:"relation"`
	Properties          map[string]string        `json:"properties"`
	NewRelation         string                   `json:"new_relation"`
	Depth               int                      `json:"depth"`
	Preview             bool                     `json:"preview"`
	Port                int                      `json:"port"`
	Username            string                   `json:"username"`
	Password            string                   `json:"password"`
	Owner               string                   `json:"owner"`
	PrivateKeyPath      string                   `json:"private_key_path"`
	Tags                string                   `json:"tags"`
	Direction           string                   `json:"direction"`
	LocalPath           string                   `json:"local_path"`
	RemotePath          string                   `json:"remote_path"`
	ToolName            string                   `json:"tool_name"`
	Tool                string                   `json:"tool"`         // Hallucination fallback
	ToolCallAction      string                   `json:"tool_call"`    // MiniMax format: {"tool_call": "action_name"}
	Arguments           interface{}              `json:"arguments"`    // Hallucination fallback
	ActionInput         map[string]interface{}   `json:"action_input"` // LangChain-style nested params
	Label               string                   `json:"label"`
	ArtifactType        string                   `json:"artifact_type"`
	Command             string                   `json:"command"`
	ThresholdLow        int                      `json:"threshold_low"`
	ThresholdMedium     int                      `json:"threshold_medium"`
	Pinned              bool                     `json:"pinned"`
	Locked              bool                     `json:"locked"`
	IPAddress           string                   `json:"ip_address"`
	To                  string                   `json:"to"`
	CC                  string                   `json:"cc"`
	Subject             string                   `json:"subject"`
	Folder              string                   `json:"folder"`
	Limit               int                      `json:"limit"`
	Account             string                   `json:"account"` // email account ID (multi-account)
	ChannelID           string                   `json:"channel_id"`
	Message             string                   `json:"message"`
	// Telnyx fields
	CallControlID string   `json:"call_control_id,omitempty"`
	MaxDigits     int      `json:"max_digits,omitempty"`
	AudioURL      string   `json:"audio_url,omitempty"`
	MediaURLs     []string `json:"media_urls,omitempty"`
	// Notes / To-Do fields
	Title              string `json:"title"`
	Priority           int    `json:"priority"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	IncludeArchived    bool   `json:"include_archived,omitempty"`
	DueDate            string `json:"due_date"`
	Category           string `json:"category"`
	Done               int    `json:"done"` // -1=all, 0=open, 1=done (filter for list)
	TaskID             string `json:"task_id"`
	Result             string `json:"result"`
	Error              string `json:"error"`
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
	Specialist   string   `json:"specialist"` // specialist role for spawn_specialist
	// TTS / Chromecast fields
	Text        string  `json:"text"`
	DeviceAddr  string  `json:"device_addr"`
	DeviceName  string  `json:"device_name"`
	DevicePort  int     `json:"device_port"`
	Volume      float64 `json:"volume"`
	ContentType string  `json:"content_type"`
	Language    string  `json:"language"`
	// MDNS / UPnP fields
	ServiceType string `json:"service_type"`
	Timeout     int    `json:"timeout"`
	// Network scan auto-registration
	AutoRegister      bool     `json:"auto_register,omitempty"`
	RegisterType      string   `json:"register_type,omitempty"`
	RegisterTags      []string `json:"register_tags,omitempty"`
	OverwriteExisting bool     `json:"overwrite_existing,omitempty"`
	// MeshCentral fields
	MeshID      string `json:"mesh_id"`
	NodeID      string `json:"node_id"`
	PowerAction int    `json:"power_action"`
	// Remote Control fields
	DeviceID  string `json:"device_id"`
	Recursive bool   `json:"recursive"`
	FullPage  bool   `json:"full_page"`
	Count     int    `json:"count"`
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
	// Sandbox fields
	SandboxLang string   `json:"sandbox_lang"` // language for execute_sandbox (python, javascript, go, etc.)
	Libraries   []string `json:"libraries"`    // optional packages to install before running sandbox code
	// Vault secret injection for Python tools
	VaultKeys     []string `json:"vault_keys,omitempty"`     // vault secret keys to inject as AURAGO_SECRET_<KEY> env vars
	CredentialIDs []string `json:"credential_ids,omitempty"` // credential UUIDs to inject as AURAGO_CRED_<NAME>_* env vars
	// Homepage fields
	Framework  string   `json:"framework"`   // web framework: next, vite, astro, svelte, vue, html
	Viewport   string   `json:"viewport"`    // screenshot viewport: "1280x720"
	Packages   []string `json:"packages"`    // npm packages to install
	ProjectDir string   `json:"project_dir"` // subdirectory within /workspace
	BuildDir   string   `json:"build_dir"`   // build output directory (auto-detected if empty)
	Template   string   `json:"template"`    // project template: portfolio, blog, landing, dashboard
	AutoFix    bool     `json:"auto_fix"`    // attempt auto-fix on build failure
	GitMessage string   `json:"git_message"` // commit message for git operations
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
	Offset int `json:"offset"` // pagination offset
	// Cheat Sheet fields
	Active       *bool  `json:"active,omitempty"`        // pointer so nil = not provided vs false = explicitly inactive
	AttachmentID string `json:"attachment_id,omitempty"` // attachment ID for cheatsheet attach/detach
	// Media Registry / Homepage Registry fields
	MediaType string `json:"media_type,omitempty"` // image, tts, audio, music
	TagMode   string `json:"tag_mode,omitempty"`   // add, remove, set (for media_registry tag op)
	Reason    string `json:"reason,omitempty"`     // edit reason (homepage_registry log_edit)
	Problem   string `json:"problem,omitempty"`    // problem description (homepage_registry log_problem/resolve_problem)
	Status    string `json:"status,omitempty"`     // project status: active, archived, maintenance
	Notes     string `json:"notes,omitempty"`      // additional notes
	// Document Creator fields
	PaperSize   string `json:"paper_size,omitempty"`   // A4, A3, Letter, Legal
	Landscape   bool   `json:"landscape,omitempty"`    // landscape orientation
	Sections    string `json:"sections,omitempty"`     // JSON array of document sections for Maroto
	SourceFiles string `json:"source_files,omitempty"` // JSON array of file paths for merge/convert
	Filename    string `json:"filename,omitempty"`     // output filename without extension
	// Archive fields
	Format string `json:"format,omitempty"` // zip or tar.gz
	// DNS Lookup fields
	RecordType string `json:"record_type,omitempty"` // A, AAAA, MX, NS, TXT, CNAME, PTR, all
	// Crawler fields
	MaxDepth       int    `json:"max_depth,omitempty"`       // link depth to follow (1-5)
	MaxPages       int    `json:"max_pages,omitempty"`       // max pages to crawl (1-100)
	AllowedDomains string `json:"allowed_domains,omitempty"` // comma-separated domain whitelist
	// Port Scanner fields
	PortRange string `json:"port_range,omitempty"` // e.g. "80", "80,443", "1-1024", "common"
	TimeoutMs int    `json:"timeout_ms,omitempty"` // per-port timeout in milliseconds
	// S3 Storage fields
	Bucket            string `json:"bucket,omitempty"`             // S3 bucket name
	Prefix            string `json:"prefix,omitempty"`             // S3 object key prefix for listing
	DestinationBucket string `json:"destination_bucket,omitempty"` // target bucket (copy/move)
	DestinationKey    string `json:"destination_key,omitempty"`    // target key (copy/move)
	// PDF Operations fields
	OutputFile    string `json:"output_file,omitempty"`    // output file path
	Pages         string `json:"pages,omitempty"`          // page numbers (e.g. "1,3,5" or page range)
	WatermarkText string `json:"watermark_text,omitempty"` // text for PDF watermark
	// Image Processing fields
	Width        int    `json:"width,omitempty"`         // image width in pixels
	Height       int    `json:"height,omitempty"`        // image height in pixels
	QualityPct   int    `json:"quality_pct,omitempty"`   // quality percentage (1-100)
	OutputFormat string `json:"output_format,omitempty"` // target format (png, jpeg, gif, bmp, tiff)
	CropX        int    `json:"crop_x,omitempty"`        // crop start X
	CropY        int    `json:"crop_y,omitempty"`        // crop start Y
	CropWidth    int    `json:"crop_width,omitempty"`    // crop width
	CropHeight   int    `json:"crop_height,omitempty"`   // crop height
	Angle        int    `json:"angle,omitempty"`         // rotation angle (90, 180, 270)
	// WHOIS fields
	IncludeRaw bool `json:"include_raw,omitempty"` // include raw WHOIS response
	// Site Monitor fields
	MonitorID string `json:"monitor_id,omitempty"` // site monitor ID
	Interval  string `json:"interval,omitempty"`   // monitoring interval description
	// Form Automation fields
	Fields        string `json:"fields,omitempty"`         // JSON map of CSS selector → value
	ScreenshotDir string `json:"screenshot_dir,omitempty"` // directory for post-action screenshot
	// UPnP Scan fields
	SearchTarget string `json:"search_target,omitempty"` // UPnP search target (e.g. "ssdp:all")
	TimeoutSecs  int    `json:"timeout_secs,omitempty"`  // discovery timeout in seconds
	DelaySeconds int    `json:"delay_seconds,omitempty"` // delay before a background task starts
	IntervalSecs int    `json:"interval_seconds,omitempty"`
	// Address Book fields
	Email          string `json:"email,omitempty"`        // contact email
	Phone          string `json:"phone,omitempty"`        // contact phone number
	Mobile         string `json:"mobile,omitempty"`       // contact mobile number
	ContactAddress string `json:"address,omitempty"`      // contact postal address
	Relationship   string `json:"relationship,omitempty"` // contact relationship type
	// SQL Connections fields
	ConnectionName string `json:"connection_name,omitempty"` // target database connection name
	SQLQuery       string `json:"sql_query,omitempty"`       // SQL statement to execute
	TableName      string `json:"table_name,omitempty"`      // table name for describe operations
	DatabaseName   string `json:"database_name,omitempty"`   // database name or sqlite file path
	SSLMode        string `json:"ssl_mode,omitempty"`        // SSL mode: disable, require, verify-ca, verify-full
	AllowRead      *bool  `json:"allow_read,omitempty"`      // permission: SELECT
	AllowWrite     *bool  `json:"allow_write,omitempty"`     // permission: INSERT
	AllowChange    *bool  `json:"allow_change,omitempty"`    // permission: UPDATE
	AllowDelete    *bool  `json:"allow_delete,omitempty"`    // permission: DELETE
	DockerTemplate string `json:"docker_template,omitempty"` // docker template: postgres, mysql, mariadb
}

func (tc *ToolCall) UnmarshalJSON(data []byte) error {
	// Preserve existing field-based decoding but also build a robust flat args map
	// so tool-specific decoders can read from `tc.Params` only.
	type toolCallAlias ToolCall

	// Start from the current state so absent JSON fields do not zero-out values that
	// were set programmatically (e.g. native-tool shortcut prefill).
	decoded := toolCallAlias(*tc)
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*tc = ToolCall(decoded)

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		// If raw parsing fails, keep the typed decode and leave Params as-is.
		return nil
	}

	merged := make(map[string]interface{})
	// Start with the explicit `params` object if present.
	if rawParams, ok := raw["params"].(map[string]interface{}); ok {
		for k, v := range rawParams {
			merged[k] = v
		}
	}
	// Then merge in any already-decoded Params (e.g. native call path).
	for k, v := range tc.Params {
		merged[k] = v
	}
	// Finally overlay all top-level keys (excluding the params container itself).
	for k, v := range raw {
		if k == "params" {
			continue
		}
		merged[k] = v
	}

	tc.Params = merged
	return nil
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
	ContactsDB         *sql.DB
	PlannerDB          *sql.DB
	SQLConnectionsDB   *sql.DB
	SQLConnectionPool  *sqlconnections.ConnectionPool
	RemoteHub          *remote.RemoteHub
	Vault              *security.Vault
	Registry           *tools.ProcessRegistry
	Manifest           *tools.Manifest
	CronManager        *tools.CronManager
	MissionManagerV2   *tools.MissionManagerV2
	CoAgentRegistry    *CoAgentRegistry
	BudgetTracker      *budget.Tracker
	DaemonSupervisor   *tools.DaemonSupervisor
	LLMGuardian        *security.LLMGuardian
	SessionID          string
	IsMaintenance      bool
	SurgeryPlan        string
	IsMission          bool   // true when triggered by a mission (skips RAG, personality, profiling)
	MissionID          string // mission ID for logging/tracking
	MessageSource      string // origin channel: "web_chat", "telegram", "discord", "a2a", "sms", "mission"
	VoiceOutputActive  bool   // true when the user's speaker toggle is on
}

func dispatchInner(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	sessionID := dc.SessionID
	logger := dc.Logger

	// Co-Agent blacklist: co-agents (identified by sessionID prefix) cannot access secrets,
	// mutate memory-like stores, or orchestrate additional autonomous work.
	isCoAgent := strings.HasPrefix(sessionID, "coagent-") || strings.HasPrefix(sessionID, "specialist-")
	if isCoAgent {
		switch tc.Action {
		case "manage_memory", "core_memory":
			if tc.Operation != "read" && tc.Operation != "query" && tc.Operation != "" {
				return `Tool Output: {"status": "error", "message": "Co-Agents cannot modify memory. Only read/query operations are allowed."}`
			}
		case "remember":
			return `Tool Output: {"status": "error", "message": "Co-Agents cannot store new facts or memories."}`
		case "manage_knowledge", "knowledge_graph":
			if tc.Operation != "query" && tc.Operation != "search" && tc.Operation != "get" && tc.Operation != "get_node" && tc.Operation != "get_neighbors" && tc.Operation != "subgraph" && tc.Operation != "" {
				return `Tool Output: {"status": "error", "message": "Co-Agents cannot modify the knowledge graph. Only read operations are allowed."}`
			}
		case "get_secret", "secrets_vault":
			return `Tool Output: {"status": "error", "message": "Co-Agents cannot access the secrets vault."}`
		case "manage_notes", "notes", "todo":
			if tc.Operation != "list" {
				return `Tool Output: {"status": "error", "message": "Co-Agents cannot modify notes. Only 'list' is allowed."}`
			}
		case "manage_journal", "journal":
			if tc.Operation != "list" && tc.Operation != "search" && tc.Operation != "get_summary" {
				return `Tool Output: {"status": "error", "message": "Co-Agents cannot modify journal entries. Only list, search, and get_summary are allowed."}`
			}
		case "manage_plan":
			if tc.Operation != "list" && tc.Operation != "get" {
				return `Tool Output: {"status": "error", "message": "Co-Agents cannot modify plans. Only list and get are allowed."}`
			}
		case "manage_appointments":
			if tc.Operation != "list" && tc.Operation != "get" {
				return `Tool Output: {"status": "error", "message": "Co-Agents cannot modify appointments. Only list and get are allowed."}`
			}
		case "manage_todos":
			if tc.Operation != "list" && tc.Operation != "get" {
				return `Tool Output: {"status": "error", "message": "Co-Agents cannot modify todos. Only list and get are allowed."}`
			}
		case "co_agent", "co_agents":
			return `Tool Output: {"status": "error", "message": "Co-Agents cannot spawn sub-agents."}`
		case "follow_up":
			return `Tool Output: {"status": "error", "message": "Co-Agents cannot schedule follow-ups."}`
		case "wait_for_event":
			return `Tool Output: {"status": "error", "message": "Co-Agents cannot schedule wait events."}`
		case "cron_scheduler":
			return `Tool Output: {"status": "error", "message": "Co-Agents cannot manage cron jobs."}`
		case "manage_daemon":
			if tc.Operation != "list" && tc.Operation != "status" {
				return `Tool Output: {"status": "error", "message": "Co-Agents cannot control daemons. Only list and status are allowed."}`
			}
		}
	}

	// Specialist-specific tool restrictions (additional to the generic co-agent blacklist)
	specialistRole := extractSpecialistRole(sessionID)
	if specialistRole != "" {
		if blocked := checkSpecialistToolRestriction(specialistRole, tc.Action, tc.Operation); blocked != "" {
			return blocked
		}
	}

	// Route to sub-dispatchers
	if result, ok := dispatchExec(ctx, tc, dc); ok {
		return result
	}
	if result, ok := dispatchComm(ctx, tc, dc); ok {
		return result
	}
	if result, ok := dispatchServices(ctx, tc, dc); ok {
		return result
	}
	if result, ok := dispatchInfra(ctx, tc, dc); ok {
		return result
	}

	// Alias resolution: LLMs (especially reasoning models) sometimes call homepage
	// sub-operations as top-level actions. Redirect transparently before failing.
	homepageSubOps := map[string]bool{
		"optimize_images": true, "test_connection": true, "screenshot": true,
		"lighthouse": true, "tunnel": true, "dev": true, "build": true,
		"rebuild": true, "install_deps": true, "lint": true, "webserver_start": true,
		"webserver_stop": true, "webserver_status": true, "init_project": true,
		"edit_file": true, "json_edit": true, "yaml_edit": true, "xml_edit": true,
		"deploy_netlify": true, "publish_local": true, "deploy": true,
		"list_files": true, "read_file": true, "write_file": true,
	}
	if homepageSubOps[tc.Action] {
		logger.Info("Redirecting direct sub-operation call to homepage tool", "action", tc.Action)
		if tc.Action == "deploy" {
			tc.Operation = "deploy"
		} else {
			tc.Operation = tc.Action
		}
		tc.Action = "homepage"
		if result, ok := dispatchServices(ctx, tc, dc); ok {
			return result
		}
	}
	// Redirect top-level git commands to execute_shell
	gitAliases := map[string]string{
		"git_status":   "git status",
		"git_log":      "git log --oneline -10",
		"git_diff":     "git diff",
		"git_commit":   "git add -A && git commit -m \"" + tc.Message + "\"",
		"git_push":     "git push",
		"git_pull":     "git pull",
		"git_rollback": "git reset --hard HEAD~1",
	}
	if gitCmd, ok := gitAliases[tc.Action]; ok {
		logger.Info("Redirecting git alias to execute_shell", "action", tc.Action)
		if tc.Path != "" {
			tc.Command = "cd " + tc.Path + " && " + gitCmd
		} else {
			tc.Command = gitCmd
		}
		tc.Action = "execute_shell"
		if result, ok := dispatchExec(ctx, tc, dc); ok {
			return result
		}
	}
	// Redirect bare "exec" to execute_shell when a command is provided
	if tc.Action == "exec" && tc.Command != "" {
		logger.Info("Redirecting exec alias to execute_shell")
		tc.Action = "execute_shell"
		if result, ok := dispatchExec(ctx, tc, dc); ok {
			return result
		}
	}

	logger.Warn("LLM requested unknown action", "action", tc.Action)
	hint := ""
	switch tc.Action {
	case "exec":
		hint = " Use execute_shell with a 'command' field to run shell commands."
	case "git", "git_status", "git_log", "git_diff", "git_commit", "git_push", "git_pull":
		hint = " Use execute_shell with the git command in the 'command' field."
	}
	return fmt.Sprintf("Tool Output: ERROR unknown action '%s'.%s Available actions are listed in the tool schema.", tc.Action, hint)
}
