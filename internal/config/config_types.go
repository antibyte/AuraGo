package config

import "gopkg.in/yaml.v3"

// Defined here to avoid a circular dependency on the security package.
type SecretReader interface {
	ReadSecret(key string) (string, error)
}

// SecretWriter is the write side of the vault, used for one-time migrations.
type SecretWriter interface {
	WriteSecret(key, value string) error
}

// SecretReadWriter combines read and write vault access.
type SecretReadWriter interface {
	SecretReader
	SecretWriter
}

// OAuthToken represents an OAuth2 token stored in the vault.
type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Expiry       string `json:"expiry"` // RFC3339
}

// IndexingDirectory describes a single directory to index with an optional custom collection name.
// When Collection is empty, the default collection "file_index" is used.
type IndexingDirectory struct {
	Path       string `yaml:"path"`
	Collection string `yaml:"collection"` // optional; empty = default "file_index"
}

// SpecialistConfig holds per-role configuration for a specialist co-agent.
// Empty LLM.Provider inherits from co_agents.llm, which in turn falls back to the main LLM.
type SpecialistConfig struct {
	Enabled          bool   `yaml:"enabled"`
	AdditionalPrompt string `yaml:"additional_prompt,omitempty"`
	CheatsheetID     string `yaml:"cheatsheet_id,omitempty"`
	LLM              struct {
		Provider     string `yaml:"provider"`         // provider entry ID (empty = inherit co_agents.llm)
		ProviderType string `yaml:"-"       json:"-"` // resolved
		BaseURL      string `yaml:"-"       json:"-"` // resolved
		APIKey       string `yaml:"-"       json:"-"` // resolved
		Model        string `yaml:"-"       json:"-"` // resolved
	} `yaml:"llm"`
	CircuitBreaker struct {
		MaxToolCalls   int `yaml:"max_tool_calls"`  // 0 = inherit co_agents value
		TimeoutSeconds int `yaml:"timeout_seconds"` // 0 = inherit co_agents value
		MaxTokens      int `yaml:"max_tokens"`      // 0 = inherit co_agents value
	} `yaml:"circuit_breaker"`
}

// ValidSpecialistRoles lists all recognized specialist role names.
var ValidSpecialistRoles = map[string]bool{
	"researcher": true,
	"coder":      true,
	"designer":   true,
	"security":   true,
	"writer":     true,
}

// ProviderEntry defines a named LLM provider connection that can be referenced
// by multiple config slots (LLM, Fallback, Vision, Whisper, Embeddings, etc.).
type ProviderEntry struct {
	ID      string `yaml:"id"       json:"id"`         // unique slug, e.g. "main", "vision", "local-ollama"
	Name    string `yaml:"name"     json:"name"`       // human-readable label shown in UI
	Type    string `yaml:"type"     json:"type"`       // openai, openrouter, ollama, anthropic, google, custom
	BaseURL string `yaml:"base_url" json:"base_url"`   // API base URL
	APIKey  string `yaml:"-" vault:"api_key" json:"-"` // API key (vault-only)
	Model   string `yaml:"model"    json:"model"`      // default model name

	// Cloudflare Workers AI — required when Type is "workers-ai"
	AccountID string `yaml:"account_id,omitempty" json:"account_id"` // Cloudflare account ID

	// OAuth2 Authorization Code flow (optional, alternative to static API key)
	AuthType          string `yaml:"auth_type,omitempty"           json:"auth_type"`       // "api_key" (default) or "oauth2"
	OAuthAuthURL      string `yaml:"oauth_auth_url,omitempty"      json:"oauth_auth_url"`  // authorization endpoint
	OAuthTokenURL     string `yaml:"oauth_token_url,omitempty"     json:"oauth_token_url"` // token exchange endpoint
	OAuthClientID     string `yaml:"oauth_client_id,omitempty"     json:"oauth_client_id"` // client ID
	OAuthClientSecret string `yaml:"-" vault:"oauth_client_secret" json:"-"`               // client secret (vault-only)
	OAuthScopes       string `yaml:"oauth_scopes,omitempty"        json:"oauth_scopes"`    // space-separated scopes

	// Per-provider model cost overrides (used by budget tracker)
	Models []ModelCost `yaml:"models,omitempty" json:"models,omitempty"`
}

// EmailAccount defines a single IMAP/SMTP email account.
type EmailAccount struct {
	ID            string `yaml:"id"             json:"id"`   // unique slug, e.g. "personal", "work"
	Name          string `yaml:"name"           json:"name"` // human-readable label
	IMAPHost      string `yaml:"imap_host"      json:"imap_host"`
	IMAPPort      int    `yaml:"imap_port"      json:"imap_port"`
	SMTPHost      string `yaml:"smtp_host"      json:"smtp_host"`
	SMTPPort      int    `yaml:"smtp_port"      json:"smtp_port"`
	Username      string `yaml:"username"       json:"username"`
	Password      string `yaml:"-"              json:"-"` // excluded from YAML (secret)
	FromAddress   string `yaml:"from_address"   json:"from_address"`
	WatchEnabled  bool   `yaml:"watch_enabled"  json:"watch_enabled"`
	WatchInterval int    `yaml:"watch_interval_seconds" json:"watch_interval_seconds"`
	WatchFolder   string `yaml:"watch_folder"   json:"watch_folder"`
	Disabled      bool   `yaml:"disabled"       json:"-"` // if true, account is inactive (default false = active)
	ReadOnly      bool   `yaml:"readonly"       json:"-"` // if true, only fetch/read, block send
}

type WebhookParameter struct {
	Name        string `yaml:"name" json:"name"`               // e.g. "message", "user_id"
	Type        string `yaml:"type" json:"type"`               // "string", "number", "boolean"
	Description string `yaml:"description" json:"description"` // Crucial for LLM
	Required    bool   `yaml:"required" json:"required"`
}

type OutgoingWebhook struct {
	ID           string             `yaml:"id" json:"id"`
	Name         string             `yaml:"name" json:"name"`               // Agent uses this name
	Description  string             `yaml:"description" json:"description"` // Tells agent when to use it
	Method       string             `yaml:"method" json:"method"`           // GET, POST, PUT, DELETE
	URL          string             `yaml:"url" json:"url"`                 // Can contain {{variables}}
	Headers      map[string]string  `yaml:"headers" json:"headers"`
	Parameters   []WebhookParameter `yaml:"parameters" json:"parameters"`
	PayloadType  string             `yaml:"payload_type" json:"payload_type"`   // "json", "form", "custom"
	BodyTemplate string             `yaml:"body_template" json:"body_template"` // Context for custom templating
}

// GotenbergConfig holds connection parameters for the Gotenberg Docker sidecar.
type GotenbergConfig struct {
	URL     string `yaml:"url"`     // Gotenberg API URL (default: "http://gotenberg:3000")
	Timeout int    `yaml:"timeout"` // request timeout in seconds (default: 120)
}

// DocumentCreatorConfig holds settings for the document_creator tool.
type DocumentCreatorConfig struct {
	Enabled   bool            `yaml:"enabled"`    // enable document_creator tool
	Backend   string          `yaml:"backend"`    // "maroto" (default, built-in) or "gotenberg" (Docker sidecar)
	OutputDir string          `yaml:"output_dir"` // directory for generated documents (default: "data/documents")
	Gotenberg GotenbergConfig `yaml:"gotenberg"`
}

// MediaConversionConfig holds settings for the media_conversion tool.
type MediaConversionConfig struct {
	Enabled         bool   `yaml:"enabled"`           // enable media_conversion tool
	ReadOnly        bool   `yaml:"readonly"`          // block conversion writes when true; info remains allowed
	FFmpegPath      string `yaml:"ffmpeg_path"`       // optional ffmpeg binary path override
	ImageMagickPath string `yaml:"imagemagick_path"`  // optional ImageMagick binary path override
	TimeoutSeconds  int    `yaml:"timeout_seconds"`   // per-conversion timeout in seconds
}

// BrowserAutomationViewport defines the browser viewport used for new sessions.
type BrowserAutomationViewport struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
}

// BrowserAutomationConfig holds the settings for the optional Playwright sidecar.
type BrowserAutomationConfig struct {
	Enabled            bool                      `yaml:"enabled"`              // enable browser automation integration
	Mode               string                    `yaml:"mode"`                 // "sidecar" (default)
	URL                string                    `yaml:"url"`                  // sidecar base URL
	ContainerName      string                    `yaml:"container_name"`       // managed Docker container name
	Image              string                    `yaml:"image"`                // sidecar Docker image
	AutoStart          bool                      `yaml:"auto_start"`           // auto-start the sidecar container when enabled
	AutoBuild          bool                      `yaml:"auto_build"`           // auto-build the sidecar image when missing locally
	DockerfileDir      string                    `yaml:"dockerfile_dir"`       // build context containing Dockerfile.browser_automation
	SessionTTLMinutes  int                       `yaml:"session_ttl_minutes"`  // session expiry in minutes
	MaxSessions        int                       `yaml:"max_sessions"`         // max concurrent sessions
	AllowFileUploads   bool                      `yaml:"allow_file_uploads"`   // allow upload_file operation
	AllowFileDownloads bool                      `yaml:"allow_file_downloads"` // allow browser downloads
	AllowedDownloadDir string                    `yaml:"allowed_download_dir"` // host/workspace dir for downloads
	Viewport           BrowserAutomationViewport `yaml:"viewport"`             // default viewport for sessions
	Headless           bool                      `yaml:"headless"`             // run browser headless
	ReadOnly           bool                      `yaml:"readonly"`             // block mutating actions when true
	ScreenshotsDir     string                    `yaml:"screenshots_dir"`      // workspace-relative screenshot directory
}

// MQTTTLS holds TLS configuration for MQTT connections.
type MQTTTLS struct {
	Enabled            bool   `yaml:"enabled"`              // enable TLS encryption
	CAFile             string `yaml:"ca_file"`              // path to CA certificate file
	CertFile           string `yaml:"cert_file"`            // path to client certificate file
	KeyFile            string `yaml:"key_file"`             // path to client key file
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"` // skip TLS certificate verification (for testing only)
}

// MQTTBuffer holds message buffer configuration for MQTT.
type MQTTBuffer struct {
	MaxMessages int `yaml:"max_messages"`  // max messages to buffer (default: 500, 0 = use default)
	MaxAgeHours int `yaml:"max_age_hours"` // max age of messages in hours before cleanup (0 = disabled)
}

// HeartbeatTimeWindow defines a single time window with start/end time and interval.
type HeartbeatTimeWindow struct {
	Start    string `yaml:"start"`    // Format "HH:MM", e.g. "08:00"
	End      string `yaml:"end"`      // Format "HH:MM", e.g. "22:00"
	Interval string `yaml:"interval"` // "15m", "30m", "1h", "2h", "4h", "6h", "12h"
}

// HeartbeatConfig holds settings for the background wake-up scheduler.
type HeartbeatConfig struct {
	Enabled           bool                `yaml:"enabled"`
	CheckTasks        bool                `yaml:"check_tasks"`
	CheckAppointments bool                `yaml:"check_appointments"`
	CheckEmails       bool                `yaml:"check_emails"`
	AdditionalPrompt  string              `yaml:"additional_prompt,omitempty"`
	DayTimeWindow     HeartbeatTimeWindow `yaml:"day_time_window"`
	NightTimeWindow   HeartbeatTimeWindow `yaml:"night_time_window"`
}

type Config struct {
	ConfigPath    string          `yaml:"-"`          // runtime-only: absolute path to the config file
	Runtime       Runtime         `yaml:"-" json:"-"` // runtime-only: detected environment capabilities
	Providers     []ProviderEntry `yaml:"providers"`
	EmailAccounts []EmailAccount  `yaml:"email_accounts"`
	Server        struct {
		Host                 string `yaml:"host"`
		Port                 int    `yaml:"port"`
		BridgeAddress        string `yaml:"bridge_address"`
		MaxBodyBytes         int64  `yaml:"max_body_bytes"`
		UILanguage           string `yaml:"ui_language"`
		OAuthRedirectBaseURL string `yaml:"oauth_redirect_base_url"` // override for OAuth callback (e.g. http://localhost:8088)
		MasterKey            string `yaml:"-"`                       // ENV-only (AURAGO_MASTER_KEY)
		HTTPS                struct {
			Enabled     bool   `yaml:"enabled"`
			CertMode    string `yaml:"cert_mode"` // "auto" (Let's Encrypt), "custom" (uploaded cert), "selfsigned" (auto-generated)
			Domain      string `yaml:"domain"`
			Email       string `yaml:"email"`
			CertFile    string `yaml:"cert_file"`    // custom mode: path to PEM certificate
			KeyFile     string `yaml:"key_file"`     // custom mode: path to PEM private key
			HTTPSPort   int    `yaml:"https_port"`   // default: 443
			HTTPPort    int    `yaml:"http_port"`    // default: 80 (for redirect)
			BehindProxy bool   `yaml:"behind_proxy"` // trust X-Forwarded-* headers
		} `yaml:"https"`
	} `yaml:"server"`
	LLM struct {
		Provider                     string   `yaml:"provider"`           // provider entry ID (references Providers[].ID)
		ProviderType                 string   `yaml:"-"       json:"-"`   // resolved: openai, openrouter, ollama etc.
		BaseURL                      string   `yaml:"-"       json:"-"`   // resolved from provider entry
		APIKey                       string   `yaml:"-"       json:"-"`   // resolved from provider entry
		Model                        string   `yaml:"-"       json:"-"`   // resolved from provider entry
		AccountID                    string   `yaml:"-"       json:"-"`   // resolved from provider entry (workers-ai)
		LegacyURL                    string   `yaml:"base_url" json:"-"`  // legacy/compat: inline base URL from old config format
		LegacyAPIKey                 string   `yaml:"api_key"  json:"-"`  // legacy/compat: inline API key from old config format
		LegacyModel                  string   `yaml:"model"    json:"-"`  // legacy/compat: inline model from old config format
		HelperEnabled                bool     `yaml:"helper_enabled"`     // enable the dedicated helper LLM for internal analysis/background tasks
		HelperProvider               string   `yaml:"helper_provider"`    // provider entry ID for helper/background LLM tasks
		HelperProviderType           string   `yaml:"-"         json:"-"` // resolved helper provider type
		HelperBaseURL                string   `yaml:"-"         json:"-"` // resolved helper base URL
		HelperAPIKey                 string   `yaml:"-"         json:"-"` // resolved helper API key
		HelperAccountID              string   `yaml:"-"         json:"-"` // resolved helper account ID (workers-ai)
		HelperModel                  string   `yaml:"helper_model"`       // optional helper model override (empty = provider default)
		HelperResolvedModel          string   `yaml:"-"         json:"-"` // resolved helper model
		UseNativeFunctions           bool     `yaml:"use_native_functions"`
		Temperature                  float64  `yaml:"temperature"`                     // 0.0–2.0; default 0.7; 0 = provider default
		Multimodal                   bool     `yaml:"multimodal"`                      // enable image inputs (MultiContent) in the main chat loop
		MultimodalProviderTypesExtra []string `yaml:"multimodal_provider_types_extra"` // extra provider types treated as multimodal-capable (in addition to built-ins)
		StructuredOutputs            bool     `yaml:"structured_outputs"`              // enable structured output mode (only for supported models)
		AnthropicThinking            struct {
			Enabled        bool     `yaml:"enabled"`
			BudgetTokens   int      `yaml:"budget_tokens"`
			ModelAllowlist []string `yaml:"model_allowlist"`
		} `yaml:"anthropic_thinking"`
	} `yaml:"llm"`
	Directories struct {
		DataDir      string `yaml:"data_dir"`
		WorkspaceDir string `yaml:"workspace_dir"`
		ToolsDir     string `yaml:"tools_dir"`
		PromptsDir   string `yaml:"prompts_dir"`
		SkillsDir    string `yaml:"skills_dir"`
		VectorDBDir  string `yaml:"vectordb_dir"`
	} `yaml:"directories"`
	SQLite struct {
		ShortTermPath        string `yaml:"short_term_path"`
		LongTermPath         string `yaml:"long_term_path"`
		InventoryPath        string `yaml:"inventory_path"`
		InvasionPath         string `yaml:"invasion_path"`
		CheatsheetPath       string `yaml:"cheatsheet_path"`
		ImageGalleryPath     string `yaml:"image_gallery_path"`
		RemoteControlPath    string `yaml:"remote_control_path"`
		MediaRegistryPath    string `yaml:"media_registry_path"`
		HomepageRegistryPath string `yaml:"homepage_registry_path"`
		ContactsPath         string `yaml:"contacts_path"`
		PlannerPath          string `yaml:"planner_path"`
		SiteMonitorPath      string `yaml:"site_monitor_path"`
		SQLConnectionsPath   string `yaml:"sql_connections_path"`
		SkillsPath           string `yaml:"skills_path"`
		KnowledgeGraphPath   string `yaml:"knowledge_graph_path"`
		OptimizationPath     string `yaml:"optimization_path"`
		PreparedMissionsPath string `yaml:"prepared_missions_path"`
		MissionHistoryPath   string `yaml:"mission_history_path"`
		PushPath             string `yaml:"push_path"`
	} `yaml:"sqlite"`
	Embeddings struct {
		Provider         string `yaml:"provider"`          // "disabled" or provider entry ID
		ProviderType     string `yaml:"-"       json:"-"`  // resolved
		BaseURL          string `yaml:"-"       json:"-"`  // resolved from provider
		APIKey           string `yaml:"-"       json:"-"`  // resolved from provider
		Model            string `yaml:"-"       json:"-"`  // resolved from provider
		InternalModel    string `yaml:"internal_model"`    // legacy/compat: model when using main LLM provider
		ExternalURL      string `yaml:"external_url"`      // legacy/compat: dedicated endpoint URL
		ExternalModel    string `yaml:"external_model"`    // legacy/compat: dedicated endpoint model
		LegacyAPIKey     string `yaml:"api_key"  json:"-"` // legacy/compat: separate API key
		Multimodal       bool   `yaml:"multimodal"`        // enable multimodal embeddings (images, audio)
		MultimodalFormat string `yaml:"multimodal_format"` // "auto", "openai", "vertex" — API format for multimodal
		LocalOllama      struct {
			Enabled       bool   `yaml:"enabled"`        // auto-manage an Ollama container for local embeddings
			Model         string `yaml:"model"`          // embedding model (default: "nomic-embed-text")
			ContainerPort int    `yaml:"container_port"` // host port for the managed container (default: 11435)
			UseHostGPU    bool   `yaml:"use_host_gpu"`   // pass GPU devices into the container (Linux only)
			GPUBackend    string `yaml:"gpu_backend"`    // "auto", "nvidia", "amd", "intel", "vulkan" (default: "auto")
		} `yaml:"local_ollama"`
	} `yaml:"embeddings"`
	Agent struct {
		SystemLanguage                  string `yaml:"system_language"`
		StepDelaySeconds                int    `yaml:"step_delay_seconds"`
		MemoryCompressionCharLimit      int    `yaml:"memory_compression_char_limit"`
		SystemPromptTokenBudget         int    `yaml:"system_prompt_token_budget"`
		SystemPromptTokenBudgetAuto     bool   `yaml:"-"`                                   // true when user set budget to 0 (automatic mode)
		AdaptiveSystemPromptTokenBudget bool   `yaml:"adaptive_system_prompt_token_budget"` // adapt system prompt token budget to enabled tools/integrations (default: true)
		OptimizerEnabled                bool   `yaml:"optimizer_enabled"`
		ContextWindow                   int    `yaml:"context_window"`
		ShowToolResults                 bool   `yaml:"show_tool_results"`
		WorkflowFeedback                bool   `yaml:"workflow_feedback"`
		DebugMode                       bool   `yaml:"debug_mode"`
		CoreMemoryMaxEntries            int    `yaml:"core_memory_max_entries"` // 0 = unlimited; default 200
		CoreMemoryCapMode               string `yaml:"core_memory_cap_mode"`    // "soft" (default) | "hard"
		ToolOutputLimit                 int    `yaml:"tool_output_limit"`       // max characters of a single tool result added to context (0 = unlimited, default: 50000)
		SudoEnabled                     bool   `yaml:"sudo_enabled"`            // allow execute_sudo tool (password must be stored in vault as "sudo_password")
		SudoUnrestricted                bool   `yaml:"sudo_unrestricted"`       // allow sudo to write outside the install directory (requires removing ProtectSystem=strict from systemd unit)
		// ── Danger Zone: tool capability gates (all default true) ──
		AllowShell           bool   `yaml:"allow_shell"`            // allow execute_shell
		AllowPython          bool   `yaml:"allow_python"`           // allow execute_python / save_tool / execute_skill
		AllowFilesystemWrite bool   `yaml:"allow_filesystem_write"` // allow filesystem write operations
		AllowNetworkRequests bool   `yaml:"allow_network_requests"` // allow api_request
		AllowRemoteShell     bool   `yaml:"allow_remote_shell"`     // allow execute_remote_shell
		AllowSelfUpdate      bool   `yaml:"allow_self_update"`      // allow manage_updates
		AllowMCP             bool   `yaml:"allow_mcp"`              // allow MCP (Model Context Protocol) server connections
		AllowWebScraper      *bool  `yaml:"allow_web_scraper"`      // deprecated: migrated to tools.web_scraper.enabled
		AdditionalPrompt     string `yaml:"additional_prompt"`      // extra instructions always appended to the system prompt
		AdaptiveTools        struct {
			Enabled                   bool     `yaml:"enabled"`                      // enable adaptive tool filtering (default: false)
			MaxTools                  int      `yaml:"max_tools"`                    // maximum tool schemas to send to LLM (0 = unlimited, default: 60)
			DecayHalfLifeDays         float64  `yaml:"decay_half_life_days"`         // usage score halves after this many days (default: 7)
			AlwaysInclude             []string `yaml:"always_include"`               // tools always included regardless of usage
			WeightSuccessRate         bool     `yaml:"weight_success_rate"`          // penalise tools with low success rate in scoring (default: true)
			CleanTransitionsAfterDays int      `yaml:"clean_transitions_after_days"` // remove stale tool transitions after N days (default: 90)
		} `yaml:"adaptive_tools"`
		Recovery struct {
			MaxProvider422Recoveries int `yaml:"max_provider_422_recoveries"`  // max automatic retries after provider 422 validation errors (default: 3)
			MinMessagesForEmptyRetry int `yaml:"min_messages_for_empty_retry"` // minimum conversation messages required before retrying on empty LLM response (default: 5)
			DuplicateConsecutiveHits int `yaml:"duplicate_consecutive_hits"`   // duplicate tool calls in a row before circuit breaker triggers (default: 2)
			DuplicateFrequencyHits   int `yaml:"duplicate_frequency_hits"`     // total identical tool call repetitions before circuit breaker triggers (default: 3)
			IdenticalToolErrorHits   int `yaml:"identical_tool_error_hits"`    // repeated identical tool errors before retry breaker triggers (default: 3)
		} `yaml:"recovery"`
		BackgroundTasks struct {
			Enabled                bool `yaml:"enabled"`                    // enable persistent background task execution
			FollowUpDelaySeconds   int  `yaml:"follow_up_delay_seconds"`    // delay before first follow-up execution (default: 2)
			HTTPTimeoutSeconds     int  `yaml:"http_timeout_seconds"`       // loopback execution timeout for background prompts (default: 120)
			MaxRetries             int  `yaml:"max_retries"`                // retries for failed background prompt execution (default: 2)
			RetryDelaySeconds      int  `yaml:"retry_delay_seconds"`        // base retry delay for failed background prompt execution (default: 60)
			WaitPollIntervalSecs   int  `yaml:"wait_poll_interval_seconds"` // poll interval for wait_for_event tasks (default: 5)
			WaitDefaultTimeoutSecs int  `yaml:"wait_default_timeout_secs"`  // default timeout for wait_for_event tasks (default: 600)
		} `yaml:"background_tasks"`
		MaxToolGuides int `yaml:"max_tool_guides"` // maximum tool guide documents injected into prompt (default: 5)

		OutputCompression struct {
			Enabled           bool `yaml:"enabled"`            // master toggle for command-aware output compression (default: true)
			MinChars          int  `yaml:"min_chars"`          // only compress outputs exceeding this size (default: 500)
			PreserveErrors    bool `yaml:"preserve_errors"`    // never compress outputs that contain error markers (default: true)
			ShellCompression  bool `yaml:"shell_compression"`  // enable shell-specific filters: git, docker, test, grep, find, ls (default: true)
			PythonCompression bool `yaml:"python_compression"` // enable python traceback filtering and output dedup (default: true)
			APICompression    bool `yaml:"api_compression"`    // enable JSON compaction and null-field removal (default: true)
		} `yaml:"output_compression"`

		AnnouncementDetector struct {
			Enabled    bool `yaml:"enabled"`     // enable announcement-only response detection (default: true)
			MaxRetries int  `yaml:"max_retries"` // max corrective retries per user turn (default: 2)
		} `yaml:"announcement_detector"`

		// ── Legacy personality fields — read-only for YAML migration to Personality section ──
		LegacyPersonalityEngine         bool   `yaml:"personality_engine"         json:"-"`  // migrated → Personality.Engine
		LegacyPersonalityEngineV2       bool   `yaml:"personality_engine_v2"      json:"-"`  // migrated → Personality.EngineV2
		LegacyPersonalityV2Provider     string `yaml:"personality_v2_provider"    json:"-"`  // migrated → Personality.V2Provider
		LegacyPersonalityV2Model        string `yaml:"personality_v2_model"       json:"-"`  // migrated → Personality.V2Model
		LegacyPersonalityV2URL          string `yaml:"personality_v2_url"         json:"-"`  // migrated → Personality.V2URL
		LegacyPersonalityV2APIKey       string `yaml:"personality_v2_api_key"     json:"-"`  // migrated → Personality.V2APIKey
		LegacyPersonalityV2TimeoutSecs  int    `yaml:"personality_v2_timeout_secs" json:"-"` // migrated → Personality.V2TimeoutSecs
		LegacyCorePersonality           string `yaml:"core_personality"           json:"-"`  // migrated → Personality.CorePersonality
		LegacyUserProfiling             bool   `yaml:"user_profiling"             json:"-"`  // migrated → Personality.UserProfiling
		LegacyUserProfilingThreshold    int    `yaml:"user_profiling_threshold"   json:"-"`  // migrated → Personality.UserProfilingThreshold
		LegacyEmotionSynthesizerEnabled bool   `yaml:"emotion_synthesizer_enabled" json:"-"` // not a real field – unused, kept for zero-value detection
	} `yaml:"agent"`

	Heartbeat HeartbeatConfig `yaml:"heartbeat"`

	// Personality holds all settings related to the personality engine and user profiling.
	// Fields were previously part of the Agent section.
	Personality struct {
		Engine                 bool   `yaml:"engine"`                     // enable personality engine (mood, micro-traits)
		EngineV2               bool   `yaml:"engine_v2"`                  // enable V2 engine with async LLM mood analysis
		V2Provider             string `yaml:"v2_provider"       json:"-"` // legacy provider entry ID for V2 analysis; helper-owned runtime prefers llm.helper_*
		V2Model                string `yaml:"v2_model"          json:"-"` // legacy: model name (hidden from UI, used for migration)
		V2URL                  string `yaml:"v2_url"            json:"-"` // legacy: base URL (hidden from UI, used for migration)
		V2APIKey               string `yaml:"v2_api_key"        json:"-"` // legacy: API key (hidden from UI, used for migration)
		V2ProviderType         string `yaml:"-"                 json:"-"` // resolved
		V2ResolvedURL          string `yaml:"-"                 json:"-"` // resolved
		V2ResolvedKey          string `yaml:"-"                 json:"-"` // resolved
		V2ResolvedModel        string `yaml:"-"                 json:"-"` // resolved
		CorePersonality        string `yaml:"core_personality"`           // active personality profile name
		UserProfiling          bool   `yaml:"user_profiling"`             // opt-in: collect user profile via V2 analysis
		UserProfilingThreshold int    `yaml:"user_profiling_threshold"`   // min confidence for profile summary (default: 3)
		V2TimeoutSecs          int    `yaml:"v2_timeout_secs"  json:"-"`  // timeout for V2 mood analysis LLM call (default: 30)
		EmotionSynthesizer     struct {
			Enabled             bool `yaml:"enabled"`                // enable LLM-based emotion synthesis (default: false)
			MinIntervalSecs     int  `yaml:"min_interval_seconds"`   // minimum seconds between syntheses (default: 60)
			MaxHistoryEntries   int  `yaml:"max_history_entries"`    // max emotion history entries to retain (default: 100)
			TriggerOnMoodChange bool `yaml:"trigger_on_mood_change"` // only synthesize when mood changes (default: true)
			TriggerAlways       bool `yaml:"trigger_always"`         // synthesize on every message (default: false)
		} `yaml:"emotion_synthesizer"`
		InnerVoice struct {
			Enabled         bool `yaml:"enabled"`            // enable inner voice system (subconscious nudge engine)
			MinIntervalSecs int  `yaml:"min_interval_secs"`  // minimum seconds between inner voice generations (default: 60)
			MaxPerSession   int  `yaml:"max_per_session"`    // max inner voice generations per session (default: 20)
			DecayTurns      int  `yaml:"decay_turns"`        // turns after which inner voice fades from prompt (default: 3)
			DecayMaxAgeSecs int  `yaml:"decay_max_age_secs"` // max seconds before inner voice expires regardless of turns (default: 300 = 5min)
			ErrorStreakMin  int  `yaml:"error_streak_min"`   // consecutive errors before triggering inner voice (default: 2)
		} `yaml:"inner_voice"`
	} `yaml:"personality"`
	CircuitBreaker struct {
		MaxToolCalls                 int      `yaml:"max_tool_calls"`
		LLMTimeoutSeconds            int      `yaml:"llm_timeout_seconds"`
		LLMPerAttemptTimeoutSeconds  int      `yaml:"llm_per_attempt_timeout_seconds"`
		LLMStreamChunkTimeoutSeconds int      `yaml:"llm_stream_chunk_timeout_seconds"`
		MaintenanceTimeoutMinutes    int      `yaml:"maintenance_timeout_minutes"`
		RetryIntervals               []string `yaml:"retry_intervals"`
	} `yaml:"circuit_breaker"`
	Telegram struct {
		UserID               int64  `yaml:"telegram_user_id"`
		BotToken             string `yaml:"-" vault:"bot_token"` // vault-only
		MaxConcurrentWorkers int    `yaml:"max_concurrent_workers"`
	} `yaml:"telegram"`
	Whisper struct {
		Provider     string `yaml:"provider"`          // provider entry ID
		ProviderType string `yaml:"-"       json:"-"`  // resolved
		BaseURL      string `yaml:"-"       json:"-"`  // resolved
		APIKey       string `yaml:"-"       json:"-"`  // resolved
		Model        string `yaml:"-"       json:"-"`  // resolved
		Mode         string `yaml:"mode"`              // "whisper" (default), "multimodal", "local"
		LegacyAPIKey string `yaml:"api_key"  json:"-"` // legacy/compat
		LegacyURL    string `yaml:"base_url" json:"-"` // legacy/compat
		LegacyModel  string `yaml:"model"    json:"-"` // legacy/compat
	} `yaml:"whisper"`
	Vision struct {
		Provider     string `yaml:"provider"`          // provider entry ID
		ProviderType string `yaml:"-"       json:"-"`  // resolved
		BaseURL      string `yaml:"-"       json:"-"`  // resolved
		APIKey       string `yaml:"-"       json:"-"`  // resolved
		Model        string `yaml:"-"       json:"-"`  // resolved
		LegacyAPIKey string `yaml:"api_key"  json:"-"` // legacy/compat
		LegacyURL    string `yaml:"base_url" json:"-"` // legacy/compat
		LegacyModel  string `yaml:"model"    json:"-"` // legacy/compat
	} `yaml:"vision"`
	FallbackLLM struct {
		Enabled              bool   `yaml:"enabled"`
		Provider             string `yaml:"provider"`          // provider entry ID
		ProviderType         string `yaml:"-"       json:"-"`  // resolved
		BaseURL              string `yaml:"-"       json:"-"`  // resolved
		APIKey               string `yaml:"-"       json:"-"`  // resolved
		Model                string `yaml:"-"       json:"-"`  // resolved
		AccountID            string `yaml:"-"       json:"-"`  // resolved from provider entry (workers-ai)
		LegacyURL            string `yaml:"base_url" json:"-"` // legacy/compat
		LegacyAPIKey         string `yaml:"api_key"  json:"-"` // legacy/compat
		LegacyModel          string `yaml:"model"    json:"-"` // legacy/compat
		ProbeIntervalSeconds int    `yaml:"probe_interval_seconds"`
		ErrorThreshold       int    `yaml:"error_threshold"`
	} `yaml:"fallback_llm"`
	Maintenance struct {
		Enabled         bool   `yaml:"enabled"`
		Time            string `yaml:"time"`
		LifeboatEnabled bool   `yaml:"lifeboat_enabled"`
		LifeboatPort    int    `yaml:"lifeboat_port"`
	} `yaml:"maintenance"`
	Journal struct {
		AutoEntries  bool `yaml:"auto_entries"`  // auto-create journal entries from tool chains (default true)
		DailySummary bool `yaml:"daily_summary"` // generate daily summaries during maintenance (default true)
	} `yaml:"journal"`
	Consolidation struct {
		Enabled           bool   `yaml:"enabled"`             // enable nightly STM→LTM consolidation (default true)
		AutoOptimize      bool   `yaml:"auto_optimize"`       // run optimize_memory after consolidation (default true)
		ArchiveRetainDays int    `yaml:"archive_retain_days"` // keep archived messages for N days (default 30)
		MaxBatchMessages  int    `yaml:"max_batch_messages"`  // max messages per consolidation batch (default 200)
		OptimizeThreshold int    `yaml:"optimize_threshold"`  // priority threshold for auto-optimize (default 1)
		Model             string `yaml:"model"`               // optional model override for nightly consolidation (empty = main llm model)
	} `yaml:"consolidation"`
	MemoryAnalysis struct {
		Enabled               bool    `yaml:"enabled"`                 // deprecated compatibility flag; memory analysis is now adaptive and always active
		Preset                string  `yaml:"preset"`                  // deprecated compatibility field; rollout is now adaptive
		RealTime              bool    `yaml:"real_time"`               // deprecated compatibility field; real-time extraction is now adaptive
		Provider              string  `yaml:"provider"       json:"-"` // legacy provider entry; helper-owned runtime prefers llm.helper_*
		Model                 string  `yaml:"model"          json:"-"` // model override (optional)
		AutoConfirm           float64 `yaml:"auto_confirm_threshold"`  // confidence threshold for auto-store (default 0.92)
		QueryExpansion        bool    `yaml:"query_expansion"`         // deprecated compatibility field; retrieval tuning is now adaptive
		LLMReranking          bool    `yaml:"llm_reranking"`           // deprecated compatibility field; retrieval tuning is now adaptive
		UnifiedMemoryBlock    bool    `yaml:"unified_memory_block"`    // deprecated compatibility field; unified memory context is always active
		EffectivenessTracking bool    `yaml:"effectiveness_tracking"`  // deprecated compatibility field; effectiveness tracking is always active
		ProviderType          string  `yaml:"-" json:"-"`              // resolved
		BaseURL               string  `yaml:"-" json:"-"`              // resolved
		APIKey                string  `yaml:"-" json:"-"`              // resolved
		ResolvedModel         string  `yaml:"-" json:"-"`              // resolved
		WeeklyReflection      bool    `yaml:"weekly_reflection"`       // deprecated compatibility field; weekly reflection scheduling is always active
		ReflectionDay         string  `yaml:"reflection_day"`          // day for weekly reflection (default "sunday")
	} `yaml:"memory_analysis"`
	LLMGuardian struct {
		Enabled            bool              `yaml:"enabled"`               // enable LLM-based security checks before tool execution
		Provider           string            `yaml:"provider"`              // provider entry ID (falls back to main LLM)
		Model              string            `yaml:"model"`                 // model override (optional)
		DefaultLevel       string            `yaml:"default_level"`         // off, low, medium, high (default "medium")
		FailSafe           string            `yaml:"fail_safe"`             // block, allow, quarantine on guardian error (default "quarantine")
		CacheTTL           int               `yaml:"cache_ttl"`             // cache TTL in seconds (default 300)
		MaxChecksPerMin    int               `yaml:"max_checks_per_minute"` // rate limit (default 60)
		ToolOverrides      map[string]string `yaml:"tool_overrides"`        // per-tool level overrides (e.g. execute_shell: high)
		AllowClarification bool              `yaml:"allow_clarification"`   // agent can justify blocked actions (1 retry)
		TimeoutSecs        int               `yaml:"timeout_secs"`          // timeout per guardian check in seconds (default 30)
		ScanDocuments      bool              `yaml:"scan_documents"`        // LLM-scan documents & webhooks for threats
		ScanEmails         bool              `yaml:"scan_emails"`           // LLM-scan emails for phishing/injection
		ProviderType       string            `yaml:"-" json:"-"`            // resolved
		BaseURL            string            `yaml:"-" json:"-"`            // resolved
		APIKey             string            `yaml:"-" json:"-"`            // resolved
		ResolvedModel      string            `yaml:"-" json:"-"`            // resolved
	} `yaml:"llm_guardian"`
	Guardian struct {
		MaxScanBytes  int `yaml:"max_scan_bytes"`  // max bytes scanned by regex guardian before windowing (default 16384)
		ScanEdgeBytes int `yaml:"scan_edge_bytes"` // bytes kept from start and end when windowing large inputs (default 6144)
		PromptSec     struct {
			Preset    string `yaml:"preset"`    // "strict", "moderate", "lenient" (default: strict)
			Spotlight bool   `yaml:"spotlight"` // default: true
			Canary    bool   `yaml:"canary"`    // default: true
		} `yaml:"promptsec"`
	} `yaml:"guardian"`
	Logging struct {
		LogDir          string `yaml:"log_dir"`
		EnableFileLog   bool   `yaml:"enable_file_log"`
		EnablePromptLog bool   `yaml:"enable_prompt_log"`
	} `yaml:"logging"`
	Discord struct {
		Enabled          bool   `yaml:"enabled"`
		ReadOnly         bool   `yaml:"readonly"`            // true = only fetch/list, block send
		BotToken         string `yaml:"-" vault:"bot_token"` // vault-only
		GuildID          string `yaml:"guild_id"`
		AllowedUserID    string `yaml:"allowed_user_id"`
		DefaultChannelID string `yaml:"default_channel_id"`
	} `yaml:"discord"`
	Email struct {
		Enabled       bool   `yaml:"enabled"`
		ReadOnly      bool   `yaml:"readonly"` // true = only fetch, block send
		IMAPHost      string `yaml:"imap_host"`
		IMAPPort      int    `yaml:"imap_port"`
		SMTPHost      string `yaml:"smtp_host"`
		SMTPPort      int    `yaml:"smtp_port"`
		Username      string `yaml:"username"`
		Password      string `yaml:"-" json:"-"`
		FromAddress   string `yaml:"from_address"`
		WatchEnabled  bool   `yaml:"watch_enabled"`
		WatchInterval int    `yaml:"watch_interval_seconds"`
		WatchFolder   string `yaml:"watch_folder"`
	} `yaml:"email"` // legacy single-account; migrated to EmailAccounts at startup
	HomeAssistant struct {
		Enabled     bool   `yaml:"enabled"`
		ReadOnly    bool   `yaml:"readonly"` // true = only read states, block call_service
		URL         string `yaml:"url"`
		AccessToken string `yaml:"-" vault:"access_token"` // vault-only
	} `yaml:"home_assistant"`
	FritzBox struct {
		Enabled  bool   `yaml:"enabled"`
		Host     string `yaml:"host"`                                 // hostname or IP, default: fritz.box
		Port     int    `yaml:"port"`                                 // TR-064 port, default: 49000
		HTTPS    bool   `yaml:"https"`                                // use HTTPS for TR-064, default: true
		Timeout  int    `yaml:"timeout"`                              // HTTP timeout in seconds, default: 10
		Username string `yaml:"username"`                             // Fritz!Box username (leave empty to use no username)
		Password string `yaml:"-" vault:"fritzbox_password" json:"-"` // vault-only

		// Feature groups – all gated individually
		System struct {
			Enabled     bool `yaml:"enabled"`
			ReadOnly    bool `yaml:"readonly"` // true = only info/log, block reboot
			SubFeatures struct {
				DeviceInfo   bool `yaml:"device_info"`
				Uptime       bool `yaml:"uptime"`
				Log          bool `yaml:"log"`
				Temperatures bool `yaml:"temperatures"`
			} `yaml:"sub_features"`
		} `yaml:"system"`

		Network struct {
			Enabled     bool `yaml:"enabled"`
			ReadOnly    bool `yaml:"readonly"` // true = only read, block wlan toggle/port-forward
			SubFeatures struct {
				WLAN           bool `yaml:"wlan"`
				GuestWLAN      bool `yaml:"guest_wlan"`
				DECT           bool `yaml:"dect"`
				Mesh           bool `yaml:"mesh"`
				Hosts          bool `yaml:"hosts"`
				WakeOnLAN      bool `yaml:"wake_on_lan"`
				PortForwarding bool `yaml:"port_forwarding"`
			} `yaml:"sub_features"`
		} `yaml:"network"`

		Telephony struct {
			Enabled     bool `yaml:"enabled"`
			ReadOnly    bool `yaml:"readonly"` // true = only read lists, block deflection/phonebook changes
			SubFeatures struct {
				CallLists      bool `yaml:"call_lists"`
				Phonebooks     bool `yaml:"phonebooks"`
				CallDeflection bool `yaml:"call_deflection"`
				TAM            bool `yaml:"tam"`
			} `yaml:"sub_features"`
			Polling struct {
				Enabled             bool `yaml:"enabled"`
				IntervalSeconds     int  `yaml:"interval_seconds"`       // default: 60
				DedupWindowMinutes  int  `yaml:"dedup_window_minutes"`   // default: 5 – ignore duplicate events within this window
				MaxCallbacksPerHour int  `yaml:"max_callbacks_per_hour"` // default: 20 – rate limit LLM callbacks
			} `yaml:"polling"`
		} `yaml:"telephony"`

		SmartHome struct {
			Enabled     bool `yaml:"enabled"`
			ReadOnly    bool `yaml:"readonly"` // true = only device status, block switch/temp/color changes
			SubFeatures struct {
				Devices   bool `yaml:"devices"`
				Switches  bool `yaml:"switches"`
				Heating   bool `yaml:"heating"`
				Blinds    bool `yaml:"blinds"`
				Lamps     bool `yaml:"lamps"`
				Templates bool `yaml:"templates"`
			} `yaml:"sub_features"`
		} `yaml:"smart_home"`

		Storage struct {
			Enabled     bool `yaml:"enabled"`
			ReadOnly    bool `yaml:"readonly"` // true = only info, block FTP toggle/share changes
			SubFeatures struct {
				NAS         bool `yaml:"nas"`
				FTP         bool `yaml:"ftp"`
				USBDevices  bool `yaml:"usb_devices"`
				MediaServer bool `yaml:"media_server"`
			} `yaml:"sub_features"`
		} `yaml:"storage"`

		TV struct {
			Enabled     bool `yaml:"enabled"`  // Cable-only: DVB-C channel list & stream URLs
			ReadOnly    bool `yaml:"readonly"` // true = read-only access (TV is read-only by nature)
			SubFeatures struct {
				ChannelList bool `yaml:"channel_list"`
				StreamURLs  bool `yaml:"stream_urls"`
			} `yaml:"sub_features"`
		} `yaml:"tv"`
	} `yaml:"fritzbox"`
	Telnyx struct {
		Enabled             bool     `yaml:"enabled"`
		ReadOnly            bool     `yaml:"readonly"`                    // true = receive only, no outbound
		APIKey              string   `yaml:"-" vault:"telnyx_api_key"`    // vault-only
		APISecret           string   `yaml:"-" vault:"telnyx_api_secret"` // webhook signature verification
		PhoneNumber         string   `yaml:"phone_number"`                // primary Telnyx number (E.164)
		MessagingProfileID  string   `yaml:"messaging_profile_id"`        // Telnyx messaging profile
		ConnectionID        string   `yaml:"connection_id"`               // SIP connection ID for voice calls
		WebhookPath         string   `yaml:"webhook_path"`                // default: /api/telnyx/webhook
		AllowedNumbers      []string `yaml:"allowed_numbers"`             // E.164 whitelist (empty = allow all)
		MaxConcurrentCalls  int      `yaml:"max_concurrent_calls"`        // default: 3
		MaxSMSPerMinute     int      `yaml:"max_sms_per_minute"`          // rate limit, default: 10
		VoiceLanguage       string   `yaml:"voice_language"`              // BCP-47, default: en
		VoiceGender         string   `yaml:"voice_gender"`                // male/female, default: female
		RecordCalls         bool     `yaml:"record_calls"`                // auto-record all calls
		TranscribeVoicemail bool     `yaml:"transcribe_voicemail"`        // auto-transcribe via LLM
		RelayToAgent        bool     `yaml:"relay_to_agent"`              // forward incoming SMS to agent loop
		CallTimeout         int      `yaml:"call_timeout"`                // max call duration seconds, default: 300
	} `yaml:"telnyx"`
	MeshCentral struct {
		Enabled           bool     `yaml:"enabled"`
		ReadOnly          bool     `yaml:"readonly"`           // true = only list operations, block wake/power/run_command
		BlockedOperations []string `yaml:"blocked_operations"` // explicit operation deny-list (case-insensitive)
		URL               string   `yaml:"url"`
		Username          string   `yaml:"username"`
		Password          string   `yaml:"-" vault:"password" json:"-"`    // vault-only
		LoginToken        string   `yaml:"-" vault:"login_token" json:"-"` // vault-only
		Insecure          bool     `yaml:"insecure"`                       // skip TLS certificate verification (default: false)
	} `yaml:"meshcentral"`
	Docker struct {
		Enabled  bool   `yaml:"enabled"`
		ReadOnly bool   `yaml:"readonly"` // true = only list/inspect/logs/stats, block create/start/stop/remove/exec
		Host     string `yaml:"host"`     // e.g. unix:///var/run/docker.sock or tcp://localhost:2375
	} `yaml:"docker"`
	CoAgents struct {
		Enabled             bool `yaml:"enabled"`
		MaxConcurrent       int  `yaml:"max_concurrent"`
		BudgetQuotaPercent  int  `yaml:"budget_quota_percent"`     // share of daily budget reserved for co-agents (0 = disabled)
		MaxContextHints     int  `yaml:"max_context_hints"`        // maximum number of context_hints accepted per request
		MaxContextHintChars int  `yaml:"max_context_hint_chars"`   // maximum characters per context hint
		MaxResultBytes      int  `yaml:"max_result_bytes"`         // truncate co-agent result after this many bytes
		QueueWhenBusy       bool `yaml:"queue_when_busy"`          // queue co-agents instead of rejecting when all slots are occupied
		CleanupIntervalMins int  `yaml:"cleanup_interval_minutes"` // registry cleanup tick interval (default: 10)
		CleanupMaxAgeMins   int  `yaml:"cleanup_max_age_minutes"`  // finished co-agent retention before cleanup (default: 30)
		LLM                 struct {
			Provider     string `yaml:"provider"`          // provider entry ID
			ProviderType string `yaml:"-"       json:"-"`  // resolved
			BaseURL      string `yaml:"-"       json:"-"`  // resolved
			APIKey       string `yaml:"-"       json:"-"`  // resolved
			Model        string `yaml:"-"       json:"-"`  // resolved
			LegacyURL    string `yaml:"base_url" json:"-"` // legacy/compat
			LegacyAPIKey string `yaml:"api_key"  json:"-"` // legacy/compat
			LegacyModel  string `yaml:"model"    json:"-"` // legacy/compat
		} `yaml:"llm"`
		CircuitBreaker struct {
			MaxToolCalls   int `yaml:"max_tool_calls"`
			TimeoutSeconds int `yaml:"timeout_seconds"`
			MaxTokens      int `yaml:"max_tokens"`
		} `yaml:"circuit_breaker"`
		RetryPolicy struct {
			MaxRetries             int      `yaml:"max_retries"`              // retries for transient co-agent LLM failures (default: 1)
			RetryDelaySeconds      int      `yaml:"retry_delay_seconds"`      // base delay between retries (default: 5)
			RetryableErrorPatterns []string `yaml:"retryable_error_patterns"` // lower-cased substrings treated as transient
		} `yaml:"retry_policy"`
		Specialists struct {
			Researcher SpecialistConfig `yaml:"researcher"`
			Coder      SpecialistConfig `yaml:"coder"`
			Designer   SpecialistConfig `yaml:"designer"`
			Security   SpecialistConfig `yaml:"security"`
			Writer     SpecialistConfig `yaml:"writer"`
		} `yaml:"specialists"`
	} `yaml:"co_agents"`
	A2A struct {
		Server struct {
			Enabled           bool   `yaml:"enabled"`
			Port              int    `yaml:"port"`               // 0 = use main server port (shared mux), >0 = dedicated A2A port
			BasePath          string `yaml:"base_path"`          // URL prefix for A2A endpoints (default: "/a2a")
			AgentName         string `yaml:"agent_name"`         // name in Agent Card
			AgentDescription  string `yaml:"agent_description"`  // description in Agent Card
			AgentVersion      string `yaml:"agent_version"`      // version in Agent Card
			AgentURL          string `yaml:"agent_url"`          // public URL override for Agent Card (auto-detected if empty)
			Streaming         bool   `yaml:"streaming"`          // enable SSE streaming support
			PushNotifications bool   `yaml:"push_notifications"` // enable push notification support
			Bindings          struct {
				REST     bool `yaml:"rest"`      // enable REST (HTTP+JSON) binding
				JSONRPC  bool `yaml:"json_rpc"`  // enable JSON-RPC 2.0 binding
				GRPC     bool `yaml:"grpc"`      // enable gRPC binding
				GRPCPort int  `yaml:"grpc_port"` // separate port for gRPC (default: 50051)
			} `yaml:"bindings"`
			Skills []A2ASkill `yaml:"skills"` // skills advertised in Agent Card
		} `yaml:"server"`
		Client struct {
			Enabled      bool             `yaml:"enabled"`
			RemoteAgents []A2ARemoteAgent `yaml:"remote_agents"` // configured remote A2A agents
		} `yaml:"client"`
		Auth struct {
			APIKeyEnabled bool   `yaml:"api_key_enabled"`             // enable API Key authentication
			APIKey        string `yaml:"-" vault:"a2a_api_key"`       // API Key (vault-only)
			BearerEnabled bool   `yaml:"bearer_enabled"`              // enable Bearer token authentication
			BearerSecret  string `yaml:"-" vault:"a2a_bearer_secret"` // Bearer token secret (vault-only)
		} `yaml:"auth"`
		LLM struct {
			Provider     string `yaml:"provider"`          // references ProviderEntry ID (empty = use main LLM)
			ProviderType string `yaml:"-" json:"-"`        // resolved: provider type
			BaseURL      string `yaml:"-" json:"-"`        // resolved from provider entry
			APIKey       string `yaml:"-" json:"-"`        // resolved from provider entry
			Model        string `yaml:"-" json:"-"`        // resolved from provider entry
			LegacyURL    string `yaml:"base_url" json:"-"` // legacy/compat
			LegacyAPIKey string `yaml:"api_key"  json:"-"` // legacy/compat
			LegacyModel  string `yaml:"model"    json:"-"` // legacy/compat
		} `yaml:"llm"`
	} `yaml:"a2a"`
	Budget struct {
		Enabled          bool           `yaml:"enabled"`
		DailyLimitUSD    float64        `yaml:"daily_limit_usd"`
		Enforcement      string         `yaml:"enforcement"` // "warn" | "partial" | "full"
		ResetHour        int            `yaml:"reset_hour"`
		WarningThreshold float64        `yaml:"warning_threshold"` // 0.0–1.0
		Models           []ModelCost    `yaml:"models"`
		DefaultCost      ModelCostRates `yaml:"default_cost"`
	} `yaml:"budget"`
	WebDAV struct {
		Enabled  bool   `yaml:"enabled"`
		ReadOnly bool   `yaml:"readonly"`  // true = only list/read/info, block write/delete/move/mkdir
		AuthType string `yaml:"auth_type"` // basic | bearer
		URL      string `yaml:"url"`       // e.g. https://cloud.example.com/remote.php/dav/files/user/
		Username string `yaml:"username"`
		Password string `yaml:"-" json:"-"`
		Token    string `yaml:"-" json:"-"`
	} `yaml:"webdav"`
	Koofr struct {
		Enabled     bool   `yaml:"enabled"`
		ReadOnly    bool   `yaml:"readonly"` // true = only list/read, block write/delete/move
		Username    string `yaml:"username"`
		AppPassword string `yaml:"-" json:"-"`
		BaseURL     string `yaml:"base_url"` // default: https://app.koofr.net
	} `yaml:"koofr"`
	S3 struct {
		Enabled      bool   `yaml:"enabled"`
		ReadOnly     bool   `yaml:"readonly"`             // true = only list/download, block upload/delete/copy/move
		Endpoint     string `yaml:"endpoint"`             // e.g. https://s3.amazonaws.com or http://minio.local:9000
		Region       string `yaml:"region"`               // e.g. us-east-1
		Bucket       string `yaml:"bucket"`               // default bucket name (optional, can pass per-call)
		UsePathStyle bool   `yaml:"use_path_style"`       // true for MinIO / S3-compatible (path-style vs virtual-hosted)
		Insecure     bool   `yaml:"insecure"`             // allow HTTP (non-TLS) endpoints
		AccessKey    string `yaml:"-" vault:"access_key"` // S3 Access Key ID (vault: s3_access_key)
		SecretKey    string `yaml:"-" vault:"secret_key"` // S3 Secret Access Key (vault: s3_secret_key)
	} `yaml:"s3"`
	PaperlessNGX struct {
		Enabled  bool   `yaml:"enabled"`
		ReadOnly bool   `yaml:"readonly"` // true = only search/get/download/list, block upload/update/delete
		URL      string `yaml:"url"`      // e.g. https://paperless.example.com
		APIToken string `yaml:"-" json:"-"`
	} `yaml:"paperless_ngx"`
	TTS struct {
		Provider            string `yaml:"provider"`              // "google", "elevenlabs", or "piper"
		Language            string `yaml:"language"`              // BCP-47 language code for Google TTS (e.g. "de", "en")
		CacheRetentionHours int    `yaml:"cache_retention_hours"` // remove cached TTS files older than this many hours (0 disables age-based cleanup)
		CacheMaxFiles       int    `yaml:"cache_max_files"`       // max cached TTS files to retain (0 disables count-based cleanup)
		ElevenLabs          struct {
			APIKey  string `yaml:"-" vault:"api_key"` // vault-only
			VoiceID string `yaml:"voice_id"`          // default voice ID
			ModelID string `yaml:"model_id"`          // e.g. "eleven_multilingual_v2"
		} `yaml:"elevenlabs"`
		MiniMax struct {
			APIKey  string  `yaml:"-" vault:"api_key"` // vault-only
			VoiceID string  `yaml:"voice_id"`          // e.g. "English_expressive_narrator"
			ModelID string  `yaml:"model_id"`          // "speech-2.8-hd" or "speech-2.8-turbo"
			Speed   float64 `yaml:"speed"`             // 0.5–2.0; 0 means default (1.0)
		} `yaml:"minimax"`
		Piper struct {
			Enabled       bool   `yaml:"enabled"`        // auto-manage a Piper TTS container
			Voice         string `yaml:"voice"`          // e.g. "de_DE-thorsten-high"
			SpeakerID     int    `yaml:"speaker_id"`     // multi-speaker model speaker index
			ContainerPort int    `yaml:"container_port"` // host port mapped to container 10200 (default 10200)
			DataPath      string `yaml:"data_path"`      // voice model storage directory (default "data/piper")
			Image         string `yaml:"image"`          // Docker image (default "rhasspy/wyoming-piper:latest")
		} `yaml:"piper"`
	} `yaml:"tts"`
	MediaRegistry struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"media_registry"`
	Chromecast struct {
		Enabled bool `yaml:"enabled"`
		TTSPort int  `yaml:"tts_port"`
	} `yaml:"chromecast"`
	Homepage struct {
		Enabled                  bool `yaml:"enabled"`
		AllowDeploy              bool `yaml:"allow_deploy"`
		AllowContainerManagement bool `yaml:"allow_container_management"`
		AllowLocalServer         bool `yaml:"allow_local_server"` // Danger Zone: allow Python HTTP server fallback when Docker unavailable
		// AllowTemporaryTokenBudgetOverflow temporarily scales the system prompt token budget
		// for homepage action chains relative to the increased homepage circuit breaker steps.
		AllowTemporaryTokenBudgetOverflow bool   `yaml:"allow_temporary_token_budget_overflow"`
		DeployHost                        string `yaml:"deploy_host"`
		DeployPort                        int    `yaml:"deploy_port"`
		DeployUser                        string `yaml:"deploy_user"`
		DeployPassword                    string `yaml:"-" json:"-"` // vault-only
		DeployKey                         string `yaml:"-" json:"-"` // vault-only (SSH private key)
		DeployPath                        string `yaml:"deploy_path"`
		DeployMethod                      string `yaml:"deploy_method"` // "sftp" or "scp"
		WebServerEnabled                  bool   `yaml:"webserver_enabled"`
		WebServerPort                     int    `yaml:"webserver_port"`
		WebServerDomain                   string `yaml:"webserver_domain"`
		WebServerInternalOnly             bool   `yaml:"webserver_internal_only"` // bind on 127.0.0.1 only
		WorkspacePath                     string `yaml:"workspace_path"`
		// CircuitBreakerMaxCalls setzt ein eigenes Tool-Call-Limit wenn Homepage aktiv ist.
		// Gilt temporär ab dem ersten Homepage-Aufruf bis zum Ende der Aktionskette. (Standard: 35)
		CircuitBreakerMaxCalls int `yaml:"circuit_breaker_max_calls"`
	} `yaml:"homepage"`
	Notifications struct {
		Ntfy struct {
			Enabled bool   `yaml:"enabled"`
			URL     string `yaml:"url"`             // e.g. "https://ntfy.sh" or self-hosted
			Topic   string `yaml:"topic"`           // e.g. "aurago"
			Token   string `yaml:"-" vault:"token"` // auth token (vault-only)
		} `yaml:"ntfy"`
		Pushover struct {
			Enabled  bool   `yaml:"enabled"`
			UserKey  string `yaml:"-" vault:"user_key"`  // from vault
			AppToken string `yaml:"-" vault:"app_token"` // from vault
		} `yaml:"pushover"`
	} `yaml:"notifications"`
	Auth struct {
		Enabled             bool   `yaml:"enabled"`                  // enable login page protection
		PasswordHash        string `yaml:"-" vault:"password_hash"`  // bcrypt hash (vault-only)
		SessionSecret       string `yaml:"-" vault:"session_secret"` // HMAC key for session cookies (vault-only)
		SessionTimeoutHours int    `yaml:"session_timeout_hours"`    // how long a session stays valid (default 24h)
		TOTPSecret          string `yaml:"-" vault:"totp_secret"`    // base32 TOTP secret for 2FA (vault-only)
		TOTPEnabled         bool   `yaml:"totp_enabled"`             // whether TOTP 2FA is active
		MaxLoginAttempts    int    `yaml:"max_login_attempts"`       // failed attempts before lockout (default 5)
		LockoutMinutes      int    `yaml:"lockout_minutes"`          // lockout duration in minutes (default 15)
	} `yaml:"auth"`
	WebConfig struct {
		Enabled bool `yaml:"enabled"` // false = /config endpoint disabled for security
	} `yaml:"web_config"`
	VirusTotal struct {
		Enabled bool   `yaml:"enabled"`
		APIKey  string `yaml:"-" vault:"api_key"` // vault-only
	} `yaml:"virustotal"`
	GolangciLint struct {
		Enabled bool `yaml:"enabled"` // enable the golangci_lint agent tool
	} `yaml:"golangci_lint"`
	BraveSearch struct {
		Enabled bool   `yaml:"enabled"`
		APIKey  string `yaml:"-" vault:"api_key"` // Brave Search Subscription Token (vault-only)
		Country string `yaml:"country"`           // default country filter (e.g. "DE", "US"; empty = global)
		Lang    string `yaml:"lang"`              // default search language (e.g. "de", "en"; empty = API default)
	} `yaml:"brave_search"`
	Webhooks struct {
		Enabled        bool              `yaml:"enabled"`
		ReadOnly       bool              `yaml:"readonly"`         // true = only list/get/logs, block create/update/delete
		MaxPayloadSize int               `yaml:"max_payload_size"` // max body bytes per request (default 65536)
		RateLimit      int               `yaml:"rate_limit"`       // max requests per minute per token (0 = unlimited)
		Outgoing       []OutgoingWebhook `yaml:"outgoing"`         // configured outgoing webhooks for the agent
	} `yaml:"webhooks"`
	Proxmox struct {
		Enabled  bool   `yaml:"enabled"`
		ReadOnly bool   `yaml:"readonly"`         // true = only list/status, block start/stop/reboot/suspend/snapshot
		URL      string `yaml:"url"`              // e.g. "https://pve.example.com:8006"
		TokenID  string `yaml:"token_id"`         // e.g. "user@pam!tokenname"
		Secret   string `yaml:"-" vault:"secret"` // API token secret (from vault)
		Node     string `yaml:"node"`             // default node name (e.g. "pve")
		Insecure bool   `yaml:"insecure"`         // skip TLS verification for self-signed certs
	} `yaml:"proxmox"`
	Ollama struct {
		Enabled         bool   `yaml:"enabled"`
		ReadOnly        bool   `yaml:"readonly"` // true = only list/show/running, block pull/delete/copy
		URL             string `yaml:"url"`      // e.g. "http://localhost:11434"
		ManagedInstance struct {
			Enabled       bool     `yaml:"enabled"`        // auto-manage an Ollama Docker container
			ContainerPort int      `yaml:"container_port"` // host port for the managed container (default: 11434)
			UseHostGPU    bool     `yaml:"use_host_gpu"`   // pass GPU devices into the container (Linux only)
			GPUBackend    string   `yaml:"gpu_backend"`    // "auto", "nvidia", "amd", "intel", "vulkan" (default: "auto")
			DefaultModels []string `yaml:"default_models"` // models to auto-pull after container start
			MemoryLimit   string   `yaml:"memory_limit"`   // Docker memory limit, e.g. "8g" (empty = unlimited)
			VolumePath    string   `yaml:"volume_path"`    // host path for persistent model storage (empty = Docker volume)
		} `yaml:"managed_instance"`
	} `yaml:"ollama"`
	RocketChat struct {
		Enabled      bool     `yaml:"enabled"`
		URL          string   `yaml:"url"`                  // e.g. "https://chat.example.com"
		UserID       string   `yaml:"user_id"`              // bot user ID
		AuthToken    string   `yaml:"-" vault:"auth_token"` // auth token (vault-only)
		Channel      string   `yaml:"channel"`              // default channel to listen on
		Alias        string   `yaml:"alias"`                // display name
		AllowedUsers []string `yaml:"allowed_users"`        // allowed user IDs or usernames; empty = deny all
	} `yaml:"rocketchat"`
	Tailscale struct {
		Enabled  bool   `yaml:"enabled"`
		ReadOnly bool   `yaml:"readonly"`          // true = only list/status/dns, block enable/disable routes
		APIKey   string `yaml:"-" vault:"api_key"` // Tailscale API key (vault-only)
		Tailnet  string `yaml:"tailnet"`           // Tailnet name, e.g. "example.com" or "-" for default
		TsNet    struct {
			Enabled           bool   `yaml:"enabled"`             // enable tsnet embedded Tailscale node (independent of API integration)
			Hostname          string `yaml:"hostname"`            // MagicDNS hostname, e.g. "aurago" → aurago.tailnet-name.ts.net
			StateDir          string `yaml:"state_dir"`           // persistent state directory (default: data/tsnet)
			ServeHTTP         bool   `yaml:"serve_http"`          // expose AuraGo's web UI over the tailnet on 443/80
			ExposeHomepage    bool   `yaml:"expose_homepage"`     // expose the Homepage/Caddy web server over the tailnet on 8443
			Funnel            bool   `yaml:"funnel"`              // expose AuraGo publicly via Tailscale Funnel on 443
			AllowHTTPFallback bool   `yaml:"allow_http_fallback"` // fall back to HTTP on :80 when HTTPS cert is unavailable (default: false)
			AuthKey           string `yaml:"-"`                   // tsnet auth key (vault-only: tailscale_tsnet_authkey)
		} `yaml:"tsnet"`
	} `yaml:"tailscale"`
	CloudflareTunnel struct {
		Enabled        bool                    `yaml:"enabled"`         // master toggle
		ReadOnly       bool                    `yaml:"readonly"`        // agent: status-only, no start/stop/route changes
		Mode           string                  `yaml:"mode"`            // "auto" (default), "docker", "native"
		AutoStart      bool                    `yaml:"auto_start"`      // start tunnel on AuraGo boot
		AuthMethod     string                  `yaml:"auth_method"`     // "token" (default), "named", "quick"
		TunnelName     string                  `yaml:"tunnel_name"`     // named tunnel: tunnel name
		AccountID      string                  `yaml:"account_id"`      // Cloudflare account ID (for API access)
		TunnelID       string                  `yaml:"tunnel_id"`       // optional: explicit tunnel UUID (used to auto-configure noTLSVerify via API)
		LoopbackPort   int                     `yaml:"loopback_port"`   // plain-HTTP loopback port for cloudflared on 127.0.0.1 (0=disabled, >0=port, e.g. 18080)
		ExposeWebUI    bool                    `yaml:"expose_web_ui"`   // auto-route AuraGo web UI through tunnel
		ExposeHomepage bool                    `yaml:"expose_homepage"` // auto-route homepage web server through tunnel
		CustomIngress  []CloudflareIngressRule `yaml:"custom_ingress"`  // additional ingress rules
		MetricsPort    int                     `yaml:"metrics_port"`    // cloudflared metrics (0=disabled)
		LogLevel       string                  `yaml:"log_level"`       // "info" (default), "debug", "warn", "error"
	} `yaml:"cloudflare_tunnel"`
	Ansible struct {
		Enabled          bool   `yaml:"enabled"`
		ReadOnly         bool   `yaml:"readonly"`          // true = only status/list/inventory/ping/facts, block adhoc/playbook
		Mode             string `yaml:"mode"`              // "sidecar" (default) or "local" (calls ansible CLI directly)
		URL              string `yaml:"url"`               // sidecar mode: API URL (e.g. "http://127.0.0.1:5001")
		Token            string `yaml:"-" vault:"token"`   // sidecar mode: Bearer token (vault-only)
		Timeout          int    `yaml:"timeout"`           // max seconds for playbook runs (default 300)
		PlaybooksDir     string `yaml:"playbooks_dir"`     // directory containing playbooks (local mode: path; sidecar mode: host mount)
		DefaultInventory string `yaml:"default_inventory"` // default inventory file path (local mode: path; sidecar mode: host dir mount)
		Image            string `yaml:"image"`             // sidecar mode: Docker image (default: aurago-ansible:latest)
		ContainerName    string `yaml:"container_name"`    // sidecar mode: container name (default: aurago_ansible)
		AutoBuild        bool   `yaml:"auto_build"`        // sidecar mode: build image automatically at startup if not found locally
		DockerfileDir    string `yaml:"dockerfile_dir"`    // sidecar mode: directory containing Dockerfile.ansible (default: ".")
	} `yaml:"ansible"`
	InvasionControl struct {
		Enabled  bool `yaml:"enabled"`  // enable Invasion Control sub-agent management UI
		ReadOnly bool `yaml:"readonly"` // true = only list/status, block deploy/stop/send_task/send_secret
	} `yaml:"invasion_control"`
	RemoteControl struct {
		Enabled            bool     `yaml:"enabled"`
		ReadOnly           bool     `yaml:"readonly"`              // global read-only override for all remote devices
		DiscoveryPort      int      `yaml:"discovery_port"`        // UDP broadcast discovery port (default: 8092)
		MaxFileSizeMB      int      `yaml:"max_file_size_mb"`      // max file read/write size in MB (default: 50)
		AutoApprove        bool     `yaml:"auto_approve"`          // auto-approve new devices (NOT recommended)
		AllowedPaths       []string `yaml:"allowed_paths"`         // global path whitelist (empty = all)
		AuditLog           bool     `yaml:"audit_log"`             // log all operations (default: true)
		SSHInsecureHostKey bool     `yaml:"ssh_insecure_host_key"` // skip SSH host key verification (disables MITM protection)
	} `yaml:"remote_control"`
	BrowserAutomation BrowserAutomationConfig `yaml:"browser_automation"`
	SecurityProxy     struct {
		Enabled      bool   `yaml:"enabled"`
		Domain       string `yaml:"domain"`      // primary domain for TLS (e.g. "aurago.example.com")
		Email        string `yaml:"email"`       // ACME email for Let's Encrypt
		HTTPSPort    int    `yaml:"https_port"`  // external HTTPS port (default: 443)
		HTTPPort     int    `yaml:"http_port"`   // external HTTP port for ACME challenge (default: 80)
		DockerHost   string `yaml:"docker_host"` // Docker host override (empty = auto-detect)
		RateLimiting struct {
			Enabled           bool `yaml:"enabled"`
			RequestsPerSecond int  `yaml:"requests_per_second"` // default: 10
			Burst             int  `yaml:"burst"`               // default: 50
		} `yaml:"rate_limiting"`
		IPFilter struct {
			Enabled   bool     `yaml:"enabled"`
			Mode      string   `yaml:"mode"`      // "allowlist" or "blocklist" (default: "blocklist")
			Addresses []string `yaml:"addresses"` // IP addresses or CIDR ranges
		} `yaml:"ip_filter"`
		BasicAuth struct {
			Enabled bool `yaml:"enabled"`
			// Username/password stored in vault as proxy_basic_auth_user / proxy_basic_auth_pass
		} `yaml:"basic_auth"`
		GeoBlocking struct {
			Enabled          bool     `yaml:"enabled"`
			AllowedCountries []string `yaml:"allowed_countries"` // ISO 3166-1 alpha-2 codes
		} `yaml:"geo_blocking"`
		AdditionalRoutes []ProxyRoute `yaml:"additional_routes"`
	} `yaml:"security_proxy"`
	EggMode struct {
		Enabled       bool   `yaml:"enabled"`         // true = this instance is a worker egg
		MasterURL     string `yaml:"master_url"`      // WebSocket URL of the master (ws[s]://host:port/api/invasion/ws)
		SharedKey     string `yaml:"-" json:"-"`      // vault-only: egg_shared_key (hex-encoded AES-256 shared key)
		EggID         string `yaml:"egg_id"`          // UUID of this egg record on master
		NestID        string `yaml:"nest_id"`         // UUID of the nest this egg is deployed in
		TLSSkipVerify bool   `yaml:"tls_skip_verify"` // skip TLS certificate verification (for self-signed certs)
	} `yaml:"egg_mode"`

	Indexing struct {
		Enabled             bool                `yaml:"enabled"`
		Directories         []IndexingDirectory `yaml:"directories"`
		PollIntervalSeconds int                 `yaml:"poll_interval_seconds"` // default 60
		Extensions          []string            `yaml:"extensions"`            // default: .txt, .md, .json, .csv, .log, .yaml, .yml, .pdf, .docx, .xlsx, .pptx, .odt, .rtf
		IndexImages         bool                `yaml:"index_images"`          // send images to Vision LLM for analysis and index results
	} `yaml:"indexing"`

	GitHub struct {
		Enabled        bool     `yaml:"enabled"`
		ReadOnly       bool     `yaml:"readonly"`        // true = only list/get/search, block create/delete/update
		Token          string   `yaml:"-" vault:"token"` // Personal Access Token (from vault)
		Owner          string   `yaml:"owner"`           // GitHub username or organisation
		DefaultPrivate bool     `yaml:"default_private"` // true = new repos are private by default
		BaseURL        string   `yaml:"base_url"`        // API base URL (default: https://api.github.com), for GitHub Enterprise
		AllowedRepos   []string `yaml:"allowed_repos"`   // repos agent may access; empty = only agent-created repos
	} `yaml:"github"`
	Firewall struct {
		Enabled             bool   `yaml:"enabled"`
		Mode                string `yaml:"mode"`                  // "readonly" or "guard"
		PollIntervalSeconds int    `yaml:"poll_interval_seconds"` // default 60
	} `yaml:"firewall"`
	Netlify struct {
		Enabled             bool   `yaml:"enabled"`
		ReadOnly            bool   `yaml:"readonly"`              // true = only list/get, block create/update/delete/deploy
		AllowDeploy         bool   `yaml:"allow_deploy"`          // allow deploying sites
		AllowSiteManagement bool   `yaml:"allow_site_management"` // allow creating/updating/deleting sites
		AllowEnvManagement  bool   `yaml:"allow_env_management"`  // allow managing environment variables
		DefaultSiteID       string `yaml:"default_site_id"`       // default site ID for operations
		TeamSlug            string `yaml:"team_slug"`             // Netlify team/account slug
		Token               string `yaml:"-" vault:"token"`       // Personal Access Token (from vault)
	} `yaml:"netlify"`
	Vercel struct {
		Enabled                bool   `yaml:"enabled"`
		ReadOnly               bool   `yaml:"readonly"`                 // true = only list/get, block create/update/delete/deploy
		AllowDeploy            bool   `yaml:"allow_deploy"`             // allow creating deployments and promotions
		AllowProjectManagement bool   `yaml:"allow_project_management"` // allow creating/updating projects
		AllowEnvManagement     bool   `yaml:"allow_env_management"`     // allow managing environment variables
		AllowDomainManagement  bool   `yaml:"allow_domain_management"`  // allow managing project domains and aliases
		DefaultProjectID       string `yaml:"default_project_id"`       // default project ID/name for operations
		TeamID                 string `yaml:"team_id"`                  // Vercel team ID for scoped API calls
		TeamSlug               string `yaml:"team_slug"`                // Vercel team slug for scoped API calls
		Token                  string `yaml:"-" vault:"token"`          // Personal Access Token (from vault)
	} `yaml:"vercel"`
	AdGuard struct {
		Enabled  bool   `yaml:"enabled"`
		ReadOnly bool   `yaml:"readonly"` // true = only read status/stats/logs, block all mutations
		URL      string `yaml:"url"`      // e.g. http://192.168.1.1:3000
		Username string `yaml:"username"`
		Password string `yaml:"-" vault:"adguard_password"`
	} `yaml:"adguard"`

	MQTT struct {
		Enabled        bool       `yaml:"enabled"`
		ReadOnly       bool       `yaml:"readonly"` // true = only subscribe/get_messages/unsubscribe, block publish
		Broker         string     `yaml:"broker"`   // e.g. tcp://localhost:1883, mqtts://broker:8883
		ClientID       string     `yaml:"client_id"`
		Username       string     `yaml:"username"`
		Password       string     `yaml:"-" json:"-"`
		Topics         []string   `yaml:"topics"`          // topics to subscribe to on connect
		QoS            int        `yaml:"qos"`             // 0, 1, or 2
		RelayToAgent   bool       `yaml:"relay_to_agent"`  // forward incoming messages to agent
		ConnectTimeout int        `yaml:"connect_timeout"` // connection timeout in seconds (default: 15)
		TLS            MQTTTLS    `yaml:"tls"`
		Buffer         MQTTBuffer `yaml:"buffer"`
	} `yaml:"mqtt"`

	MCP struct {
		Enabled               bool                     `yaml:"enabled"`
		Servers               []MCPServer              `yaml:"servers"`
		Secrets               []MCPSecret              `yaml:"secrets"`
		PreferredCapabilities MCPPreferredCapabilities `yaml:"preferred_capabilities"`
	} `yaml:"mcp"`
	Tools struct {
		Memory struct {
			Enabled  bool `yaml:"enabled"`  // default true; disable to block manage_memory/core_memory
			ReadOnly bool `yaml:"readonly"` // true = only read/query, block store/delete/save_core/delete_core
		} `yaml:"memory"`
		KnowledgeGraph struct {
			Enabled         bool `yaml:"enabled"`          // default true; disable to block knowledge_graph
			ReadOnly        bool `yaml:"readonly"`         // true = only query/search, block add/delete
			AutoExtraction  bool `yaml:"auto_extraction"`  // nightly batch entity extraction from conversations
			PromptInjection bool `yaml:"prompt_injection"` // inject relevant KG context into system prompt
			MaxPromptNodes  int  `yaml:"max_prompt_nodes"` // max nodes to inject into prompt (default 5)
			MaxPromptChars  int  `yaml:"max_prompt_chars"` // max chars for KG context in prompt (default 800)
			RetrievalFusion bool `yaml:"retrieval_fusion"` // cross-reference RAG↔KG for bidirectional enrichment (default true)
		} `yaml:"knowledge_graph"`
		SecretsVault struct {
			Enabled  bool `yaml:"enabled"`  // default true; disable to block secrets_vault
			ReadOnly bool `yaml:"readonly"` // true = only get/list, block set/delete
		} `yaml:"secrets_vault"`
		Scheduler struct {
			Enabled  bool `yaml:"enabled"`  // default true; disable to block cron_scheduler
			ReadOnly bool `yaml:"readonly"` // true = only list, block add/remove/enable/disable
		} `yaml:"scheduler"`
		Notes struct {
			Enabled  bool `yaml:"enabled"`  // default true; disable to block manage_notes
			ReadOnly bool `yaml:"readonly"` // true = only list/get, block add/update/toggle/delete
		} `yaml:"notes"`
		Missions struct {
			Enabled  bool `yaml:"enabled"`  // default true; disable to block manage_missions
			ReadOnly bool `yaml:"readonly"` // true = only list/get, block create/update/delete/run
		} `yaml:"missions"`
		StopProcess struct {
			Enabled bool `yaml:"enabled"` // default true; disable to block process kill
		} `yaml:"stop_process"`
		Inventory struct {
			Enabled bool `yaml:"enabled"` // default true; disable to block register_device/register_server
		} `yaml:"inventory"`
		MemoryMaintenance struct {
			Enabled bool `yaml:"enabled"` // default true; disable to block archive_memory/optimize_memory
		} `yaml:"memory_maintenance"`
		Journal struct {
			Enabled  bool `yaml:"enabled"`  // default true; enable manage_journal tool
			ReadOnly bool `yaml:"readonly"` // true = only list/search, block add/delete
		} `yaml:"journal"`
		WOL struct {
			Enabled bool `yaml:"enabled"` // enable wake_on_lan tool (send magic packet to devices with MAC address)
		} `yaml:"wol"`
		WebScraper struct {
			Enabled         bool   `yaml:"enabled"`                   // enable web_scraper (default true)
			SummaryMode     bool   `yaml:"summary_mode"`              // send scraped content to a separate LLM for summarisation before returning to agent
			SummaryProvider string `yaml:"summary_provider" json:"-"` // legacy provider entry for summarisation; helper-owned runtime prefers llm.helper_*
			// resolved fields (populated by ResolveProviders)
			SummaryBaseURL string `yaml:"-" json:"-"`
			SummaryAPIKey  string `yaml:"-" json:"-"`
			SummaryModel   string `yaml:"-" json:"-"`
		} `yaml:"web_scraper"`
		Wikipedia struct {
			SummaryMode     bool   `yaml:"summary_mode"`              // summarise Wikipedia content via a separate LLM before returning to agent
			SummaryProvider string `yaml:"summary_provider" json:"-"` // legacy provider entry for summarisation; helper-owned runtime prefers llm.helper_*
			// resolved fields (populated by ResolveProviders)
			SummaryBaseURL string `yaml:"-" json:"-"`
			SummaryAPIKey  string `yaml:"-" json:"-"`
			SummaryModel   string `yaml:"-" json:"-"`
		} `yaml:"wikipedia"`
		DDGSearch struct {
			SummaryMode     bool   `yaml:"summary_mode"`              // summarise DDG search results via a separate LLM before returning to agent
			SummaryProvider string `yaml:"summary_provider" json:"-"` // legacy provider entry for summarisation; helper-owned runtime prefers llm.helper_*
			// resolved fields (populated by ResolveProviders)
			SummaryBaseURL string `yaml:"-" json:"-"`
			SummaryAPIKey  string `yaml:"-" json:"-"`
			SummaryModel   string `yaml:"-" json:"-"`
		} `yaml:"ddg_search"`
		PDFExtractor struct {
			Enabled         bool   `yaml:"enabled"`                   // enable pdf_extractor (default true)
			SummaryMode     bool   `yaml:"summary_mode"`              // summarise extracted PDF text via a separate LLM before returning to agent
			SummaryProvider string `yaml:"summary_provider" json:"-"` // legacy provider entry for summarisation; helper-owned runtime prefers llm.helper_*
			// resolved fields (populated by ResolveProviders)
			SummaryBaseURL string `yaml:"-" json:"-"`
			SummaryAPIKey  string `yaml:"-" json:"-"`
			SummaryModel   string `yaml:"-" json:"-"`
		} `yaml:"pdf_extractor"`
		MediaConversion MediaConversionConfig `yaml:"media_conversion"`
		DocumentCreator DocumentCreatorConfig `yaml:"document_creator"`
		WebCapture      struct {
			Enabled bool `yaml:"enabled"` // enable web_capture tool (screenshot/pdf via headless Chromium, default true)
		} `yaml:"web_capture"`
		BrowserAutomation struct {
			Enabled bool `yaml:"enabled"` // enable browser_automation tool (Playwright sidecar driven browser sessions)
		} `yaml:"browser_automation"`
		NetworkPing struct {
			Enabled bool `yaml:"enabled"` // enable network_ping tool (ICMP echo, default true)
		} `yaml:"network_ping"`
		NetworkScan struct {
			Enabled bool `yaml:"enabled"` // enable network_scan / mdns_scan tool (mDNS/Bonjour discovery)
		} `yaml:"network_scan"`
		FormAutomation struct {
			Enabled bool `yaml:"enabled"` // enable form_automation tool (headless browser form fill/submit)
		} `yaml:"form_automation"`
		UPnPScan struct {
			Enabled bool `yaml:"enabled"` // enable upnp_scan tool (UPnP/SSDP device discovery)
		} `yaml:"upnp_scan"`
		Contacts struct {
			Enabled bool `yaml:"enabled"` // enable address_book tool (contact management)
		} `yaml:"contacts"`
		Planner struct {
			Enabled bool `yaml:"enabled"` // enable manage_appointments and manage_todos tools
		} `yaml:"planner"`
		PythonSecretInjection struct {
			Enabled bool `yaml:"enabled"` // allow Python tools to request vault secrets via vault_keys parameter
		} `yaml:"python_secret_injection"`
		PythonToolBridge struct {
			Enabled      bool     `yaml:"enabled"`       // allow Python skills to call internal AuraGo tools via loopback API (default: false)
			AllowedTools []string `yaml:"allowed_tools"` // tools skills may invoke; empty = none (must be explicitly listed)
			// AllowedSQLConnections restricts sql_query tool usage for Python skills.
			// Empty = block all SQL bridge calls (when sql_query is otherwise allowed).
			AllowedSQLConnections []string `yaml:"allowed_sql_connections"`
		} `yaml:"python_tool_bridge"`
		PythonTimeoutSeconds     int `yaml:"python_timeout_seconds"`     // foreground Python/shell execution timeout (default: 30)
		SkillTimeoutSeconds      int `yaml:"skill_timeout_seconds"`      // skill execution timeout (default: 120)
		BackgroundTimeoutSeconds int `yaml:"background_timeout_seconds"` // background Python/shell/tool execution timeout (default: 3600)
		SkillManager             struct {
			Enabled          bool `yaml:"enabled"`            // enable skill manager web UI and API (default: true)
			AllowUploads     bool `yaml:"allow_uploads"`      // allow uploading new skills via web UI (default: true)
			ReadOnly         bool `yaml:"readonly"`           // read-only mode: list/view only (default: false)
			RequireScan      bool `yaml:"require_scan"`       // require security scan before enabling (default: true)
			RequireSandbox   bool `yaml:"require_sandbox"`    // require sandbox for skill execution (default: false)
			MaxUploadSizeMB  int  `yaml:"max_upload_size_mb"` // max upload file size in MB (default: 1)
			AutoEnableClean  bool `yaml:"auto_enable_clean"`  // auto-enable skills that pass all scans (default: false)
			ScanWithGuardian bool `yaml:"scan_with_guardian"` // use LLM Guardian for code review on upload (default: false, costs tokens)
		} `yaml:"skill_manager"`
		DaemonSkills struct {
			Enabled              bool    `yaml:"enabled"`                // enable daemon skill support (default: false, opt-in)
			MaxConcurrentDaemons int     `yaml:"max_concurrent_daemons"` // max simultaneous daemon processes (default: 5)
			GlobalRateLimitSecs  int     `yaml:"global_rate_limit_secs"` // minimum seconds between any two wake-ups (default: 60)
			MaxWakeUpsPerHour    int     `yaml:"max_wakeups_per_hour"`   // per-skill circuit breaker threshold (default: 6)
			MaxBudgetPerHourUSD  float64 `yaml:"max_budget_per_hour"`    // hourly wake-up cost cap in USD (default: 0.50)
		} `yaml:"daemon_skills"`
	} `yaml:"tools"`
	MissionPreparation struct {
		Enabled              bool    `yaml:"enabled"`                // enable mission preparation feature (default: false)
		Provider             string  `yaml:"provider"`               // provider entry ID; empty = use main LLM provider
		TimeoutSeconds       int     `yaml:"timeout_seconds"`        // LLM call timeout in seconds (default: 120)
		MaxEssentialTools    int     `yaml:"max_essential_tools"`    // max tools to include in preparation guide (default: 5)
		CacheExpiryHours     int     `yaml:"cache_expiry_hours"`     // prepared data cache lifetime in hours (default: 24)
		MinConfidence        float64 `yaml:"min_confidence"`         // minimum confidence to mark as prepared (default: 0.5)
		AutoPrepareScheduled bool    `yaml:"auto_prepare_scheduled"` // auto-prepare scheduled missions (default: true)
		// resolved fields (populated by ResolveProviders)
		ProviderType string `yaml:"-" json:"-"` // resolved: provider type
		BaseURL      string `yaml:"-" json:"-"` // resolved from provider entry
		APIKey       string `yaml:"-" json:"-"` // resolved from provider entry
		Model        string `yaml:"-" json:"-"` // resolved from provider entry
	} `yaml:"mission_preparation"`
	Sandbox struct {
		Enabled        bool   `yaml:"enabled"`
		Backend        string `yaml:"backend"`         // "docker" (default), "podman"
		DockerHost     string `yaml:"docker_host"`     // override; empty = inherit from docker.host or auto-detect
		Image          string `yaml:"image"`           // container image (default: "python:3.11-slim")
		AutoInstall    bool   `yaml:"auto_install"`    // auto-install llm-sandbox Python package (default: true)
		PoolSize       int    `yaml:"pool_size"`       // pre-warmed container pool size (0 = no pooling)
		TimeoutSeconds int    `yaml:"timeout_seconds"` // execution timeout per run (default: 30)
		NetworkEnabled bool   `yaml:"network_enabled"` // allow sandbox containers to access the network
		KeepAlive      bool   `yaml:"keep_alive"`      // keep sandbox MCP server running between calls
	} `yaml:"sandbox"`
	ShellSandbox struct {
		Enabled       bool `yaml:"enabled"`          // enable Landlock-based shell sandbox (default: false)
		MaxMemoryMB   int  `yaml:"max_memory_mb"`    // RLIMIT_AS in MiB (default: 1024)
		MaxCPUSeconds int  `yaml:"max_cpu_seconds"`  // RLIMIT_CPU in seconds (default: 30)
		MaxProcesses  int  `yaml:"max_processes"`    // RLIMIT_NPROC (default: 50)
		MaxFileSizeMB int  `yaml:"max_file_size_mb"` // RLIMIT_FSIZE in MiB (default: 100)
		AllowedPaths  []struct {
			Path     string `yaml:"path"`
			ReadOnly bool   `yaml:"readonly"`
		} `yaml:"allowed_paths"` // additional paths beyond defaults
	} `yaml:"shell_sandbox"`
	AIGateway struct {
		Enabled   bool   `yaml:"enabled"`
		AccountID string `yaml:"account_id"`               // Cloudflare account ID
		GatewayID string `yaml:"gateway_id"`               // AI Gateway name/slug
		Token     string `yaml:"-" vault:"token" json:"-"` // optional Cloudflare AI Gateway token (vault-only)
	} `yaml:"ai_gateway"`
	MCPServer struct {
		Enabled           bool     `yaml:"enabled"`
		AllowedTools      []string `yaml:"allowed_tools"`       // tool names to expose; empty = none
		RequireAuth       bool     `yaml:"require_auth"`        // require Bearer token or session cookie
		VSCodeDebugBridge bool     `yaml:"vscode_debug_bridge"` // enable the VS Code live-debug bridge preset
	} `yaml:"mcp_server"`
	N8n struct {
		Enabled        bool     `yaml:"enabled"`
		ReadOnly       bool     `yaml:"readonly"`         // true = only read operations, block write
		WebhookBaseURL string   `yaml:"webhook_base_url"` // Full n8n webhook URL (e.g. https://n8n.example.com/webhook/abc-123)
		AllowedEvents  []string `yaml:"allowed_events"`   // events that trigger n8n webhooks
		RequireToken   bool     `yaml:"require_token"`    // require Bearer token auth (default: true)
		AllowedTools   []string `yaml:"allowed_tools"`    // tools n8n can execute; empty = all enabled
		RateLimitRPS   int      `yaml:"rate_limit_rps"`   // requests per second limit (0 = unlimited)
		Scopes         []string `yaml:"scopes"`           // allowed operation scopes; empty = all (n8n:read, n8n:chat, n8n:tools, n8n:memory, n8n:missions, n8n:admin)
	} `yaml:"n8n"`
	SQLConnections struct {
		Enabled              bool `yaml:"enabled"`
		ReadOnly             bool `yaml:"readonly"`         // global read-only: block all mutating queries regardless of connection permissions
		AllowManagement      bool `yaml:"allow_management"` // allow agent to create/update/delete connections (default: false)
		MaxPoolSize          int  `yaml:"max_pool_size"`
		ConnectionTimeoutSec int  `yaml:"connection_timeout_sec"`
		QueryTimeoutSec      int  `yaml:"query_timeout_sec"`
		MaxResultRows        int  `yaml:"max_result_rows"`
		// Rate limiting: minimum seconds between accesses per connection (0 = disabled)
		RateLimitWindowSec int `yaml:"rate_limit_window_sec"`
		// Idle TTL: how long to keep idle connections before evicting them (in seconds, 0 = use default)
		IdleTTLSec int `yaml:"idle_ttl_sec"`
	} `yaml:"sql_connections"`
	GoogleWorkspace struct {
		Enabled       bool   `yaml:"enabled"`
		ReadOnly      bool   `yaml:"readonly"`       // true = only read operations, block send/create/update/write
		Gmail         bool   `yaml:"gmail"`          // Gmail read access
		GmailSend     bool   `yaml:"gmail_send"`     // Gmail send (requires !readonly)
		Calendar      bool   `yaml:"calendar"`       // Calendar read access
		CalendarWrite bool   `yaml:"calendar_write"` // Calendar create/update (requires !readonly)
		Drive         bool   `yaml:"drive"`          // Drive read access
		Docs          bool   `yaml:"docs"`           // Docs read access
		DocsWrite     bool   `yaml:"docs_write"`     // Docs create/write (requires !readonly)
		Sheets        bool   `yaml:"sheets"`         // Sheets read access
		SheetsWrite   bool   `yaml:"sheets_write"`   // Sheets write (requires !readonly)
		ClientID      string `yaml:"client_id"`      // Google OAuth2 Client ID
		ClientSecret  string `yaml:"-" json:"-"`     // vault-only: google_workspace_client_secret
		AccessToken   string `yaml:"-" json:"-"`     // resolved from OAuth token in vault
		RefreshToken  string `yaml:"-" json:"-"`     // resolved from OAuth token in vault
		TokenExpiry   string `yaml:"-" json:"-"`     // resolved: RFC3339 expiry
	} `yaml:"google_workspace"`
	ImageGeneration struct {
		Enabled           bool   `yaml:"enabled"`
		Provider          string `yaml:"provider"`           // references ProviderEntry ID
		Model             string `yaml:"model"`              // default model override (empty = use provider default)
		DefaultSize       string `yaml:"default_size"`       // e.g. "1024x1024"
		DefaultQuality    string `yaml:"default_quality"`    // "standard", "hd"
		DefaultStyle      string `yaml:"default_style"`      // "natural", "vivid"
		PromptEnhancement bool   `yaml:"prompt_enhancement"` // LLM improves prompt before generation
		MaxMonthly        int    `yaml:"max_monthly"`        // 0 = unlimited
		MaxDaily          int    `yaml:"max_daily"`          // 0 = unlimited
		// resolved fields (populated by ResolveProviders)
		ProviderType  string `yaml:"-" json:"-"` // resolved: openai, openrouter, stability, ideogram, google, etc.
		BaseURL       string `yaml:"-" json:"-"` // resolved from provider entry
		APIKey        string `yaml:"-" json:"-"` // resolved from provider entry
		ResolvedModel string `yaml:"-" json:"-"` // resolved: model from provider if not overridden
	} `yaml:"image_generation"`
	MusicGeneration struct {
		Enabled  bool   `yaml:"enabled"`
		Provider string `yaml:"provider"`  // references ProviderEntry ID
		Model    string `yaml:"model"`     // model override (empty = use provider default)
		MaxDaily int    `yaml:"max_daily"` // 0 = unlimited
		// resolved fields (populated by ResolveProviders)
		ProviderType  string `yaml:"-" json:"-"` // minimax, google, google_lyria, etc.
		BaseURL       string `yaml:"-" json:"-"` // resolved from provider entry
		APIKey        string `yaml:"-" json:"-"` // resolved from provider entry
		ResolvedModel string `yaml:"-" json:"-"` // resolved: model from provider if not overridden
	} `yaml:"music_generation"`
	OneDrive struct {
		Enabled      bool   `yaml:"enabled"`
		ReadOnly     bool   `yaml:"readonly"`   // true = only list/read/search/quota, block upload/delete/move/copy/share/mkdir
		ClientID     string `yaml:"client_id"`  // Azure App Registration Client ID (public client for Device Code flow)
		ClientSecret string `yaml:"-" json:"-"` // vault-only: onedrive_client_secret (optional, only for confidential apps)
		TenantID     string `yaml:"tenant_id"`  // "common" (default), "consumers", "organizations", or tenant UUID
		AccessToken  string `yaml:"-" json:"-"` // resolved from OAuth token in vault
		RefreshToken string `yaml:"-" json:"-"` // resolved from OAuth token in vault
		TokenExpiry  string `yaml:"-" json:"-"` // resolved: RFC3339 expiry
	} `yaml:"onedrive"`

	// TrueNAS integration for ZFS storage management
	TrueNAS TrueNASConfig `yaml:"truenas"`

	// Uptime Kuma monitoring integration
	UptimeKuma UptimeKumaConfig `yaml:"uptime_kuma"`

	// Jellyfin media server integration
	Jellyfin JellyfinConfig `yaml:"jellyfin"`

	// Obsidian knowledge management via Local REST API plugin
	Obsidian ObsidianConfig `yaml:"obsidian"`

	// LDAP/Active Directory integration
	LDAP LDAPConfig `yaml:"ldap"`

	// gwProvider is a synthetic ProviderEntry used by FindProvider for Google Workspace OAuth.
	gwProvider ProviderEntry `yaml:"-" json:"-"`
}

// TrueNASConfig holds configuration for TrueNAS integration.
type TrueNASConfig struct {
	Enabled          bool   `yaml:"enabled"`
	ReadOnly         bool   `yaml:"readonly"`          // true = only list/read, block create/update/delete/rollback
	AllowDestructive bool   `yaml:"allow_destructive"` // allow dataset deletion, snapshot rollback, pool scrub
	Host             string `yaml:"host"`              // TrueNAS hostname or IP (e.g. "truenas.local")
	Port             int    `yaml:"port"`              // API port (default: 443)
	UseHTTPS         bool   `yaml:"use_https"`         // use HTTPS (default: true)
	APIKey           string `yaml:"-" json:"-"`        // vault-only: truenas_api_key
	InsecureSSL      bool   `yaml:"insecure_ssl"`      // skip TLS verification for self-signed certs (default: false)
	ConnectTimeout   int    `yaml:"connect_timeout"`   // connection timeout in seconds (default: 30)
	RequestTimeout   int    `yaml:"request_timeout"`   // request timeout in seconds (default: 60)
	// Default settings for new shares
	DefaultShares struct {
		SMBEnabled   bool   `yaml:"smb_enabled"`   // enable SMB by default
		NFSEnabled   bool   `yaml:"nfs_enabled"`   // enable NFS by default
		AtimeEnabled bool   `yaml:"atime_enabled"` // enable access time tracking
		Compression  string `yaml:"compression"`   // default compression: "lz4", "gzip", "zle", "off"
	} `yaml:"default_shares,omitempty"`
	// Snapshot retention policies
	SnapshotRetention struct {
		Enabled      bool `yaml:"enabled"`       // enable automatic snapshot cleanup
		DefaultDays  int  `yaml:"default_days"`  // default retention in days (0 = forever)
		MaxSnapshots int  `yaml:"max_snapshots"` // maximum snapshots per dataset (0 = unlimited)
	} `yaml:"snapshot_retention,omitempty"`
}

// UptimeKumaConfig holds configuration for the read-only Uptime Kuma metrics integration.
type UptimeKumaConfig struct {
	Enabled             bool   `yaml:"enabled"`
	BaseURL             string `yaml:"base_url"`
	APIKey              string `yaml:"-" json:"-"`
	InsecureSSL         bool   `yaml:"insecure_ssl"`
	RequestTimeout      int    `yaml:"request_timeout"`
	PollIntervalSeconds int    `yaml:"poll_interval_seconds"`
	RelayToAgent        bool   `yaml:"relay_to_agent"`
	RelayInstruction    string `yaml:"relay_instruction"`
}

// JellyfinConfig holds configuration for Jellyfin media server integration.
type JellyfinConfig struct {
	Enabled          bool   `yaml:"enabled"`
	ReadOnly         bool   `yaml:"readonly"`          // true = only read/list, block playback control/refresh/delete
	AllowDestructive bool   `yaml:"allow_destructive"` // allow item deletion
	Host             string `yaml:"host"`              // Jellyfin hostname or IP (e.g. "jellyfin.local")
	Port             int    `yaml:"port"`              // API port (default: 8096)
	UseHTTPS         bool   `yaml:"use_https"`         // use HTTPS (default: false)
	APIKey           string `yaml:"-" json:"-"`        // vault-only: jellyfin_api_key
	InsecureSSL      bool   `yaml:"insecure_ssl"`      // skip TLS verification for self-signed certs
	ConnectTimeout   int    `yaml:"connect_timeout"`   // connection timeout in seconds (default: 30)
	RequestTimeout   int    `yaml:"request_timeout"`   // request timeout in seconds (default: 60)
}

// ObsidianConfig holds configuration for Obsidian integration via Local REST API plugin.
type ObsidianConfig struct {
	Enabled          bool   `yaml:"enabled"`
	ReadOnly         bool   `yaml:"readonly"`          // true = only read/list/search, block create/update/delete
	AllowDestructive bool   `yaml:"allow_destructive"` // allow note deletion
	Host             string `yaml:"host"`              // Obsidian host (default: 127.0.0.1)
	Port             int    `yaml:"port"`              // API port (default: 27124)
	UseHTTPS         bool   `yaml:"use_https"`         // use HTTPS (default: true)
	APIKey           string `yaml:"-" json:"-"`        // vault-only: obsidian_api_key
	InsecureSSL      bool   `yaml:"insecure_ssl"`      // skip TLS verification for self-signed certs (default: true)
	ConnectTimeout   int    `yaml:"connect_timeout"`   // connection timeout in seconds (default: 10)
	RequestTimeout   int    `yaml:"request_timeout"`   // request timeout in seconds (default: 30)
}

// UnmarshalYAML keeps backward compatibility with the legacy read_only key
// while preferring the canonical readonly key when both are present.
func (c *ObsidianConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawObsidianConfig ObsidianConfig

	var raw rawObsidianConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}

	*c = ObsidianConfig(raw)
	if obsidianConfigHasYAMLKey(value, "readonly") || !obsidianConfigHasYAMLKey(value, "read_only") {
		return nil
	}

	var legacy struct {
		ReadOnly bool `yaml:"read_only"`
	}
	if err := value.Decode(&legacy); err != nil {
		return err
	}
	c.ReadOnly = legacy.ReadOnly
	return nil
}

func obsidianConfigHasYAMLKey(node *yaml.Node, key string) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	if node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return true
		}
	}
	return false
}

// LDAPConfig holds configuration for LDAP/Active Directory integration.
type LDAPConfig struct {
	Enabled            bool   `yaml:"enabled"`
	ReadOnly           bool   `yaml:"readonly"`             // true = only search/authenticate, block mutations
	Host               string `yaml:"host"`                 // LDAP server hostname or IP
	Port               int    `yaml:"port"`                 // LDAPS port (default: 636) or LDAP port (default: 389)
	UseTLS             bool   `yaml:"use_tls"`              // use LDAPS (default: true)
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"` // skip TLS certificate verification (default: false)
	BaseDN             string `yaml:"base_dn"`              // base DN for searches (e.g. "dc=example,dc=com")
	BindDN             string `yaml:"bind_dn"`              // service account DN for binding
	BindPassword       string `yaml:"-" json:"-"`           // vault-only: ldap_bind_password
	UserSearchBase     string `yaml:"user_search_base"`     // subtree for user searches (default: BaseDN)
	GroupSearchBase    string `yaml:"group_search_base"`    // subtree for group searches (default: BaseDN)
	ConnectTimeout     int    `yaml:"connect_timeout"`      // connection timeout in seconds (default: 10)
	RequestTimeout     int    `yaml:"request_timeout"`      // request timeout in seconds (default: 30)
}

// A2ASkill describes a skill advertised in the A2A Agent Card.
type A2ASkill struct {
	ID          string   `yaml:"id"          json:"id"`
	Name        string   `yaml:"name"        json:"name"`
	Description string   `yaml:"description" json:"description"`
	Tags        []string `yaml:"tags"        json:"tags"`
}

// A2ARemoteAgent describes a remote A2A agent that AuraGo can connect to.
type A2ARemoteAgent struct {
	ID          string `yaml:"id"           json:"id"`
	Name        string `yaml:"name"         json:"name"`
	CardURL     string `yaml:"card_url"     json:"card_url"` // URL to fetch the Agent Card (e.g. https://host/.well-known/agent-card.json)
	APIKey      string `yaml:"-"            json:"-"`        // resolved from vault: a2a_remote_{id}_api_key
	BearerToken string `yaml:"-"            json:"-"`        // resolved from vault: a2a_remote_{id}_bearer_token
	Enabled     bool   `yaml:"enabled"      json:"enabled"`
}

// MCPServer describes one external MCP server in the config.
type MCPServer struct {
	Name               string            `yaml:"name"                    json:"name"`
	Command            string            `yaml:"command"                 json:"command"`
	Args               []string          `yaml:"args"                    json:"args"`
	Env                map[string]string `yaml:"env"                     json:"env"`
	Enabled            bool              `yaml:"enabled"                 json:"enabled"`
	Runtime            string            `yaml:"runtime,omitempty"       json:"runtime"`
	DockerImage        string            `yaml:"docker_image,omitempty"  json:"docker_image"`
	DockerCommand      string            `yaml:"docker_command,omitempty" json:"docker_command"`
	AllowLocalFallback bool              `yaml:"allow_local_fallback,omitempty" json:"allow_local_fallback"`
	HostWorkdir        string            `yaml:"host_workdir,omitempty"  json:"host_workdir"`
	ContainerWorkdir   string            `yaml:"container_workdir,omitempty" json:"container_workdir"`
}

// MCPSecret stores a vault-backed MCP secret alias visible in the config UI.
type MCPSecret struct {
	Alias       string `yaml:"alias"                 json:"alias"`
	Label       string `yaml:"label,omitempty"       json:"label"`
	Description string `yaml:"description,omitempty" json:"description"`
}

// MCPPreferredToolSelection binds an AuraGo capability to a specific external MCP tool.
type MCPPreferredToolSelection struct {
	Server string `yaml:"server" json:"server"`
	Tool   string `yaml:"tool"   json:"tool"`
}

// MCPPreferredCapabilities stores user-friendly backend mappings for selected AuraGo capabilities.
type MCPPreferredCapabilities struct {
	WebSearch MCPPreferredToolSelection `yaml:"web_search" json:"web_search"`
	Vision    MCPPreferredToolSelection `yaml:"vision"     json:"vision"`
}

// ProxyRoute defines an additional reverse proxy route for the security proxy.
type ProxyRoute struct {
	Name     string `yaml:"name"     json:"name"`     // display name
	Domain   string `yaml:"domain"   json:"domain"`   // hostname to match (e.g. "grafana.example.com")
	Upstream string `yaml:"upstream" json:"upstream"` // backend URL (e.g. "http://localhost:3000")
}

// CloudflareIngressRule defines a custom ingress entry for cloudflared.
type CloudflareIngressRule struct {
	Hostname string `yaml:"hostname" json:"hostname"` // e.g. "app.example.com"
	Service  string `yaml:"service"  json:"service"`  // e.g. "http://localhost:8088"
	Path     string `yaml:"path"     json:"path"`     // optional URL path prefix
}

// ModelCost defines per-model token pricing.
type ModelCost struct {
	Name             string  `yaml:"name"`
	InputPerMillion  float64 `yaml:"input_per_million"`
	OutputPerMillion float64 `yaml:"output_per_million"`
}

// ModelCostRates defines fallback pricing for unlisted models.
type ModelCostRates struct {
	InputPerMillion  float64 `yaml:"input_per_million"`
	OutputPerMillion float64 `yaml:"output_per_million"`
}

// GetSpecialist returns a pointer to the SpecialistConfig for the given role.
// Returns nil if the role is unknown.
func (c *Config) GetSpecialist(role string) *SpecialistConfig {
	switch role {
	case "researcher":
		return &c.CoAgents.Specialists.Researcher
	case "coder":
		return &c.CoAgents.Specialists.Coder
	case "designer":
		return &c.CoAgents.Specialists.Designer
	case "security":
		return &c.CoAgents.Specialists.Security
	case "writer":
		return &c.CoAgents.Specialists.Writer
	default:
		return nil
	}
}
