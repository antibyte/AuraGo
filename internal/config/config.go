package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// WriteFileAtomic writes a file via temp file + rename to avoid partial writes.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		_ = tmp.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	success = true
	return nil
}

var defaultIndexingExtensions = []string{".txt", ".md", ".json", ".csv", ".log", ".yaml", ".yml", ".pdf", ".docx", ".xlsx", ".pptx", ".odt", ".rtf"}
var legacyIndexingExtensions = []string{".txt", ".md", ".json", ".csv", ".log", ".yaml", ".yml"}

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
	cfg.Tools.KnowledgeGraph.RetrievalFusion = true
	cfg.Tools.SecretsVault.Enabled = true
	cfg.Tools.Scheduler.Enabled = true
	cfg.Tools.Notes.Enabled = true
	cfg.Tools.Missions.Enabled = true
	cfg.Tools.StopProcess.Enabled = true
	cfg.Tools.Inventory.Enabled = true
	cfg.Tools.MemoryMaintenance.Enabled = true
	cfg.Tools.Journal.Enabled = true
	cfg.Tools.WebScraper.Enabled = true

	// Announcement detector: enabled by default; YAML can override with 'enabled: false'.
	cfg.Agent.AnnouncementDetector.Enabled = true

	// FritzBox defaults: disabled by default; system group enabled + readonly when fritzbox.enabled is set.
	cfg.FritzBox.Host = "fritz.box"
	cfg.FritzBox.Port = 49000
	cfg.FritzBox.HTTPS = true
	cfg.FritzBox.Timeout = 10
	cfg.FritzBox.System.Enabled = true
	cfg.FritzBox.System.ReadOnly = true
	cfg.FritzBox.System.SubFeatures.DeviceInfo = true
	cfg.FritzBox.System.SubFeatures.Uptime = true
	cfg.FritzBox.System.SubFeatures.Log = true
	cfg.FritzBox.Telephony.Polling.IntervalSeconds = 60
	cfg.FritzBox.Telephony.Polling.DedupWindowMinutes = 5
	cfg.FritzBox.Telephony.Polling.MaxCallbacksPerHour = 20

	// Document Creator defaults: Maroto backend, Gotenberg sidecar URL.
	// Use Docker-internal hostname when running inside a container, otherwise localhost.
	cfg.Tools.DocumentCreator.Backend = "maroto"
	cfg.Tools.DocumentCreator.OutputDir = "data/documents"
	if _, err := os.Stat("/.dockerenv"); err == nil {
		cfg.Tools.DocumentCreator.Gotenberg.URL = "http://gotenberg:3000"
	} else {
		cfg.Tools.DocumentCreator.Gotenberg.URL = "http://127.0.0.1:3000"
	}
	cfg.Tools.DocumentCreator.Gotenberg.Timeout = 120

	cfg.Tools.PythonTimeoutSeconds = 30
	cfg.Tools.SkillTimeoutSeconds = 120
	cfg.Tools.BackgroundTimeoutSeconds = 3600
	cfg.Tools.SkillManager.Enabled = true
	cfg.Tools.SkillManager.AllowUploads = true
	cfg.Tools.SkillManager.RequireScan = true
	cfg.Tools.SkillManager.RequireSandbox = false
	cfg.Tools.SkillManager.MaxUploadSizeMB = 1
	// Daemon Skills defaults: disabled by default (opt-in, potentially costly).
	cfg.Tools.DaemonSkills.MaxConcurrentDaemons = 5
	cfg.Tools.DaemonSkills.GlobalRateLimitSecs = 60
	cfg.Tools.DaemonSkills.MaxWakeUpsPerHour = 6
	cfg.Tools.DaemonSkills.MaxBudgetPerHourUSD = 0.50

	cfg.Tools.WebCapture.Enabled = true
	cfg.Tools.NetworkPing.Enabled = true
	cfg.Tools.NetworkScan.Enabled = true
	cfg.Tools.Contacts.Enabled = true
	cfg.Tools.Planner.Enabled = true
	// form_automation and upnp_scan default to false (opt-in; require headless browser / LAN access)

	// Mission Preparation defaults: disabled by default, uses main LLM provider.
	cfg.MissionPreparation.TimeoutSeconds = 120
	cfg.MissionPreparation.MaxEssentialTools = 5
	cfg.MissionPreparation.CacheExpiryHours = 24
	cfg.MissionPreparation.MinConfidence = 0.5
	cfg.MissionPreparation.AutoPrepareScheduled = true

	// Journal system defaults: auto-entries and daily summaries enabled by default.
	cfg.Journal.AutoEntries = true
	cfg.Journal.DailySummary = true

	// Consolidation defaults: nightly STM→LTM consolidation enabled by default.
	cfg.Consolidation.Enabled = true
	cfg.Consolidation.AutoOptimize = true
	cfg.Consolidation.ArchiveRetainDays = 30
	cfg.Consolidation.MaxBatchMessages = 200
	cfg.Consolidation.OptimizeThreshold = 1

	// Helper LLM defaults: disabled until explicitly configured.
	cfg.LLM.HelperEnabled = false
	cfg.LLM.AnthropicThinking.Enabled = false
	cfg.LLM.AnthropicThinking.BudgetTokens = 10000

	// Memory analysis defaults: adaptive and always active; legacy flags remain for compatibility only.
	cfg.MemoryAnalysis.Enabled = true
	cfg.MemoryAnalysis.Preset = "adaptive"
	cfg.MemoryAnalysis.RealTime = true
	cfg.MemoryAnalysis.AutoConfirm = 0.92
	cfg.MemoryAnalysis.QueryExpansion = true
	cfg.MemoryAnalysis.LLMReranking = true
	cfg.MemoryAnalysis.UnifiedMemoryBlock = true
	cfg.MemoryAnalysis.EffectivenessTracking = true
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
	cfg.Embeddings.MultimodalFormat = "auto"

	// Piper TTS container defaults
	cfg.TTS.CacheRetentionHours = 168
	cfg.TTS.CacheMaxFiles = 500
	cfg.TTS.Piper.ContainerPort = 10200
	cfg.TTS.Piper.DataPath = "data/piper"
	cfg.TTS.Piper.Image = "rhasspy/wyoming-piper:latest"
	cfg.TTS.Piper.Voice = "de_DE-thorsten-high"

	// Music Generation defaults
	// (Provider must be configured via Provider Management — no defaults for API keys)

	cfg.LLMGuardian.TimeoutSecs = 30
	cfg.LLMGuardian.ScanDocuments = false
	cfg.LLMGuardian.ScanEmails = false
	cfg.Guardian.MaxScanBytes = 16 * 1024
	cfg.Guardian.ScanEdgeBytes = 6 * 1024
	cfg.Guardian.PromptSec.Preset = "strict"
	cfg.Guardian.PromptSec.Spotlight = true
	cfg.Guardian.PromptSec.Canary = true

	// n8n integration defaults: disabled by default, token auth required when enabled.
	cfg.N8n.Enabled = false
	cfg.N8n.ReadOnly = false
	cfg.N8n.RequireToken = true
	cfg.N8n.RateLimitRPS = 10
	cfg.N8n.AllowedEvents = []string{"agent.response", "agent.error", "mission.completed"}

	// OneDrive defaults: "common" tenant allows both personal and work accounts.
	cfg.OneDrive.TenantID = "common"

	// WebDAV defaults: use classic Basic Auth unless explicitly switched to Bearer.
	cfg.WebDAV.AuthType = "basic"

	// SQL Connections defaults: disabled by default; agent must opt-in.
	cfg.SQLConnections.Enabled = false
	cfg.SQLConnections.ReadOnly = false        // global read-only off by default
	cfg.SQLConnections.AllowManagement = false // agent cannot manage connections by default
	cfg.SQLConnections.MaxPoolSize = 5
	cfg.SQLConnections.ConnectionTimeoutSec = 30
	cfg.SQLConnections.QueryTimeoutSec = 120
	cfg.SQLConnections.MaxResultRows = 1000
	cfg.SQLConnections.RateLimitWindowSec = 1 // per-connection rate limit: 1 second between accesses (0 = disabled)
	cfg.SQLConnections.IdleTTLSec = 600       // idle TTL: 10 minutes before connection eviction

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
		_ = WriteFileAtomic(backupPath, data, 0o600)

		return nil, fmt.Errorf("failed to unmarshal config (backup saved to %s): %w", backupPath, err)
	}

	switch strings.ToLower(strings.TrimSpace(cfg.WebDAV.AuthType)) {
	case "", "basic":
		cfg.WebDAV.AuthType = "basic"
	case "bearer":
		cfg.WebDAV.AuthType = "bearer"
	default:
		cfg.WebDAV.AuthType = "basic"
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
	if cfg.SQLite.ContactsPath == "" {
		cfg.SQLite.ContactsPath = "./data/contacts.db"
	}
	cfg.SQLite.ContactsPath = resolvePath(configDir, cfg.SQLite.ContactsPath)
	if cfg.SQLite.PlannerPath == "" {
		cfg.SQLite.PlannerPath = "./data/planner.db"
	}
	cfg.SQLite.PlannerPath = resolvePath(configDir, cfg.SQLite.PlannerPath)
	if cfg.SQLite.RemoteControlPath == "" {
		cfg.SQLite.RemoteControlPath = "./data/remote_control.db"
	}
	cfg.SQLite.RemoteControlPath = resolvePath(configDir, cfg.SQLite.RemoteControlPath)
	if cfg.SQLite.SiteMonitorPath == "" {
		cfg.SQLite.SiteMonitorPath = "./data/site_monitor.db"
	}
	cfg.SQLite.SiteMonitorPath = resolvePath(configDir, cfg.SQLite.SiteMonitorPath)
	if cfg.SQLite.SQLConnectionsPath == "" {
		cfg.SQLite.SQLConnectionsPath = "./data/sql_connections.db"
	}
	cfg.SQLite.SQLConnectionsPath = resolvePath(configDir, cfg.SQLite.SQLConnectionsPath)
	if cfg.SQLite.SkillsPath == "" {
		cfg.SQLite.SkillsPath = "./data/skills.db"
	}
	cfg.SQLite.SkillsPath = resolvePath(configDir, cfg.SQLite.SkillsPath)

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

	// Migrate legacy agent.personality_* fields → new personality section.
	cfg.MigrateAgentToPersonality()

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
	if cfg.CircuitBreaker.LLMPerAttemptTimeoutSeconds <= 0 {
		cfg.CircuitBreaker.LLMPerAttemptTimeoutSeconds = 60
	}
	if cfg.CircuitBreaker.LLMStreamChunkTimeoutSeconds <= 0 {
		cfg.CircuitBreaker.LLMStreamChunkTimeoutSeconds = 30
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
	// HTTPPort defaults to 0 = no HTTP redirect server.
	// Let's Encrypt (auto mode) will override this to 80 at startup since ACME needs it.
	// For self-signed / custom certs, the redirect is optional — leave 0 as-is.
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
	if cfg.Personality.CorePersonality == "" {
		cfg.Personality.CorePersonality = "neutral"
	}
	if cfg.Agent.CoreMemoryMaxEntries <= 0 {
		cfg.Agent.CoreMemoryMaxEntries = 200
	}
	if cfg.Agent.CoreMemoryCapMode == "" {
		cfg.Agent.CoreMemoryCapMode = "soft"
	}
	if cfg.Personality.UserProfilingThreshold <= 0 {
		cfg.Personality.UserProfilingThreshold = 3
	}
	if cfg.Personality.V2TimeoutSecs <= 0 {
		cfg.Personality.V2TimeoutSecs = 30
	}
	if cfg.Agent.ToolOutputLimit <= 0 {
		cfg.Agent.ToolOutputLimit = 50000
	}
	// V2 requires V1 — automatically enable V1 when V2 is on.
	if cfg.Personality.EngineV2 && !cfg.Personality.Engine {
		cfg.Personality.Engine = true
	}
	// Emotion Synthesizer defaults
	if cfg.Personality.EmotionSynthesizer.MinIntervalSecs <= 0 {
		cfg.Personality.EmotionSynthesizer.MinIntervalSecs = 60
	}
	if cfg.Personality.EmotionSynthesizer.MaxHistoryEntries <= 0 {
		cfg.Personality.EmotionSynthesizer.MaxHistoryEntries = 100
	}
	// Emotion Synthesizer requires Personality Engine V2
	if cfg.Personality.EmotionSynthesizer.Enabled && !cfg.Personality.EngineV2 {
		cfg.Personality.EngineV2 = true
		cfg.Personality.Engine = true
	}
	// Inner Voice defaults
	if cfg.Personality.InnerVoice.MinIntervalSecs <= 0 {
		cfg.Personality.InnerVoice.MinIntervalSecs = 60
	}
	if cfg.Personality.InnerVoice.MaxPerSession <= 0 {
		cfg.Personality.InnerVoice.MaxPerSession = 20
	}
	if cfg.Personality.InnerVoice.DecayTurns <= 0 {
		cfg.Personality.InnerVoice.DecayTurns = 3
	}
	if cfg.Personality.InnerVoice.ErrorStreakMin <= 0 {
		cfg.Personality.InnerVoice.ErrorStreakMin = 2
	}
	// Inner Voice requires Emotion Synthesizer + V2
	if cfg.Personality.InnerVoice.Enabled && !cfg.Personality.EmotionSynthesizer.Enabled {
		cfg.Personality.EmotionSynthesizer.Enabled = true
	}
	if cfg.Personality.InnerVoice.Enabled && !cfg.Personality.EngineV2 {
		cfg.Personality.EngineV2 = true
		cfg.Personality.Engine = true
	}
	// Auto-enable InnerVoice when EmotionSynthesizer is active — they are designed as a pair.
	// EmotionSynthesizer analyses emotional state; InnerVoice is the mechanism that exposes
	// that state as subconscious nudges in the system prompt.  Having one without the other
	// produces no visible effect for the user.
	if cfg.Personality.EmotionSynthesizer.Enabled && cfg.Personality.EngineV2 && !cfg.Personality.InnerVoice.Enabled {
		cfg.Personality.InnerVoice.Enabled = true
	}
	if cfg.Agent.SystemPromptTokenBudget <= 0 {
		cfg.Agent.SystemPromptTokenBudgetAuto = true
		cfg.Agent.SystemPromptTokenBudget = 0
	} else {
		cfg.Agent.SystemPromptTokenBudgetAuto = false
	}
	if !yamlHasPath(data, "agent", "adaptive_system_prompt_token_budget") {
		cfg.Agent.AdaptiveSystemPromptTokenBudget = true
	}
	if !yamlHasPath(data, "agent", "optimizer_enabled") {
		cfg.Agent.OptimizerEnabled = true
	}
	// Adaptive tools defaults
	// CHANGE LOG 2026-04-11: Reduced default from 60 to 16 based on research showing
	// function-calling accuracy degrades significantly beyond ~20 tools (OpenAI, Anthropic, Barres et al. 2025).
	// 60 was too permissive; new users without explicit config would send 50-150 schemas.
	// Existing configs with explicit values are preserved. MaxTools=0 disables the cap (not recommended).
	if cfg.Agent.AdaptiveTools.MaxTools <= 0 && cfg.Agent.AdaptiveTools.Enabled {
		cfg.Agent.AdaptiveTools.MaxTools = 16
	}
	if cfg.Agent.AdaptiveTools.DecayHalfLifeDays <= 0 {
		cfg.Agent.AdaptiveTools.DecayHalfLifeDays = 7.0
	}
	if cfg.Agent.AdaptiveTools.CleanTransitionsAfterDays <= 0 {
		cfg.Agent.AdaptiveTools.CleanTransitionsAfterDays = 90
	}
	// WeightSuccessRate defaults to true when omitted, but must preserve an
	// explicit user-provided false value from YAML.
	if cfg.Agent.AdaptiveTools.Enabled &&
		!cfg.Agent.AdaptiveTools.WeightSuccessRate &&
		!yamlHasPath(data, "agent", "adaptive_tools", "weight_success_rate") {
		cfg.Agent.AdaptiveTools.WeightSuccessRate = true
	}
	if len(cfg.Agent.AdaptiveTools.AlwaysInclude) == 0 && cfg.Agent.AdaptiveTools.Enabled {
		cfg.Agent.AdaptiveTools.AlwaysInclude = []string{
			"filesystem", "file_editor", "shell", "manage_memory", "query_memory",
			"execute_python", "docker", "api_request",
		}
	}
	if cfg.Agent.MaxToolGuides <= 0 {
		cfg.Agent.MaxToolGuides = 5
	}
	// AnnouncementDetector defaults — enabled by default.
	if cfg.Agent.AnnouncementDetector.MaxRetries <= 0 {
		cfg.Agent.AnnouncementDetector.MaxRetries = 2
	}
	if cfg.Agent.Recovery.MaxProvider422Recoveries <= 0 {
		cfg.Agent.Recovery.MaxProvider422Recoveries = 3
	}
	if cfg.Agent.Recovery.MinMessagesForEmptyRetry <= 0 {
		cfg.Agent.Recovery.MinMessagesForEmptyRetry = 5
	}
	if cfg.Agent.Recovery.DuplicateConsecutiveHits <= 0 {
		cfg.Agent.Recovery.DuplicateConsecutiveHits = 2
	}
	if cfg.Agent.Recovery.DuplicateFrequencyHits <= 0 {
		cfg.Agent.Recovery.DuplicateFrequencyHits = 3
	}
	if cfg.Agent.Recovery.IdenticalToolErrorHits <= 0 {
		cfg.Agent.Recovery.IdenticalToolErrorHits = 3
	}
	if !yamlHasPath(data, "agent", "background_tasks", "enabled") {
		cfg.Agent.BackgroundTasks.Enabled = true
	}
	if cfg.Agent.BackgroundTasks.FollowUpDelaySeconds <= 0 {
		cfg.Agent.BackgroundTasks.FollowUpDelaySeconds = 2
	}
	if cfg.Agent.BackgroundTasks.HTTPTimeoutSeconds <= 0 {
		cfg.Agent.BackgroundTasks.HTTPTimeoutSeconds = 120
	}
	if cfg.Agent.BackgroundTasks.MaxRetries < 0 {
		cfg.Agent.BackgroundTasks.MaxRetries = 0
	}
	if !yamlHasPath(data, "agent", "background_tasks", "max_retries") {
		cfg.Agent.BackgroundTasks.MaxRetries = 2
	}
	if cfg.Agent.BackgroundTasks.RetryDelaySeconds <= 0 {
		cfg.Agent.BackgroundTasks.RetryDelaySeconds = 60
	}
	if cfg.Agent.BackgroundTasks.WaitPollIntervalSecs <= 0 {
		cfg.Agent.BackgroundTasks.WaitPollIntervalSecs = 5
	}
	if cfg.Agent.BackgroundTasks.WaitDefaultTimeoutSecs <= 0 {
		cfg.Agent.BackgroundTasks.WaitDefaultTimeoutSecs = 600
	}
	if cfg.CoAgents.MaxConcurrent <= 0 {
		cfg.CoAgents.MaxConcurrent = 3
	}
	if cfg.CoAgents.BudgetQuotaPercent < 0 {
		cfg.CoAgents.BudgetQuotaPercent = 0
	}
	if cfg.CoAgents.MaxContextHints <= 0 {
		cfg.CoAgents.MaxContextHints = 6
	}
	if cfg.CoAgents.MaxContextHintChars <= 0 {
		cfg.CoAgents.MaxContextHintChars = 180
	}
	if cfg.CoAgents.MaxResultBytes <= 0 {
		cfg.CoAgents.MaxResultBytes = 100000
	}
	if !yamlHasPath(data, "co_agents", "queue_when_busy") {
		cfg.CoAgents.QueueWhenBusy = true
	}
	if cfg.CoAgents.CleanupIntervalMins <= 0 {
		cfg.CoAgents.CleanupIntervalMins = 10
	}
	if cfg.CoAgents.CleanupMaxAgeMins <= 0 {
		cfg.CoAgents.CleanupMaxAgeMins = 30
	}
	if cfg.CoAgents.RetryPolicy.MaxRetries < 0 {
		cfg.CoAgents.RetryPolicy.MaxRetries = 0
	}
	if !yamlHasPath(data, "co_agents", "retry_policy", "max_retries") {
		cfg.CoAgents.RetryPolicy.MaxRetries = 1
	}
	if cfg.CoAgents.RetryPolicy.RetryDelaySeconds <= 0 {
		cfg.CoAgents.RetryPolicy.RetryDelaySeconds = 5
	}
	if len(cfg.CoAgents.RetryPolicy.RetryableErrorPatterns) == 0 {
		cfg.CoAgents.RetryPolicy.RetryableErrorPatterns = []string{
			"deadline exceeded",
			"timeout",
			"timed out",
			"temporarily unavailable",
			"temporary failure",
			"rate limit",
			"too many requests",
			"connection reset",
			"connection refused",
			"connection aborted",
			"broken pipe",
			"eof",
			"unavailable",
			"network is unreachable",
		}
	}
	// LLM defaults
	if cfg.LLM.Temperature == 0 {
		cfg.LLM.Temperature = 0.7
	}
	if cfg.Agent.ContextWindow <= 0 {
		cfg.Agent.ContextWindow = 0 // 0 = agent loop defaults to 163840 (160k context guard)
	}

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
	if cfg.S3.Region == "" {
		cfg.S3.Region = "us-east-1"
	}

	// Homepage defaults
	if cfg.Homepage.WebServerPort <= 0 {
		cfg.Homepage.WebServerPort = 8080
	}
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

	// Telnyx defaults
	if cfg.Telnyx.WebhookPath == "" {
		cfg.Telnyx.WebhookPath = "/api/telnyx/webhook"
	}
	if cfg.Telnyx.MaxConcurrentCalls <= 0 {
		cfg.Telnyx.MaxConcurrentCalls = 3
	}
	if cfg.Telnyx.MaxSMSPerMinute <= 0 {
		cfg.Telnyx.MaxSMSPerMinute = 10
	}
	if cfg.Telnyx.VoiceLanguage == "" {
		cfg.Telnyx.VoiceLanguage = "en"
	}
	if cfg.Telnyx.VoiceGender == "" {
		cfg.Telnyx.VoiceGender = "female"
	}
	if cfg.Telnyx.CallTimeout <= 0 {
		cfg.Telnyx.CallTimeout = 300
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

	// A2A defaults
	if cfg.A2A.Server.BasePath == "" {
		cfg.A2A.Server.BasePath = "/a2a"
	}
	if cfg.A2A.Server.AgentName == "" {
		cfg.A2A.Server.AgentName = "AuraGo"
	}
	if cfg.A2A.Server.AgentVersion == "" {
		cfg.A2A.Server.AgentVersion = "1.0.0"
	}
	if cfg.A2A.Server.Bindings.GRPCPort <= 0 {
		cfg.A2A.Server.Bindings.GRPCPort = 50051
	}
	// Default: enable REST binding when server is enabled
	if cfg.A2A.Server.Enabled && !cfg.A2A.Server.Bindings.REST && !cfg.A2A.Server.Bindings.JSONRPC && !cfg.A2A.Server.Bindings.GRPC {
		cfg.A2A.Server.Bindings.REST = true
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
	if cfg.Webhooks.RateLimit <= 0 {
		cfg.Webhooks.RateLimit = 60 // 60 requests per minute per token (0 = unlimited)
	}

	// Ollama defaults
	if cfg.Ollama.URL == "" {
		cfg.Ollama.URL = "http://localhost:11434"
	}
	if cfg.Ollama.ManagedInstance.ContainerPort <= 0 {
		cfg.Ollama.ManagedInstance.ContainerPort = 11434
	}
	if cfg.Ollama.ManagedInstance.GPUBackend == "" {
		cfg.Ollama.ManagedInstance.GPUBackend = "auto"
	}
	// When managed instance is active, point the Ollama URL to the local container.
	if cfg.Ollama.ManagedInstance.Enabled {
		cfg.Ollama.URL = fmt.Sprintf("http://localhost:%d", cfg.Ollama.ManagedInstance.ContainerPort)
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
		cfg.Ansible.URL = "http://127.0.0.1:5001"
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
	if cfg.Ansible.Image == "" {
		cfg.Ansible.Image = "aurago-ansible:latest"
	}
	if cfg.Ansible.ContainerName == "" {
		cfg.Ansible.ContainerName = "aurago_ansible"
	}

	// MQTT defaults
	if cfg.MQTT.ClientID == "" {
		cfg.MQTT.ClientID = "aurago"
	}
	if cfg.MQTT.QoS < 0 || cfg.MQTT.QoS > 2 {
		cfg.MQTT.QoS = 0
	}
	if cfg.MQTT.ConnectTimeout <= 0 {
		cfg.MQTT.ConnectTimeout = 15
	}
	if cfg.MQTT.Buffer.MaxMessages <= 0 {
		cfg.MQTT.Buffer.MaxMessages = 500
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
		cfg.Indexing.Extensions = append([]string(nil), defaultIndexingExtensions...)
	} else if usesLegacyDefaultIndexingExtensions(cfg.Indexing.Extensions) {
		cfg.Indexing.Extensions = append([]string(nil), defaultIndexingExtensions...)
	}
	if len(cfg.Indexing.Directories) == 0 {
		cfg.Indexing.Directories = []IndexingDirectory{{Path: "./knowledge"}}
	}
	// Resolve indexing directory paths to absolute paths
	for i := range cfg.Indexing.Directories {
		cfg.Indexing.Directories[i].Path = resolvePath(configDir, cfg.Indexing.Directories[i].Path)
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

	// Shell Sandbox defaults (Landlock-based native OS sandbox)
	if cfg.ShellSandbox.MaxMemoryMB <= 0 {
		cfg.ShellSandbox.MaxMemoryMB = 1024
	}
	if cfg.ShellSandbox.MaxCPUSeconds <= 0 {
		cfg.ShellSandbox.MaxCPUSeconds = 30
	}
	if cfg.ShellSandbox.MaxProcesses <= 0 {
		cfg.ShellSandbox.MaxProcesses = 50
	}
	if cfg.ShellSandbox.MaxFileSizeMB <= 0 {
		cfg.ShellSandbox.MaxFileSizeMB = 100
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

func usesLegacyDefaultIndexingExtensions(exts []string) bool {
	if len(exts) != len(legacyIndexingExtensions) {
		return false
	}

	normalized := make([]string, len(exts))
	for i, ext := range exts {
		normalized[i] = strings.ToLower(strings.TrimSpace(ext))
	}
	slices.Sort(normalized)

	expected := append([]string(nil), legacyIndexingExtensions...)
	slices.Sort(expected)

	return slices.Equal(normalized, expected)
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
	if _, ok := rawCfg["personality"]; !ok {
		rawCfg["personality"] = map[string]interface{}{}
	}
	if personalitySection, ok := rawCfg["personality"].(map[string]interface{}); ok {
		personalitySection["core_personality"] = c.Personality.CorePersonality
	}
	if serverSection, ok := rawCfg["server"].(map[string]interface{}); ok {
		serverSection["ui_language"] = c.Server.UILanguage
	}
	if authSection, ok := rawCfg["auth"].(map[string]interface{}); ok {
		authSection["enabled"] = c.Auth.Enabled
	}
	if _, ok := rawCfg["webhooks"]; !ok {
		rawCfg["webhooks"] = map[string]interface{}{}
	}
	if webhooksSection, ok := rawCfg["webhooks"].(map[string]interface{}); ok {
		webhooksSection["outgoing"] = c.Webhooks.Outgoing
	}

	// 3. Write back with all original fields (including API keys) intact
	data, err := yaml.Marshal(rawCfg)
	if err != nil {
		return fmt.Errorf("failed to marshal patched config: %w", err)
	}

	perm := os.FileMode(0o600)
	if info, statErr := os.Stat(absPath); statErr == nil {
		perm = info.Mode().Perm()
		if perm == 0 {
			perm = 0o600
		}
	}
	if err := WriteFileAtomic(absPath, data, perm); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// fixCommonConfigIssues fixes common YAML corruption issues before parsing.
func fixCommonConfigIssues(data []byte) []byte {
	content := string(data)

	// Fix 1: Normalize indentation (convert tabs to 4 spaces)
	content = regexp.MustCompile(`\t`).ReplaceAllString(content, "    ")

	// Fix 2: Remove trailing whitespace
	content = regexp.MustCompile(`[ \t]+$`).ReplaceAllString(content, "")

	// Fix 3: Ensure consistent line endings
	content = regexp.MustCompile(`\r\n`).ReplaceAllString(content, "\n")

	// Fix 4: indexing.directories uses []IndexingDirectory{{path, collection}}.
	// Legacy configs often have bare strings like "- ./knowledge" which fail
	// yaml.Unmarshal. Convert them to "- path: ./knowledge".
	content = fixBareStringDirectoryItems(content)

	return []byte(content)
}

// fixBareStringDirectoryItems converts bare string items in indexing.directories
// from "- ./knowledge" to "- path: ./knowledge".
// Only bare list items that look like paths (starting with ./  ../  /) are converted.
// Regular list items like "- .txt" or "- value" are left untouched.
func fixBareStringDirectoryItems(content string) string {
	// (?m) enables multiline mode so ^/$ match line boundaries.
	// The pattern matches bare list items that are clearly paths:
	//   - ./something   (relative path)  → group 2 = "./"  group 3 = "something"
	//   - ../something  (parent-relative) → group 2 = "../" group 3 = "something"
	//   - /something    (absolute path)  → group 2 = "/"   group 3 = "something"
	// Items like "- .txt" won't match because `.` followed by `t` doesn't form ./ or ../.
	re := regexp.MustCompile(`(?m)^(\s*-\s+)(\./|../|/)([^\n]*)$`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) == 4 {
			return parts[1] + "path: " + parts[2] + parts[3]
		}
		return match
	})
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

func yamlHasPath(data []byte, path ...string) bool {
	if len(path) == 0 {
		return false
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false
	}

	node := &root
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}

	for _, key := range path {
		if node == nil || node.Kind != yaml.MappingNode {
			return false
		}

		found := false
		for i := 0; i+1 < len(node.Content); i += 2 {
			k := node.Content[i]
			v := node.Content[i+1]
			if k.Value == key {
				node = v
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// splitLines splits a string into lines
func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}
