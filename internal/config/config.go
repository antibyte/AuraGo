package config

import (
	"aurago/internal/chunking"
	"aurago/internal/kgquality"
	"bytes"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultVirtualComputersBoringdURL = "http://127.0.0.1:18082"

var directMLMigrationWarning sync.Once

func normalizeVirtualComputersBoringdURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	switch strings.ToLower(strings.TrimRight(trimmed, "/")) {
	case "", "http://127.0.0.1:8080", "http://localhost:8080", "http://127.0.0.1:18080", "http://localhost:18080":
		return DefaultVirtualComputersBoringdURL
	default:
		return trimmed
	}
}

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
	if err := replaceFileAtomic(tmpPath, path); err != nil {
		return err
	}
	success = true
	return nil
}

func replaceFileAtomic(tmpPath, path string) error {
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		err = os.Rename(tmpPath, path)
		if err == nil {
			return nil
		}
		if runtime.GOOS != "windows" || !os.IsPermission(err) {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 15 * time.Millisecond)
	}
	return err
}

var defaultIndexingExtensions = []string{".txt", ".md", ".json", ".csv", ".log", ".yaml", ".yml", ".pdf", ".docx", ".xlsx", ".pptx", ".odt", ".ods", ".odp", ".rtf"}
var legacyIndexingExtensions = []string{".txt", ".md", ".json", ".csv", ".log", ".yaml", ".yml"}
var defaultWorkspaceSearchExcludes = []string{
	".git",
	"node_modules",
	"__pycache__",
	"venv",
	".venv",
	".env",
	"*.db",
	"*.sqlite",
	"*.sqlite3",
	"vault.bin",
	"data/vault.bin",
}
var configSaveMu sync.Mutex

// DefaultWorkspaceSearchExcludes returns the safe default skip list for the resident workspace index.
func DefaultWorkspaceSearchExcludes() []string {
	return append([]string(nil), defaultWorkspaceSearchExcludes...)
}

const (
	defaultManifestTsNetPort      = 443
	legacyManifestTsNetPort       = 8444
	dograhDefaultAPIImage         = "ghcr.io/dograh-hq/dograh-api:latest"
	dograhDefaultUIImage          = "ghcr.io/dograh-hq/dograh-ui:latest"
	dograhLegacyDockerHubAPIImage = "dograhai/dograh-api:latest"
	dograhLegacyDockerHubUIImage  = "dograhai/dograh-ui:latest"
)

const defaultWriterSpecialistAdditionalPrompt = `## Multilingual natural writing defaults

You are the AuraGo Writer Specialist. Produce text that sounds like a capable human writing in the requested language, not like translated model output.

Language and audience
- Write in the task language, or in AuraGo's system language if none is specified. Preserve mixed-language passages unless translation is requested.
- Match the medium, audience, formality, regional variant, and pronoun convention. For German, preserve Du/Sie when implied. For languages with script-specific punctuation, use natural native punctuation.
- If the task is a rewrite, preserve meaning, facts, names, numbers, structure, and the author's intent. Do not add claims, citations, or biographical filler.

Voice calibration
- If a writing sample or existing draft is provided, mirror its register, rhythm, paragraph length, terminology, and punctuation habits. Do not upgrade casual wording into corporate prose.
- If no sample is provided, write plainly with varied sentence length, concrete detail, and a natural rhythm suitable for the genre.

AI-pattern cleanup
- Remove formulaic openings and closings, excessive politeness, generic upbeat conclusions, over-signposting, forced three-part lists, vague authority claims, inflated significance, promotional adjectives, needless hedging, passive actor hiding, synonym cycling, and abstract filler.
- Prefer direct verbs, specific nouns, named sources when available, and simple sentence shapes.
- Do not use English-only rules blindly. Dashes, title case, punctuation, sentence length, and paragraph shape must follow the target language and medium.
- Avoid repeated decorative dashes, emojis, bold labels, and inline header lists unless they are normal for the requested format.
- Remove translationese: word-for-word calques, unnatural connector stacking, English idioms transplanted into another language, and register shifts that native speakers would notice.

Safety and honesty
- Do not invent facts, citations, quotes, dates, or personal details. If something is unknown, say so briefly or omit it.
- Do not claim text was written by a human or help evade disclosure rules. The goal is clear, natural writing, not deception.

Process
- Internally do a quick pass: draft, check what still sounds generic or model-like, then revise.
- Return only the final usable text unless the main task asks for analysis, options, or an edit report.`

func defaultSidecarURL(runningInDocker bool, service string, port int) string {
	if runningInDocker {
		return fmt.Sprintf("http://%s:%d", service, port)
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func NormalizeLegacySidecarURL(raw string, runningInDocker bool, service string, port int) string {
	if runningInDocker {
		return raw
	}

	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return raw
	}

	host := parsed.Hostname()
	if !strings.EqualFold(host, service) {
		return raw
	}

	if parsed.Port() != "" && parsed.Port() != fmt.Sprintf("%d", port) {
		return raw
	}

	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}
	parsed.Host = net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port))
	return parsed.String()
}

func normalizeDograhDefaultImage(raw, currentDefault, legacyDefault string) string {
	image := strings.TrimSpace(raw)
	if image == "" || strings.EqualFold(image, legacyDefault) {
		return currentDefault
	}
	return image
}

func normalizeSpaceAgentURLAndPort(publicURL string, port int, runningInDocker bool) (string, int) {
	const oldConflictingPort = 3000
	const defaultSpaceAgentPort = 3100
	portWasUnset := port <= 0
	if port <= 0 {
		port = defaultSpaceAgentPort
	}
	normalizedURL := NormalizeLegacySidecarURL(publicURL, runningInDocker, "space-agent", port)
	if (portWasUnset || port == oldConflictingPort || port == defaultSpaceAgentPort) && spaceAgentUsesLegacyDefaultURL(normalizedURL, runningInDocker) {
		port = defaultSpaceAgentPort
		normalizedURL = ""
	}
	if strings.TrimSpace(normalizedURL) == "" {
		normalizedURL = ""
	}
	return normalizedURL, port
}

func spaceAgentUsesLegacyDefaultURL(raw string, runningInDocker bool) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return false
	}
	host := strings.Trim(strings.ToLower(parsed.Hostname()), "[]")
	port := parsed.Port()
	if port != "" && port != "3000" {
		return false
	}
	if runningInDocker {
		return host == "space-agent"
	}
	return host == "127.0.0.1" || host == "localhost" || host == "::1" || host == "space-agent"
}

func sanitizeDockerTag(raw string) string {
	tag := strings.TrimSpace(raw)
	if tag == "" {
		return "main"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "@", "-", " ", "-")
	tag = replacer.Replace(tag)
	tag = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`).ReplaceAllString(tag, "-")
	tag = strings.Trim(tag, ".-")
	if tag == "" {
		return "main"
	}
	return tag
}

func sanitizeTsnetHostname(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" {
		return "aurago-space-agent"
	}
	name = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "aurago-space-agent"
	}
	return name
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
	data = normalizeFritzBoxLegacyKeys(data)

	var cfg Config
	cfg.ModelCatalog.Enabled = true
	cfg.ModelCatalog.CatalogOnlyVisible = true
	// Tools section defaults: all tools are enabled by default (opt-in to disable).
	// These are set before unmarshal so that keys absent from the YAML file keep the
	// correct default; explicit 'enabled: false' in the YAML will still override them.
	cfg.Tools.Memory.Enabled = true
	cfg.Tools.Memory.OnDemandRetrieval.Enabled = true
	cfg.Tools.Memory.OnDemandRetrieval.MaxEssentialMemories = 1
	cfg.Tools.Memory.OnDemandRetrieval.MaxAvailableMemories = 6
	cfg.Tools.Memory.OnDemandRetrieval.MaxAvailableKGNodes = 6
	cfg.Tools.Memory.OnDemandRetrieval.MaxAvailableChars = 1600
	cfg.Tools.Memory.OnDemandRetrieval.DedupeScope = "turn"
	cfg.Tools.KnowledgeGraph.Enabled = true
	cfg.Tools.KnowledgeGraph.AutoExtraction = true
	cfg.Tools.KnowledgeGraph.PromptInjection = true
	cfg.Tools.KnowledgeGraph.MaxPromptNodes = 5
	cfg.Tools.KnowledgeGraph.MaxPromptChars = 800
	cfg.Tools.KnowledgeGraph.RetrievalFusion = true
	cfg.Tools.KnowledgeGraph.MinSemanticSimilarity = 0.60
	cfg.Tools.KnowledgeGraph.ExcludeNodeTypes = []string{"activity_entity", "unknown"}
	cfg.Tools.KnowledgeGraph.SemanticReindexInterval = "5m"
	cfg.Tools.KnowledgeGraph.ProtectOptimizeSources = []string{"planner", "inventory_sync", "manual", "file_sync", "core_memory"}
	cfg.Tools.KnowledgeGraph.ProtectIDPrefixes = []string{"core_fact_", "dev_", "contact_"}
	kgQualityPolicy := kgquality.DefaultPolicy()
	cfg.Tools.KnowledgeGraph.PendingCoMentionTTLDays = kgQualityPolicy.PendingCoMentionTTLDays
	cfg.Tools.KnowledgeGraph.LowConfidenceCoMentionMinWeight = kgQualityPolicy.LowConfidenceCoMentionMinWeight
	cfg.Tools.KnowledgeGraph.HideLowConfidenceByDefault = kgQualityPolicy.HideLowConfidenceByDefault
	cfg.Tools.SecretsVault.Enabled = true
	cfg.Tools.Scheduler.Enabled = true
	cfg.Tools.Notes.Enabled = true
	cfg.Tools.Missions.Enabled = true
	cfg.Tools.StopProcess.Enabled = true
	cfg.Tools.Inventory.Enabled = true
	cfg.Tools.MemoryMaintenance.Enabled = true
	cfg.Tools.Journal.Enabled = true
	cfg.Tools.WebScraper.Enabled = true
	cfg.WorkspaceSearch.Enabled = true
	cfg.WorkspaceSearch.MaxFileSizeMB = 10
	cfg.WorkspaceSearch.MaxIndexSizeMB = 256
	cfg.WorkspaceSearch.MaxResults = 100
	cfg.WorkspaceSearch.PollIntervalSeconds = 5
	cfg.WorkspaceSearch.FuzzyThreshold = 0.35
	cfg.WorkspaceSearch.Exclude = DefaultWorkspaceSearchExcludes()

	// Structural text-only continuation recovery: enabled by default; YAML can override with 'enabled: false'.
	cfg.Agent.AnnouncementDetector.Enabled = true

	// FritzBox defaults: disabled by default; system group enabled + readonly when fritzbox.enabled is set.
	cfg.FritzBox.Host = "fritz.box"
	cfg.FritzBox.Port = 49000
	cfg.FritzBox.HTTPS = false
	cfg.FritzBox.Timeout = 10
	cfg.FritzBox.System.Enabled = true
	cfg.FritzBox.System.ReadOnly = true
	cfg.FritzBox.System.SubFeatures.DeviceInfo = true
	cfg.FritzBox.System.SubFeatures.Uptime = true
	cfg.FritzBox.System.SubFeatures.Log = true
	cfg.FritzBox.Network.SubFeatures.WLAN = true
	cfg.FritzBox.Network.SubFeatures.Hosts = true
	cfg.FritzBox.Network.SubFeatures.WakeOnLAN = true
	cfg.FritzBox.Network.SubFeatures.PortForwarding = true
	cfg.FritzBox.Telephony.SubFeatures.CallLists = true
	cfg.FritzBox.Telephony.SubFeatures.Phonebooks = true
	cfg.FritzBox.Telephony.SubFeatures.TAM = true
	cfg.FritzBox.SmartHome.SubFeatures.Devices = true
	cfg.FritzBox.SmartHome.SubFeatures.Switches = true
	cfg.FritzBox.SmartHome.SubFeatures.Heating = true
	cfg.FritzBox.SmartHome.SubFeatures.Lamps = true
	cfg.FritzBox.SmartHome.SubFeatures.Templates = true
	cfg.FritzBox.Storage.SubFeatures.NAS = true
	cfg.FritzBox.Storage.SubFeatures.FTP = true
	cfg.FritzBox.Storage.SubFeatures.MediaServer = true
	cfg.FritzBox.TV.SubFeatures.ChannelList = true
	cfg.FritzBox.TV.SubFeatures.StreamURLs = true
	cfg.FritzBox.Telephony.Polling.IntervalSeconds = 60
	cfg.FritzBox.Telephony.Polling.DedupWindowMinutes = 5
	cfg.FritzBox.Telephony.Polling.MaxCallbacksPerHour = 20

	runningInDocker := probeDockerContainer()
	cfg.Directories.AgentSkillsDir = "./agent_workspace/agent_skills"

	// Document Creator defaults: Maroto backend, Gotenberg sidecar URL.
	// Use Docker-internal hostname when running inside a Docker container, otherwise localhost.
	cfg.Tools.MediaConversion.ReadOnly = false
	cfg.Tools.MediaConversion.TimeoutSeconds = 120
	cfg.Tools.MediaConversion.MaxFileSizeMB = 500
	cfg.Tools.VideoDownload.Enabled = true
	cfg.Tools.VideoDownload.AllowDownload = false
	cfg.Tools.VideoDownload.AllowTranscribe = false
	cfg.Tools.VideoDownload.Mode = "docker"
	cfg.Tools.VideoDownload.DownloadDir = "data/downloads"
	cfg.Tools.VideoDownload.MaxFileSizeMB = 500
	cfg.Tools.VideoDownload.TimeoutSeconds = 300
	cfg.Tools.VideoDownload.DefaultFormat = "best"
	cfg.Tools.VideoDownload.MaxSearchResults = 10
	cfg.Tools.VideoDownload.ContainerImage = "ghcr.io/jauderho/yt-dlp:latest"
	cfg.Tools.VideoDownload.AutoPull = true
	cfg.Tools.SendYouTubeVideo.Enabled = true
	cfg.Tools.DocumentCreator.Backend = "maroto"
	cfg.Tools.DocumentCreator.OutputDir = "data/documents"
	cfg.Tools.DocumentCreator.Gotenberg.URL = defaultSidecarURL(runningInDocker, "gotenberg", 3000)
	cfg.Tools.DocumentCreator.Gotenberg.Timeout = 120

	// Browser Automation defaults: disabled, Docker sidecar, local loopback outside Docker.
	cfg.BrowserAutomation.Mode = "sidecar"
	cfg.BrowserAutomation.URL = defaultSidecarURL(runningInDocker, "browser-automation", 7331)
	cfg.BrowserAutomation.ContainerName = "aurago_browser_automation"
	cfg.BrowserAutomation.Image = "aurago-browser-automation:latest"
	cfg.BrowserAutomation.AutoStart = true
	cfg.BrowserAutomation.AutoBuild = true
	cfg.BrowserAutomation.DockerfileDir = "."
	cfg.BrowserAutomation.SessionTTLMinutes = 30
	cfg.BrowserAutomation.MaxSessions = 3
	cfg.BrowserAutomation.AllowFileUploads = true
	cfg.BrowserAutomation.AllowFileDownloads = true
	cfg.BrowserAutomation.AllowedDownloadDir = "browser_downloads"
	cfg.BrowserAutomation.Viewport.Width = 1280
	cfg.BrowserAutomation.Viewport.Height = 720
	cfg.BrowserAutomation.Headless = true
	cfg.BrowserAutomation.ReadOnly = false
	cfg.BrowserAutomation.ScreenshotsDir = "browser_screenshots"
	cfg.BrowserAutomation.CloakHumanPreset = "default"

	// Space Agent defaults: disabled by default, managed Docker sidecar when enabled.
	cfg.SpaceAgent.AutoStart = true
	cfg.SpaceAgent.RepoURL = "https://github.com/agent0ai/space-agent"
	cfg.SpaceAgent.GitRef = "main"
	cfg.SpaceAgent.ContainerName = "aurago_space_agent"
	cfg.SpaceAgent.Image = "aurago-space-agent:main"
	cfg.SpaceAgent.Host = "0.0.0.0"
	cfg.SpaceAgent.Port = 3100
	cfg.SpaceAgent.HTTPSEnabled = true
	cfg.SpaceAgent.HTTPSPort = 3101
	cfg.SpaceAgent.CustomwarePath = "data/sidecars/space-agent/customware"
	cfg.SpaceAgent.DataPath = "data/sidecars/space-agent/data"
	cfg.SpaceAgent.AdminUser = "admin"
	cfg.SpaceAgent.PublicURL = ""

	// Manifest defaults: disabled by default, managed Docker gateway when enabled.
	cfg.Manifest.AutoStart = true
	cfg.Manifest.Mode = "managed"
	cfg.Manifest.URL = defaultSidecarURL(runningInDocker, "manifest", 2099)
	cfg.Manifest.ExternalBaseURL = "https://app.manifest.build/v1"
	cfg.Manifest.ContainerName = "aurago_manifest"
	cfg.Manifest.Image = "manifestdotbuild/manifest:5"
	cfg.Manifest.Host = "127.0.0.1"
	cfg.Manifest.Port = 2099
	cfg.Manifest.HostPort = 2099
	cfg.Manifest.NetworkName = "aurago_manifest"
	cfg.Manifest.PostgresContainerName = "aurago_manifest_postgres"
	cfg.Manifest.PostgresImage = "postgres:15-alpine"
	cfg.Manifest.PostgresUser = "manifest"
	cfg.Manifest.PostgresDatabase = "manifest"
	cfg.Manifest.PostgresVolume = "aurago_manifest_pgdata"
	cfg.Manifest.Routing.SpecificityMode = "off"
	cfg.Manifest.Routing.Headers = map[string]string{}

	// OmniRoute defaults: disabled by default, managed Docker gateway when enabled.
	cfg.OmniRoute.AutoStart = true
	cfg.OmniRoute.Mode = "managed"
	cfg.OmniRoute.URL = defaultSidecarURL(runningInDocker, "omniroute", 20128)
	cfg.OmniRoute.ExternalBaseURL = "http://127.0.0.1:20128/v1"
	cfg.OmniRoute.ContainerName = "aurago_omniroute"
	cfg.OmniRoute.Image = "diegosouzapw/omniroute:3.8.39"
	cfg.OmniRoute.Host = "127.0.0.1"
	cfg.OmniRoute.Port = 20128
	cfg.OmniRoute.HostPort = 20128
	cfg.OmniRoute.NetworkName = "aurago_omniroute"
	cfg.OmniRoute.DataVolume = "aurago_omniroute_data"
	cfg.OmniRoute.HealthPath = "/api/monitoring/health"
	cfg.OmniRoute.MemoryMB = 512

	// Dograh defaults: disabled by default, managed Docker stack when enabled.
	cfg.Dograh.AutoStart = true
	cfg.Dograh.Mode = "managed"
	cfg.Dograh.ReadOnly = true
	cfg.Dograh.AllowTestCalls = false
	cfg.Dograh.APIURL = defaultSidecarURL(runningInDocker, "dograh-api", 8000)
	cfg.Dograh.UIURL = defaultSidecarURL(runningInDocker, "dograh-ui", 3010)
	cfg.Dograh.Host = "127.0.0.1"
	cfg.Dograh.APIPort = 8000
	cfg.Dograh.APIHostPort = 8000
	cfg.Dograh.UIPort = 3010
	cfg.Dograh.UIHostPort = 3010
	cfg.Dograh.APIContainerName = "aurago_dograh_api"
	cfg.Dograh.UIContainerName = "aurago_dograh_ui"
	cfg.Dograh.PostgresContainerName = "aurago_dograh_postgres"
	cfg.Dograh.RedisContainerName = "aurago_dograh_redis"
	cfg.Dograh.MinioContainerName = "aurago_dograh_minio"
	cfg.Dograh.CoturnContainerName = "aurago_dograh_coturn"
	cfg.Dograh.NetworkName = "aurago_dograh"
	cfg.Dograh.APIImage = dograhDefaultAPIImage
	cfg.Dograh.UIImage = dograhDefaultUIImage
	cfg.Dograh.PostgresImage = "pgvector/pgvector:pg17"
	cfg.Dograh.RedisImage = "redis:7"
	cfg.Dograh.MinioImage = "minio/minio:latest"
	cfg.Dograh.CoturnImage = "coturn/coturn:4.8.0"
	cfg.Dograh.PostgresUser = "postgres"
	cfg.Dograh.PostgresDatabase = "postgres"
	cfg.Dograh.PostgresVolume = "aurago_dograh_pgdata"
	cfg.Dograh.RedisVolume = "aurago_dograh_redisdata"
	cfg.Dograh.MinioVolume = "aurago_dograh_minio"
	cfg.Dograh.MinioRootUser = "minioadmin"
	cfg.Dograh.MinioBucket = "dograh"
	cfg.Dograh.TelemetryEnabled = false
	cfg.Dograh.TurnEnabled = false
	cfg.Dograh.HealthPath = "/api/v1/health"
	cfg.Dograh.MCPClientEnabled = true
	cfg.Dograh.MCPServerToolEnabled = true
	cfg.Dograh.AuraGoMCPToolName = "AuraGo"
	cfg.Dograh.CallbackWebhookEnabled = true
	cfg.Dograh.CallbackWebhookSlug = "dograh-callback"

	// Virtual Desktop defaults: disabled by default, first-party browser desktop
	// with a project-local persistent workspace when explicitly enabled.
	cfg.VirtualDesktop.WorkspaceDir = "agent_workspace/virtual_desktop"
	cfg.VirtualDesktop.MaxFileSizeMB = 50
	cfg.VirtualDesktop.ControlLevel = "confirm_destructive"
	cfg.VirtualDesktop.MaxWSClients = 8
	cfg.VirtualDesktop.RemoteMaxSessionMinutes = 60
	cfg.VirtualDesktop.RemoteIdleTimeoutMinutes = 5
	cfg.VirtualDesktop.AllowGeneratedApps = true
	cfg.VirtualDesktop.CodeStudio.Enabled = true
	cfg.VirtualDesktop.CodeStudio.Image = "ghcr.io/antibyte/aurago-code-studio:latest"
	cfg.VirtualDesktop.CodeStudio.AutoStart = false
	cfg.VirtualDesktop.CodeStudio.AutoStopMinutes = 30
	cfg.VirtualDesktop.CodeStudio.MaxMemoryMB = 4096
	cfg.VirtualDesktop.CodeStudio.MaxCPUCores = 2
	cfg.VirtualDesktop.OpenSCAD.Enabled = true
	cfg.VirtualDesktop.OpenSCAD.Image = "openscad/openscad:latest"
	cfg.VirtualDesktop.OpenSCAD.AutoStart = false
	cfg.VirtualDesktop.OpenSCAD.AutoStopMinutes = 20
	cfg.VirtualDesktop.OpenSCAD.MaxMemoryMB = 2048
	cfg.VirtualDesktop.OpenSCAD.MaxCPUCores = 2
	cfg.VirtualDesktop.OpenSCAD.MaxConcurrentJobs = 1
	cfg.VirtualDesktop.OpenSCAD.GeometryBackend = "auto"
	cfg.VirtualDesktop.OpenSCAD.DefaultExports = []string{"png", "stl"}
	cfg.VirtualDesktop.OpenSCAD.MaxSourceKB = 512
	cfg.VirtualDesktop.OpenSCAD.MaxOutputMB = 100
	cfg.VirtualDesktop.OpenSCAD.RenderTimeoutSeconds = 120
	cfg.VirtualDesktop.OpenSCAD.MaxRenderTimeoutSeconds = 600
	cfg.VirtualDesktop.OpenSCAD.JobRetentionDays = 7

	cfg.VirtualComputers.Provider = "boring_computers"
	cfg.VirtualComputers.ControlPlane.Mode = "ssh_host"
	cfg.VirtualComputers.ControlPlane.SSHPort = 22
	cfg.VirtualComputers.ControlPlane.InstallDir = "/opt/boring-computers"
	cfg.VirtualComputers.ControlPlane.BoringdURL = DefaultVirtualComputersBoringdURL
	cfg.VirtualComputers.Storage.Bucket = "boring-volumes"
	cfg.VirtualComputers.Storage.UseSSL = true
	cfg.VirtualComputers.DefaultTemplate = "python"
	cfg.VirtualComputers.DefaultTTLSeconds = 600
	cfg.VirtualComputers.MaxTTLSeconds = 900
	cfg.VirtualComputers.MaxRunningMachines = 3
	cfg.VirtualComputers.MaxForks = 3

	cfg.Tools.PythonTimeoutSeconds = 30
	cfg.Tools.SkillTimeoutSeconds = 120
	cfg.Tools.BackgroundTimeoutSeconds = 3600
	cfg.Tools.SkillManager.Enabled = true
	cfg.Tools.SkillManager.AllowUploads = true
	cfg.Tools.SkillManager.RequireScan = true
	cfg.Tools.SkillManager.RequireSandbox = false
	cfg.Tools.SkillManager.MaxUploadSizeMB = 1
	cfg.Tools.SkillManager.AllowedScriptLanguages = []string{"python"}
	cfg.Tools.SkillManager.AllowBinaryAssets = true
	cfg.Tools.SkillManager.SkillSpector.CommandPath = "skillspector"
	cfg.Tools.SkillManager.SkillSpector.TimeoutSeconds = 60
	cfg.Tools.SkillManager.SkillSpector.MaxOutputKB = 512
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
	cfg.Consolidation.ChatSessionLimit = 10
	cfg.Consolidation.StmRetentionMessages = 500

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
	cfg.LLMGuardian.FailSafe = "block"
	cfg.LLMGuardian.CacheTTL = 300
	cfg.LLMGuardian.MaxChecksPerMin = 60
	cfg.LLMGuardian.AllowClarification = false
	cfg.Embeddings.LocalOllama.Model = "nomic-embed-text"
	cfg.Embeddings.LocalOllama.ContainerPort = 11435
	cfg.Embeddings.LocalOllama.GPUBackend = "auto"
	cfg.Embeddings.MultimodalFormat = "auto"
	cfg.Embeddings.Local.Backend = "auto"
	cfg.Embeddings.Local.ContextSize = 2048
	cfg.Embeddings.Local.BatchSize = 2048

	// Local TTS provider defaults
	cfg.TTS.CacheRetentionHours = 168
	cfg.TTS.CacheMaxFiles = 500
	cfg.TTS.Piper.ContainerPort = 10200
	cfg.TTS.Piper.DataPath = "data/piper"
	cfg.TTS.Piper.Image = "rhasspy/wyoming-piper:latest"
	cfg.TTS.Piper.Voice = "de_DE-thorsten-high"
	cfg.TTS.Supertonic.URL = "http://127.0.0.1:7788"
	cfg.TTS.Supertonic.ContainerName = "aurago-supertonic-tts"
	cfg.TTS.Supertonic.Image = "ghcr.io/antibyte/aurago-supertonic:latest"
	cfg.TTS.Supertonic.ContainerPort = 7788
	cfg.TTS.Supertonic.DataPath = "data/supertonic"
	cfg.TTS.Supertonic.Model = "supertonic-3"
	cfg.TTS.Supertonic.Voice = "M1"
	cfg.TTS.Supertonic.Speed = 1.0
	cfg.TTS.Supertonic.Steps = 8
	cfg.TTS.Supertonic.ResponseFormat = "wav"

	// Music Generation defaults
	// (Provider must be configured via Provider Management — no defaults for API keys)

	// Video Generation defaults
	cfg.VideoGeneration.DefaultDurationSeconds = 6
	cfg.VideoGeneration.DefaultResolution = "768P"
	cfg.VideoGeneration.DefaultAspectRatio = "16:9"
	cfg.VideoGeneration.PollIntervalSeconds = 10
	cfg.VideoGeneration.TimeoutSeconds = 600

	cfg.LLMGuardian.TimeoutSecs = 30
	cfg.LLMGuardian.ScanDocuments = false
	cfg.LLMGuardian.ScanEmails = false
	cfg.Guardian.MaxScanBytes = 16 * 1024
	cfg.Guardian.ScanEdgeBytes = 6 * 1024
	cfg.Guardian.PromptSec.Preset = "strict"
	cfg.Guardian.PromptSec.Spotlight = true
	cfg.Guardian.PromptSec.Canary = true
	cfg.Guardian.PromptSec.Sanitizer.Normalize = true
	cfg.Guardian.PromptSec.Sanitizer.Dehomoglyph = true
	cfg.Guardian.PromptSec.Sanitizer.Decode = true
	cfg.Guardian.PromptSec.Embedding.Enabled = false
	cfg.Guardian.PromptSec.Embedding.Threshold = 0.65
	cfg.Guardian.PromptSec.Policy = ""
	cfg.Guardian.PromptSec.Taint.Enabled = false
	cfg.Guardian.PromptSec.Taint.DefaultLevel = "untrusted"
	cfg.Guardian.PromptSec.Structure.Enabled = false
	cfg.Guardian.PromptSec.Structure.Mode = "sandwich"
	cfg.Guardian.PromptSec.LLMJudge.Enabled = false
	cfg.Guardian.PromptSec.LLMJudge.Mode = "uncertain"
	cfg.Guardian.PromptSec.LLMJudge.TimeoutSecs = 2
	cfg.Guardian.PromptSec.UseSanitizedOutput = false

	// Task rules are core prompt guardrails and are enabled by default.
	cfg.Rules.Enabled = true

	// OneDrive defaults: "common" tenant allows both personal and work accounts.
	cfg.OneDrive.TenantID = "common"

	// Uptime Kuma defaults: disabled until explicitly configured.
	cfg.UptimeKuma.RequestTimeout = 15
	cfg.UptimeKuma.PollIntervalSeconds = 30
	cfg.UptimeKuma.RelayInstruction = ""

	// AgentMail defaults: disabled until explicitly configured.
	cfg.AgentMail.BaseURL = "https://api.agentmail.to"
	cfg.AgentMail.WebSocketURL = "wss://ws.agentmail.to/v0"
	cfg.AgentMail.UseWebSocket = true
	cfg.AgentMail.PollIntervalSeconds = 120
	cfg.AgentMail.MaxAttachmentMB = 10

	// Grafana defaults: read-only observability access when explicitly enabled.
	cfg.Grafana.RequestTimeout = 15
	cfg.Grafana.ReadOnly = true

	// Frigate defaults: disabled by default, read-only when enabled.
	cfg.Frigate.Enabled = false
	cfg.Frigate.ReadOnly = true
	cfg.Frigate.InternalPort = false
	cfg.Frigate.EventRelay = false
	cfg.Frigate.ReviewRelay = false
	cfg.Frigate.StoreMedia = true
	cfg.Frigate.MQTTTopicPrefix = "frigate"

	// 3D printer defaults: disabled by default, read-only when enabled.
	cfg.ThreeDPrinters.Enabled = false
	cfg.ThreeDPrinters.ReadOnly = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Enabled = false
	cfg.ThreeDPrinters.Klipper.Enabled = false

	// LDAP defaults: disabled by default, read-only when enabled.
	cfg.LDAP.Enabled = false
	cfg.LDAP.ReadOnly = true
	cfg.LDAP.UseTLS = true

	// YepAPI defaults: disabled by default, all services opt-in.
	cfg.YepAPI.Enabled = false
	cfg.YepAPI.BaseURL = "https://api.yepapi.com"
	cfg.YepAPI.SEO.Enabled = false
	cfg.YepAPI.SERP.Enabled = false
	cfg.YepAPI.Scraping.Enabled = false
	cfg.YepAPI.YouTube.Enabled = false
	cfg.YepAPI.TikTok.Enabled = false
	cfg.YepAPI.Instagram.Enabled = false
	cfg.YepAPI.Amazon.Enabled = false
	cfg.LDAP.Port = 636
	cfg.LDAP.ConnectTimeout = 10
	cfg.LDAP.RequestTimeout = 30

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

	// Composio defaults: disabled by default, read-only once enabled.
	cfg.Composio.BaseURL = "https://backend.composio.dev/api/v3.1"
	cfg.Composio.UserID = "aurago-default"
	cfg.Composio.ReadOnly = true
	cfg.Composio.AllowDestructive = false
	cfg.Composio.AllowNaturalLanguageInput = false
	cfg.Composio.RequestTimeoutSeconds = 60
	cfg.Composio.CacheTTLSeconds = 300
	cfg.Composio.MaxResultBytes = 262144

	// Manus defaults: disabled, read-only, and deny all mutating operations.
	cfg.Manus.Enabled = false
	cfg.Manus.ReadOnly = true
	cfg.Manus.AllowCreateTasks = false
	cfg.Manus.AllowSendMessages = false
	cfg.Manus.AllowStopTasks = false
	cfg.Manus.AllowFileUploads = false
	cfg.Manus.AllowFileDownloads = false
	cfg.Manus.DefaultAgentProfile = "manus-1.6"
	cfg.Manus.RequestTimeoutSeconds = 60
	cfg.Manus.PollIntervalSeconds = 5
	cfg.Manus.MaxWaitSeconds = 60
	cfg.Manus.MaxResultBytes = 262144
	cfg.Manus.MaxFileSizeMB = 20

	// Hugging Face defaults: disabled, read-only, and CPU-only unless explicitly expanded.
	cfg.HuggingFace.Enabled = false
	cfg.HuggingFace.ReadOnly = true
	cfg.HuggingFace.AllowWrites = false
	cfg.HuggingFace.AllowDelete = false
	cfg.HuggingFace.AllowJobs = false
	cfg.HuggingFace.AllowScheduledJobs = false
	cfg.HuggingFace.AllowedHardware = []string{"cpu-basic"}
	cfg.HuggingFace.MaxDownloadMB = 512
	cfg.HuggingFace.MaxUploadMB = 512
	cfg.HuggingFace.MaxDatasetRows = 100
	cfg.HuggingFace.JobDefaultTimeoutMinutes = 30
	cfg.HuggingFace.JobMaxRuntimeMinutes = 120
	cfg.HuggingFace.RequestTimeoutSeconds = 60
	cfg.HuggingFace.MaxResultBytes = 524288
	cfg.HuggingFace.HubBaseURL = "https://huggingface.co"
	cfg.HuggingFace.DatasetBaseURL = "https://datasets-server.huggingface.co"
	cfg.HuggingFace.JobsBaseURL = "https://huggingface.co/api/jobs"
	cfg.HuggingFace.RouterBaseURL = "https://router.huggingface.co/v1"

	// EvoMap defaults: disabled by default and read-only once enabled.
	cfg.Evomap.Enabled = false
	cfg.Evomap.ReadOnly = true
	cfg.Evomap.BaseURL = "https://evomap.ai"
	cfg.Evomap.RequestTimeoutSeconds = 30
	cfg.Evomap.MaxResultBytes = 262144
	cfg.Evomap.KGEnabled = false
	cfg.Evomap.AllowPublish = false
	cfg.Evomap.AllowReport = false
	cfg.Evomap.AllowBounties = false

	// Remote control defaults: disabled by default, read-only for newly enrolled
	// devices unless mutable operations are explicitly enabled, and audit logging stays on.
	cfg.RemoteControl.ConnectionMode = "auto"
	cfg.RemoteControl.ReadOnly = true
	cfg.RemoteControl.DiscoveryPort = 8092
	cfg.RemoteControl.MaxFileSizeMB = 50
	cfg.RemoteControl.AuditLog = true

	// Package manager defaults: disabled by default and mutable only when the
	// danger-zone gate and package-manager section are both explicitly enabled.
	cfg.PackageManager.Enabled = false
	cfg.PackageManager.ReadOnly = false
	cfg.PackageManager.AutoDetect = true
	cfg.PackageManager.AllowInstall = true
	cfg.PackageManager.AllowRemove = true
	cfg.PackageManager.AllowUpgrade = true

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
	normalizeDeprecatedEmbeddingBackend(&cfg)

	NormalizeAIGatewayConfig(&cfg)
	NormalizeCloudflareTunnelConfig(&cfg)
	NormalizeRealtimeSpeechConfig(&cfg.RealtimeSpeech)
	if err := ValidateRealtimeSpeechConfig(cfg.RealtimeSpeech); err != nil {
		return nil, err
	}

	switch strings.ToLower(strings.TrimSpace(cfg.WebDAV.AuthType)) {
	case "", "basic":
		cfg.WebDAV.AuthType = "basic"
	case "bearer":
		cfg.WebDAV.AuthType = "bearer"
	default:
		cfg.WebDAV.AuthType = "basic"
	}
	if strings.TrimSpace(cfg.Composio.BaseURL) == "" {
		cfg.Composio.BaseURL = "https://backend.composio.dev/api/v3.1"
	}
	cfg.Composio.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.Composio.BaseURL), "/")
	if strings.TrimSpace(cfg.Composio.UserID) == "" {
		cfg.Composio.UserID = "aurago-default"
	}
	if cfg.Composio.RequestTimeoutSeconds <= 0 {
		cfg.Composio.RequestTimeoutSeconds = 60
	}
	if cfg.Composio.CacheTTLSeconds <= 0 {
		cfg.Composio.CacheTTLSeconds = 300
	}
	if cfg.Composio.MaxResultBytes <= 0 {
		cfg.Composio.MaxResultBytes = 262144
	}
	if strings.TrimSpace(cfg.HuggingFace.HubBaseURL) == "" {
		cfg.HuggingFace.HubBaseURL = "https://huggingface.co"
	}
	cfg.HuggingFace.HubBaseURL = strings.TrimRight(strings.TrimSpace(cfg.HuggingFace.HubBaseURL), "/")
	if strings.TrimSpace(cfg.HuggingFace.DatasetBaseURL) == "" {
		cfg.HuggingFace.DatasetBaseURL = "https://datasets-server.huggingface.co"
	}
	cfg.HuggingFace.DatasetBaseURL = strings.TrimRight(strings.TrimSpace(cfg.HuggingFace.DatasetBaseURL), "/")
	if strings.TrimSpace(cfg.HuggingFace.JobsBaseURL) == "" {
		cfg.HuggingFace.JobsBaseURL = "https://huggingface.co/api/jobs"
	}
	cfg.HuggingFace.JobsBaseURL = strings.TrimRight(strings.TrimSpace(cfg.HuggingFace.JobsBaseURL), "/")
	if strings.TrimSpace(cfg.HuggingFace.RouterBaseURL) == "" {
		cfg.HuggingFace.RouterBaseURL = "https://router.huggingface.co/v1"
	}
	cfg.HuggingFace.RouterBaseURL = strings.TrimRight(strings.TrimSpace(cfg.HuggingFace.RouterBaseURL), "/")
	if cfg.HuggingFace.MaxDownloadMB <= 0 {
		cfg.HuggingFace.MaxDownloadMB = 512
	}
	if cfg.HuggingFace.MaxUploadMB <= 0 {
		cfg.HuggingFace.MaxUploadMB = 512
	}
	if cfg.HuggingFace.MaxDatasetRows <= 0 {
		cfg.HuggingFace.MaxDatasetRows = 100
	}
	if cfg.HuggingFace.JobDefaultTimeoutMinutes <= 0 {
		cfg.HuggingFace.JobDefaultTimeoutMinutes = 30
	}
	if cfg.HuggingFace.JobMaxRuntimeMinutes <= 0 {
		cfg.HuggingFace.JobMaxRuntimeMinutes = 120
	}
	if cfg.HuggingFace.RequestTimeoutSeconds <= 0 {
		cfg.HuggingFace.RequestTimeoutSeconds = 60
	}
	if cfg.HuggingFace.MaxResultBytes <= 0 {
		cfg.HuggingFace.MaxResultBytes = 524288
	}
	if len(cfg.HuggingFace.AllowedHardware) == 0 {
		cfg.HuggingFace.AllowedHardware = []string{"cpu-basic"}
	}
	if strings.TrimSpace(cfg.Evomap.BaseURL) == "" {
		cfg.Evomap.BaseURL = "https://evomap.ai"
	}
	cfg.Evomap.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.Evomap.BaseURL), "/")
	if cfg.Evomap.RequestTimeoutSeconds <= 0 {
		cfg.Evomap.RequestTimeoutSeconds = 30
	}
	if cfg.Evomap.MaxResultBytes <= 0 {
		cfg.Evomap.MaxResultBytes = 262144
	}
	if !yamlHasPath(data, "llm_guardian", "max_checks_per_minute") && cfg.LLMGuardian.MaxChecksPerMinOld > 0 {
		cfg.LLMGuardian.MaxChecksPerMin = cfg.LLMGuardian.MaxChecksPerMinOld
	}

	cfg.BrowserAutomation.URL = NormalizeLegacySidecarURL(cfg.BrowserAutomation.URL, runningInDocker, "browser-automation", 7331)
	cfg.SpaceAgent.PublicURL, cfg.SpaceAgent.Port = normalizeSpaceAgentURLAndPort(cfg.SpaceAgent.PublicURL, cfg.SpaceAgent.Port, runningInDocker)
	cfg.Manifest.URL = NormalizeLegacySidecarURL(cfg.Manifest.URL, runningInDocker, "manifest", 2099)
	cfg.OmniRoute.URL = NormalizeLegacySidecarURL(cfg.OmniRoute.URL, runningInDocker, "omniroute", 20128)
	cfg.Dograh.APIURL = NormalizeLegacySidecarURL(cfg.Dograh.APIURL, runningInDocker, "dograh-api", 8000)
	cfg.Dograh.UIURL = NormalizeLegacySidecarURL(cfg.Dograh.UIURL, runningInDocker, "dograh-ui", 3010)
	if strings.TrimSpace(cfg.SpaceAgent.RepoURL) == "" {
		cfg.SpaceAgent.RepoURL = "https://github.com/agent0ai/space-agent"
	}
	if strings.TrimSpace(cfg.SpaceAgent.GitRef) == "" {
		cfg.SpaceAgent.GitRef = "main"
	}
	if strings.TrimSpace(cfg.SpaceAgent.ContainerName) == "" {
		cfg.SpaceAgent.ContainerName = "aurago_space_agent"
	}
	if strings.TrimSpace(cfg.SpaceAgent.Image) == "" {
		cfg.SpaceAgent.Image = "aurago-space-agent:" + sanitizeDockerTag(cfg.SpaceAgent.GitRef)
	}
	if strings.TrimSpace(cfg.SpaceAgent.Host) == "" {
		cfg.SpaceAgent.Host = "0.0.0.0"
	}
	if cfg.SpaceAgent.HTTPSPort <= 0 {
		cfg.SpaceAgent.HTTPSPort = 3101
	}
	if strings.TrimSpace(cfg.SpaceAgent.AdminUser) == "" {
		cfg.SpaceAgent.AdminUser = "admin"
	}
	if strings.TrimSpace(cfg.SpaceAgent.CustomwarePath) == "" {
		cfg.SpaceAgent.CustomwarePath = "data/sidecars/space-agent/customware"
	}
	if strings.TrimSpace(cfg.SpaceAgent.DataPath) == "" {
		cfg.SpaceAgent.DataPath = "data/sidecars/space-agent/data"
	}
	if strings.TrimSpace(cfg.Manifest.Mode) == "" {
		cfg.Manifest.Mode = "managed"
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Manifest.Mode)) {
	case "external":
		cfg.Manifest.Mode = "external"
	default:
		cfg.Manifest.Mode = "managed"
	}
	if strings.TrimSpace(cfg.Manifest.URL) == "" {
		cfg.Manifest.URL = defaultSidecarURL(runningInDocker, "manifest", 2099)
	}
	if strings.TrimSpace(cfg.Manifest.ExternalBaseURL) == "" {
		cfg.Manifest.ExternalBaseURL = "https://app.manifest.build/v1"
	}
	if strings.TrimSpace(cfg.Manifest.ContainerName) == "" {
		cfg.Manifest.ContainerName = "aurago_manifest"
	}
	if strings.TrimSpace(cfg.Manifest.Image) == "" {
		cfg.Manifest.Image = "manifestdotbuild/manifest:5"
	}
	if strings.TrimSpace(cfg.Manifest.Host) == "" {
		cfg.Manifest.Host = "127.0.0.1"
	}
	if cfg.Manifest.Port <= 0 {
		cfg.Manifest.Port = 2099
	}
	if cfg.Manifest.HostPort <= 0 {
		cfg.Manifest.HostPort = cfg.Manifest.Port
	}
	if strings.TrimSpace(cfg.Manifest.NetworkName) == "" {
		cfg.Manifest.NetworkName = "aurago_manifest"
	}
	if strings.TrimSpace(cfg.Manifest.PostgresContainerName) == "" {
		cfg.Manifest.PostgresContainerName = "aurago_manifest_postgres"
	}
	if strings.TrimSpace(cfg.Manifest.PostgresImage) == "" {
		cfg.Manifest.PostgresImage = "postgres:15-alpine"
	}
	if strings.TrimSpace(cfg.Manifest.PostgresUser) == "" {
		cfg.Manifest.PostgresUser = "manifest"
	}
	if strings.TrimSpace(cfg.Manifest.PostgresDatabase) == "" {
		cfg.Manifest.PostgresDatabase = "manifest"
	}
	if strings.TrimSpace(cfg.Manifest.PostgresVolume) == "" {
		cfg.Manifest.PostgresVolume = "aurago_manifest_pgdata"
	}
	NormalizeManifestRoutingConfig(&cfg.Manifest.Routing)
	if strings.TrimSpace(cfg.OmniRoute.Mode) == "" {
		cfg.OmniRoute.Mode = "managed"
	}
	switch strings.ToLower(strings.TrimSpace(cfg.OmniRoute.Mode)) {
	case "external":
		cfg.OmniRoute.Mode = "external"
	default:
		cfg.OmniRoute.Mode = "managed"
	}
	if strings.TrimSpace(cfg.OmniRoute.URL) == "" {
		cfg.OmniRoute.URL = defaultSidecarURL(runningInDocker, "omniroute", 20128)
	}
	if strings.TrimSpace(cfg.OmniRoute.ExternalBaseURL) == "" {
		cfg.OmniRoute.ExternalBaseURL = "http://127.0.0.1:20128/v1"
	}
	if strings.TrimSpace(cfg.OmniRoute.ContainerName) == "" {
		cfg.OmniRoute.ContainerName = "aurago_omniroute"
	}
	if strings.TrimSpace(cfg.OmniRoute.Image) == "" {
		cfg.OmniRoute.Image = "diegosouzapw/omniroute:3.8.39"
	}
	if strings.TrimSpace(cfg.OmniRoute.Host) == "" {
		cfg.OmniRoute.Host = "127.0.0.1"
	}
	if cfg.OmniRoute.Port <= 0 {
		cfg.OmniRoute.Port = 20128
	}
	if cfg.OmniRoute.HostPort <= 0 {
		cfg.OmniRoute.HostPort = cfg.OmniRoute.Port
	}
	if strings.TrimSpace(cfg.OmniRoute.NetworkName) == "" {
		cfg.OmniRoute.NetworkName = "aurago_omniroute"
	}
	if strings.TrimSpace(cfg.OmniRoute.DataVolume) == "" {
		cfg.OmniRoute.DataVolume = "aurago_omniroute_data"
	}
	if strings.TrimSpace(cfg.OmniRoute.HealthPath) == "" {
		cfg.OmniRoute.HealthPath = "/api/monitoring/health"
	}
	if cfg.OmniRoute.MemoryMB <= 0 {
		cfg.OmniRoute.MemoryMB = 512
	}
	if strings.TrimSpace(cfg.Dograh.Mode) == "" {
		cfg.Dograh.Mode = "managed"
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Dograh.Mode)) {
	case "external":
		cfg.Dograh.Mode = "external"
	default:
		cfg.Dograh.Mode = "managed"
	}
	if strings.TrimSpace(cfg.Dograh.APIURL) == "" {
		cfg.Dograh.APIURL = defaultSidecarURL(runningInDocker, "dograh-api", 8000)
	}
	if strings.TrimSpace(cfg.Dograh.UIURL) == "" {
		cfg.Dograh.UIURL = defaultSidecarURL(runningInDocker, "dograh-ui", 3010)
	}
	if strings.TrimSpace(cfg.Dograh.Host) == "" {
		cfg.Dograh.Host = "127.0.0.1"
	}
	if cfg.Dograh.APIPort <= 0 {
		cfg.Dograh.APIPort = 8000
	}
	if cfg.Dograh.APIHostPort <= 0 {
		cfg.Dograh.APIHostPort = cfg.Dograh.APIPort
	}
	if cfg.Dograh.UIPort <= 0 {
		cfg.Dograh.UIPort = 3010
	}
	if cfg.Dograh.UIHostPort <= 0 {
		cfg.Dograh.UIHostPort = cfg.Dograh.UIPort
	}
	if strings.TrimSpace(cfg.Dograh.APIContainerName) == "" {
		cfg.Dograh.APIContainerName = "aurago_dograh_api"
	}
	if strings.TrimSpace(cfg.Dograh.UIContainerName) == "" {
		cfg.Dograh.UIContainerName = "aurago_dograh_ui"
	}
	if strings.TrimSpace(cfg.Dograh.PostgresContainerName) == "" {
		cfg.Dograh.PostgresContainerName = "aurago_dograh_postgres"
	}
	if strings.TrimSpace(cfg.Dograh.RedisContainerName) == "" {
		cfg.Dograh.RedisContainerName = "aurago_dograh_redis"
	}
	if strings.TrimSpace(cfg.Dograh.MinioContainerName) == "" {
		cfg.Dograh.MinioContainerName = "aurago_dograh_minio"
	}
	if strings.TrimSpace(cfg.Dograh.CoturnContainerName) == "" {
		cfg.Dograh.CoturnContainerName = "aurago_dograh_coturn"
	}
	if strings.TrimSpace(cfg.Dograh.NetworkName) == "" {
		cfg.Dograh.NetworkName = "aurago_dograh"
	}
	cfg.Dograh.APIImage = normalizeDograhDefaultImage(cfg.Dograh.APIImage, dograhDefaultAPIImage, dograhLegacyDockerHubAPIImage)
	cfg.Dograh.UIImage = normalizeDograhDefaultImage(cfg.Dograh.UIImage, dograhDefaultUIImage, dograhLegacyDockerHubUIImage)
	if strings.TrimSpace(cfg.Dograh.PostgresImage) == "" {
		cfg.Dograh.PostgresImage = "pgvector/pgvector:pg17"
	}
	if strings.TrimSpace(cfg.Dograh.RedisImage) == "" {
		cfg.Dograh.RedisImage = "redis:7"
	}
	if strings.TrimSpace(cfg.Dograh.MinioImage) == "" {
		cfg.Dograh.MinioImage = "minio/minio:latest"
	}
	if strings.TrimSpace(cfg.Dograh.CoturnImage) == "" {
		cfg.Dograh.CoturnImage = "coturn/coturn:4.8.0"
	}
	if strings.TrimSpace(cfg.Dograh.PostgresUser) == "" {
		cfg.Dograh.PostgresUser = "postgres"
	}
	if strings.TrimSpace(cfg.Dograh.PostgresDatabase) == "" {
		cfg.Dograh.PostgresDatabase = "postgres"
	}
	if strings.TrimSpace(cfg.Dograh.PostgresVolume) == "" {
		cfg.Dograh.PostgresVolume = "aurago_dograh_pgdata"
	}
	if strings.TrimSpace(cfg.Dograh.RedisVolume) == "" {
		cfg.Dograh.RedisVolume = "aurago_dograh_redisdata"
	}
	if strings.TrimSpace(cfg.Dograh.MinioVolume) == "" {
		cfg.Dograh.MinioVolume = "aurago_dograh_minio"
	}
	if strings.TrimSpace(cfg.Dograh.MinioRootUser) == "" {
		cfg.Dograh.MinioRootUser = "minioadmin"
	}
	if strings.TrimSpace(cfg.Dograh.MinioBucket) == "" {
		cfg.Dograh.MinioBucket = "dograh"
	}
	if strings.TrimSpace(cfg.Dograh.HealthPath) == "" {
		cfg.Dograh.HealthPath = "/api/v1/health"
	}
	if strings.TrimSpace(cfg.Dograh.AuraGoMCPToolName) == "" {
		cfg.Dograh.AuraGoMCPToolName = "AuraGo"
	}
	if strings.TrimSpace(cfg.Dograh.CallbackWebhookSlug) == "" {
		cfg.Dograh.CallbackWebhookSlug = "dograh-callback"
	}
	if strings.TrimSpace(cfg.VirtualDesktop.WorkspaceDir) == "" {
		cfg.VirtualDesktop.WorkspaceDir = "agent_workspace/virtual_desktop"
	}
	if cfg.VirtualDesktop.MaxFileSizeMB <= 0 {
		cfg.VirtualDesktop.MaxFileSizeMB = 50
	}
	if strings.TrimSpace(cfg.VirtualDesktop.ControlLevel) == "" {
		cfg.VirtualDesktop.ControlLevel = "confirm_destructive"
	}
	if cfg.VirtualDesktop.MaxWSClients <= 0 {
		cfg.VirtualDesktop.MaxWSClients = 8
	}
	if cfg.VirtualDesktop.RemoteMaxSessionMinutes <= 0 {
		cfg.VirtualDesktop.RemoteMaxSessionMinutes = 60
	}
	if cfg.VirtualDesktop.RemoteIdleTimeoutMinutes <= 0 {
		cfg.VirtualDesktop.RemoteIdleTimeoutMinutes = 5
	}
	if strings.TrimSpace(cfg.VirtualDesktop.CodeStudio.Image) == "" {
		cfg.VirtualDesktop.CodeStudio.Image = "ghcr.io/antibyte/aurago-code-studio:latest"
	}
	if strings.EqualFold(strings.TrimSpace(cfg.VirtualDesktop.CodeStudio.Image), "aurago/code-studio:latest") {
		cfg.VirtualDesktop.CodeStudio.Image = "ghcr.io/antibyte/aurago-code-studio:latest"
	}
	if cfg.VirtualDesktop.CodeStudio.AutoStopMinutes <= 0 {
		cfg.VirtualDesktop.CodeStudio.AutoStopMinutes = 30
	}
	if cfg.VirtualDesktop.CodeStudio.MaxMemoryMB <= 0 {
		cfg.VirtualDesktop.CodeStudio.MaxMemoryMB = 4096
	}
	if cfg.VirtualDesktop.CodeStudio.MaxCPUCores <= 0 {
		cfg.VirtualDesktop.CodeStudio.MaxCPUCores = 2
	}
	if strings.TrimSpace(cfg.VirtualDesktop.OpenSCAD.Image) == "" {
		cfg.VirtualDesktop.OpenSCAD.Image = "openscad/openscad:latest"
	}
	if cfg.VirtualDesktop.OpenSCAD.AutoStopMinutes <= 0 {
		cfg.VirtualDesktop.OpenSCAD.AutoStopMinutes = 20
	}
	if cfg.VirtualDesktop.OpenSCAD.MaxMemoryMB <= 0 {
		cfg.VirtualDesktop.OpenSCAD.MaxMemoryMB = 2048
	}
	if cfg.VirtualDesktop.OpenSCAD.MaxCPUCores <= 0 {
		cfg.VirtualDesktop.OpenSCAD.MaxCPUCores = 2
	}
	if cfg.VirtualDesktop.OpenSCAD.MaxConcurrentJobs <= 0 {
		cfg.VirtualDesktop.OpenSCAD.MaxConcurrentJobs = 1
	}
	cfg.VirtualDesktop.OpenSCAD.GeometryBackend = normalizeOpenSCADGeometryBackend(cfg.VirtualDesktop.OpenSCAD.GeometryBackend)
	if len(cfg.VirtualDesktop.OpenSCAD.DefaultExports) == 0 {
		cfg.VirtualDesktop.OpenSCAD.DefaultExports = []string{"png", "stl"}
	}
	if cfg.VirtualDesktop.OpenSCAD.MaxSourceKB <= 0 {
		cfg.VirtualDesktop.OpenSCAD.MaxSourceKB = 512
	}
	if cfg.VirtualDesktop.OpenSCAD.MaxOutputMB <= 0 {
		cfg.VirtualDesktop.OpenSCAD.MaxOutputMB = 100
	}
	if cfg.VirtualDesktop.OpenSCAD.RenderTimeoutSeconds <= 0 {
		cfg.VirtualDesktop.OpenSCAD.RenderTimeoutSeconds = 120
	}
	if cfg.VirtualDesktop.OpenSCAD.MaxRenderTimeoutSeconds <= 0 {
		cfg.VirtualDesktop.OpenSCAD.MaxRenderTimeoutSeconds = 600
	}
	if cfg.VirtualDesktop.OpenSCAD.JobRetentionDays <= 0 {
		cfg.VirtualDesktop.OpenSCAD.JobRetentionDays = 7
	}
	if strings.TrimSpace(cfg.VirtualComputers.Provider) == "" {
		cfg.VirtualComputers.Provider = "boring_computers"
	}
	if strings.TrimSpace(cfg.VirtualComputers.ControlPlane.Mode) == "" {
		cfg.VirtualComputers.ControlPlane.Mode = "ssh_host"
	}
	if cfg.VirtualComputers.ControlPlane.SSHPort <= 0 {
		cfg.VirtualComputers.ControlPlane.SSHPort = 22
	}
	if strings.TrimSpace(cfg.VirtualComputers.ControlPlane.InstallDir) == "" {
		cfg.VirtualComputers.ControlPlane.InstallDir = "/opt/boring-computers"
	}
	cfg.VirtualComputers.ControlPlane.BoringdURL = normalizeVirtualComputersBoringdURL(cfg.VirtualComputers.ControlPlane.BoringdURL)
	if strings.TrimSpace(cfg.VirtualComputers.Storage.Bucket) == "" {
		cfg.VirtualComputers.Storage.Bucket = "boring-volumes"
	}
	if strings.TrimSpace(cfg.VirtualComputers.DefaultTemplate) == "" {
		cfg.VirtualComputers.DefaultTemplate = "python"
	}
	if cfg.VirtualComputers.DefaultTTLSeconds <= 0 {
		cfg.VirtualComputers.DefaultTTLSeconds = 600
	}
	if cfg.VirtualComputers.DefaultTTLSeconds < 15 {
		cfg.VirtualComputers.DefaultTTLSeconds = 15
	}
	if cfg.VirtualComputers.MaxTTLSeconds <= 0 || cfg.VirtualComputers.MaxTTLSeconds > 900 {
		cfg.VirtualComputers.MaxTTLSeconds = 900
	}
	if cfg.VirtualComputers.DefaultTTLSeconds > cfg.VirtualComputers.MaxTTLSeconds {
		cfg.VirtualComputers.DefaultTTLSeconds = cfg.VirtualComputers.MaxTTLSeconds
	}
	if cfg.VirtualComputers.MaxRunningMachines <= 0 {
		cfg.VirtualComputers.MaxRunningMachines = 3
	}
	if cfg.VirtualComputers.MaxForks <= 0 {
		cfg.VirtualComputers.MaxForks = 3
	}
	cfg.Directories.WorkspaceDir = normalizeDockerWorkspaceDir(configDir, cfg.Directories.WorkspaceDir, runningInDocker)
	if strings.TrimSpace(cfg.Docker.Host) == "" {
		cfg.Docker.Host = strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	}

	// Resolve absolute paths for directories
	cfg.Directories.DataDir = resolvePath(configDir, cfg.Directories.DataDir)
	cfg.Directories.WorkspaceDir = resolvePath(configDir, cfg.Directories.WorkspaceDir)
	cfg.Directories.ToolsDir = resolvePath(configDir, cfg.Directories.ToolsDir)
	cfg.Directories.PromptsDir = resolvePath(configDir, cfg.Directories.PromptsDir)
	cfg.Directories.SkillsDir = resolvePath(configDir, cfg.Directories.SkillsDir)
	cfg.Directories.AgentSkillsDir = resolvePath(configDir, cfg.Directories.AgentSkillsDir)
	cfg.Directories.VectorDBDir = resolvePath(configDir, cfg.Directories.VectorDBDir)
	cfg.SpaceAgent.CustomwarePath = resolvePath(configDir, cfg.SpaceAgent.CustomwarePath)
	cfg.SpaceAgent.DataPath = resolvePath(configDir, cfg.SpaceAgent.DataPath)
	cfg.VirtualDesktop.WorkspaceDir = resolvePath(configDir, cfg.VirtualDesktop.WorkspaceDir)

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
	if cfg.SQLite.VirtualDesktopPath == "" {
		cfg.SQLite.VirtualDesktopPath = "./data/virtual_desktop.db"
	}
	cfg.SQLite.VirtualDesktopPath = resolvePath(configDir, cfg.SQLite.VirtualDesktopPath)
	if cfg.SQLite.VirtualComputersPath == "" {
		cfg.SQLite.VirtualComputersPath = "./data/virtual_computers.db"
	}
	cfg.SQLite.VirtualComputersPath = resolvePath(configDir, cfg.SQLite.VirtualComputersPath)
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
	if cfg.SQLite.KnowledgeGraphPath == "" {
		cfg.SQLite.KnowledgeGraphPath = "./data/knowledge_graph.db"
	}
	cfg.SQLite.KnowledgeGraphPath = resolvePath(configDir, cfg.SQLite.KnowledgeGraphPath)
	if cfg.SQLite.OptimizationPath == "" {
		cfg.SQLite.OptimizationPath = "./data/optimization.db"
	}
	cfg.SQLite.OptimizationPath = resolvePath(configDir, cfg.SQLite.OptimizationPath)
	if cfg.SQLite.PreparedMissionsPath == "" {
		cfg.SQLite.PreparedMissionsPath = "./data/prepared_missions.db"
	}
	cfg.SQLite.PreparedMissionsPath = resolvePath(configDir, cfg.SQLite.PreparedMissionsPath)
	if cfg.SQLite.MissionHistoryPath == "" {
		cfg.SQLite.MissionHistoryPath = "./data/mission_history.db"
	}
	cfg.SQLite.MissionHistoryPath = resolvePath(configDir, cfg.SQLite.MissionHistoryPath)
	if cfg.SQLite.PushPath == "" {
		cfg.SQLite.PushPath = "./data/push.db"
	}
	cfg.SQLite.PushPath = resolvePath(configDir, cfg.SQLite.PushPath)
	if cfg.SQLite.LaunchpadPath == "" {
		cfg.SQLite.LaunchpadPath = "./data/launchpad.db"
	}
	cfg.SQLite.LaunchpadPath = resolvePath(configDir, cfg.SQLite.LaunchpadPath)

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
	// The canonical tools.web_scraper.enabled value wins when both fields exist.
	if cfg.Agent.AllowWebScraper != nil && !yamlHasPath(data, "tools", "web_scraper", "enabled") {
		cfg.Tools.WebScraper.Enabled = *cfg.Agent.AllowWebScraper
	}

	// Migrate legacy agent.personality_* fields → new personality section.
	cfg.MigrateAgentToPersonality()

	// Fill defaults for expanded guardian.promptsec options.
	cfg.MigratePromptSecDefaults(data)

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
		cfg.CircuitBreaker.LLMPerAttemptTimeoutSeconds = 120
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
	if strings.TrimSpace(cfg.CircuitBreaker.FinalRetryInterval) == "" {
		cfg.CircuitBreaker.FinalRetryInterval = "30s"
	}

	if cfg.Server.Host == "" {
		cfg.Server.Host = "127.0.0.1"
	}
	// DebugPProf defaults to false for security; must be explicitly enabled.
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
	if cfg.Agent.MaxConcurrentLoops <= 0 {
		cfg.Agent.MaxConcurrentLoops = 8
	}
	if !yamlHasPath(data, "maintenance", "enabled") {
		cfg.Maintenance.Enabled = true
	}
	if cfg.Maintenance.Retention.PatternsDays <= 0 {
		cfg.Maintenance.Retention.PatternsDays = 90
	}
	if cfg.Maintenance.Retention.ArchiveEventsDays <= 0 {
		cfg.Maintenance.Retention.ArchiveEventsDays = 90
	}
	if cfg.Maintenance.Retention.MoodLogDays <= 0 {
		cfg.Maintenance.Retention.MoodLogDays = 30
	}
	if cfg.Maintenance.Retention.ErrorPatternsDays <= 0 {
		cfg.Maintenance.Retention.ErrorPatternsDays = 7
	}
	if cfg.Maintenance.Retention.ProfileStaleDays <= 0 {
		cfg.Maintenance.Retention.ProfileStaleDays = 30
	}
	if cfg.Maintenance.Retention.DoneNotesDays <= 0 {
		cfg.Maintenance.Retention.DoneNotesDays = 7
	}
	if cfg.Maintenance.Retention.OperationalIssuesDays <= 0 {
		cfg.Maintenance.Retention.OperationalIssuesDays = 30
	}
	if cfg.Agent.MemoryCompressionCharLimit <= 0 {
		cfg.Agent.MemoryCompressionCharLimit = 100000
	}
	if cfg.Personality.CorePersonality == "" {
		cfg.Personality.CorePersonality = "neutral"
	}
	if cfg.Agent.CoreMemoryMaxEntries <= 0 {
		cfg.Agent.CoreMemoryMaxEntries = 80
	}
	if cfg.Agent.CoreMemoryCapMode == "" {
		cfg.Agent.CoreMemoryCapMode = "hard"
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
	if cfg.Agent.DiscoverToolsSnapshotTTLMinutes <= 0 {
		cfg.Agent.DiscoverToolsSnapshotTTLMinutes = 5
	}
	// Output compression defaults (enabled by default).
	// Uses yamlHasPath to distinguish "not configured" from "explicitly disabled".
	if !yamlHasPath(data, "agent", "output_compression", "enabled") {
		cfg.Agent.OutputCompression.Enabled = true
	}
	if cfg.Agent.OutputCompression.MinChars <= 0 {
		cfg.Agent.OutputCompression.MinChars = 500
	}
	if !yamlHasPath(data, "agent", "output_compression", "preserve_errors") {
		cfg.Agent.OutputCompression.PreserveErrors = true
	}
	if !yamlHasPath(data, "agent", "output_compression", "shell_compression") {
		cfg.Agent.OutputCompression.ShellCompression = true
	}
	if !yamlHasPath(data, "agent", "output_compression", "python_compression") {
		cfg.Agent.OutputCompression.PythonCompression = true
	}
	if !yamlHasPath(data, "agent", "output_compression", "api_compression") {
		cfg.Agent.OutputCompression.APICompression = true
	}
	if !yamlHasPath(data, "agent", "output_compression", "repetitive_substitution", "lzw_enabled") {
		cfg.Agent.OutputCompression.RepetitiveSubstitution.LZWEnabled = true
	}
	if cfg.Agent.OutputCompression.RepetitiveSubstitution.MinPhraseChars <= 0 {
		cfg.Agent.OutputCompression.RepetitiveSubstitution.MinPhraseChars = 15
	}
	if cfg.Agent.OutputCompression.RepetitiveSubstitution.MinOccurrences <= 0 {
		cfg.Agent.OutputCompression.RepetitiveSubstitution.MinOccurrences = 3
	}
	if cfg.Agent.OutputCompression.RepetitiveSubstitution.MinSavingsPercent <= 0 {
		cfg.Agent.OutputCompression.RepetitiveSubstitution.MinSavingsPercent = 15
	}
	if cfg.Agent.OutputCompression.RepetitiveSubstitution.MaxInputChars <= 0 {
		cfg.Agent.OutputCompression.RepetitiveSubstitution.MaxInputChars = 50000
	}
	if cfg.Agent.OutputCompression.RepetitiveSubstitution.MaxDictionaryEntries <= 0 {
		cfg.Agent.OutputCompression.RepetitiveSubstitution.MaxDictionaryEntries = 16
	}
	if cfg.Agent.OutputCompression.TOONJSON.MinSavingsPercent <= 0 {
		cfg.Agent.OutputCompression.TOONJSON.MinSavingsPercent = 10
	}
	if cfg.Agent.OutputCompression.TOONJSON.MaxRows <= 0 {
		cfg.Agent.OutputCompression.TOONJSON.MaxRows = 200
	}
	if cfg.Agent.OutputCompression.SmartCrusher.MaxRows <= 0 {
		cfg.Agent.OutputCompression.SmartCrusher.MaxRows = 50
	}
	if cfg.Agent.OutputCompression.Reversible.MaxAgeHours <= 0 {
		cfg.Agent.OutputCompression.Reversible.MaxAgeHours = 24
	}
	if !yamlHasPath(data, "agent", "output_compression", "reversible", "primary_output_vault") {
		cfg.Agent.OutputCompression.Reversible.PrimaryOutputVault = true
	}
	if cfg.Agent.OutputCompression.Reversible.MaxInlineChars <= 0 {
		cfg.Agent.OutputCompression.Reversible.MaxInlineChars = 6000
	}
	// Enable all headroom features by default — validated in production.
	cfg.Agent.OutputCompression.SmartCrusher.Enabled = true
	cfg.Agent.OutputCompression.Reversible.Enabled = true
	cfg.Agent.ImportanceScoring.Enabled = true
	cfg.Agent.AutoLearning.Enabled = true
	if cfg.Agent.ImportanceScoring.Mode == "" {
		cfg.Agent.ImportanceScoring.Mode = "active"
	}
	if cfg.Agent.AutoLearning.Mode == "" {
		cfg.Agent.AutoLearning.Mode = "active"
	}
	// PreserveErrors defaults to true when not explicitly set (zero value = true)
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
	if !yamlHasPath(data, "personality", "emotion_synthesizer", "trigger_on_mood_change") {
		cfg.Personality.EmotionSynthesizer.TriggerOnMoodChange = true
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
	if cfg.Personality.InnerVoice.DecayMaxAgeSecs <= 0 {
		cfg.Personality.InnerVoice.DecayMaxAgeSecs = 300 // 5 minutes
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
	// InnerVoice is prompt-injected agent self-talk and must stay explicit opt-in.
	// EmotionSynthesizer can run on its own to update mood state without injecting
	// subconscious text into recovery prompts.
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
	// Adaptive tools defaults. Missing configs opt into progressive discovery so
	// regular turns do not carry every native schema.
	if !yamlHasPath(data, "agent", "adaptive_tools", "enabled") {
		cfg.Agent.AdaptiveTools.Enabled = true
	}
	if cfg.Agent.AdaptiveTools.Enabled &&
		yamlHasPath(data, "agent", "adaptive_tools", "max_tools") &&
		yamlHasPath(data, "agent", "adaptive_tools", "max_total_tools") &&
		yamlHasPath(data, "agent", "adaptive_tools", "max_schema_tokens") &&
		cfg.Agent.AdaptiveTools.MaxTools == 16 &&
		cfg.Agent.AdaptiveTools.MaxTotalTools == 32 &&
		cfg.Agent.AdaptiveTools.MaxSchemaTokens == 0 {
		cfg.Agent.AdaptiveTools.MaxTools = 10
		cfg.Agent.AdaptiveTools.MaxTotalTools = 20
		cfg.Agent.AdaptiveTools.MaxSchemaTokens = 6500
	}
	// Existing explicit values are preserved. Omitted caps get aggressive
	// defaults when adaptive filtering is active.
	if cfg.Agent.AdaptiveTools.MaxTools <= 0 &&
		cfg.Agent.AdaptiveTools.Enabled &&
		!yamlHasPath(data, "agent", "adaptive_tools", "max_tools") {
		cfg.Agent.AdaptiveTools.MaxTools = 10
	}
	if cfg.Agent.AdaptiveTools.MaxTotalTools <= 0 &&
		cfg.Agent.AdaptiveTools.Enabled &&
		!yamlHasPath(data, "agent", "adaptive_tools", "max_total_tools") {
		cfg.Agent.AdaptiveTools.MaxTotalTools = 20
	}
	if cfg.Agent.AdaptiveTools.SessionToolRetentionTurns <= 0 && cfg.Agent.AdaptiveTools.Enabled {
		cfg.Agent.AdaptiveTools.SessionToolRetentionTurns = 8
	}
	if cfg.Agent.AdaptiveTools.MaxSchemaTokens < 0 {
		cfg.Agent.AdaptiveTools.MaxSchemaTokens = 0
	} else if cfg.Agent.AdaptiveTools.MaxSchemaTokens == 0 &&
		cfg.Agent.AdaptiveTools.Enabled &&
		!yamlHasPath(data, "agent", "adaptive_tools", "max_schema_tokens") {
		cfg.Agent.AdaptiveTools.MaxSchemaTokens = 6500
	}
	if !cfg.Agent.AdaptiveTools.ProviderProfilesEnabled &&
		!yamlHasPath(data, "agent", "adaptive_tools", "provider_profiles_enabled") {
		cfg.Agent.AdaptiveTools.ProviderProfilesEnabled = true
	}
	if cfg.Agent.AdaptiveTools.DecayHalfLifeDays <= 0 {
		cfg.Agent.AdaptiveTools.DecayHalfLifeDays = 7.0
	}
	if cfg.Agent.AdaptiveTools.CleanTransitionsAfterDays <= 0 {
		cfg.Agent.AdaptiveTools.CleanTransitionsAfterDays = 90
	}
	if !yamlHasPath(data, "agent", "history_compaction", "enabled") {
		cfg.Agent.HistoryCompaction.Enabled = true
	}
	if cfg.Agent.HistoryCompaction.KeepRecentToolRoundsFull <= 0 {
		cfg.Agent.HistoryCompaction.KeepRecentToolRoundsFull = 2
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
			"filesystem", "query_memory", "manage_memory", "execute_shell",
		}
	}
	if cfg.Agent.MaxToolGuides <= 0 {
		cfg.Agent.MaxToolGuides = 3
	}
	// AnnouncementDetector defaults: structural text-only continuation recovery.
	if cfg.Agent.AnnouncementDetector.MaxRetries <= 0 {
		cfg.Agent.AnnouncementDetector.MaxRetries = 2
	}
	// ReuseFirst defaults — auto-materialisation on, but gated by run-success signal.
	// Both flags default to true when not explicitly provided in YAML.
	if !yamlHasPath(data, "agent", "reuse_first", "auto_materialize") {
		cfg.Agent.ReuseFirst.AutoMaterialize = true
	}
	if !yamlHasPath(data, "agent", "reuse_first", "require_success_signal") {
		cfg.Agent.ReuseFirst.RequireSuccessSignal = true
	}
	if cfg.Agent.ReuseFirst.MinSteps <= 0 {
		cfg.Agent.ReuseFirst.MinSteps = 3
	}
	if cfg.Agent.ReuseFirst.MaxArtifactsPerSession <= 0 {
		cfg.Agent.ReuseFirst.MaxArtifactsPerSession = 1
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

	// Heartbeat defaults
	if cfg.Heartbeat.DayTimeWindow.Start == "" {
		cfg.Heartbeat.DayTimeWindow.Start = "08:00"
	}
	if cfg.Heartbeat.DayTimeWindow.End == "" {
		cfg.Heartbeat.DayTimeWindow.End = "22:00"
	}
	if cfg.Heartbeat.DayTimeWindow.Interval == "" {
		cfg.Heartbeat.DayTimeWindow.Interval = "1h"
	}
	if cfg.Heartbeat.NightTimeWindow.Start == "" {
		cfg.Heartbeat.NightTimeWindow.Start = "22:00"
	}
	if cfg.Heartbeat.NightTimeWindow.End == "" {
		cfg.Heartbeat.NightTimeWindow.End = "08:00"
	}
	if cfg.Heartbeat.NightTimeWindow.Interval == "" {
		cfg.Heartbeat.NightTimeWindow.Interval = "4h"
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
	if !yamlHasPath(data, "co_agents", "specialists", "writer", "additional_prompt") {
		cfg.CoAgents.Specialists.Writer.AdditionalPrompt = defaultWriterSpecialistAdditionalPrompt
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
	if strings.TrimSpace(cfg.AgentMail.BaseURL) == "" {
		cfg.AgentMail.BaseURL = "https://api.agentmail.to"
	}
	if strings.TrimSpace(cfg.AgentMail.WebSocketURL) == "" {
		cfg.AgentMail.WebSocketURL = "wss://ws.agentmail.to/v0"
	}
	if cfg.AgentMail.PollIntervalSeconds <= 0 {
		cfg.AgentMail.PollIntervalSeconds = 120
	}
	if cfg.AgentMail.MaxAttachmentMB < 0 {
		cfg.AgentMail.MaxAttachmentMB = 10
	}
	if cfg.AgentMail.MaxAttachmentMB == 0 {
		cfg.AgentMail.MaxAttachmentMB = 10
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
	if cfg.Webhooks.RateLimit < 0 {
		cfg.Webhooks.RateLimit = 0 // 0 means unlimited
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
	if strings.TrimSpace(cfg.Tailscale.TsNet.SpaceAgentHostname) == "" {
		base := strings.TrimSpace(cfg.Tailscale.TsNet.Hostname)
		if base == "" {
			base = "aurago"
		}
		cfg.Tailscale.TsNet.SpaceAgentHostname = sanitizeTsnetHostname(base + "-space-agent")
	}
	if strings.TrimSpace(cfg.Tailscale.TsNet.ManifestHostname) == "" {
		base := strings.TrimSpace(cfg.Tailscale.TsNet.Hostname)
		if base == "" {
			base = "aurago"
		}
		cfg.Tailscale.TsNet.ManifestHostname = sanitizeTsnetHostname(base + "-manifest")
	}
	if cfg.Tailscale.TsNet.ManifestPort <= 0 || cfg.Tailscale.TsNet.ManifestPort == legacyManifestTsNetPort {
		cfg.Tailscale.TsNet.ManifestPort = defaultManifestTsNetPort
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
	if cfg.MQTT.Buffer.MaxPayloadBytes <= 0 {
		cfg.MQTT.Buffer.MaxPayloadBytes = 256 << 10
	}
	if cfg.MQTT.Availability.Topic == "" {
		cfg.MQTT.Availability.Topic = "aurago/status"
	}
	if cfg.MQTT.Availability.OnlinePayload == "" {
		cfg.MQTT.Availability.OnlinePayload = "online"
	}
	if cfg.MQTT.Availability.OfflinePayload == "" {
		cfg.MQTT.Availability.OfflinePayload = "offline"
	}
	if cfg.MQTT.Availability.QoS < 0 || cfg.MQTT.Availability.QoS > 2 {
		cfg.MQTT.Availability.QoS = 1
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
	cfg.Indexing.Chunking = normalizeIndexingChunkingConfig(cfg.Indexing.Chunking)
	// Resolve indexing directory paths to absolute paths
	for i := range cfg.Indexing.Directories {
		cfg.Indexing.Directories[i].Path = resolvePath(configDir, cfg.Indexing.Directories[i].Path)
	}

	// Workspace search defaults.
	if cfg.WorkspaceSearch.MaxFileSizeMB <= 0 {
		cfg.WorkspaceSearch.MaxFileSizeMB = 10
	}
	if cfg.WorkspaceSearch.MaxIndexSizeMB <= 0 {
		cfg.WorkspaceSearch.MaxIndexSizeMB = 256
	}
	if cfg.WorkspaceSearch.MaxResults <= 0 {
		cfg.WorkspaceSearch.MaxResults = 100
	}
	if cfg.WorkspaceSearch.PollIntervalSeconds <= 0 {
		cfg.WorkspaceSearch.PollIntervalSeconds = 5
	}
	if cfg.WorkspaceSearch.FuzzyThreshold <= 0 {
		cfg.WorkspaceSearch.FuzzyThreshold = 0.35
	}
	if len(cfg.WorkspaceSearch.Exclude) == 0 {
		cfg.WorkspaceSearch.Exclude = DefaultWorkspaceSearchExcludes()
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

func normalizeDeprecatedEmbeddingBackend(cfg *Config) {
	if cfg == nil || !strings.EqualFold(strings.TrimSpace(cfg.Embeddings.Local.Backend), "directml") {
		return
	}
	cfg.Embeddings.Local.Backend = "auto"
	directMLMigrationWarning.Do(func() {
		slog.Warn("The deprecated local embedding backend directml was normalized to auto; CUDA, Vulkan, or CPU will be selected")
	})
}

func normalizeOpenSCADGeometryBackend(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "manifold":
		return "manifold"
	case "cgal":
		return "cgal"
	default:
		return "auto"
	}
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

func normalizeIndexingChunkingConfig(cfg IndexingChunkingConfig) IndexingChunkingConfig {
	opts := chunking.NormalizeOptionsWithDefaults(chunking.Options{
		Strategy:     cfg.Strategy,
		MaxChars:     cfg.MaxChars,
		OverlapChars: cfg.OverlapChars,
		MaxChunks:    cfg.MaxChunksPerFile,
	})
	return IndexingChunkingConfig{
		Strategy:         opts.Strategy,
		MaxChars:         opts.MaxChars,
		OverlapChars:     opts.OverlapChars,
		MaxChunksPerFile: opts.MaxChunks,
	}
}

// NormalizeManifestRoutingConfig applies conservative defaults to Manifest routing hints.
func NormalizeManifestRoutingConfig(cfg *ManifestRoutingConfig) {
	switch strings.ToLower(strings.TrimSpace(cfg.SpecificityMode)) {
	case "fixed":
		cfg.SpecificityMode = "fixed"
	case "auto":
		cfg.SpecificityMode = "auto"
	default:
		cfg.SpecificityMode = "off"
	}
	cfg.Specificity = strings.ToLower(strings.TrimSpace(cfg.Specificity))
	if !IsValidManifestSpecificityCategory(cfg.Specificity) {
		cfg.Specificity = ""
	}
	if cfg.Headers == nil {
		cfg.Headers = map[string]string{}
	}
}

// NormalizeAIGatewayConfig applies conservative defaults for Cloudflare AI Gateway.
func NormalizeAIGatewayConfig(c *Config) {
	if c == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(c.AIGateway.Mode)) {
	case "", "auto":
		c.AIGateway.Mode = "auto"
	case "openai_compatible", "provider_native":
		c.AIGateway.Mode = strings.ToLower(strings.TrimSpace(c.AIGateway.Mode))
	default:
		c.AIGateway.Mode = "auto"
	}
	switch strings.ToLower(strings.TrimSpace(c.AIGateway.LogMode)) {
	case "", "metadata_only":
		c.AIGateway.LogMode = "metadata_only"
	case "off", "full":
		c.AIGateway.LogMode = strings.ToLower(strings.TrimSpace(c.AIGateway.LogMode))
	default:
		c.AIGateway.LogMode = "metadata_only"
	}
	if c.AIGateway.Metadata == nil {
		c.AIGateway.Metadata = map[string]string{}
	}
	normalizedMetadata := make(map[string]string, len(c.AIGateway.Metadata))
	for key, value := range c.AIGateway.Metadata {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" || len(normalizedMetadata) >= 5 {
			continue
		}
		normalizedMetadata[key] = value
	}
	c.AIGateway.Metadata = normalizedMetadata
	if c.AIGateway.RequestTimeoutMS < 0 {
		c.AIGateway.RequestTimeoutMS = 0
	}
	if c.AIGateway.MaxAttempts < 0 {
		c.AIGateway.MaxAttempts = 0
	}
	if c.AIGateway.MaxAttempts > 5 {
		c.AIGateway.MaxAttempts = 5
	}
	if c.AIGateway.RetryDelayMS < 0 {
		c.AIGateway.RetryDelayMS = 0
	}
	if c.AIGateway.RetryDelayMS > 5000 {
		c.AIGateway.RetryDelayMS = 5000
	}
	switch strings.ToLower(strings.TrimSpace(c.AIGateway.Backoff)) {
	case "", "constant", "linear", "exponential":
		c.AIGateway.Backoff = strings.ToLower(strings.TrimSpace(c.AIGateway.Backoff))
	default:
		c.AIGateway.Backoff = ""
	}
}

// NormalizeCloudflareTunnelConfig applies conservative runtime-safe defaults.
func NormalizeCloudflareTunnelConfig(c *Config) {
	if c == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(c.CloudflareTunnel.Mode)) {
	case "", "auto":
		c.CloudflareTunnel.Mode = "auto"
	case "docker", "native":
		c.CloudflareTunnel.Mode = strings.ToLower(strings.TrimSpace(c.CloudflareTunnel.Mode))
	default:
		c.CloudflareTunnel.Mode = "auto"
	}
	switch strings.ToLower(strings.TrimSpace(c.CloudflareTunnel.AuthMethod)) {
	case "", "token":
		c.CloudflareTunnel.AuthMethod = "token"
	case "named", "quick":
		c.CloudflareTunnel.AuthMethod = strings.ToLower(strings.TrimSpace(c.CloudflareTunnel.AuthMethod))
	default:
		c.CloudflareTunnel.AuthMethod = "token"
	}
	switch strings.ToLower(strings.TrimSpace(c.CloudflareTunnel.LogLevel)) {
	case "", "info":
		c.CloudflareTunnel.LogLevel = "info"
	case "debug", "warn", "error":
		c.CloudflareTunnel.LogLevel = strings.ToLower(strings.TrimSpace(c.CloudflareTunnel.LogLevel))
	default:
		c.CloudflareTunnel.LogLevel = "info"
	}
	if c.CloudflareTunnel.LoopbackPort < 0 {
		c.CloudflareTunnel.LoopbackPort = 0
	}
	if c.CloudflareTunnel.MetricsPort < 0 {
		c.CloudflareTunnel.MetricsPort = 0
	}
}

// IsValidManifestSpecificityCategory reports whether value is a Manifest specificity category.
func IsValidManifestSpecificityCategory(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "coding",
		"web_browsing",
		"data_analysis",
		"image_generation",
		"video_generation",
		"social_media",
		"email_management",
		"calendar_management",
		"trading":
		return true
	default:
		return false
	}
}

// Save persists the configuration to the specified path using a targeted patch
// strategy: the original file is read as YAML nodes, only the changed runtime
// fields are updated, and the document is written back atomically. This keeps
// sensitive fields intact and preserves existing YAML comments where possible.
func (c *Config) Save(path string) error {
	configSaveMu.Lock()
	defer configSaveMu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// 1. Read the existing config file into a generic map
	original, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read config file for patching: %w", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(original, &root); err != nil {
		return fmt.Errorf("failed to unmarshal config for patching: %w", err)
	}

	// 2. Patch only the fields that are safe to change at runtime
	patches := []struct {
		path  []string
		value interface{}
	}{
		{[]string{"personality", "core_personality"}, c.Personality.CorePersonality},
		{[]string{"server", "ui_language"}, c.Server.UILanguage},
		{[]string{"auth", "enabled"}, c.Auth.Enabled},
		{[]string{"webhooks", "outgoing"}, c.Webhooks.Outgoing},
		{[]string{"uptime_kuma", "enabled"}, c.UptimeKuma.Enabled},
		{[]string{"uptime_kuma", "base_url"}, c.UptimeKuma.BaseURL},
		{[]string{"uptime_kuma", "insecure_ssl"}, c.UptimeKuma.InsecureSSL},
		{[]string{"uptime_kuma", "request_timeout"}, c.UptimeKuma.RequestTimeout},
		{[]string{"uptime_kuma", "poll_interval_seconds"}, c.UptimeKuma.PollIntervalSeconds},
		{[]string{"uptime_kuma", "relay_to_agent"}, c.UptimeKuma.RelayToAgent},
		{[]string{"uptime_kuma", "relay_instruction"}, c.UptimeKuma.RelayInstruction},
		{[]string{"email", "relay_cheatsheet_id"}, c.Email.RelayCheatsheetID},
		{[]string{"agentmail", "enabled"}, c.AgentMail.Enabled},
		{[]string{"agentmail", "readonly"}, c.AgentMail.ReadOnly},
		{[]string{"agentmail", "inbox_id"}, c.AgentMail.InboxID},
		{[]string{"agentmail", "auto_create_inbox"}, c.AgentMail.AutoCreateInbox},
		{[]string{"agentmail", "username"}, c.AgentMail.Username},
		{[]string{"agentmail", "domain"}, c.AgentMail.Domain},
		{[]string{"agentmail", "display_name"}, c.AgentMail.DisplayName},
		{[]string{"agentmail", "use_websocket"}, c.AgentMail.UseWebSocket},
		{[]string{"agentmail", "poll_interval_seconds"}, c.AgentMail.PollIntervalSeconds},
		{[]string{"agentmail", "relay_to_agent"}, c.AgentMail.RelayToAgent},
		{[]string{"agentmail", "relay_cheatsheet_id"}, c.AgentMail.RelayCheatsheetID},
		{[]string{"agentmail", "max_attachment_mb"}, c.AgentMail.MaxAttachmentMB},
		{[]string{"agentmail", "base_url"}, c.AgentMail.BaseURL},
		{[]string{"agentmail", "websocket_url"}, c.AgentMail.WebSocketURL},
		{[]string{"grafana", "enabled"}, c.Grafana.Enabled},
		{[]string{"grafana", "base_url"}, c.Grafana.BaseURL},
		{[]string{"grafana", "readonly"}, c.Grafana.ReadOnly},
		{[]string{"grafana", "insecure_ssl"}, c.Grafana.InsecureSSL},
		{[]string{"grafana", "request_timeout"}, c.Grafana.RequestTimeout},
		{[]string{"evomap", "enabled"}, c.Evomap.Enabled},
		{[]string{"evomap", "readonly"}, c.Evomap.ReadOnly},
		{[]string{"evomap", "base_url"}, c.Evomap.BaseURL},
		{[]string{"evomap", "node_id"}, c.Evomap.NodeID},
		{[]string{"evomap", "request_timeout_seconds"}, c.Evomap.RequestTimeoutSeconds},
		{[]string{"evomap", "max_result_bytes"}, c.Evomap.MaxResultBytes},
		{[]string{"evomap", "kg_enabled"}, c.Evomap.KGEnabled},
		{[]string{"evomap", "allow_publish"}, c.Evomap.AllowPublish},
		{[]string{"evomap", "allow_report"}, c.Evomap.AllowReport},
		{[]string{"evomap", "allow_bounties"}, c.Evomap.AllowBounties},
		{[]string{"huggingface", "enabled"}, c.HuggingFace.Enabled},
		{[]string{"huggingface", "read_only"}, c.HuggingFace.ReadOnly},
		{[]string{"huggingface", "allow_writes"}, c.HuggingFace.AllowWrites},
		{[]string{"huggingface", "allow_delete"}, c.HuggingFace.AllowDelete},
		{[]string{"huggingface", "allow_jobs"}, c.HuggingFace.AllowJobs},
		{[]string{"huggingface", "allow_scheduled_jobs"}, c.HuggingFace.AllowScheduledJobs},
		{[]string{"huggingface", "allowed_namespaces"}, c.HuggingFace.AllowedNamespaces},
		{[]string{"huggingface", "allowed_repos"}, c.HuggingFace.AllowedRepos},
		{[]string{"huggingface", "allowed_hardware"}, c.HuggingFace.AllowedHardware},
		{[]string{"huggingface", "max_download_mb"}, c.HuggingFace.MaxDownloadMB},
		{[]string{"huggingface", "max_upload_mb"}, c.HuggingFace.MaxUploadMB},
		{[]string{"huggingface", "max_dataset_rows"}, c.HuggingFace.MaxDatasetRows},
		{[]string{"huggingface", "job_default_timeout_minutes"}, c.HuggingFace.JobDefaultTimeoutMinutes},
		{[]string{"huggingface", "job_max_runtime_minutes"}, c.HuggingFace.JobMaxRuntimeMinutes},
		{[]string{"huggingface", "request_timeout_seconds"}, c.HuggingFace.RequestTimeoutSeconds},
		{[]string{"huggingface", "max_result_bytes"}, c.HuggingFace.MaxResultBytes},
		{[]string{"huggingface", "hub_base_url"}, c.HuggingFace.HubBaseURL},
		{[]string{"huggingface", "dataset_base_url"}, c.HuggingFace.DatasetBaseURL},
		{[]string{"huggingface", "jobs_base_url"}, c.HuggingFace.JobsBaseURL},
		{[]string{"huggingface", "router_base_url"}, c.HuggingFace.RouterBaseURL},
		{[]string{"frigate", "enabled"}, c.Frigate.Enabled},
		{[]string{"frigate", "readonly"}, c.Frigate.ReadOnly},
		{[]string{"frigate", "url"}, c.Frigate.URL},
		{[]string{"frigate", "internal_port"}, c.Frigate.InternalPort},
		{[]string{"frigate", "insecure"}, c.Frigate.Insecure},
		{[]string{"frigate", "default_camera"}, c.Frigate.DefaultCamera},
		{[]string{"frigate", "event_relay"}, c.Frigate.EventRelay},
		{[]string{"frigate", "review_relay"}, c.Frigate.ReviewRelay},
		{[]string{"frigate", "store_media"}, c.Frigate.StoreMedia},
		{[]string{"frigate", "mqtt_topic_prefix"}, c.Frigate.MQTTTopicPrefix},
		{[]string{"three_d_printers", "enabled"}, c.ThreeDPrinters.Enabled},
		{[]string{"three_d_printers", "readonly"}, c.ThreeDPrinters.ReadOnly},
		{[]string{"three_d_printers", "default_printer"}, c.ThreeDPrinters.DefaultPrinter},
		{[]string{"three_d_printers", "elegoo_centauri_carbon", "enabled"}, c.ThreeDPrinters.ElegooCentauriCarbon.Enabled},
		{[]string{"three_d_printers", "elegoo_centauri_carbon", "printers"}, c.ThreeDPrinters.ElegooCentauriCarbon.Printers},
		{[]string{"three_d_printers", "klipper", "enabled"}, c.ThreeDPrinters.Klipper.Enabled},
		{[]string{"three_d_printers", "klipper", "printers"}, c.ThreeDPrinters.Klipper.Printers},
		{[]string{"ai_gateway", "enabled"}, c.AIGateway.Enabled},
		{[]string{"ai_gateway", "account_id"}, c.AIGateway.AccountID},
		{[]string{"ai_gateway", "gateway_id"}, c.AIGateway.GatewayID},
		{[]string{"ai_gateway", "mode"}, c.AIGateway.Mode},
		{[]string{"ai_gateway", "log_mode"}, c.AIGateway.LogMode},
		{[]string{"ai_gateway", "metadata"}, c.AIGateway.Metadata},
		{[]string{"ai_gateway", "request_timeout_ms"}, c.AIGateway.RequestTimeoutMS},
		{[]string{"ai_gateway", "max_attempts"}, c.AIGateway.MaxAttempts},
		{[]string{"ai_gateway", "retry_delay_ms"}, c.AIGateway.RetryDelayMS},
		{[]string{"ai_gateway", "backoff"}, c.AIGateway.Backoff},
		{[]string{"tailscale", "enabled"}, c.Tailscale.Enabled},
		{[]string{"tailscale", "readonly"}, c.Tailscale.ReadOnly},
		{[]string{"tailscale", "tailnet"}, c.Tailscale.Tailnet},
		{[]string{"tailscale", "tsnet", "enabled"}, c.Tailscale.TsNet.Enabled},
		{[]string{"tailscale", "tsnet", "hostname"}, c.Tailscale.TsNet.Hostname},
		{[]string{"tailscale", "tsnet", "state_dir"}, c.Tailscale.TsNet.StateDir},
		{[]string{"tailscale", "tsnet", "serve_http"}, c.Tailscale.TsNet.ServeHTTP},
		{[]string{"tailscale", "tsnet", "expose_homepage"}, c.Tailscale.TsNet.ExposeHomepage},
		{[]string{"tailscale", "tsnet", "expose_space_agent"}, c.Tailscale.TsNet.ExposeSpaceAgent},
		{[]string{"tailscale", "tsnet", "space_agent_hostname"}, c.Tailscale.TsNet.SpaceAgentHostname},
		{[]string{"tailscale", "tsnet", "expose_manifest"}, c.Tailscale.TsNet.ExposeManifest},
		{[]string{"tailscale", "tsnet", "manifest_hostname"}, c.Tailscale.TsNet.ManifestHostname},
		{[]string{"tailscale", "tsnet", "manifest_port"}, c.Tailscale.TsNet.ManifestPort},
		{[]string{"tailscale", "tsnet", "funnel"}, c.Tailscale.TsNet.Funnel},
		{[]string{"tailscale", "tsnet", "allow_http_fallback"}, c.Tailscale.TsNet.AllowHTTPFallback},
		{[]string{"space_agent", "enabled"}, c.SpaceAgent.Enabled},
		{[]string{"space_agent", "auto_start"}, c.SpaceAgent.AutoStart},
		{[]string{"space_agent", "repo_url"}, c.SpaceAgent.RepoURL},
		{[]string{"space_agent", "git_ref"}, c.SpaceAgent.GitRef},
		{[]string{"space_agent", "container_name"}, c.SpaceAgent.ContainerName},
		{[]string{"space_agent", "image"}, c.SpaceAgent.Image},
		{[]string{"space_agent", "host"}, c.SpaceAgent.Host},
		{[]string{"space_agent", "port"}, c.SpaceAgent.Port},
		{[]string{"space_agent", "https_enabled"}, c.SpaceAgent.HTTPSEnabled},
		{[]string{"space_agent", "https_port"}, c.SpaceAgent.HTTPSPort},
		{[]string{"space_agent", "customware_path"}, c.SpaceAgent.CustomwarePath},
		{[]string{"space_agent", "data_path"}, c.SpaceAgent.DataPath},
		{[]string{"space_agent", "admin_user"}, c.SpaceAgent.AdminUser},
		{[]string{"space_agent", "public_url"}, c.SpaceAgent.PublicURL},
		{[]string{"manifest", "routing", "enabled"}, c.Manifest.Routing.Enabled},
		{[]string{"manifest", "routing", "specificity_mode"}, c.Manifest.Routing.SpecificityMode},
		{[]string{"manifest", "routing", "specificity"}, c.Manifest.Routing.Specificity},
		{[]string{"manifest", "routing", "headers"}, c.Manifest.Routing.Headers},
		{[]string{"omniroute", "enabled"}, c.OmniRoute.Enabled},
		{[]string{"omniroute", "auto_start"}, c.OmniRoute.AutoStart},
		{[]string{"omniroute", "mode"}, c.OmniRoute.Mode},
		{[]string{"omniroute", "url"}, c.OmniRoute.URL},
		{[]string{"omniroute", "external_base_url"}, c.OmniRoute.ExternalBaseURL},
		{[]string{"omniroute", "container_name"}, c.OmniRoute.ContainerName},
		{[]string{"omniroute", "image"}, c.OmniRoute.Image},
		{[]string{"omniroute", "host"}, c.OmniRoute.Host},
		{[]string{"omniroute", "port"}, c.OmniRoute.Port},
		{[]string{"omniroute", "host_port"}, c.OmniRoute.HostPort},
		{[]string{"omniroute", "network_name"}, c.OmniRoute.NetworkName},
		{[]string{"omniroute", "data_volume"}, c.OmniRoute.DataVolume},
		{[]string{"omniroute", "health_path"}, c.OmniRoute.HealthPath},
		{[]string{"omniroute", "memory_mb"}, c.OmniRoute.MemoryMB},
		{[]string{"virtual_desktop", "enabled"}, c.VirtualDesktop.Enabled},
		{[]string{"virtual_desktop", "readonly"}, c.VirtualDesktop.ReadOnly},
		{[]string{"virtual_desktop", "allow_agent_control"}, c.VirtualDesktop.AllowAgentControl},
		{[]string{"virtual_desktop", "allow_generated_apps"}, c.VirtualDesktop.AllowGeneratedApps},
		{[]string{"virtual_desktop", "allow_python_jobs"}, c.VirtualDesktop.AllowPythonJobs},
		{[]string{"virtual_desktop", "workspace_dir"}, c.VirtualDesktop.WorkspaceDir},
		{[]string{"virtual_desktop", "max_file_size_mb"}, c.VirtualDesktop.MaxFileSizeMB},
		{[]string{"virtual_desktop", "control_level"}, c.VirtualDesktop.ControlLevel},
		{[]string{"virtual_desktop", "max_ws_clients"}, c.VirtualDesktop.MaxWSClients},
		{[]string{"virtual_desktop", "remote_max_session_minutes"}, c.VirtualDesktop.RemoteMaxSessionMinutes},
		{[]string{"virtual_desktop", "remote_idle_timeout_minutes"}, c.VirtualDesktop.RemoteIdleTimeoutMinutes},
		{[]string{"virtual_desktop", "code_studio", "enabled"}, c.VirtualDesktop.CodeStudio.Enabled},
		{[]string{"virtual_desktop", "code_studio", "image"}, c.VirtualDesktop.CodeStudio.Image},
		{[]string{"virtual_desktop", "code_studio", "auto_start"}, c.VirtualDesktop.CodeStudio.AutoStart},
		{[]string{"virtual_desktop", "code_studio", "auto_stop_minutes"}, c.VirtualDesktop.CodeStudio.AutoStopMinutes},
		{[]string{"virtual_desktop", "code_studio", "max_memory_mb"}, c.VirtualDesktop.CodeStudio.MaxMemoryMB},
		{[]string{"virtual_desktop", "code_studio", "max_cpu_cores"}, c.VirtualDesktop.CodeStudio.MaxCPUCores},
		{[]string{"virtual_desktop", "openscad", "enabled"}, c.VirtualDesktop.OpenSCAD.Enabled},
		{[]string{"virtual_desktop", "openscad", "image"}, c.VirtualDesktop.OpenSCAD.Image},
		{[]string{"virtual_desktop", "openscad", "auto_start"}, c.VirtualDesktop.OpenSCAD.AutoStart},
		{[]string{"virtual_desktop", "openscad", "auto_stop_minutes"}, c.VirtualDesktop.OpenSCAD.AutoStopMinutes},
		{[]string{"virtual_desktop", "openscad", "max_memory_mb"}, c.VirtualDesktop.OpenSCAD.MaxMemoryMB},
		{[]string{"virtual_desktop", "openscad", "max_cpu_cores"}, c.VirtualDesktop.OpenSCAD.MaxCPUCores},
		{[]string{"virtual_desktop", "openscad", "max_concurrent_jobs"}, c.VirtualDesktop.OpenSCAD.MaxConcurrentJobs},
		{[]string{"virtual_desktop", "openscad", "default_exports"}, c.VirtualDesktop.OpenSCAD.DefaultExports},
		{[]string{"virtual_desktop", "openscad", "max_source_kb"}, c.VirtualDesktop.OpenSCAD.MaxSourceKB},
		{[]string{"virtual_desktop", "openscad", "max_output_mb"}, c.VirtualDesktop.OpenSCAD.MaxOutputMB},
		{[]string{"virtual_desktop", "openscad", "render_timeout_seconds"}, c.VirtualDesktop.OpenSCAD.RenderTimeoutSeconds},
		{[]string{"virtual_desktop", "openscad", "max_render_timeout_seconds"}, c.VirtualDesktop.OpenSCAD.MaxRenderTimeoutSeconds},
		{[]string{"virtual_desktop", "openscad", "job_retention_days"}, c.VirtualDesktop.OpenSCAD.JobRetentionDays},
	}
	for _, patch := range patches {
		if err := setYAMLPathValue(&root, patch.path, patch.value); err != nil {
			return fmt.Errorf("failed to patch config path %s: %w", strings.Join(patch.path, "."), err)
		}
	}

	// 3. Write back with all original fields (including API keys) intact
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		_ = enc.Close()
		return fmt.Errorf("failed to marshal patched config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("failed to finalize patched config: %w", err)
	}
	data := buf.Bytes()

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

func setYAMLPathValue(root *yaml.Node, path []string, value interface{}) error {
	if len(path) == 0 {
		return nil
	}

	doc := root
	if doc.Kind == 0 {
		doc.Kind = yaml.DocumentNode
	}
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
		}
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		doc.Kind = yaml.MappingNode
		doc.Tag = "!!map"
		doc.Content = nil
	}

	current := doc
	for _, key := range path[:len(path)-1] {
		next := mappingNodeValue(current, key)
		if next == nil {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
			next = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			current.Content = append(current.Content, keyNode, next)
		} else if next.Kind == 0 {
			next.Kind = yaml.MappingNode
			next.Tag = "!!map"
		}
		if next.Kind != yaml.MappingNode {
			next.Kind = yaml.MappingNode
			next.Tag = "!!map"
			next.Content = nil
		}
		current = next
	}

	replacement, err := yamlNodeForValue(value)
	if err != nil {
		return err
	}
	lastKey := path[len(path)-1]
	for i := 0; i+1 < len(current.Content); i += 2 {
		if current.Content[i].Value == lastKey {
			existing := current.Content[i+1]
			replacement.HeadComment = existing.HeadComment
			replacement.LineComment = existing.LineComment
			replacement.FootComment = existing.FootComment
			current.Content[i+1] = replacement
			return nil
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: lastKey}
	current.Content = append(current.Content, keyNode, replacement)
	return nil
}

func mappingNodeValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func yamlNodeForValue(value interface{}) (*yaml.Node, error) {
	raw, err := yaml.Marshal(value)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0], nil
	}
	return &doc, nil
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

func normalizeFritzBoxLegacyKeys(data []byte) []byte {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return data
	}

	root := yamlDocumentRoot(&doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return data
	}
	fritzbox := yamlMappingValue(root, "fritzbox")
	if fritzbox == nil || fritzbox.Kind != yaml.MappingNode {
		return data
	}

	if yamlMappingValue(fritzbox, "smart_home") == nil {
		if legacy := yamlMappingValue(fritzbox, "smarthome"); legacy != nil {
			yamlAppendMapping(fritzbox, "smart_home", cloneYAMLNode(legacy))
		}
	}

	telephony := yamlMappingValue(fritzbox, "telephony")
	subFeatures := yamlMappingValue(telephony, "sub_features")
	if subFeatures != nil && subFeatures.Kind == yaml.MappingNode && yamlMappingValue(subFeatures, "call_lists") == nil {
		if legacy := yamlMappingValue(subFeatures, "call_list"); legacy != nil {
			yamlAppendMapping(subFeatures, "call_lists", cloneYAMLNode(legacy))
		}
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return data
	}
	return out
}

func yamlDocumentRoot(doc *yaml.Node) *yaml.Node {
	if doc == nil {
		return nil
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return doc
}

func yamlMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func yamlAppendMapping(node *yaml.Node, key string, value *yaml.Node) {
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	clone := *node
	if len(node.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			clone.Content[i] = cloneYAMLNode(child)
		}
	}
	return &clone
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

func normalizeDockerWorkspaceDir(configDir, workspaceDir string, runningInDocker bool) string {
	if !runningInDocker {
		return workspaceDir
	}

	cleanConfigDir := filepath.Clean(configDir)
	if cleanConfigDir != filepath.Clean("/app/data") {
		return workspaceDir
	}

	switch filepath.ToSlash(strings.TrimSpace(workspaceDir)) {
	case "", "./agent_workspace/workdir", "agent_workspace/workdir":
		return "/app/agent_workspace/workdir"
	default:
		return workspaceDir
	}
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
