package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

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
	cfg.Tools.KnowledgeGraph.AutoExtraction = true
	cfg.Tools.KnowledgeGraph.PromptInjection = true
	cfg.Tools.KnowledgeGraph.MaxPromptNodes = 5
	cfg.Tools.KnowledgeGraph.MaxPromptChars = 800
	cfg.Tools.SecretsVault.Enabled = true
	cfg.Tools.Scheduler.Enabled = true
	cfg.Tools.Notes.Enabled = true
	cfg.Tools.Missions.Enabled = true
	cfg.Tools.StopProcess.Enabled = true
	cfg.Tools.Inventory.Enabled = true
	cfg.Tools.MemoryMaintenance.Enabled = true
	cfg.Tools.Journal.Enabled = true
	cfg.Tools.WebScraper.Enabled = true

	// Document Creator defaults: Maroto backend, Gotenberg sidecar URL.
	cfg.Tools.DocumentCreator.Backend = "maroto"
	cfg.Tools.DocumentCreator.OutputDir = "data/documents"
	cfg.Tools.DocumentCreator.Gotenberg.URL = "http://gotenberg:3000"
	cfg.Tools.DocumentCreator.Gotenberg.Timeout = 120

	// Journal system defaults: auto-entries and daily summaries enabled by default.
	cfg.Journal.AutoEntries = true
	cfg.Journal.DailySummary = true

	// Consolidation defaults: nightly STM→LTM consolidation enabled by default.
	cfg.Consolidation.Enabled = true
	cfg.Consolidation.AutoOptimize = true
	cfg.Consolidation.ArchiveRetainDays = 30
	cfg.Consolidation.MaxBatchMessages = 200
	cfg.Consolidation.OptimizeThreshold = 1

	// Memory analysis defaults: disabled by default (opt-in), falls back to main LLM.
	cfg.MemoryAnalysis.AutoConfirm = 0.92
	cfg.MemoryAnalysis.WeeklyReflection = true
	cfg.MemoryAnalysis.ReflectionDay = "sunday"

	// LLM Guardian defaults: disabled by default, medium protection when enabled.
	cfg.LLMGuardian.DefaultLevel = "medium"
	cfg.LLMGuardian.FailSafe = "quarantine"
	cfg.LLMGuardian.CacheTTL = 300
	cfg.LLMGuardian.MaxChecksPerMin = 60
	cfg.LLMGuardian.AllowClarification = false
	cfg.Embeddings.LocalOllama.Model = "nomic-embed-text"
	cfg.Embeddings.LocalOllama.ContainerPort = 11435
	cfg.Embeddings.LocalOllama.GPUBackend = "auto"

	cfg.LLMGuardian.TimeoutSecs = 30
	cfg.LLMGuardian.ScanDocuments = false
	cfg.LLMGuardian.ScanEmails = false

	// OneDrive defaults: "common" tenant allows both personal and work accounts.
	cfg.OneDrive.TenantID = "common"

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

	// Resolve document creator output directory
	cfg.Tools.DocumentCreator.OutputDir = resolvePath(configDir, cfg.Tools.DocumentCreator.OutputDir)

	// Resolve absolute paths for SQLite
	cfg.SQLite.ShortTermPath = resolvePath(configDir, cfg.SQLite.ShortTermPath)
	cfg.SQLite.LongTermPath = resolvePath(configDir, cfg.SQLite.LongTermPath)
	cfg.SQLite.InventoryPath = resolvePath(configDir, cfg.SQLite.InventoryPath)
	if cfg.SQLite.InvasionPath == "" {
		cfg.SQLite.InvasionPath = "./data/invasion.db"
	}
	cfg.SQLite.InvasionPath = resolvePath(configDir, cfg.SQLite.InvasionPath)
	if cfg.SQLite.CheatsheetPath == "" {
		cfg.SQLite.CheatsheetPath = "./data/cheatsheets.db"
	}
	cfg.SQLite.CheatsheetPath = resolvePath(configDir, cfg.SQLite.CheatsheetPath)
	if cfg.SQLite.ImageGalleryPath == "" {
		cfg.SQLite.ImageGalleryPath = "./data/image_gallery.db"
	}
	cfg.SQLite.ImageGalleryPath = resolvePath(configDir, cfg.SQLite.ImageGalleryPath)
	if cfg.SQLite.MediaRegistryPath == "" {
		cfg.SQLite.MediaRegistryPath = "./data/media_registry.db"
	}
	cfg.SQLite.MediaRegistryPath = resolvePath(configDir, cfg.SQLite.MediaRegistryPath)
	if cfg.SQLite.HomepageRegistryPath == "" {
		cfg.SQLite.HomepageRegistryPath = "./data/homepage_registry.db"
	}
	cfg.SQLite.HomepageRegistryPath = resolvePath(configDir, cfg.SQLite.HomepageRegistryPath)

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
	// Only apply when the field is explicitly set to false in the YAML.
	// A nil pointer means the field was absent — don't disable the scraper.
	if cfg.Agent.AllowWebScraper != nil && !*cfg.Agent.AllowWebScraper {
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
		cfg.CircuitBreaker.LLMTimeoutSeconds = 600 // 10 minutes
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
	if cfg.Server.HTTPS.HTTPSPort <= 0 {
		cfg.Server.HTTPS.HTTPSPort = 443
	}
	if cfg.Server.HTTPS.HTTPPort <= 0 {
		cfg.Server.HTTPS.HTTPPort = 80
	}
	if cfg.Server.HTTPS.Enabled && cfg.Server.HTTPS.CertMode == "" {
		cfg.Server.HTTPS.CertMode = "auto"
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
	if cfg.Agent.ToolOutputLimit <= 0 {
		cfg.Agent.ToolOutputLimit = 50000
	}
	// V2 requires V1 — automatically enable V1 when V2 is on.
	if cfg.Agent.PersonalityEngineV2 && !cfg.Agent.PersonalityEngine {
		cfg.Agent.PersonalityEngine = true
	}
	if cfg.Agent.SystemPromptTokenBudget <= 0 {
		cfg.Agent.SystemPromptTokenBudget = 12288
	}
	// Adaptive tools defaults
	if cfg.Agent.AdaptiveTools.MaxTools <= 0 && cfg.Agent.AdaptiveTools.Enabled {
		cfg.Agent.AdaptiveTools.MaxTools = 60
	}
	if cfg.Agent.AdaptiveTools.DecayHalfLifeDays <= 0 {
		cfg.Agent.AdaptiveTools.DecayHalfLifeDays = 7.0
	}
	if len(cfg.Agent.AdaptiveTools.AlwaysInclude) == 0 && cfg.Agent.AdaptiveTools.Enabled {
		cfg.Agent.AdaptiveTools.AlwaysInclude = []string{
			"filesystem", "shell", "manage_memory", "query_memory",
			"execute_python", "docker", "api_request",
		}
	}
	if cfg.Agent.MaxToolGuides <= 0 {
		cfg.Agent.MaxToolGuides = 5
	}
	// LLM defaults
	if cfg.LLM.Temperature == 0 {
		cfg.LLM.Temperature = 0.7
	}
	if cfg.Agent.ContextWindow <= 0 {
		cfg.Agent.ContextWindow = 0 // 0 = agent loop defaults to 163840 (160k context guard)
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

	// Homepage defaults
	if cfg.Homepage.CircuitBreakerMaxCalls <= 0 {
		cfg.Homepage.CircuitBreakerMaxCalls = 35
	}
	if cfg.Homepage.CircuitBreakerMaxCalls > 100 {
		cfg.Homepage.CircuitBreakerMaxCalls = 100
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

	// Tailscale tsnet defaults
	if cfg.Tailscale.TsNet.Hostname == "" {
		cfg.Tailscale.TsNet.Hostname = "aurago"
	}
	if cfg.Tailscale.TsNet.StateDir == "" {
		cfg.Tailscale.TsNet.StateDir = filepath.Join(cfg.Directories.DataDir, "tsnet")
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

	// Security Proxy defaults
	if cfg.SecurityProxy.HTTPSPort <= 0 {
		cfg.SecurityProxy.HTTPSPort = 443
	}
	if cfg.SecurityProxy.HTTPPort <= 0 {
		cfg.SecurityProxy.HTTPPort = 80
	}
	if cfg.SecurityProxy.RateLimiting.RequestsPerSecond <= 0 {
		cfg.SecurityProxy.RateLimiting.RequestsPerSecond = 10
	}
	if cfg.SecurityProxy.RateLimiting.Burst <= 0 {
		cfg.SecurityProxy.RateLimiting.Burst = 50
	}
	if cfg.SecurityProxy.IPFilter.Mode == "" {
		cfg.SecurityProxy.IPFilter.Mode = "blocklist"
	}
	if cfg.SecurityProxy.DockerHost == "" && cfg.Docker.Host != "" {
		cfg.SecurityProxy.DockerHost = cfg.Docker.Host
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
