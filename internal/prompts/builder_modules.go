package prompts

import (
	"aurago/internal/memory"
	"aurago/internal/security"
	promptsembed "aurago/prompts"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func parseOrFallback(filename, content string, logger *slog.Logger) PromptModule {
	mod, err := parsePromptModule(content)
	if err != nil {
		if logger != nil {
			logger.Debug("Prompt module has no valid YAML frontmatter, using raw content as fallback",
				"file", filename, "error", err)
		}
		return PromptModule{
			Metadata: PromptMetadata{
				ID:       strings.TrimSuffix(filepath.Base(filename), ".md"),
				Priority: 100,
				Tags:     []string{"core"},
			},
			Content: content,
		}
	}
	return *mod
}

// GetActivePromptOverrides is a function hook to break import cycles.
var GetActivePromptOverrides func() map[string]string

func loadPromptModules(dir string, logger *slog.Logger) []PromptModule {
	// --- Fast path: check cache validity (TTL + disk files) ---
	promptCacheMu.RLock()
	cached, ok := promptCacheByDir[dir]
	promptCacheMu.RUnlock()

	if ok {
		if time.Since(cached.checked) < 60*time.Second {
			return cached.modules
		}
		if !promptCacheStale(dir, cached.mtimes) {
			promptCacheMu.Lock()
			c := promptCacheByDir[dir]
			c.checked = time.Now()
			promptCacheByDir[dir] = c
			promptCacheMu.Unlock()
			return cached.modules
		}
	}

	// --- Slow path: embedded FS is the immutable system base; disk overlays user customizations ---
	//
	// System prompts (identity.md, rules.md, etc.) live only in the binary embed.
	// Root-level tools_*.md files are legacy/manual material and are intentionally
	// kept out of the global prompt; detailed manuals are loaded via discover_tools
	// or targeted dynamic guide injection.
	// Users may add or override any prompt by placing a same-named .md file in
	// the on-disk promptsDir.  The disk copy always wins over the embedded copy.
	moduleMap := make(map[string]PromptModule)

	// 1. Seed from embedded FS (system prompts — tamper-proof in the binary)
	_ = fs.WalkDir(promptsembed.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		// Only root-level .md files belong to the module system; sub-directories
		// (personalities/, templates/, tools_manuals/) are handled separately.
		if strings.Contains(path, "/") || !strings.HasSuffix(path, ".md") {
			return nil
		}
		if isRootToolPromptModule(path) {
			return nil
		}
		data, err := fs.ReadFile(promptsembed.FS, path)
		if err != nil {
			return nil
		}
		moduleMap[strings.ToLower(path)] = parseOrFallback(path, string(data), logger)
		return nil
	})

	// 2. Overlay with on-disk files (user identity.md or custom prompts override the embedded versions)
	mtimes := make(map[string]time.Time)
	if files, err := os.ReadDir(dir); err == nil {
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
				continue
			}
			if isRootToolPromptModule(file.Name()) {
				continue
			}
			path := filepath.Join(dir, file.Name())
			info, err := file.Info()
			if err == nil {
				mtimes[path] = info.ModTime()
			}
			data, err := os.ReadFile(path)
			if err != nil {
				logger.Warn("Failed to read prompt file", "path", path, "error", err)
				continue
			}
			key := strings.ToLower(file.Name())
			moduleMap[key] = parseOrFallback(file.Name(), string(data), logger)
		}
	} else if len(moduleMap) == 0 {
		logger.Error("Failed to read prompts directory and no embedded modules loaded", "path", dir, "error", err)
	}

	// Convert map to slice
	modules := make([]PromptModule, 0, len(moduleMap))
	for _, m := range moduleMap {
		modules = append(modules, m)
	}

	// Sort modules by ID to ensure deterministic output
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Metadata.ID < modules[j].Metadata.ID
	})

	// Update cache
	promptCacheMu.Lock()
	promptCacheByDir[dir] = promptDirCache{modules: modules, mtimes: mtimes, checked: time.Now()}
	promptCacheMu.Unlock()

	if ok {
		logger.Debug("[PromptCache] Reloaded (files changed)", "dir", dir, "count", len(modules))
	} else {
		logger.Debug("[PromptCache] Populated", "dir", dir, "count", len(modules))
	}

	return modules
}

func isRootToolPromptModule(name string) bool {
	base := strings.ToLower(filepath.Base(name))
	return strings.HasPrefix(base, "tools_") && strings.HasSuffix(base, ".md")
}

// promptCacheStale returns true if any tracked file has a newer ModTime,
// or if the directory now has different files than when the cache was built.
func promptCacheStale(dir string, mtimes map[string]time.Time) bool {
	files, err := os.ReadDir(dir)
	if err != nil {
		return true
	}
	newCount := 0
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}
		if isRootToolPromptModule(file.Name()) {
			continue
		}
		newCount++
		path := filepath.Join(dir, file.Name())
		info, err := file.Info()
		if err != nil {
			return true
		}
		if cached, ok := mtimes[path]; !ok || info.ModTime().After(cached) {
			return true
		}
	}
	return newCount != len(mtimes)
}

// ClearPromptCache empties the in-memory cache of parsed prompt modules.
func ClearPromptCache() {
	promptCacheMu.Lock()
	promptCacheByDir = make(map[string]promptDirCache)
	promptCacheMu.Unlock()

	guideCacheMu.Lock()
	guideCache = make(map[string]guideCacheEntry)
	guideCacheMu.Unlock()
}

func parsePromptModule(raw string) (*PromptModule, error) {
	// Strip UTF-8 BOM (\xEF\xBB\xBF) and leading blank lines so files saved by
	// Windows editors or tools that prepend a BOM are accepted without error.
	raw = strings.TrimPrefix(raw, "\xEF\xBB\xBF")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.TrimLeft(raw, "\r\n ")
	if !strings.HasPrefix(raw, "---") {
		return nil, fmt.Errorf("no frontmatter found")
	}

	// Strip the leading "---\n" then split on the closing "\n---\n".
	// This avoids false splits on horizontal rules (---) inside the body.
	inner := raw[3:] // remove leading "---"
	inner = strings.TrimLeft(inner, "\r\n")
	idx := strings.Index(inner, "\n---\n")
	if idx < 0 {
		return nil, fmt.Errorf("invalid frontmatter format")
	}

	frontmatter := inner[:idx]
	body := inner[idx+4:]
	body = strings.TrimLeft(body, "\r\n")

	var meta PromptMetadata
	err := yaml.Unmarshal([]byte(frontmatter), &meta)
	if err != nil {
		return nil, err
	}

	return &PromptModule{
		Metadata: meta,
		Content:  strings.TrimSpace(body),
	}, nil
}

func filterModules(modules []PromptModule, flags *ContextFlags) []PromptModule {
	// Pre-allocate with estimated capacity (typically 50-70% of modules match)
	filtered := make([]PromptModule, 0, len(modules))
	for _, mod := range modules {
		if mod.ShouldInclude(flags) {
			filtered = append(filtered, mod)
		}
	}
	return filtered
}

func normalizePromptCondition(cond string) string {
	cond = strings.TrimSpace(strings.ToLower(cond))
	if strings.HasPrefix(cond, "tools.") && strings.HasSuffix(cond, ".enabled") {
		name := strings.TrimSuffix(strings.TrimPrefix(cond, "tools."), ".enabled")
		name = strings.NewReplacer("-", "_", ".", "_").Replace(name)
		return name + "_enabled"
	}
	return cond
}

func matchPromptCondition(cond string, flags *ContextFlags) bool {
	if flags == nil {
		return false
	}
	switch normalizePromptCondition(cond) {
	case "is_error":
		return flags.IsErrorState
	case "requires_coding":
		return flags.RequiresCoding
	case "lifeboat":
		return flags.LifeboatEnabled
	case "lifeboat_intent":
		return flags.LifeboatIntent
	case "capability_creation_intent":
		return flags.CapabilityCreationIntent
	case "daemon_skills_intent":
		return flags.DaemonSkillsIntent
	case "maintenance":
		return flags.IsMaintenanceMode
	case "coagent":
		return flags.IsCoAgent
	case "egg":
		return flags.IsEgg
	case "main_agent":
		return !flags.IsCoAgent && !flags.IsEgg
	case "discord_enabled":
		return flags.DiscordEnabled
	case "telegram_enabled":
		return flags.TelegramEnabled
	case "email_enabled":
		return flags.EmailEnabled
	case "docker_enabled":
		return flags.DockerEnabled
	case "home_assistant_enabled":
		return flags.HomeAssistantEnabled
	case "webdav_enabled":
		return flags.WebDAVEnabled
	case "koofr_enabled":
		return flags.KoofrEnabled
	case "paperless_ngx_enabled":
		return flags.PaperlessNGXEnabled
	case "chromecast_enabled":
		return flags.ChromecastEnabled
	case "coagent_enabled":
		return flags.CoAgentEnabled
	case "google_workspace_enabled":
		return flags.GoogleWorkspaceEnabled
	case "onedrive_enabled":
		return flags.OneDriveEnabled
	case "proxmox_enabled":
		return flags.ProxmoxEnabled
	case "frigate_enabled":
		return flags.FrigateEnabled
	case "three_d_printer_enabled":
		return flags.ThreeDPrinterEnabled
	case "ollama_enabled":
		return flags.OllamaEnabled
	case "tailscale_enabled":
		return flags.TailscaleEnabled
	case "cloudflare_tunnel_enabled":
		return flags.CloudflareTunnelEnabled
	case "ansible_enabled":
		return flags.AnsibleEnabled
	case "invasion_control_enabled":
		return flags.InvasionControlEnabled
	case "github_enabled":
		return flags.GitHubEnabled
	case "mqtt_enabled":
		return flags.MQTTEnabled
	case "mcp_enabled":
		return flags.MCPEnabled
	case "meshcentral_enabled":
		return flags.MeshCentralEnabled
	case "sandbox_enabled":
		return flags.SandboxEnabled
	case "memory_enabled":
		return flags.MemoryEnabled
	case "knowledge_graph_enabled":
		return flags.KnowledgeGraphEnabled
	case "secrets_vault_enabled":
		return flags.SecretsVaultEnabled
	case "scheduler_enabled":
		return flags.SchedulerEnabled
	case "notes_enabled":
		return flags.NotesEnabled
	case "missions_enabled":
		return flags.MissionsEnabled
	case "allow_shell":
		return flags.AllowShell
	case "allow_python":
		return flags.AllowPython
	case "allow_filesystem_write":
		return flags.AllowFilesystemWrite
	case "allow_network_requests":
		return flags.AllowNetworkRequests
	case "allow_remote_shell":
		return flags.AllowRemoteShell
	case "allow_self_update":
		return flags.AllowSelfUpdate
	case "sudo_enabled":
		return flags.SudoEnabled
	case "allow_package_manager":
		return flags.PackageManagerEnabled
	case "video_download_enabled":
		return flags.VideoDownloadEnabled
	case "wol_enabled":
		return flags.WOLEnabled
	case "virustotal_enabled":
		return flags.VirusTotalEnabled
	case "golangci_lint_enabled":
		return flags.GolangciLintEnabled
	case "brave_search_enabled":
		return flags.BraveSearchEnabled
	case "space_agent_enabled":
		return flags.SpaceAgentEnabled
	case "homepage_enabled":
		return flags.HomepageEnabled
	case "homepage_allow_local_server":
		return flags.HomepageAllowLocalServer
	case "netlify_enabled":
		return flags.NetlifyEnabled
	case "vercel_enabled":
		return flags.VercelEnabled
	case "image_generation_enabled":
		return flags.ImageGenerationEnabled
	case "music_generation_enabled":
		return flags.MusicGenerationEnabled
	case "video_generation_enabled":
		return flags.VideoGenerationEnabled
	case "remote_control_enabled":
		return flags.RemoteControlEnabled
	case "is_docker":
		return flags.IsDocker
	case "media_registry_enabled":
		return flags.MediaRegistryEnabled
	case "homepage_registry_enabled":
		return flags.HomepageRegistryEnabled
	case "document_creator_enabled":
		return flags.DocumentCreatorEnabled
	case "media_conversion_enabled":
		return flags.MediaConversionEnabled
	case "browser_automation_enabled":
		return flags.BrowserAutomationEnabled
	case "network_ping_enabled":
		return flags.NetworkPingEnabled
	case "s3_enabled":
		return flags.S3Enabled
	case "network_scan_enabled":
		return flags.NetworkScanEnabled
	case "upnp_scan_enabled":
		return flags.UPnPScanEnabled
	case "web_scraper_enabled":
		return flags.WebScraperEnabled
	case "form_automation_enabled":
		return flags.FormAutomationEnabled
	case "a2a_enabled":
		return flags.A2AEnabled
	case "fritzbox_system_enabled":
		return flags.FritzBoxSystemEnabled
	case "fritzbox_network_enabled":
		return flags.FritzBoxNetworkEnabled
	case "fritzbox_telephony_enabled":
		return flags.FritzBoxTelephonyEnabled
	case "fritzbox_smarthome_enabled":
		return flags.FritzBoxSmartHomeEnabled
	case "fritzbox_storage_enabled":
		return flags.FritzBoxStorageEnabled
	case "fritzbox_tv_enabled":
		return flags.FritzBoxTVEnabled
	case "adguard_enabled":
		return flags.AdGuardEnabled
	case "uptime_kuma_enabled":
		return flags.UptimeKumaEnabled
	case "grafana_enabled":
		return flags.GrafanaEnabled
	case "jellyfin_enabled":
		return flags.JellyfinEnabled
	case "truenas_enabled":
		return flags.TrueNASEnabled
	case "telnyx_enabled":
		return flags.TelnyxEnabled
	case "journal_enabled":
		return flags.JournalEnabled
	case "specialists_available":
		return flags.SpecialistsAvailable
	case "minimax_tts_enabled":
		return flags.MiniMaxTTSEnabled
	default:
		return false
	}
}

// guideConditionsAllow decides whether a tool manual may be injected dynamically.
// Nil flags skips enforcement (explicit discover_tools lookups).
func guideConditionsAllow(conditions []string, flags *ContextFlags) bool {
	if len(conditions) == 0 || flags == nil {
		return true
	}
	return anyPromptConditionMatches(conditions, flags)
}

// moduleConditionsAllow decides whether a static prompt module should load.
// Nil flags always denies conditioned modules.
func moduleConditionsAllow(conditions []string, flags *ContextFlags) bool {
	if flags == nil {
		return false
	}
	return anyPromptConditionMatches(conditions, flags)
}

func anyPromptConditionMatches(conditions []string, flags *ContextFlags) bool {
	for _, cond := range conditions {
		if matchPromptCondition(cond, flags) {
			return true
		}
	}
	return false
}

func (m *PromptModule) ShouldInclude(flags *ContextFlags) bool {
	// Mandatory tag always wins
	for _, tag := range m.Metadata.Tags {
		if tag == "mandatory" {
			return true
		}
	}

	// If no conditions, check if it's "core"
	if len(m.Metadata.Conditions) == 0 {
		for _, tag := range m.Metadata.Tags {
			if tag == "core" {
				return true
			}
		}
		return false
	}

	return moduleConditionsAllow(m.Metadata.Conditions, flags)
}

func evictGuideCacheLocked() {
	if len(guideCache) <= 1000 {
		return
	}
	// Coarse eviction: drop roughly half of the cache to avoid a full reset.
	target := len(guideCache) / 2
	for k := range guideCache {
		delete(guideCache, k)
		if len(guideCache) <= target {
			break
		}
	}
}

func parseToolGuideRaw(raw string) (content string, conditions []string) {
	mod, err := parsePromptModule(raw)
	if err != nil {
		return strings.TrimSpace(raw), nil
	}
	return mod.Content, mod.Metadata.Conditions
}

// readToolGuide reads a tool guide file with caching.
// Guides exceeding the token limit are truncated to prevent prompt bloat.
// It first tries the on-disk path (allowing user overrides), then falls back
// to the embedded FS baked into the binary. When flags is nil, frontmatter
// conditions are not enforced (used by explicit discover_tools lookups).
func readToolGuide(path string, flags *ContextFlags) (string, bool) {
	const maxGuideTokens = 2048

	if content, ok := activeToolGuideOverride(path); ok {
		if flags != nil {
			conditions, sourceFound := loadToolGuideConditions(path)
			if !sourceFound || !guideConditionsAllow(conditions, flags) {
				return "", false
			}
		}
		return truncateGuide(content, maxGuideTokens), true
	}

	guideCacheMu.RLock()
	cached, ok := guideCache[path]
	guideCacheMu.RUnlock()

	if ok {
		info, err := os.Stat(path)
		if err == nil && !info.ModTime().After(cached.mtime) {
			if !guideConditionsAllow(cached.conditions, flags) {
				return "", false
			}
			return cached.content, true
		}
		// If the disk file disappeared but we have a cache entry from embed,
		// the zero mtime sentinel means "from embed, always valid".
		if cached.mtime.IsZero() {
			if !guideConditionsAllow(cached.conditions, flags) {
				return "", false
			}
			return cached.content, true
		}
	}

	// 1. Try on-disk file first (user overrides)
	data, err := os.ReadFile(path)
	fromEmbed := false
	if err != nil {
		// 2. Fallback: extract relative embed path (e.g. "tools_manuals/docker.md")
		data, ok = readToolGuideEmbed(path)
		if !ok {
			return "", false
		}
		fromEmbed = true
	}

	body, conditions := parseToolGuideRaw(string(data))
	content := truncateGuide(body, maxGuideTokens)
	entry := guideCacheEntry{content: content, conditions: conditions}
	if !fromEmbed {
		if info, statErr := os.Stat(path); statErr == nil {
			entry.mtime = info.ModTime()
		}
	}
	guideCacheMu.Lock()
	evictGuideCacheLocked()
	guideCache[path] = entry
	guideCacheMu.Unlock()

	if !guideConditionsAllow(conditions, flags) {
		return "", false
	}
	return content, true
}

func loadToolGuideConditions(path string) (conditions []string, found bool) {
	if data, err := os.ReadFile(path); err == nil {
		_, conditions = parseToolGuideRaw(string(data))
		return conditions, true
	}
	if data, ok := readToolGuideEmbed(path); ok {
		_, conditions = parseToolGuideRaw(string(data))
		return conditions, true
	}
	return nil, false
}

func activeToolGuideOverride(path string) (string, bool) {
	if GetActivePromptOverrides == nil {
		return "", false
	}

	toolName := toolGuideNameFromPath(path)
	if toolName == "" {
		return "", false
	}

	overrides := GetActivePromptOverrides()
	if len(overrides) == 0 {
		return "", false
	}

	for name, raw := range overrides {
		if normalizeToolGuideOverrideName(name) != toolName {
			continue
		}
		return SanitizeToolGuideOverride(raw)
	}

	return "", false
}

func toolGuideNameFromPath(path string) string {
	base := filepath.Base(filepath.Clean(path))
	ext := filepath.Ext(base)
	if ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return normalizeToolGuideOverrideName(base)
}

func normalizeToolGuideOverrideName(name string) string {
	name = filepath.Base(filepath.ToSlash(strings.TrimSpace(name)))
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return strings.ToLower(name)
}

// SanitizeToolGuideOverride removes optimizer reasoning traces and rejects
// prompt echoes before an optimized manual can enter the runtime prompt.
func SanitizeToolGuideOverride(raw string) (string, bool) {
	content := strings.TrimSpace(security.StripThinkingTags(raw))
	content = stripSingleMarkdownFence(content)
	if content == "" {
		return "", false
	}

	lower := strings.ToLower(content)
	rejectMarkers := []string{
		"<think",
		"<thinking",
		"</think",
		"</thinking",
		"<current_manual",
		"</current_manual>",
		"recent execution errors:",
		"reply only with the new markdown manual",
	}
	for _, marker := range rejectMarkers {
		if strings.Contains(lower, marker) {
			return "", false
		}
	}

	return content, true
}

func stripSingleMarkdownFence(content string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "```") {
		return content
	}

	lines := strings.Split(content, "\n")
	if len(lines) < 2 || !strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		return content
	}

	return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
}

// ReadToolGuide is the exported variant of readToolGuide.
// It reads and caches a tool guide by its filesystem path without enforcing
// frontmatter conditions so explicit discover_tools lookups still work.
func ReadToolGuide(path string) (string, bool) {
	return readToolGuide(path, nil)
}

// readToolGuideEmbed tries to load a tool guide from the embedded FS.
// It derives the embed-relative path by finding the "tools_manuals" segment.
func readToolGuideEmbed(osPath string) ([]byte, bool) {
	// Normalise to forward slashes so the split works on Windows too.
	norm := filepath.ToSlash(osPath)
	const marker = "tools_manuals/"
	idx := strings.LastIndex(norm, marker)
	if idx < 0 {
		return nil, false
	}
	embedPath := norm[idx:] // e.g. "tools_manuals/docker.md"
	data, err := fs.ReadFile(promptsembed.FS, embedPath)
	if err != nil {
		return nil, false
	}
	return data, true
}

// truncateGuide trims whitespace and limits content to maxTokens using the
// shared tiktoken-based CountTokens function.  When truncation is needed the
// content is cut at the last newline boundary before the limit so that
// sentences/sections remain intact.
func truncateGuide(raw string, maxTokens int) string {
	content := strings.TrimSpace(raw)
	if CountTokens(content) <= maxTokens {
		return content
	}

	// Binary search for the longest rune-length prefix that fits in maxTokens.
	lo, hi := 0, len(content)
	for lo < hi {
		mid := lo + (hi-lo)/2
		// Step back to a rune boundary
		for mid > lo && !isRuneBoundary(content, mid) {
			mid--
		}
		if CountTokens(content[:mid]) <= maxTokens {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	cut := lo - 1
	if cut <= 0 {
		return content[:min(200, len(content))] + "\n[...truncated]"
	}

	// Try to cut at the last newline before the cut point for cleaner output.
	if idx := strings.LastIndex(content[:cut], "\n"); idx > cut/2 {
		cut = idx
	}
	return content[:cut] + "\n[...truncated]"
}

// isRuneBoundary reports whether byte index i is at the start of a UTF-8 rune in s.
func isRuneBoundary(s string, i int) bool {
	if i == 0 || i == len(s) {
		return true
	}
	return (s[i] & 0xC0) != 0x80
}

// isToolPathSafe returns true when path is confirmed to be within baseDir,
// preventing path traversal via crafted tool names or injected index paths.
// path must have already been cleaned with filepath.Clean before calling.
func isToolPathSafe(path, baseDir string) bool {
	if baseDir == "" {
		return false
	}
	cleanPath, err := normalizeToolGuidePathForContainment(path)
	if err != nil {
		return false
	}
	cleanBase, err := normalizeToolGuidePathForContainment(baseDir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil || rel == "." || rel == "" {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel)
}

func normalizeToolGuidePathForContainment(path string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		abs = strings.ToLower(abs)
	}
	return filepath.Clean(abs), nil
}

// PrepareDynamicGuides orchestrates explicit, semantic, statistical, and recency-based prediction to find relevant tool documents.
// maxTotalGuides caps the number of guides returned (default: 5 if <= 0).
type DynamicGuideStrategy struct {
	PreferSemantics              bool
	DisableRecentHeuristics      bool
	DisableStatisticalHeuristics bool
	DisableFrequencyHeuristics   bool
	// AllowedTools limits guide loading to currently enabled tool names.
	// Empty means "no allowlist" for non-native/text-only sessions.
	AllowedTools []string
	// SkipTools is a list of tool names whose guides should be skipped
	// (typically tools that already have native OpenAI function schemas).
	SkipTools []string
	// Flags supplies runtime feature toggles for tools_manuals frontmatter conditions.
	Flags *ContextFlags
}

func PrepareDynamicGuides(vdb memory.VectorDB, stm *memory.SQLiteMemory, userQuery, lastTool, toolsDir string, recentTools []string, explicitTools []string, maxTotalGuides int, logger *slog.Logger) []string {
	return PrepareDynamicGuidesWithStrategy(vdb, stm, userQuery, lastTool, toolsDir, recentTools, explicitTools, maxTotalGuides, DynamicGuideStrategy{}, logger)
}

// PrepareDynamicGuidesWithStrategy behaves like PrepareDynamicGuides but allows
// the caller to selectively down-weight heuristic sources for weaker models.
func PrepareDynamicGuidesWithStrategy(vdb memory.VectorDB, stm *memory.SQLiteMemory, userQuery, lastTool, toolsDir string, recentTools []string, explicitTools []string, maxTotalGuides int, strategy DynamicGuideStrategy, logger *slog.Logger) []string {
	if maxTotalGuides <= 0 {
		maxTotalGuides = 5
	}
	var guides []string
	guideMap := make(map[string]bool)
	guideMapKey := func(path string) string {
		return strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
	}

	// Build sets before any source contributes guides. Native sessions pass an
	// allowlist of enabled tools; this prevents disabled manuals from entering
	// the prompt via stale semantic indexes or explicit guide names.
	skipSet := make(map[string]bool)
	for _, t := range strategy.SkipTools {
		if name := normalizeToolGuideOverrideName(t); name != "" {
			skipSet[name] = true
		}
	}
	allowedSet := make(map[string]bool, len(strategy.AllowedTools))
	for _, t := range strategy.AllowedTools {
		if name := normalizeToolGuideOverrideName(t); name != "" {
			allowedSet[name] = true
		}
	}
	isAllowed := func(tool string) bool {
		if len(allowedSet) == 0 {
			return true
		}
		return allowedSet[normalizeToolGuideOverrideName(tool)]
	}
	isSkipped := func(tool string) bool {
		tool = normalizeToolGuideOverrideName(tool)
		return tool == "" || !isAllowed(tool) || skipSet[tool]
	}

	// Phase Z: EXPLICIT requested tools (highest priority, injected via <workflow_plan> tag)
	for _, tool := range explicitTools {
		if len(guides) >= maxTotalGuides {
			break
		}
		if isSkipped(tool) {
			continue
		}
		cleanPath := filepath.Clean(filepath.Join(toolsDir, tool+".md"))
		if !isToolPathSafe(cleanPath, toolsDir) {
			if logger != nil {
				logger.Warn("[ToolGuides] Rejected unsafe explicit tool path", "tool", tool)
			}
			continue
		}
		guideKey := guideMapKey(cleanPath)
		if !guideMap[guideKey] {
			if content, ok := readToolGuide(cleanPath, strategy.Flags); ok {
				guides = append(guides, content)
				guideMap[guideKey] = true
			}
		}
	}

	// Helper to extract tool name from a guide path (e.g. "tools_manuals/docker.md" → "docker")
	extractToolName := func(path string) string {
		base := filepath.Base(path)
		return normalizeToolGuideOverrideName(strings.TrimSuffix(base, ".md"))
	}

	addRecentGuides := func(limit int) {
		for _, tool := range recentTools {
			if len(guides) >= limit {
				break
			}
			if isSkipped(tool) {
				continue
			}
			cleanPath := filepath.Clean(filepath.Join(toolsDir, tool+".md"))
			if !isToolPathSafe(cleanPath, toolsDir) {
				continue
			}
			guideKey := guideMapKey(cleanPath)
			if !guideMap[guideKey] {
				if content, ok := readToolGuide(cleanPath, strategy.Flags); ok {
					guides = append(guides, content)
					guideMap[guideKey] = true
				}
			}
		}
	}

	addSemanticGuides := func(limit int) {
		chromemDB, ok := vdb.(*memory.ChromemVectorDB)
		if !ok || len(guides) >= limit {
			return
		}
		paths, err := chromemDB.SearchToolGuides(userQuery, 2)
		if err != nil {
			if logger != nil {
				logger.Debug("[DynamicGuides] Semantic tool guide search unavailable", "error", err)
			}
			return
		}
		for _, p := range paths {
			if len(guides) >= limit {
				break
			}
			if isSkipped(extractToolName(p)) {
				continue
			}
			cleanPath := filepath.Clean(p)
			if !isToolPathSafe(cleanPath, toolsDir) {
				continue
			}
			guideKey := guideMapKey(cleanPath)
			if !guideMap[guideKey] {
				if content, ok := readToolGuide(cleanPath, strategy.Flags); ok {
					guides = append(guides, content)
					guideMap[guideKey] = true
				}
			}
		}
	}

	if strategy.PreferSemantics {
		addSemanticGuides(3)
		if !strategy.DisableRecentHeuristics {
			addRecentGuides(3)
		}
	} else {
		if !strategy.DisableRecentHeuristics {
			addRecentGuides(3)
		}
		addSemanticGuides(3)
	}

	// C. Statistics (Transition Graph)
	if !strategy.DisableStatisticalHeuristics && stm != nil && lastTool != "" && len(guides) < 3 {
		nextTool, err := stm.GetTopTransition(lastTool)
		if err == nil && nextTool != "" && !isSkipped(nextTool) {
			cleanPath := filepath.Clean(filepath.Join(toolsDir, nextTool+".md"))
			guideKey := guideMapKey(cleanPath)
			if isToolPathSafe(cleanPath, toolsDir) && !guideMap[guideKey] {
				if content, ok := readToolGuide(cleanPath, strategy.Flags); ok {
					guides = append(guides, content)
					guideMap[guideKey] = true
					if logger != nil {
						logger.Info("Statistically predicted next tool", "from", lastTool, "predicted", nextTool)
					}
				}
			}
		}
	}

	// C2. Global usage frequency: boost tools that are frequently used across all sessions
	if !strategy.DisableFrequencyHeuristics && len(guides) < 3 {
		for _, tool := range GetFrequentTools(3) {
			if len(guides) >= 3 {
				break
			}
			if isSkipped(tool) {
				continue
			}
			cleanPath := filepath.Clean(filepath.Join(toolsDir, tool+".md"))
			guideKey := guideMapKey(cleanPath)
			if !isToolPathSafe(cleanPath, toolsDir) || guideMap[guideKey] {
				continue
			}
			if content, ok := readToolGuide(cleanPath, strategy.Flags); ok {
				guides = append(guides, content)
				guideMap[guideKey] = true
			}
		}
	}

	// D. Limit: explicit requests get boosted allowance, capped at maxTotalGuides.
	maxGuides := maxTotalGuides
	if len(guides) > maxGuides {
		guides = guides[:maxGuides]
	}

	return guides
}

// GetCorePersonalityMeta loads and parses just the metadata for a specific core personality.
// Results are cached and invalidated when the personality file's ModTime changes.
func GetCorePersonalityMeta(promptsDir, corePersonality string) memory.PersonalityMeta {
	defaultMeta := memory.PersonalityMeta{}.Normalized()

	if corePersonality == "" {
		return defaultMeta
	}

	profilePath := filepath.Join(promptsDir, "personalities", corePersonality+".md")

	// Check cache
	metaCacheMu.RLock()
	cached, ok := metaCache[profilePath]
	metaCacheMu.RUnlock()

	if ok {
		info, err := os.Stat(profilePath)
		if err == nil && !info.ModTime().After(cached.mtime) {
			return cached.meta
		}
		if err != nil && cached.fromEmbed {
			return cached.meta
		}
	}

	data, err := os.ReadFile(profilePath)
	if err != nil {
		data, err = fs.ReadFile(promptsembed.FS, "personalities/"+corePersonality+".md")
		if err != nil {
			return defaultMeta
		}
		mod, err := parsePromptModule(string(data))
		if err != nil {
			return defaultMeta
		}
		m := mod.Metadata.Meta.Normalized()
		if err := mod.Metadata.Meta.Validate(); err != nil {
			slog.Warn("[Personality] Invalid embedded personality metadata detected",
				"profile", corePersonality, "error", err)
		}
		metaCacheMu.Lock()
		metaCache[profilePath] = metaCacheEntry{meta: m, fromEmbed: true}
		metaCacheMu.Unlock()
		return m
	}

	mod, err := parsePromptModule(string(data))
	if err != nil {
		return defaultMeta
	}

	m := mod.Metadata.Meta.Normalized()

	if err := mod.Metadata.Meta.Validate(); err != nil {
		slog.Warn("[Personality] Invalid personality metadata detected",
			"profile", corePersonality, "error", err)
	}

	// Update cache
	info, err := os.Stat(profilePath)
	if err == nil {
		metaCacheMu.Lock()
		metaCache[profilePath] = metaCacheEntry{meta: m, mtime: info.ModTime()}
		metaCacheMu.Unlock()
	}

	return m
}

// GetCorePersonalityPromptSummary returns a short summary of the active persona's
// prompt text (the markdown body without YAML frontmatter), truncated to maxLen runes.
// Used to inject persona context into emotion/inner-voice LLM prompts.
func GetCorePersonalityPromptSummary(promptsDir, corePersonality string, maxLen int) string {
	if corePersonality == "" || corePersonality == "neutral" {
		return ""
	}
	if maxLen <= 0 {
		maxLen = 300
	}

	profilePath := filepath.Join(promptsDir, "personalities", corePersonality+".md")

	var raw string
	if data, err := os.ReadFile(profilePath); err == nil {
		raw = string(data)
	} else if data, err := fs.ReadFile(promptsembed.FS, "personalities/"+corePersonality+".md"); err == nil {
		raw = string(data)
	} else {
		return ""
	}

	mod, err := parsePromptModule(raw)
	if err != nil {
		return ""
	}

	body := strings.TrimSpace(mod.Content)
	// Strip leading markdown header (e.g. "# Core Personality: Punk\n\n")
	if idx := strings.Index(body, "\n"); idx > 0 && strings.HasPrefix(body, "#") {
		body = strings.TrimSpace(body[idx+1:])
	}
	runes := []rune(body)
	if len(runes) > maxLen {
		body = string(runes[:maxLen]) + "…"
	}
	return body
}
