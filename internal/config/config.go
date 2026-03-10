package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SecretReader is a minimal interface for reading secrets from the vault.
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

	// OAuth2 Authorization Code flow (optional, alternative to static API key)
	AuthType          string `yaml:"auth_type,omitempty"           json:"auth_type"`           // "api_key" (default) or "oauth2"
	OAuthAuthURL      string `yaml:"oauth_auth_url,omitempty"      json:"oauth_auth_url"`      // authorization endpoint
	OAuthTokenURL     string `yaml:"oauth_token_url,omitempty"     json:"oauth_token_url"`     // token exchange endpoint
	OAuthClientID     string `yaml:"oauth_client_id,omitempty"     json:"oauth_client_id"`     // client ID
	OAuthClientSecret string `yaml:"-" vault:"oauth_client_secret" json:"oauth_client_secret"` // client secret (vault-only)
	OAuthScopes       string `yaml:"oauth_scopes,omitempty"        json:"oauth_scopes"`        // space-separated scopes
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
}

type Config struct {
	ConfigPath    string          `yaml:"-"` // runtime-only: absolute path to the config file
	Providers     []ProviderEntry `yaml:"providers"`
	EmailAccounts []EmailAccount  `yaml:"email_accounts"`
	Server        struct {
		Host          string `yaml:"host"`
		Port          int    `yaml:"port"`
		BridgeAddress string `yaml:"bridge_address"`
		MaxBodyBytes  int64  `yaml:"max_body_bytes"`
		UILanguage    string `yaml:"ui_language"`
		MasterKey     string `yaml:"-"` // ENV-only (AURAGO_MASTER_KEY)
		HTTPS         struct {
			Enabled bool   `yaml:"enabled"`
			Domain  string `yaml:"domain"`
			Email   string `yaml:"email"`
		} `yaml:"https"`
	} `yaml:"server"`
	LLM struct {
		Provider           string  `yaml:"provider"`          // provider entry ID (references Providers[].ID)
		ProviderType       string  `yaml:"-"       json:"-"`  // resolved: openai, openrouter, ollama etc.
		BaseURL            string  `yaml:"-"       json:"-"`  // resolved from provider entry
		APIKey             string  `yaml:"-"       json:"-"`  // resolved from provider entry
		Model              string  `yaml:"-"       json:"-"`  // resolved from provider entry
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
		ShortTermPath string `yaml:"short_term_path"`
		LongTermPath  string `yaml:"long_term_path"`
		InventoryPath string `yaml:"inventory_path"`
		InvasionPath  string `yaml:"invasion_path"`
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
	} `yaml:"embeddings"`
	Agent struct {
		SystemLanguage             string `yaml:"system_language"`
		EnableGoogleWorkspace      bool   `yaml:"enable_google_workspace"`
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
		SudoEnabled                bool   `yaml:"sudo_enabled"`                // allow execute_sudo tool (password must be stored in vault as "sudo_password")
		// ── Danger Zone: tool capability gates (all default true) ──
		AllowShell           bool   `yaml:"allow_shell"`            // allow execute_shell
		AllowPython          bool   `yaml:"allow_python"`           // allow execute_python / save_tool / execute_skill
		AllowFilesystemWrite bool   `yaml:"allow_filesystem_write"` // allow filesystem write operations
		AllowNetworkRequests bool   `yaml:"allow_network_requests"` // allow api_request
		AllowRemoteShell     bool   `yaml:"allow_remote_shell"`     // allow execute_remote_shell
		AllowSelfUpdate      bool   `yaml:"allow_self_update"`      // allow manage_updates
		AllowMCP             bool   `yaml:"allow_mcp"`              // allow MCP (Model Context Protocol) server connections
		AllowWebScraper      bool   `yaml:"allow_web_scraper"`      // allow web_scraper skill (scrape external websites)
		AdditionalPrompt     string `yaml:"additional_prompt"`      // extra instructions always appended to the system prompt
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
		AccessToken string `yaml:"-"`
	} `yaml:"home_assistant"`
	MeshCentral struct {
		Enabled           bool     `yaml:"enabled"`
		ReadOnly          bool     `yaml:"readonly"`           // true = only list operations, block wake/power/run_command
		BlockedOperations []string `yaml:"blocked_operations"` // explicit operation deny-list (case-insensitive)
		URL               string   `yaml:"url"`
		Username          string   `yaml:"username"`
		Password          string   `yaml:"-" vault:"password"`    // vault-only
		LoginToken        string   `yaml:"-" vault:"login_token"` // vault-only
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
	TTS struct {
		Provider   string `yaml:"provider"` // "google" or "elevenlabs"
		Language   string `yaml:"language"` // BCP-47 language code for Google TTS (e.g. "de", "en")
		ElevenLabs struct {
			APIKey  string `yaml:"-" vault:"api_key"` // vault-only
			VoiceID string `yaml:"voice_id"`          // default voice ID
			ModelID string `yaml:"model_id"`          // e.g. "eleven_multilingual_v2"
		} `yaml:"elevenlabs"`
	} `yaml:"tts"`
	Chromecast struct {
		Enabled bool `yaml:"enabled"`
		TTSPort int  `yaml:"tts_port"`
	} `yaml:"chromecast"`
	Homepage struct {
		Enabled                  bool   `yaml:"enabled"`
		AllowDeploy              bool   `yaml:"allow_deploy"`
		AllowContainerManagement bool   `yaml:"allow_container_management"`
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
		WorkspacePath            string `yaml:"workspace_path"`
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
	} `yaml:"tailscale"`
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
		Enabled        bool   `yaml:"enabled"`
		ReadOnly       bool   `yaml:"readonly"`        // true = only list/get/search, block create/delete/update
		Token          string `yaml:"-" vault:"token"` // Personal Access Token (from vault)
		Owner          string `yaml:"owner"`           // GitHub username or organisation
		DefaultPrivate bool   `yaml:"default_private"` // true = new repos are private by default
		BaseURL        string `yaml:"base_url"`        // API base URL (default: https://api.github.com), for GitHub Enterprise
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
			Enabled  bool `yaml:"enabled"`  // default true; disable to block knowledge_graph
			ReadOnly bool `yaml:"readonly"` // true = only query/search, block add/delete
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
}

// MCPServer describes one external MCP server in the config.
type MCPServer struct {
	Name    string            `yaml:"name"    json:"name"`
	Command string            `yaml:"command" json:"command"`
	Args    []string          `yaml:"args"    json:"args"`
	Env     map[string]string `yaml:"env"     json:"env"`
	Enabled bool              `yaml:"enabled" json:"enabled"`
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

// FindProvider returns the ProviderEntry with the given ID, or nil if not found.
func (c *Config) FindProvider(id string) *ProviderEntry {
	for i := range c.Providers {
		if c.Providers[i].ID == id {
			return &c.Providers[i]
		}
	}
	return nil
}

// FindEmailAccount returns the EmailAccount with the given ID, or nil.
func (c *Config) FindEmailAccount(id string) *EmailAccount {
	for i := range c.EmailAccounts {
		if c.EmailAccounts[i].ID == id {
			return &c.EmailAccounts[i]
		}
	}
	return nil
}

// DefaultEmailAccount returns the first EmailAccount, or nil if none exist.
func (c *Config) DefaultEmailAccount() *EmailAccount {
	if len(c.EmailAccounts) == 0 {
		return nil
	}
	return &c.EmailAccounts[0]
}

// MigrateEmailAccounts migrates the legacy single Email config into the
// EmailAccounts slice. Called once at startup during ApplyDefaults.
func (c *Config) MigrateEmailAccounts() {
	// If email_accounts already populated, nothing to migrate
	if len(c.EmailAccounts) > 0 {
		return
	}
	// Check if legacy email section has data worth migrating
	if c.Email.Username == "" && c.Email.IMAPHost == "" && c.Email.SMTPHost == "" {
		return
	}
	// Build an account from the legacy fields
	acct := EmailAccount{
		ID:            "default",
		Name:          "Default",
		IMAPHost:      c.Email.IMAPHost,
		IMAPPort:      c.Email.IMAPPort,
		SMTPHost:      c.Email.SMTPHost,
		SMTPPort:      c.Email.SMTPPort,
		Username:      c.Email.Username,
		Password:      c.Email.Password,
		FromAddress:   c.Email.FromAddress,
		WatchEnabled:  c.Email.WatchEnabled,
		WatchInterval: c.Email.WatchInterval,
		WatchFolder:   c.Email.WatchFolder,
	}
	c.EmailAccounts = append(c.EmailAccounts, acct)
}

// ResolveProviders populates the resolved (yaml:"-") fields on every LLM slot
// from the corresponding ProviderEntry.  It also handles legacy migration: if
// the Providers list is empty but inline fields exist (old-format config), it
// auto-creates provider entries and sets the references.
func (c *Config) ResolveProviders() {
	c.migrateInlineProviders()

	// ── LLM ──
	if p := c.FindProvider(c.LLM.Provider); p != nil {
		c.LLM.ProviderType = p.Type
		c.LLM.BaseURL = p.BaseURL
		c.LLM.APIKey = p.APIKey
		c.LLM.Model = p.Model
	} else if c.LLM.LegacyAPIKey != "" {
		// Legacy fallback: use inline fields from old config format
		c.LLM.BaseURL = c.LLM.LegacyURL
		c.LLM.APIKey = c.LLM.LegacyAPIKey
		c.LLM.Model = c.LLM.LegacyModel
		c.LLM.ProviderType = c.LLM.Provider // old value is the type string
	}

	// ── FallbackLLM ──
	if p := c.FindProvider(c.FallbackLLM.Provider); p != nil {
		c.FallbackLLM.ProviderType = p.Type
		c.FallbackLLM.BaseURL = p.BaseURL
		c.FallbackLLM.APIKey = p.APIKey
		c.FallbackLLM.Model = p.Model
	} else if c.FallbackLLM.LegacyAPIKey != "" {
		c.FallbackLLM.BaseURL = c.FallbackLLM.LegacyURL
		c.FallbackLLM.APIKey = c.FallbackLLM.LegacyAPIKey
		c.FallbackLLM.Model = c.FallbackLLM.LegacyModel
	}

	// ── Vision ── (falls back to main LLM if provider empty)
	if c.Vision.Provider != "" {
		if p := c.FindProvider(c.Vision.Provider); p != nil {
			c.Vision.ProviderType = p.Type
			c.Vision.BaseURL = p.BaseURL
			c.Vision.APIKey = p.APIKey
			c.Vision.Model = p.Model
		} else if c.Vision.LegacyAPIKey != "" || c.Vision.LegacyURL != "" {
			c.Vision.BaseURL = c.Vision.LegacyURL
			c.Vision.APIKey = c.Vision.LegacyAPIKey
			c.Vision.Model = c.Vision.LegacyModel
		}
	}
	if c.Vision.APIKey == "" {
		c.Vision.APIKey = c.LLM.APIKey
	}
	if c.Vision.BaseURL == "" {
		c.Vision.BaseURL = c.LLM.BaseURL
	}

	// ── Whisper ── (falls back to main LLM if provider empty)
	if c.Whisper.Provider != "" {
		if p := c.FindProvider(c.Whisper.Provider); p != nil {
			c.Whisper.ProviderType = p.Type
			c.Whisper.BaseURL = p.BaseURL
			c.Whisper.APIKey = p.APIKey
			c.Whisper.Model = p.Model
		} else if c.Whisper.LegacyAPIKey != "" || c.Whisper.LegacyURL != "" {
			c.Whisper.BaseURL = c.Whisper.LegacyURL
			c.Whisper.APIKey = c.Whisper.LegacyAPIKey
			c.Whisper.Model = c.Whisper.LegacyModel
		}
	}
	if c.Whisper.APIKey == "" {
		c.Whisper.APIKey = c.LLM.APIKey
	}
	if c.Whisper.BaseURL == "" {
		c.Whisper.BaseURL = c.LLM.BaseURL
	}

	// ── Embeddings ── ("disabled" is a special value, not a provider ID)
	if c.Embeddings.Provider != "" && c.Embeddings.Provider != "disabled" {
		if p := c.FindProvider(c.Embeddings.Provider); p != nil {
			c.Embeddings.ProviderType = p.Type
			c.Embeddings.BaseURL = p.BaseURL
			c.Embeddings.APIKey = p.APIKey
			c.Embeddings.Model = p.Model
		} else if c.Embeddings.LegacyAPIKey != "" {
			c.Embeddings.APIKey = c.Embeddings.LegacyAPIKey
		}
	}
	if c.Embeddings.APIKey == "" {
		c.Embeddings.APIKey = c.LLM.APIKey
	}

	// ── CoAgents.LLM ── (falls back to main LLM if provider empty)
	if c.CoAgents.LLM.Provider != "" {
		if p := c.FindProvider(c.CoAgents.LLM.Provider); p != nil {
			c.CoAgents.LLM.ProviderType = p.Type
			c.CoAgents.LLM.BaseURL = p.BaseURL
			c.CoAgents.LLM.APIKey = p.APIKey
			c.CoAgents.LLM.Model = p.Model
		} else if c.CoAgents.LLM.LegacyAPIKey != "" || c.CoAgents.LLM.LegacyURL != "" {
			c.CoAgents.LLM.BaseURL = c.CoAgents.LLM.LegacyURL
			c.CoAgents.LLM.APIKey = c.CoAgents.LLM.LegacyAPIKey
			c.CoAgents.LLM.Model = c.CoAgents.LLM.LegacyModel
		}
	}
	if c.CoAgents.LLM.APIKey == "" {
		c.CoAgents.LLM.APIKey = c.LLM.APIKey
	}
	if c.CoAgents.LLM.BaseURL == "" {
		c.CoAgents.LLM.BaseURL = c.LLM.BaseURL
	}
	if c.CoAgents.LLM.Model == "" {
		c.CoAgents.LLM.Model = c.LLM.Model
	}

	// ── Personality V2 ── (falls back to main LLM if provider empty)
	if c.Agent.PersonalityV2Provider != "" {
		if p := c.FindProvider(c.Agent.PersonalityV2Provider); p != nil {
			c.Agent.PersonalityV2ProviderType = p.Type
			c.Agent.PersonalityV2ResolvedURL = p.BaseURL
			c.Agent.PersonalityV2ResolvedKey = p.APIKey
			c.Agent.PersonalityV2ResolvedModel = p.Model
		}
	}
	// Legacy fallback: use inline fields if provider ref resolved nothing
	if c.Agent.PersonalityV2ResolvedModel == "" && c.Agent.PersonalityV2Model != "" {
		c.Agent.PersonalityV2ResolvedModel = c.Agent.PersonalityV2Model
	}
	if c.Agent.PersonalityV2ResolvedURL == "" && c.Agent.PersonalityV2URL != "" {
		c.Agent.PersonalityV2ResolvedURL = c.Agent.PersonalityV2URL
	}
	if c.Agent.PersonalityV2ResolvedKey == "" && c.Agent.PersonalityV2APIKey != "" {
		c.Agent.PersonalityV2ResolvedKey = c.Agent.PersonalityV2APIKey
	}

	// ── WebScraper summary ── (falls back to main LLM if provider empty)
	if c.Tools.WebScraper.SummaryProvider != "" {
		if p := c.FindProvider(c.Tools.WebScraper.SummaryProvider); p != nil {
			c.Tools.WebScraper.SummaryBaseURL = p.BaseURL
			c.Tools.WebScraper.SummaryAPIKey = p.APIKey
			c.Tools.WebScraper.SummaryModel = p.Model
		}
	}
	if c.Tools.WebScraper.SummaryAPIKey == "" {
		c.Tools.WebScraper.SummaryAPIKey = c.LLM.APIKey
	}
	if c.Tools.WebScraper.SummaryBaseURL == "" {
		c.Tools.WebScraper.SummaryBaseURL = c.LLM.BaseURL
	}
	if c.Tools.WebScraper.SummaryModel == "" {
		c.Tools.WebScraper.SummaryModel = c.LLM.Model
	}
}

// ApplyOAuthTokens reads stored OAuth2 access tokens from the vault and injects
// them into the resolved APIKey fields of providers that use auth_type "oauth2".
// Call this after ResolveProviders() whenever a vault is available.
func (c *Config) ApplyOAuthTokens(vault SecretReader) {
	if vault == nil {
		return
	}
	for i := range c.Providers {
		p := &c.Providers[i]
		if p.AuthType != "oauth2" {
			continue
		}
		raw, err := vault.ReadSecret("oauth_" + p.ID)
		if err != nil || raw == "" {
			continue
		}
		var tok OAuthToken
		if err := json.Unmarshal([]byte(raw), &tok); err != nil {
			continue
		}
		if tok.AccessToken == "" {
			continue
		}
		// Inject the access token as the API key for this provider
		p.APIKey = tok.AccessToken
	}

	// Re-resolve: copy updated APIKey from providers into the resolved slot fields.
	// We only overwrite slots that reference an oauth2 provider.
	applyIfOAuth := func(providerID string, target *string) {
		p := c.FindProvider(providerID)
		if p != nil && p.AuthType == "oauth2" && p.APIKey != "" {
			*target = p.APIKey
		}
	}
	applyIfOAuth(c.LLM.Provider, &c.LLM.APIKey)
	applyIfOAuth(c.FallbackLLM.Provider, &c.FallbackLLM.APIKey)
	applyIfOAuth(c.Vision.Provider, &c.Vision.APIKey)
	applyIfOAuth(c.Whisper.Provider, &c.Whisper.APIKey)
	applyIfOAuth(c.Embeddings.Provider, &c.Embeddings.APIKey)
	applyIfOAuth(c.CoAgents.LLM.Provider, &c.CoAgents.LLM.APIKey)
	applyIfOAuth(c.Agent.PersonalityV2Provider, &c.Agent.PersonalityV2ResolvedKey)
}

// ApplyVaultSecrets populates all vault-only secret fields from the vault.
// Must be called after Load() and before the config is used for the first time.
// After calling this, ResolveProviders() should be called again to propagate
// provider API keys into the resolved LLM/Vision/etc. slots.
func (c *Config) ApplyVaultSecrets(vault SecretReader) {
	if vault == nil {
		return
	}
	apply := func(vaultKey string, target *string) {
		if v, err := vault.ReadSecret(vaultKey); err == nil && v != "" {
			*target = v
		}
	}

	// ── Provider secrets ──
	for i := range c.Providers {
		p := &c.Providers[i]
		apply("provider_"+p.ID+"_api_key", &p.APIKey)
		apply("provider_"+p.ID+"_oauth_client_secret", &p.OAuthClientSecret)
	}

	// ── Telegram / Discord ──
	apply("telegram_bot_token", &c.Telegram.BotToken)
	apply("discord_bot_token", &c.Discord.BotToken)

	// ── MeshCentral ──
	apply("meshcentral_password", &c.MeshCentral.Password)
	apply("meshcentral_token", &c.MeshCentral.LoginToken)

	// ── Tailscale / Ansible ──
	apply("tailscale_api_key", &c.Tailscale.APIKey)
	apply("ansible_token", &c.Ansible.Token)

	// ── API keys ──
	apply("virustotal_api_key", &c.VirusTotal.APIKey)
	apply("brave_search_api_key", &c.BraveSearch.APIKey)
	apply("tts_elevenlabs_api_key", &c.TTS.ElevenLabs.APIKey)

	// ── Notifications ──
	apply("ntfy_token", &c.Notifications.Ntfy.Token)
	apply("pushover_user_key", &c.Notifications.Pushover.UserKey)
	apply("pushover_app_token", &c.Notifications.Pushover.AppToken)

	// ── Auth ──
	apply("auth_password_hash", &c.Auth.PasswordHash)
	apply("auth_session_secret", &c.Auth.SessionSecret)
	apply("auth_totp_secret", &c.Auth.TOTPSecret)

	// ── Existing vault-only fields ──
	apply("home_assistant_access_token", &c.HomeAssistant.AccessToken)
	apply("webdav_password", &c.WebDAV.Password)
	apply("koofr_password", &c.Koofr.AppPassword)
	apply("proxmox_secret", &c.Proxmox.Secret)
	apply("github_token", &c.GitHub.Token)
	apply("rocketchat_auth_token", &c.RocketChat.AuthToken)
	apply("mqtt_password", &c.MQTT.Password)

	// ── Homepage deploy secrets ──
	apply("homepage_deploy_password", &c.Homepage.DeployPassword)
	apply("homepage_deploy_key", &c.Homepage.DeployKey)

	// ── Netlify ──
	apply("netlify_token", &c.Netlify.Token)

	// ── Email account passwords ──
	apply("email_password", &c.Email.Password)
	for i := range c.EmailAccounts {
		a := &c.EmailAccounts[i]
		apply("email_"+a.ID+"_password", &a.Password)
	}
}

// migrateInlineProviders auto-creates provider entries from old-format config
// files that use inline base_url/api_key/model fields.  This is called once
// during Load() and ensures all resolved fields are populated.
func (c *Config) migrateInlineProviders() {
	if len(c.Providers) > 0 {
		return // new-format config — no migration needed
	}

	seen := map[string]bool{}

	addProvider := func(id, name, typ, baseURL, apiKey, model string) string {
		if id == "" || seen[id] {
			return id
		}
		seen[id] = true
		c.Providers = append(c.Providers, ProviderEntry{
			ID: id, Name: name, Type: typ,
			BaseURL: baseURL, APIKey: apiKey, Model: model,
		})
		return id
	}

	inferType := func(baseURL, providerHint string) string {
		if providerHint != "" {
			return strings.ToLower(providerHint)
		}
		lower := strings.ToLower(baseURL)
		switch {
		case strings.Contains(lower, "openrouter"):
			return "openrouter"
		case strings.Contains(lower, "anthropic"):
			return "anthropic"
		case strings.Contains(lower, "googleapis") || strings.Contains(lower, "generativelanguage"):
			return "google"
		case strings.Contains(lower, "11434"):
			return "ollama"
		default:
			return "openai"
		}
	}

	// Migrate main LLM (always present in old configs)
	// The old LLM.Provider was a type string like "openrouter", not an ID
	oldLLMType := c.LLM.Provider // save before overwriting
	if oldLLMType == "" {
		oldLLMType = inferType(c.LLM.LegacyURL, "")
	}
	c.LLM.Provider = addProvider("main", "Haupt-LLM", oldLLMType, c.LLM.LegacyURL, c.LLM.LegacyAPIKey, c.LLM.LegacyModel)

	// Migrate FallbackLLM
	if c.FallbackLLM.Enabled && c.FallbackLLM.LegacyURL != "" {
		fbType := inferType(c.FallbackLLM.LegacyURL, "")
		c.FallbackLLM.Provider = addProvider("fallback", "Fallback-LLM", fbType,
			c.FallbackLLM.LegacyURL, c.FallbackLLM.LegacyAPIKey, c.FallbackLLM.LegacyModel)
	}

	// Migrate Vision
	if c.Vision.LegacyURL != "" || c.Vision.LegacyModel != "" {
		vURL := c.Vision.LegacyURL
		if vURL == "" {
			vURL = c.LLM.LegacyURL
		}
		vKey := c.Vision.LegacyAPIKey
		if vKey == "" {
			vKey = c.LLM.LegacyAPIKey
		}
		vType := inferType(vURL, c.Vision.Provider)
		// Only create separate entry if different from main
		if vURL != c.LLM.LegacyURL || vKey != c.LLM.LegacyAPIKey || c.Vision.LegacyModel != c.LLM.LegacyModel {
			c.Vision.Provider = addProvider("vision", "Vision", vType, vURL, vKey, c.Vision.LegacyModel)
		} else {
			c.Vision.Provider = "main"
		}
	}

	// Migrate Whisper
	if c.Whisper.LegacyURL != "" || c.Whisper.LegacyModel != "" {
		wURL := c.Whisper.LegacyURL
		if wURL == "" {
			wURL = c.LLM.LegacyURL
		}
		wKey := c.Whisper.LegacyAPIKey
		if wKey == "" {
			wKey = c.LLM.LegacyAPIKey
		}
		wType := inferType(wURL, "")
		// Migrate old provider field as mode if it's a mode-like value
		oldProv := strings.ToLower(c.Whisper.Provider)
		if oldProv == "multimodal" || oldProv == "local" {
			c.Whisper.Mode = oldProv
		} else if oldProv == "openai" || oldProv == "openrouter" || oldProv == "ollama" {
			// Old provider type — will be migrated as provider ref
			if c.Whisper.Mode == "" {
				c.Whisper.Mode = "whisper"
			}
		}
		if wURL != c.LLM.LegacyURL || wKey != c.LLM.LegacyAPIKey || c.Whisper.LegacyModel != c.LLM.LegacyModel {
			c.Whisper.Provider = addProvider("whisper", "Whisper / STT", wType, wURL, wKey, c.Whisper.LegacyModel)
		} else {
			c.Whisper.Provider = "main"
		}
	}

	// Migrate Embeddings
	oldEmbProv := c.Embeddings.Provider
	switch oldEmbProv {
	case "internal":
		// internal means: use main LLM provider + InternalModel
		embModel := c.Embeddings.InternalModel
		if embModel == "" {
			embModel = "text-embedding-3-small"
		}
		// Create a dedicated embedding provider (same URL/key as main but different model)
		c.Embeddings.Provider = addProvider("embeddings", "Embeddings", oldLLMType, c.LLM.LegacyURL, c.LLM.LegacyAPIKey, embModel)
	case "external":
		embKey := c.Embeddings.LegacyAPIKey
		if embKey == "" || embKey == "dummy_key" {
			embKey = c.LLM.LegacyAPIKey
		}
		embModel := c.Embeddings.ExternalModel
		embURL := c.Embeddings.ExternalURL
		eType := inferType(embURL, "")
		c.Embeddings.Provider = addProvider("embeddings", "Embeddings", eType, embURL, embKey, embModel)
	case "disabled":
		// keep as "disabled" — not a provider ref
	default:
		c.Embeddings.Provider = "disabled"
	}

	// Migrate CoAgents.LLM
	if c.CoAgents.LLM.LegacyURL != "" || c.CoAgents.LLM.LegacyModel != "" {
		caURL := c.CoAgents.LLM.LegacyURL
		if caURL == "" {
			caURL = c.LLM.LegacyURL
		}
		caKey := c.CoAgents.LLM.LegacyAPIKey
		if caKey == "" {
			caKey = c.LLM.LegacyAPIKey
		}
		caModel := c.CoAgents.LLM.LegacyModel
		// Old provider field was a type string
		caOldType := c.CoAgents.LLM.Provider
		caType := inferType(caURL, caOldType)
		if caURL != c.LLM.LegacyURL || caKey != c.LLM.LegacyAPIKey || caModel != c.LLM.LegacyModel {
			c.CoAgents.LLM.Provider = addProvider("coagent", "Co-Agent LLM", caType, caURL, caKey, caModel)
		} else {
			c.CoAgents.LLM.Provider = "main"
		}
	} else if c.CoAgents.LLM.Provider == "" {
		c.CoAgents.LLM.Provider = "main"
	}

	// Migrate Personality V2
	if c.Agent.PersonalityV2URL != "" || c.Agent.PersonalityV2Model != "" {
		v2URL := c.Agent.PersonalityV2URL
		v2Key := c.Agent.PersonalityV2APIKey
		v2Model := c.Agent.PersonalityV2Model
		if v2URL != "" && (v2URL != c.LLM.LegacyURL || v2Key != c.LLM.LegacyAPIKey) {
			v2Type := inferType(v2URL, "")
			c.Agent.PersonalityV2Provider = addProvider("personality-v2", "Personality V2", v2Type, v2URL, v2Key, v2Model)
		}
		// If no separate URL, V2 uses main LLM provider (resolved in ResolveProviders)
	}
}

func Load(path string) (*Config, error) {
	absConfigPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for config: %w", err)
	}
	configDir := filepath.Dir(absConfigPath)

	data, err := os.ReadFile(absConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Pre-process: fix common YAML corruption issues
	data = fixCommonConfigIssues(data)

	var cfg Config
	// Tools section defaults: all tools are enabled by default (opt-in to disable).
	// These are set before unmarshal so that keys absent from the YAML file keep the
	// correct default; explicit 'enabled: false' in the YAML will still override them.
	cfg.Tools.Memory.Enabled = true
	cfg.Tools.KnowledgeGraph.Enabled = true
	cfg.Tools.SecretsVault.Enabled = true
	cfg.Tools.Scheduler.Enabled = true
	cfg.Tools.Notes.Enabled = true
	cfg.Tools.Missions.Enabled = true
	cfg.Tools.StopProcess.Enabled = true
	cfg.Tools.Inventory.Enabled = true
	cfg.Tools.MemoryMaintenance.Enabled = true
	cfg.Tools.WebScraper.Enabled = true

	// Danger-zone capabilities default to false (opt-in) for new installations.
	// Existing configs with explicit true/false values will be read from YAML unchanged.
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// Try to provide helpful context for the error
		lines := string(data)
		lineNum := 0
		if yamlErr, ok := err.(*yaml.TypeError); ok {
			// YAML type errors
			return nil, fmt.Errorf("config YAML type error: %w", yamlErr)
		}
		// Try to find the problematic line
		for i, line := range splitLines(lines) {
			if i < 30 { // Show context around error
				lineNum = i + 1
				_ = line
			}
		}
		_ = lineNum

		// Save corrupted config for debugging
		backupPath := absConfigPath + ".corrupted." + fmt.Sprintf("%d", time.Now().Unix())
		_ = os.WriteFile(backupPath, data, 0644)

		return nil, fmt.Errorf("failed to unmarshal config (backup saved to %s): %w", backupPath, err)
	}

	// Resolve absolute paths for directories
	cfg.Directories.DataDir = resolvePath(configDir, cfg.Directories.DataDir)
	cfg.Directories.WorkspaceDir = resolvePath(configDir, cfg.Directories.WorkspaceDir)
	cfg.Directories.ToolsDir = resolvePath(configDir, cfg.Directories.ToolsDir)
	cfg.Directories.PromptsDir = resolvePath(configDir, cfg.Directories.PromptsDir)
	cfg.Directories.SkillsDir = resolvePath(configDir, cfg.Directories.SkillsDir)
	cfg.Directories.VectorDBDir = resolvePath(configDir, cfg.Directories.VectorDBDir)

	// Resolve absolute paths for SQLite
	cfg.SQLite.ShortTermPath = resolvePath(configDir, cfg.SQLite.ShortTermPath)
	cfg.SQLite.LongTermPath = resolvePath(configDir, cfg.SQLite.LongTermPath)
	cfg.SQLite.InventoryPath = resolvePath(configDir, cfg.SQLite.InventoryPath)
	if cfg.SQLite.InvasionPath == "" {
		cfg.SQLite.InvasionPath = "./data/invasion.db"
	}
	cfg.SQLite.InvasionPath = resolvePath(configDir, cfg.SQLite.InvasionPath)

	// Resolve logging directory
	cfg.Logging.LogDir = resolvePath(configDir, cfg.Logging.LogDir)

	// --- Environment Variable Overrides ---
	// AURAGO_SERVER_HOST overrides server.host unconditionally.
	// Used in Docker to force 0.0.0.0 without touching the YAML file.
	if val := os.Getenv("AURAGO_SERVER_HOST"); val != "" {
		cfg.Server.Host = val
	}

	// --- Environment Variable Fallbacks (for secrets) ---
	if cfg.Server.MasterKey == "" {
		if val := os.Getenv("AURAGO_MASTER_KEY"); val != "" {
			cfg.Server.MasterKey = val
		}
	}

	// Migrate legacy agent.allow_web_scraper → tools.web_scraper.enabled.
	// Old configs that set allow_web_scraper: false should carry over.
	if !cfg.Agent.AllowWebScraper {
		cfg.Tools.WebScraper.Enabled = false
	}

	// Resolve provider references → populates all yaml:"-" fields.
	// Legacy migration creates provider entries from inline fields if Providers is empty.
	cfg.ResolveProviders()

	// Environment overrides for API keys (applied AFTER provider resolution so
	// they override any key from the providers list):
	if val := os.Getenv("LLM_API_KEY"); val != "" {
		cfg.LLM.APIKey = val
	} else if val := os.Getenv("OPENAI_API_KEY"); val != "" {
		cfg.LLM.APIKey = val
	} else if val := os.Getenv("ANTHROPIC_API_KEY"); val != "" {
		cfg.LLM.APIKey = val
	}
	if val := os.Getenv("CO_AGENTS_LLM_API_KEY"); val != "" {
		cfg.CoAgents.LLM.APIKey = val
	}
	if val := os.Getenv("EMBEDDINGS_API_KEY"); val != "" {
		cfg.Embeddings.APIKey = val
	}
	if val := os.Getenv("VISION_API_KEY"); val != "" {
		cfg.Vision.APIKey = val
	}
	if val := os.Getenv("WHISPER_API_KEY"); val != "" {
		cfg.Whisper.APIKey = val
	}
	if val := os.Getenv("FALLBACK_LLM_API_KEY"); val != "" {
		cfg.FallbackLLM.APIKey = val
	}

	if cfg.CircuitBreaker.MaxToolCalls <= 0 {
		cfg.CircuitBreaker.MaxToolCalls = 10 // User specifically asked for 10
	}
	if cfg.CircuitBreaker.LLMTimeoutSeconds <= 0 {
		cfg.CircuitBreaker.LLMTimeoutSeconds = 180 // 3 minutes
	}
	if cfg.CircuitBreaker.MaintenanceTimeoutMinutes <= 0 {
		cfg.CircuitBreaker.MaintenanceTimeoutMinutes = 10
	}
	if len(cfg.CircuitBreaker.RetryIntervals) == 0 {
		cfg.CircuitBreaker.RetryIntervals = []string{"10s", "2m", "10m"}
	}

	if cfg.Server.Host == "" {
		cfg.Server.Host = "127.0.0.1"
	}
	if cfg.Agent.StepDelaySeconds < 0 {
		cfg.Agent.StepDelaySeconds = 0
	}
	if cfg.Maintenance.LifeboatPort <= 0 {
		cfg.Maintenance.LifeboatPort = 8091
	}
	if cfg.Agent.MemoryCompressionCharLimit <= 0 {
		cfg.Agent.MemoryCompressionCharLimit = 100000
	}
	if cfg.Agent.CorePersonality == "" {
		cfg.Agent.CorePersonality = "neutral"
	}
	if cfg.Agent.CoreMemoryMaxEntries <= 0 {
		cfg.Agent.CoreMemoryMaxEntries = 200
	}
	if cfg.Agent.CoreMemoryCapMode == "" {
		cfg.Agent.CoreMemoryCapMode = "soft"
	}
	if cfg.Agent.UserProfilingThreshold <= 0 {
		cfg.Agent.UserProfilingThreshold = 3
	}
	if cfg.Agent.PersonalityV2TimeoutSecs <= 0 {
		cfg.Agent.PersonalityV2TimeoutSecs = 30
	}
	// V2 requires V1 — automatically enable V1 when V2 is on.
	if cfg.Agent.PersonalityEngineV2 && !cfg.Agent.PersonalityEngine {
		cfg.Agent.PersonalityEngine = true
	}
	if cfg.Agent.SystemPromptTokenBudget <= 0 {
		cfg.Agent.SystemPromptTokenBudget = 8192
	}
	// LLM defaults
	if cfg.LLM.Temperature == 0 {
		cfg.LLM.Temperature = 0.7
	}
	if cfg.Agent.ContextWindow <= 0 {
		cfg.Agent.ContextWindow = 0 // 0 = auto-detect or use budget as-is
	}
	// Default to true if not explicitly set (YAML unmarshal results in false if missing,
	// so we check if the key was present or just use a safe default approach)
	// Actually, since it's a bool, we'll just ensure it's handled.
	// We'll assume the user wants it unless they say no.
	// (Note: yaml.Unmarshal into a struct field defaults to the zero value, which is false.
	// To have a default true, we'd usually check a pointer or just set it here if not in file)
	// But since I added it to config.yaml already, it will be loaded.

	if cfg.FallbackLLM.ProbeIntervalSeconds <= 0 {
		cfg.FallbackLLM.ProbeIntervalSeconds = 60
	}
	if cfg.FallbackLLM.ErrorThreshold <= 0 {
		cfg.FallbackLLM.ErrorThreshold = 3
	}
	// Whisper mode default
	if cfg.Whisper.Mode == "" {
		cfg.Whisper.Mode = "whisper"
	}
	if cfg.Koofr.BaseURL == "" {
		cfg.Koofr.BaseURL = "https://app.koofr.net"
	}

	// Server defaults
	if cfg.Server.BridgeAddress == "" {
		cfg.Server.BridgeAddress = "localhost:8089"
	}
	if cfg.Server.MaxBodyBytes <= 0 {
		cfg.Server.MaxBodyBytes = 10 << 20 // 10 MB
	}
	if cfg.Server.UILanguage == "" {
		cfg.Server.UILanguage = "en"
	}

	// Telegram defaults
	if cfg.Telegram.MaxConcurrentWorkers <= 0 {
		cfg.Telegram.MaxConcurrentWorkers = 5
	}

	// Email defaults
	if cfg.Email.IMAPPort <= 0 {
		cfg.Email.IMAPPort = 993
	}
	if cfg.Email.SMTPPort <= 0 {
		cfg.Email.SMTPPort = 587
	}
	if cfg.Email.WatchInterval <= 0 {
		cfg.Email.WatchInterval = 120
	}
	if cfg.Email.WatchFolder == "" {
		cfg.Email.WatchFolder = "INBOX"
	}
	if cfg.Email.FromAddress == "" {
		cfg.Email.FromAddress = cfg.Email.Username
	}

	// Migrate legacy single email config → EmailAccounts slice
	cfg.MigrateEmailAccounts()

	// Apply defaults per email account
	for i := range cfg.EmailAccounts {
		a := &cfg.EmailAccounts[i]
		if a.IMAPPort <= 0 {
			a.IMAPPort = 993
		}
		if a.SMTPPort <= 0 {
			a.SMTPPort = 587
		}
		if a.WatchInterval <= 0 {
			a.WatchInterval = 120
		}
		if a.WatchFolder == "" {
			a.WatchFolder = "INBOX"
		}
		if a.FromAddress == "" {
			a.FromAddress = a.Username
		}
	}

	// Co-Agent defaults
	if cfg.CoAgents.MaxConcurrent <= 0 {
		cfg.CoAgents.MaxConcurrent = 3
	}
	if cfg.CoAgents.CircuitBreaker.MaxToolCalls <= 0 {
		cfg.CoAgents.CircuitBreaker.MaxToolCalls = 10
	}
	if cfg.CoAgents.CircuitBreaker.TimeoutSeconds <= 0 {
		cfg.CoAgents.CircuitBreaker.TimeoutSeconds = 300 // 5 minutes
	}

	// Budget defaults
	if cfg.Budget.Enforcement == "" {
		cfg.Budget.Enforcement = "warn"
	}
	if cfg.Budget.WarningThreshold <= 0 {
		cfg.Budget.WarningThreshold = 0.8
	}
	if cfg.Budget.DefaultCost.InputPerMillion <= 0 && cfg.Budget.DefaultCost.OutputPerMillion <= 0 {
		cfg.Budget.DefaultCost = ModelCostRates{InputPerMillion: 1.0, OutputPerMillion: 3.0}
	}

	// Auth defaults
	if cfg.Auth.SessionTimeoutHours <= 0 {
		cfg.Auth.SessionTimeoutHours = 24
	}
	if cfg.Auth.MaxLoginAttempts <= 0 {
		cfg.Auth.MaxLoginAttempts = 5
	}
	if cfg.Auth.LockoutMinutes <= 0 {
		cfg.Auth.LockoutMinutes = 15
	}

	// Webhook defaults
	if cfg.Webhooks.MaxPayloadSize <= 0 {
		cfg.Webhooks.MaxPayloadSize = 65536 // 64 KB
	}

	// Ollama defaults
	if cfg.Ollama.URL == "" {
		cfg.Ollama.URL = "http://localhost:11434"
	}

	// RocketChat defaults
	if cfg.RocketChat.Alias == "" {
		cfg.RocketChat.Alias = "AuraGo"
	}

	// Tailscale: environment variable fallback for API key
	if cfg.Tailscale.APIKey == "" {
		if val := os.Getenv("TAILSCALE_API_KEY"); val != "" {
			cfg.Tailscale.APIKey = val
		}
	}

	// Ansible defaults
	if cfg.Ansible.Mode == "" {
		cfg.Ansible.Mode = "sidecar"
	}
	if cfg.Ansible.URL == "" {
		cfg.Ansible.URL = "http://ansible:5001"
	}
	if cfg.Ansible.Timeout <= 0 {
		cfg.Ansible.Timeout = 300
	}
	if cfg.Ansible.Token == "" {
		if val := os.Getenv("ANSIBLE_API_TOKEN"); val != "" {
			cfg.Ansible.Token = val
		}
	}
	if cfg.Ansible.PlaybooksDir == "" {
		if val := os.Getenv("ANSIBLE_PLAYBOOKS_DIR"); val != "" {
			cfg.Ansible.PlaybooksDir = val
		}
	}
	if cfg.Ansible.DefaultInventory == "" {
		if val := os.Getenv("ANSIBLE_INVENTORY"); val != "" {
			cfg.Ansible.DefaultInventory = val
		}
	}

	// MQTT defaults
	if cfg.MQTT.ClientID == "" {
		cfg.MQTT.ClientID = "aurago"
	}
	if cfg.MQTT.QoS < 0 || cfg.MQTT.QoS > 2 {
		cfg.MQTT.QoS = 0
	}
	if cfg.MQTT.Password == "" {
		if val := os.Getenv("MQTT_PASSWORD"); val != "" {
			cfg.MQTT.Password = val
		}
	}

	// EggMode — environment variable overrides (used in Docker egg containers)
	if val := os.Getenv("AURAGO_EGG_MODE"); val == "true" || val == "1" {
		cfg.EggMode.Enabled = true
	}
	if val := os.Getenv("AURAGO_MASTER_URL"); val != "" {
		cfg.EggMode.MasterURL = val
	}
	if val := os.Getenv("AURAGO_SHARED_KEY"); val != "" {
		cfg.EggMode.SharedKey = val
	}
	if val := os.Getenv("AURAGO_EGG_ID"); val != "" {
		cfg.EggMode.EggID = val
	}
	if val := os.Getenv("AURAGO_NEST_ID"); val != "" {
		cfg.EggMode.NestID = val
	}

	// Indexing defaults
	if cfg.Indexing.PollIntervalSeconds <= 0 {
		cfg.Indexing.PollIntervalSeconds = 60
	}
	if len(cfg.Indexing.Extensions) == 0 {
		cfg.Indexing.Extensions = []string{".txt", ".md", ".json", ".csv", ".log", ".yaml", ".yml", ".pdf", ".docx", ".xlsx", ".pptx", ".odt", ".rtf"}
	}
	if len(cfg.Indexing.Directories) == 0 {
		cfg.Indexing.Directories = []string{"./knowledge"}
	}
	// Resolve indexing directories to absolute paths
	for i, dir := range cfg.Indexing.Directories {
		cfg.Indexing.Directories[i] = resolvePath(configDir, dir)
	}

	if cfg.GitHub.BaseURL == "" {
		cfg.GitHub.BaseURL = "https://api.github.com"
	}

	// Firewall defaults
	if cfg.Firewall.PollIntervalSeconds <= 0 {
		cfg.Firewall.PollIntervalSeconds = 60
	}
	if cfg.Firewall.Mode == "" {
		cfg.Firewall.Mode = "readonly"
	}

	// Sandbox defaults
	if cfg.Sandbox.Backend == "" {
		cfg.Sandbox.Backend = "docker"
	}
	if cfg.Sandbox.Image == "" {
		cfg.Sandbox.Image = "python:3.11-slim"
	}
	if cfg.Sandbox.TimeoutSeconds <= 0 {
		cfg.Sandbox.TimeoutSeconds = 30
	}
	// DockerHost: inherit from docker.host if not set explicitly
	if cfg.Sandbox.DockerHost == "" && cfg.Docker.Host != "" {
		cfg.Sandbox.DockerHost = cfg.Docker.Host
	}

	cfg.ConfigPath = absConfigPath

	return &cfg, nil
}

// Save persists the configuration to the specified path using a targeted patch
// strategy: the original file is read, parsed as a generic YAML map, only the
// changed runtime fields are updated, and the map is written back. This ensures
// that API keys, comments structure, and other sensitive fields are never lost.
func (c *Config) Save(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// 1. Read the existing config file into a generic map
	original, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read config file for patching: %w", err)
	}

	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(original, &rawCfg); err != nil {
		return fmt.Errorf("failed to unmarshal config for patching: %w", err)
	}

	// 2. Patch only the fields that are safe to change at runtime
	if agentSection, ok := rawCfg["agent"].(map[string]interface{}); ok {
		agentSection["core_personality"] = c.Agent.CorePersonality
	}
	if serverSection, ok := rawCfg["server"].(map[string]interface{}); ok {
		serverSection["ui_language"] = c.Server.UILanguage
	}

	// 3. Write back with all original fields (including API keys) intact
	data, err := yaml.Marshal(rawCfg)
	if err != nil {
		return fmt.Errorf("failed to marshal patched config: %w", err)
	}

	if err := os.WriteFile(absPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// fixCommonConfigIssues fixes common YAML corruption issues before parsing.
func fixCommonConfigIssues(data []byte) []byte {
	content := string(data)

	// Fix 1: Normalize indentation (convert tabs to 4 spaces)
	content = regexp.MustCompile(`\t`).ReplaceAllString(content, "    ")

	// Note: budget.models validation is now handled by config-merger's
	// sanitizeMergedConfig() which properly checks if items are maps.
	// The old regex-based fix here was incorrectly triggering on valid YAML.

	// Fix 2: Remove trailing whitespace
	content = regexp.MustCompile(`[ \t]+$`).ReplaceAllString(content, "")

	// Fix 3: Ensure consistent line endings
	content = regexp.MustCompile(`\r\n`).ReplaceAllString(content, "\n")

	return []byte(content)
}

func resolvePath(baseDir, targetPath string) string {
	if targetPath == "" {
		return ""
	}
	if filepath.IsAbs(targetPath) {
		return targetPath
	}
	return filepath.Join(baseDir, targetPath)
}

// splitLines splits a string into lines
func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}
