package config

// Defined here to avoid a circular dependency on the security package.
type SecretReader interface {
	ReadSecret(key string) (string, error)
}

// OAuthToken represents an OAuth2 token stored in the vault.
type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Expiry       string `json:"expiry"` // RFC3339
}

// ProviderEntry defines a named LLM provider connection that can be referenced
// by multiple config slots (LLM, Fallback, Vision, Whisper, Embeddings, etc.).
type ProviderEntry struct {
	ID      string `yaml:"id"       json:"id"`                         // unique slug, e.g. "main", "vision", "local-ollama"
	Name    string `yaml:"name"     json:"name"`                       // human-readable label shown in UI
	Type    string `yaml:"type"     json:"type"`                       // openai, openrouter, ollama, anthropic, google, custom
	BaseURL string `yaml:"base_url" json:"base_url"`                   // API base URL
	APIKey  string `yaml:"-" vault:"api_key" json:"api_key,omitempty"` // API key (vault-only)
	Model   string `yaml:"model"    json:"model"`                      // default model name

	// Cloudflare Workers AI — required when Type is "workers-ai"
	AccountID string `yaml:"account_id,omitempty" json:"account_id"` // Cloudflare account ID

	// OAuth2 Authorization Code flow (optional, alternative to static API key)
	AuthType          string `yaml:"auth_type,omitempty"           json:"auth_type"`           // "api_key" (default) or "oauth2"
	OAuthAuthURL      string `yaml:"oauth_auth_url,omitempty"      json:"oauth_auth_url"`      // authorization endpoint
	OAuthTokenURL     string `yaml:"oauth_token_url,omitempty"     json:"oauth_token_url"`     // token exchange endpoint
	OAuthClientID     string `yaml:"oauth_client_id,omitempty"     json:"oauth_client_id"`     // client ID
	OAuthClientSecret string `yaml:"-" vault:"oauth_client_secret" json:"oauth_client_secret"` // client secret (vault-only)
	OAuthScopes       string `yaml:"oauth_scopes,omitempty"        json:"oauth_scopes"`        // space-separated scopes

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
	Password      string `yaml:"-"              json:"password,omitempty"` // excluded from YAML (secret)
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
		Provider           string  `yaml:"provider"`          // provider entry ID (references Providers[].ID)
		ProviderType       string  `yaml:"-"       json:"-"`  // resolved: openai, openrouter, ollama etc.
		BaseURL            string  `yaml:"-"       json:"-"`  // resolved from provider entry
		APIKey             string  `yaml:"-"       json:"-"`  // resolved from provider entry
		Model              string  `yaml:"-"       json:"-"`  // resolved from provider entry
		AccountID          string  `yaml:"-"       json:"-"`  // resolved from provider entry (workers-ai)
		LegacyURL          string  `yaml:"base_url" json:"-"` // legacy/compat: inline base URL from old config format
		LegacyAPIKey       string  `yaml:"api_key"  json:"-"` // legacy/compat: inline API key from old config format
		LegacyModel        string  `yaml:"model"    json:"-"` // legacy/compat: inline model from old config format
		UseNativeFunctions bool    `yaml:"use_native_functions"`
		Temperature        float64 `yaml:"temperature"`        // 0.0–2.0; default 0.7; 0 = provider default
		StructuredOutputs  bool    `yaml:"structured_outputs"` // enable structured output mode (only for supported models)
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
	} `yaml:"sqlite"`
	Embeddings struct {
		Provider      string `yaml:"provider"`          // "disabled" or provider entry ID
		ProviderType  string `yaml:"-"       json:"-"`  // resolved
		BaseURL       string `yaml:"-"       json:"-"`  // resolved from provider
		APIKey        string `yaml:"-"       json:"-"`  // resolved from provider
		Model         string `yaml:"-"       json:"-"`  // resolved from provider
		InternalModel string `yaml:"internal_model"`    // legacy/compat: model when using main LLM provider
		ExternalURL   string `yaml:"external_url"`      // legacy/compat: dedicated endpoint URL
		ExternalModel string `yaml:"external_model"`    // legacy/compat: dedicated endpoint model
		LegacyAPIKey  string `yaml:"api_key"  json:"-"` // legacy/compat: separate API key
		LocalOllama   struct {
			Enabled       bool   `yaml:"enabled"`        // auto-manage an Ollama container for local embeddings
			Model         string `yaml:"model"`          // embedding model (default: "nomic-embed-text")
			ContainerPort int    `yaml:"container_port"` // host port for the managed container (default: 11435)
			UseHostGPU    bool   `yaml:"use_host_gpu"`   // pass GPU devices into the container (Linux only)
			GPUBackend    string `yaml:"gpu_backend"`    // "auto", "nvidia", "amd", "intel" (default: "auto")
		} `yaml:"local_ollama"`
	} `yaml:"embeddings"`
	Agent struct {
		SystemLanguage             string `yaml:"system_language"`
		StepDelaySeconds           int    `yaml:"step_delay_seconds"`
		MemoryCompressionCharLimit int    `yaml:"memory_compression_char_limit"`
		PersonalityEngine          bool   `yaml:"personality_engine"`
		PersonalityEngineV2        bool   `yaml:"personality_engine_v2"`
		PersonalityV2Provider      string `yaml:"personality_v2_provider"` // provider entry ID for V2 analysis
		PersonalityV2Model         string `yaml:"personality_v2_model"`    // legacy: model name
		PersonalityV2URL           string `yaml:"personality_v2_url"`      // legacy: base URL
		PersonalityV2APIKey        string `yaml:"personality_v2_api_key"`  // legacy: API key
		PersonalityV2ProviderType  string `yaml:"-"       json:"-"`        // resolved
		PersonalityV2ResolvedURL   string `yaml:"-"       json:"-"`        // resolved
		PersonalityV2ResolvedKey   string `yaml:"-"       json:"-"`        // resolved
		PersonalityV2ResolvedModel string `yaml:"-"       json:"-"`        // resolved
		CorePersonality            string `yaml:"core_personality"`
		SystemPromptTokenBudget    int    `yaml:"system_prompt_token_budget"`
		ContextWindow              int    `yaml:"context_window"`
		ShowToolResults            bool   `yaml:"show_tool_results"`
		WorkflowFeedback           bool   `yaml:"workflow_feedback"`
		DebugMode                  bool   `yaml:"debug_mode"`
		CoreMemoryMaxEntries       int    `yaml:"core_memory_max_entries"`     // 0 = unlimited; default 200
		CoreMemoryCapMode          string `yaml:"core_memory_cap_mode"`        // "soft" (default) | "hard"
		UserProfiling              bool   `yaml:"user_profiling"`              // opt-in: collect user profile via V2 analysis
		UserProfilingThreshold     int    `yaml:"user_profiling_threshold"`    // min confidence for profile summary (default: 3)
		PersonalityV2TimeoutSecs   int    `yaml:"personality_v2_timeout_secs"` // timeout for V2 mood analysis LLM call (default: 30)
		ToolOutputLimit            int    `yaml:"tool_output_limit"`           // max characters of a single tool result added to context (0 = unlimited, default: 50000)
		SudoEnabled                bool   `yaml:"sudo_enabled"`                // allow execute_sudo tool (password must be stored in vault as "sudo_password")
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
			Enabled           bool     `yaml:"enabled"`              // enable adaptive tool filtering (default: false)
			MaxTools          int      `yaml:"max_tools"`            // maximum tool schemas to send to LLM (0 = unlimited, default: 60)
			DecayHalfLifeDays float64  `yaml:"decay_half_life_days"` // usage score halves after this many days (default: 7)
			AlwaysInclude     []string `yaml:"always_include"`       // tools always included regardless of usage
		} `yaml:"adaptive_tools"`
		MaxToolGuides int `yaml:"max_tool_guides"` // maximum tool guide documents injected into prompt (default: 5)
	} `yaml:"agent"`
	CircuitBreaker struct {
		MaxToolCalls              int      `yaml:"max_tool_calls"`
		LLMTimeoutSeconds         int      `yaml:"llm_timeout_seconds"`
		MaintenanceTimeoutMinutes int      `yaml:"maintenance_timeout_minutes"`
		RetryIntervals            []string `yaml:"retry_intervals"`
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
		Enabled           bool `yaml:"enabled"`             // enable nightly STM→LTM consolidation (default true)
		AutoOptimize      bool `yaml:"auto_optimize"`       // run optimize_memory after consolidation (default true)
		ArchiveRetainDays int  `yaml:"archive_retain_days"` // keep archived messages for N days (default 30)
		MaxBatchMessages  int  `yaml:"max_batch_messages"`  // max messages per consolidation batch (default 200)
		OptimizeThreshold int  `yaml:"optimize_threshold"`  // priority threshold for auto-optimize (default 1)
	} `yaml:"consolidation"`
	MemoryAnalysis struct {
		Enabled          bool    `yaml:"enabled"`                // enable dedicated memory analysis provider
		RealTime         bool    `yaml:"real_time"`              // analyze each user message for memory-worthy content
		Provider         string  `yaml:"provider"`               // provider entry ID (falls back to main LLM)
		Model            string  `yaml:"model"`                  // model override (optional)
		AutoConfirm      float64 `yaml:"auto_confirm_threshold"` // confidence threshold for auto-store (default 0.92)
		QueryExpansion   bool    `yaml:"query_expansion"`        // expand RAG queries using analysis LLM (default false)
		LLMReranking     bool    `yaml:"llm_reranking"`          // LLM-based re-ranking of RAG candidates (default false)
		ProviderType     string  `yaml:"-" json:"-"`             // resolved
		BaseURL          string  `yaml:"-" json:"-"`             // resolved
		APIKey           string  `yaml:"-" json:"-"`             // resolved
		ResolvedModel    string  `yaml:"-" json:"-"`             // resolved
		WeeklyReflection bool    `yaml:"weekly_reflection"`      // generate weekly reflection (default true)
		ReflectionDay    string  `yaml:"reflection_day"`         // day for weekly reflection (default "sunday")
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
	Logging struct {
		LogDir        string `yaml:"log_dir"`
		EnableFileLog bool   `yaml:"enable_file_log"`
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
		Password      string `yaml:"-"`
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
	MeshCentral struct {
		Enabled           bool     `yaml:"enabled"`
		ReadOnly          bool     `yaml:"readonly"`           // true = only list operations, block wake/power/run_command
		BlockedOperations []string `yaml:"blocked_operations"` // explicit operation deny-list (case-insensitive)
		URL               string   `yaml:"url"`
		Username          string   `yaml:"username"`
		Password          string   `yaml:"-" vault:"password"`    // vault-only
		LoginToken        string   `yaml:"-" vault:"login_token"` // vault-only
		Insecure          bool     `yaml:"insecure"`              // skip TLS certificate verification (default: false)
	} `yaml:"meshcentral"`
	Docker struct {
		Enabled  bool   `yaml:"enabled"`
		ReadOnly bool   `yaml:"readonly"` // true = only list/inspect/logs/stats, block create/start/stop/remove/exec
		Host     string `yaml:"host"`     // e.g. unix:///var/run/docker.sock or tcp://localhost:2375
	} `yaml:"docker"`
	CoAgents struct {
		Enabled       bool `yaml:"enabled"`
		MaxConcurrent int  `yaml:"max_concurrent"`
		LLM           struct {
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
	} `yaml:"co_agents"`
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
		ReadOnly bool   `yaml:"readonly"` // true = only list/read/info, block write/delete/move/mkdir
		URL      string `yaml:"url"`      // e.g. https://cloud.example.com/remote.php/dav/files/user/
		Username string `yaml:"username"`
		Password string `yaml:"-"`
	} `yaml:"webdav"`
	Koofr struct {
		Enabled     bool   `yaml:"enabled"`
		ReadOnly    bool   `yaml:"readonly"` // true = only list/read, block write/delete/move
		Username    string `yaml:"username"`
		AppPassword string `yaml:"-"`
		BaseURL     string `yaml:"base_url"` // default: https://app.koofr.net
	} `yaml:"koofr"`
	PaperlessNGX struct {
		Enabled  bool   `yaml:"enabled"`
		ReadOnly bool   `yaml:"readonly"` // true = only search/get/download/list, block upload/update/delete
		URL      string `yaml:"url"`      // e.g. https://paperless.example.com
		APIToken string `yaml:"-"`
	} `yaml:"paperless_ngx"`
	TTS struct {
		Provider   string `yaml:"provider"` // "google" or "elevenlabs"
		Language   string `yaml:"language"` // BCP-47 language code for Google TTS (e.g. "de", "en")
		ElevenLabs struct {
			APIKey  string `yaml:"-" vault:"api_key"` // vault-only
			VoiceID string `yaml:"voice_id"`          // default voice ID
			ModelID string `yaml:"model_id"`          // e.g. "eleven_multilingual_v2"
		} `yaml:"elevenlabs"`
	} `yaml:"tts"`
	MediaRegistry struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"media_registry"`
	Chromecast struct {
		Enabled bool `yaml:"enabled"`
		TTSPort int  `yaml:"tts_port"`
	} `yaml:"chromecast"`
	Homepage struct {
		Enabled                  bool   `yaml:"enabled"`
		AllowDeploy              bool   `yaml:"allow_deploy"`
		AllowContainerManagement bool   `yaml:"allow_container_management"`
		AllowLocalServer         bool   `yaml:"allow_local_server"` // Danger Zone: allow Python HTTP server fallback when Docker unavailable
		DeployHost               string `yaml:"deploy_host"`
		DeployPort               int    `yaml:"deploy_port"`
		DeployUser               string `yaml:"deploy_user"`
		DeployPassword           string `yaml:"-"` // vault-only
		DeployKey                string `yaml:"-"` // vault-only (SSH private key)
		DeployPath               string `yaml:"deploy_path"`
		DeployMethod             string `yaml:"deploy_method"` // "sftp" or "scp"
		WebServerEnabled         bool   `yaml:"webserver_enabled"`
		WebServerPort            int    `yaml:"webserver_port"`
		WebServerDomain          string `yaml:"webserver_domain"`
		WebServerInternalOnly    bool   `yaml:"webserver_internal_only"` // bind on 127.0.0.1 only
		WorkspacePath            string `yaml:"workspace_path"`
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
			UserKey  string `yaml:"-"` // from vault
			AppToken string `yaml:"-"` // from vault
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
		ReadOnly bool   `yaml:"readonly"` // true = only list/status, block start/stop/reboot/suspend/snapshot
		URL      string `yaml:"url"`      // e.g. "https://pve.example.com:8006"
		TokenID  string `yaml:"token_id"` // e.g. "user@pam!tokenname"
		Secret   string `yaml:"-"`        // API token secret (from vault)
		Node     string `yaml:"node"`     // default node name (e.g. "pve")
		Insecure bool   `yaml:"insecure"` // skip TLS verification for self-signed certs
	} `yaml:"proxmox"`
	Ollama struct {
		Enabled  bool   `yaml:"enabled"`
		ReadOnly bool   `yaml:"readonly"` // true = only list/show/running, block pull/delete/copy
		URL      string `yaml:"url"`      // e.g. "http://localhost:11434"
	} `yaml:"ollama"`
	RocketChat struct {
		Enabled   bool   `yaml:"enabled"`
		URL       string `yaml:"url"`     // e.g. "https://chat.example.com"
		UserID    string `yaml:"user_id"` // bot user ID
		AuthToken string `yaml:"-"`       // auth token (from vault)
		Channel   string `yaml:"channel"` // default channel to listen on
		Alias     string `yaml:"alias"`   // display name
	} `yaml:"rocketchat"`
	Tailscale struct {
		Enabled  bool   `yaml:"enabled"`
		ReadOnly bool   `yaml:"readonly"`          // true = only list/status/dns, block enable/disable routes
		APIKey   string `yaml:"-" vault:"api_key"` // Tailscale API key (vault-only)
		Tailnet  string `yaml:"tailnet"`           // Tailnet name, e.g. "example.com" or "-" for default
		TsNet    struct {
			Enabled  bool   `yaml:"enabled"`   // enable tsnet embedded Tailscale node (independent of API integration)
			Hostname string `yaml:"hostname"`  // MagicDNS hostname, e.g. "aurago" → aurago.tailnet-name.ts.net
			StateDir string `yaml:"state_dir"` // persistent state directory (default: data/tsnet)
			Funnel   bool   `yaml:"funnel"`    // expose via Tailscale Funnel (V2 placeholder)
		} `yaml:"tsnet"`
	} `yaml:"tailscale"`
	CloudflareTunnel struct {
		Enabled        bool                    `yaml:"enabled"`         // master toggle
		ReadOnly       bool                    `yaml:"readonly"`        // agent: status-only, no start/stop/route changes
		Mode           string                  `yaml:"mode"`            // "auto" (default), "docker", "native"
		AutoStart      bool                    `yaml:"auto_start"`      // start tunnel on AuraGo boot
		AuthMethod     string                  `yaml:"auth_method"`     // "token" (default), "named", "quick"
		TunnelName     string                  `yaml:"tunnel_name"`     // named tunnel: tunnel name
		AccountID      string                  `yaml:"account_id"`      // named tunnel: Cloudflare account ID
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
		URL              string `yaml:"url"`               // sidecar mode: API URL (e.g. "http://ansible:5001")
		Token            string `yaml:"-" vault:"token"`   // sidecar mode: Bearer token (vault-only)
		Timeout          int    `yaml:"timeout"`           // max seconds for playbook runs (default 300)
		PlaybooksDir     string `yaml:"playbooks_dir"`     // local mode: directory containing playbooks
		DefaultInventory string `yaml:"default_inventory"` // local mode: default inventory file path
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
	SecurityProxy struct {
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
		Enabled   bool   `yaml:"enabled"`    // true = this instance is a worker egg
		MasterURL string `yaml:"master_url"` // WebSocket URL of the master (ws[s]://host:port/api/invasion/ws)
		SharedKey string `yaml:"shared_key"` // hex-encoded AES-256 shared key for auth + encryption
		EggID     string `yaml:"egg_id"`     // UUID of this egg record on master
		NestID    string `yaml:"nest_id"`    // UUID of the nest this egg is deployed in
	} `yaml:"egg_mode"`
	Indexing struct {
		Enabled             bool     `yaml:"enabled"`
		Directories         []string `yaml:"directories"`
		PollIntervalSeconds int      `yaml:"poll_interval_seconds"` // default 60
		Extensions          []string `yaml:"extensions"`            // default: .txt, .md, .json, .csv, .log, .yaml, .yml, .pdf, .docx, .xlsx, .pptx, .odt, .rtf
		IndexImages         bool     `yaml:"index_images"`          // send images to Vision LLM for analysis and index results
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
	AdGuard struct {
		Enabled  bool   `yaml:"enabled"`
		ReadOnly bool   `yaml:"readonly"` // true = only read status/stats/logs, block all mutations
		URL      string `yaml:"url"`      // e.g. http://192.168.1.1:3000
		Username string `yaml:"username"`
		Password string `yaml:"-" vault:"adguard_password"`
	} `yaml:"adguard"`
	MQTT struct {
		Enabled      bool     `yaml:"enabled"`
		ReadOnly     bool     `yaml:"readonly"` // true = only subscribe/get_messages/unsubscribe, block publish
		Broker       string   `yaml:"broker"`   // e.g. tcp://localhost:1883
		ClientID     string   `yaml:"client_id"`
		Username     string   `yaml:"username"`
		Password     string   `yaml:"-"`
		Topics       []string `yaml:"topics"`         // topics to subscribe to on connect
		QoS          int      `yaml:"qos"`            // 0, 1, or 2
		RelayToAgent bool     `yaml:"relay_to_agent"` // forward incoming messages to agent
	} `yaml:"mqtt"`
	MCP struct {
		Enabled bool        `yaml:"enabled"`
		Servers []MCPServer `yaml:"servers"`
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
			Enabled         bool   `yaml:"enabled"`          // enable web_scraper (default true)
			SummaryMode     bool   `yaml:"summary_mode"`     // send scraped content to a separate LLM for summarisation before returning to agent
			SummaryProvider string `yaml:"summary_provider"` // provider entry ID used for summarisation
			// resolved fields (populated by ResolveProviders)
			SummaryBaseURL string `yaml:"-" json:"-"`
			SummaryAPIKey  string `yaml:"-" json:"-"`
			SummaryModel   string `yaml:"-" json:"-"`
		} `yaml:"web_scraper"`
		Wikipedia struct {
			SummaryMode     bool   `yaml:"summary_mode"`     // summarise Wikipedia content via a separate LLM before returning to agent
			SummaryProvider string `yaml:"summary_provider"` // provider entry ID used for summarisation
			// resolved fields (populated by ResolveProviders)
			SummaryBaseURL string `yaml:"-" json:"-"`
			SummaryAPIKey  string `yaml:"-" json:"-"`
			SummaryModel   string `yaml:"-" json:"-"`
		} `yaml:"wikipedia"`
		DDGSearch struct {
			SummaryMode     bool   `yaml:"summary_mode"`     // summarise DDG search results via a separate LLM before returning to agent
			SummaryProvider string `yaml:"summary_provider"` // provider entry ID used for summarisation
			// resolved fields (populated by ResolveProviders)
			SummaryBaseURL string `yaml:"-" json:"-"`
			SummaryAPIKey  string `yaml:"-" json:"-"`
			SummaryModel   string `yaml:"-" json:"-"`
		} `yaml:"ddg_search"`
		PDFExtractor struct {
			Enabled         bool   `yaml:"enabled"`          // enable pdf_extractor (default true)
			SummaryMode     bool   `yaml:"summary_mode"`     // summarise extracted PDF text via a separate LLM before returning to agent
			SummaryProvider string `yaml:"summary_provider"` // provider entry ID used for summarisation
			// resolved fields (populated by ResolveProviders)
			SummaryBaseURL string `yaml:"-" json:"-"`
			SummaryAPIKey  string `yaml:"-" json:"-"`
			SummaryModel   string `yaml:"-" json:"-"`
		} `yaml:"pdf_extractor"`
		DocumentCreator DocumentCreatorConfig `yaml:"document_creator"`
	} `yaml:"tools"`
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
	AIGateway struct {
		Enabled   bool   `yaml:"enabled"`
		AccountID string `yaml:"account_id"` // Cloudflare account ID
		GatewayID string `yaml:"gateway_id"` // AI Gateway name/slug
	} `yaml:"ai_gateway"`
	MCPServer struct {
		Enabled      bool     `yaml:"enabled"`
		AllowedTools []string `yaml:"allowed_tools"` // tool names to expose; empty = none
		RequireAuth  bool     `yaml:"require_auth"`  // require Bearer token or session cookie
	} `yaml:"mcp_server"`
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
		ClientSecret  string `yaml:"-"`              // vault-only: google_workspace_client_secret
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
		// resolved fields (populated by ResolveProviders)
		ProviderType  string `yaml:"-" json:"-"` // resolved: openai, openrouter, stability, ideogram, google, etc.
		BaseURL       string `yaml:"-" json:"-"` // resolved from provider entry
		APIKey        string `yaml:"-" json:"-"` // resolved from provider entry
		ResolvedModel string `yaml:"-" json:"-"` // resolved: model from provider if not overridden
	} `yaml:"image_generation"`
	OneDrive struct {
		Enabled      bool   `yaml:"enabled"`
		ReadOnly     bool   `yaml:"readonly"`   // true = only list/read/search/quota, block upload/delete/move/copy/share/mkdir
		ClientID     string `yaml:"client_id"`  // Azure App Registration Client ID (public client for Device Code flow)
		ClientSecret string `yaml:"-"`          // vault-only: onedrive_client_secret (optional, only for confidential apps)
		TenantID     string `yaml:"tenant_id"`  // "common" (default), "consumers", "organizations", or tenant UUID
		AccessToken  string `yaml:"-" json:"-"` // resolved from OAuth token in vault
		RefreshToken string `yaml:"-" json:"-"` // resolved from OAuth token in vault
		TokenExpiry  string `yaml:"-" json:"-"` // resolved: RFC3339 expiry
	} `yaml:"onedrive"`

	// gwProvider is a synthetic ProviderEntry used by FindProvider for Google Workspace OAuth.
	gwProvider ProviderEntry `yaml:"-" json:"-"`
}

// MCPServer describes one external MCP server in the config.
type MCPServer struct {
	Name    string            `yaml:"name"    json:"name"`
	Command string            `yaml:"command" json:"command"`
	Args    []string          `yaml:"args"    json:"args"`
	Env     map[string]string `yaml:"env"     json:"env"`
	Enabled bool              `yaml:"enabled" json:"enabled"`
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
