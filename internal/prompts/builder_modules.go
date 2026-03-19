package prompts

import (
	"aurago/internal/memory"
	promptsembed "aurago/prompts"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func parseOrFallback(filename, content string, logger *slog.Logger) PromptModule {
	mod, err := parsePromptModule(content)
	if err != nil {
		if logger != nil {
			logger.Warn("Prompt module has no valid YAML frontmatter, using raw content as fallback",
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

func loadPromptModules(dir string, logger *slog.Logger) []PromptModule {
	// --- Fast path: check cache validity (based on disk files only) ---
	promptCacheMu.RLock()
	cached, ok := promptCacheByDir[dir]
	promptCacheMu.RUnlock()

	if ok && !promptCacheStale(dir, cached.mtimes) {
		return cached.modules
	}

	// --- Slow path: embedded FS is the immutable system base; disk overlays user customizations ---
	//
	// System prompts (rules.md, tools_*.md, etc.) live only in the binary embed.
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
		data, err := fs.ReadFile(promptsembed.FS, path)
		if err != nil {
			return nil
		}
		moduleMap[path] = parseOrFallback(path, string(data), logger)
		return nil
	})

	// 2. Overlay with on-disk files (user identity.md or custom prompts override the embedded versions)
	mtimes := make(map[string]time.Time)
	if files, err := os.ReadDir(dir); err == nil {
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
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
			moduleMap[file.Name()] = parseOrFallback(file.Name(), string(data), logger)
		}
	} else if len(moduleMap) == 0 {
		logger.Error("Failed to read prompts directory and no embedded modules loaded", "path", dir, "error", err)
	}

	// Convert map to slice
	modules := make([]PromptModule, 0, len(moduleMap))
	for _, m := range moduleMap {
		modules = append(modules, m)
	}

	// Update cache
	promptCacheMu.Lock()
	promptCacheByDir[dir] = promptDirCache{modules: modules, mtimes: mtimes}
	promptCacheMu.Unlock()

	if ok {
		logger.Debug("[PromptCache] Reloaded (files changed)", "dir", dir, "count", len(modules))
	} else {
		logger.Debug("[PromptCache] Populated", "dir", dir, "count", len(modules))
	}

	return modules
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

func parsePromptModule(raw string) (*PromptModule, error) {
	// Strip UTF-8 BOM (\xEF\xBB\xBF) and leading blank lines so files saved by
	// Windows editors or tools that prepend a BOM are accepted without error.
	raw = strings.TrimPrefix(raw, "\xEF\xBB\xBF")
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
		// Also try Windows line ending
		idx = strings.Index(inner, "\n---\r\n")
	}
	if idx < 0 {
		return nil, fmt.Errorf("invalid frontmatter format")
	}

	frontmatter := inner[:idx]
	// Determine correct body offset: handle both LF and CRLF line endings
	bodyOffset := idx + 4 // skip "\n---"
	if idx+4 < len(inner) && inner[idx+4] == '\r' {
		bodyOffset = idx + 5 // skip "\n---\r"
	}
	body := inner[bodyOffset:]
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

func filterModules(modules []PromptModule, flags ContextFlags) []PromptModule {
	// Pre-allocate with estimated capacity (typically 50-70% of modules match)
	filtered := make([]PromptModule, 0, len(modules))
	for _, mod := range modules {
		if mod.ShouldInclude(flags) {
			filtered = append(filtered, mod)
		}
	}
	return filtered
}

func (m *PromptModule) ShouldInclude(flags ContextFlags) bool {
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

	// Check specific conditions
	for _, cond := range m.Metadata.Conditions {
		switch cond {
		case "is_error":
			if flags.IsErrorState {
				return true
			}
		case "requires_coding":
			if flags.RequiresCoding {
				return true
			}
		case "lifeboat":
			if flags.LifeboatEnabled {
				return true
			}
		case "maintenance":
			if flags.IsMaintenanceMode {
				return true
			}
		case "coagent":
			if flags.IsCoAgent {
				return true
			}
		case "egg":
			if flags.IsEgg {
				return true
			}
		case "main_agent":
			if !flags.IsCoAgent && !flags.IsEgg {
				return true
			}
		// Feature-specific tool conditions
		case "discord_enabled":
			if flags.DiscordEnabled {
				return true
			}
		case "email_enabled":
			if flags.EmailEnabled {
				return true
			}
		case "docker_enabled":
			if flags.DockerEnabled {
				return true
			}
		case "home_assistant_enabled":
			if flags.HomeAssistantEnabled {
				return true
			}
		case "webdav_enabled":
			if flags.WebDAVEnabled {
				return true
			}
		case "koofr_enabled":
			if flags.KoofrEnabled {
				return true
			}
		case "paperless_ngx_enabled":
			if flags.PaperlessNGXEnabled {
				return true
			}
		case "chromecast_enabled":
			if flags.ChromecastEnabled {
				return true
			}
		case "coagent_enabled":
			if flags.CoAgentEnabled {
				return true
			}
		case "google_workspace_enabled":
			if flags.GoogleWorkspaceEnabled {
				return true
			}
		case "onedrive_enabled":
			if flags.OneDriveEnabled {
				return true
			}
		case "proxmox_enabled":
			if flags.ProxmoxEnabled {
				return true
			}
		case "ollama_enabled":
			if flags.OllamaEnabled {
				return true
			}
		case "tailscale_enabled":
			if flags.TailscaleEnabled {
				return true
			}
		case "cloudflare_tunnel_enabled":
			if flags.CloudflareTunnelEnabled {
				return true
			}
		case "ansible_enabled":
			if flags.AnsibleEnabled {
				return true
			}
		case "invasion_control_enabled":
			if flags.InvasionControlEnabled {
				return true
			}
		case "github_enabled":
			if flags.GitHubEnabled {
				return true
			}
		case "mqtt_enabled":
			if flags.MQTTEnabled {
				return true
			}
		case "mcp_enabled":
			if flags.MCPEnabled {
				return true
			}
		case "meshcentral_enabled":
			if flags.MeshCentralEnabled {
				return true
			}
		case "sandbox_enabled":
			if flags.SandboxEnabled {
				return true
			}
		case "memory_enabled":
			if flags.MemoryEnabled {
				return true
			}
		case "knowledge_graph_enabled":
			if flags.KnowledgeGraphEnabled {
				return true
			}
		case "secrets_vault_enabled":
			if flags.SecretsVaultEnabled {
				return true
			}
		case "scheduler_enabled":
			if flags.SchedulerEnabled {
				return true
			}
		case "notes_enabled":
			if flags.NotesEnabled {
				return true
			}
		case "missions_enabled":
			if flags.MissionsEnabled {
				return true
			}
		case "allow_shell":
			if flags.AllowShell {
				return true
			}
		case "allow_python":
			if flags.AllowPython {
				return true
			}
		case "allow_filesystem_write":
			if flags.AllowFilesystemWrite {
				return true
			}
		case "allow_network_requests":
			if flags.AllowNetworkRequests {
				return true
			}
		case "allow_remote_shell":
			if flags.AllowRemoteShell {
				return true
			}
		case "allow_self_update":
			if flags.AllowSelfUpdate {
				return true
			}
		case "wol_enabled":
			if flags.WOLEnabled {
				return true
			}
		case "virustotal_enabled":
			if flags.VirusTotalEnabled {
				return true
			}
		case "brave_search_enabled":
			if flags.BraveSearchEnabled {
				return true
			}
		case "homepage_enabled":
			if flags.HomepageEnabled {
				return true
			}
		case "homepage_allow_local_server":
			if flags.HomepageAllowLocalServer {
				return true
			}
		case "netlify_enabled":
			if flags.NetlifyEnabled {
				return true
			}
		case "image_generation_enabled":
			if flags.ImageGenerationEnabled {
				return true
			}
		case "is_docker":
			if flags.IsDocker {
				return true
			}
		case "media_registry_enabled":
			if flags.MediaRegistryEnabled {
				return true
			}
		case "homepage_registry_enabled":
			if flags.HomepageRegistryEnabled {
				return true
			}
		case "document_creator_enabled":
			if flags.DocumentCreatorEnabled {
				return true
			}
		case "s3_enabled":
			if flags.S3Enabled {
				return true
			}
		case "web_scraper_enabled":
			if flags.WebScraperEnabled {
				return true
			}
		case "a2a_enabled":
			if flags.A2AEnabled {
				return true
			}
		}
	}

	return false
}

// readToolGuide reads a tool guide file with caching.
// Guides exceeding 8KB are truncated to prevent prompt bloat.
// It first tries the on-disk path (allowing user overrides), then falls back
// to the embedded FS baked into the binary.
func readToolGuide(path string) (string, bool) {
	const maxGuideBytes = 8192

	guideCacheMu.RLock()
	cached, ok := guideCache[path]
	guideCacheMu.RUnlock()

	if ok {
		info, err := os.Stat(path)
		if err == nil && !info.ModTime().After(cached.mtime) {
			return cached.content, true
		}
		// If the disk file disappeared but we have a cache entry from embed,
		// the zero mtime sentinel means "from embed, always valid".
		if cached.mtime.IsZero() {
			return cached.content, true
		}
	}

	// 1. Try on-disk file first (user overrides)
	data, err := os.ReadFile(path)
	if err != nil {
		// 2. Fallback: extract relative embed path (e.g. "tools_manuals/docker.md")
		data, ok = readToolGuideEmbed(path)
		if !ok {
			return "", false
		}
		content := truncateGuide(string(data), maxGuideBytes)
		guideCacheMu.Lock()
		guideCache[path] = guideCacheEntry{content: content} // zero mtime = from embed
		guideCacheMu.Unlock()
		return content, true
	}

	content := truncateGuide(string(data), maxGuideBytes)
	info, err := os.Stat(path)
	if err == nil {
		guideCacheMu.Lock()
		guideCache[path] = guideCacheEntry{content: content, mtime: info.ModTime()}
		guideCacheMu.Unlock()
	}
	return content, true
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

// truncateGuide trims whitespace and limits content to maxBytes.
func truncateGuide(raw string, maxBytes int) string {
	content := strings.TrimSpace(raw)
	if len(content) > maxBytes {
		content = content[:maxBytes] + "\n[...truncated]"
	}
	return content
}

// isToolPathSafe returns true when path is confirmed to be within baseDir,
// preventing path traversal via crafted tool names or injected index paths.
// path must have already been cleaned with filepath.Clean before calling.
func isToolPathSafe(path, baseDir string) bool {
	if baseDir == "" {
		return false
	}
	return strings.HasPrefix(path, filepath.Clean(baseDir)+string(filepath.Separator))
}

// PrepareDynamicGuides orchestrates explicit, semantic, statistical, and recency-based prediction to find relevant tool documents.
// maxTotalGuides caps the number of guides returned (default: 5 if <= 0).
func PrepareDynamicGuides(vdb memory.VectorDB, stm *memory.SQLiteMemory, userQuery, lastTool, toolsDir string, recentTools []string, explicitTools []string, maxTotalGuides int, logger *slog.Logger) []string {
	if maxTotalGuides <= 0 {
		maxTotalGuides = 5
	}
	var guides []string
	guideMap := make(map[string]bool)

	// Phase Z: EXPLICIT requested tools (highest priority, injected via <workflow_plan> tag)
	for _, tool := range explicitTools {
		if len(guides) >= maxTotalGuides {
			break
		}
		cleanPath := filepath.Clean(filepath.Join(toolsDir, tool+".md"))
		if !isToolPathSafe(cleanPath, toolsDir) {
			if logger != nil {
				logger.Warn("[ToolGuides] Rejected unsafe explicit tool path", "tool", tool)
			}
			continue
		}
		if !guideMap[cleanPath] {
			if content, ok := readToolGuide(cleanPath); ok {
				guides = append(guides, content)
				guideMap[cleanPath] = true
			}
		}
	}

	// A. Recently used tools (lazy schema injection — high priority)
	for _, tool := range recentTools {
		if len(guides) >= 3 {
			break
		}
		cleanPath := filepath.Clean(filepath.Join(toolsDir, tool+".md"))
		if !isToolPathSafe(cleanPath, toolsDir) {
			continue
		}
		if !guideMap[cleanPath] {
			if content, ok := readToolGuide(cleanPath); ok {
				guides = append(guides, content)
				guideMap[cleanPath] = true
			}
		}
	}

	// B. Semantics (ChromaDB)
	if chromemDB, ok := vdb.(*memory.ChromemVectorDB); ok && len(guides) < 3 {
		paths, err := chromemDB.SearchToolGuides(userQuery, 2)
		if err == nil {
			for _, p := range paths {
				if len(guides) >= 3 {
					break
				}
				cleanPath := filepath.Clean(p)
				if !guideMap[cleanPath] {
					if content, ok := readToolGuide(cleanPath); ok {
						guides = append(guides, content)
						guideMap[cleanPath] = true
					}
				}
			}
		} else {
			logger.Warn("Failed semantic tool guide search", "error", err)
		}
	}

	// C. Statistics (Transition Graph)
	if stm != nil && lastTool != "" && len(guides) < 3 {
		nextTool, err := stm.GetTopTransition(lastTool)
		if err == nil && nextTool != "" {
			cleanPath := filepath.Clean(filepath.Join(toolsDir, nextTool+".md"))
			if isToolPathSafe(cleanPath, toolsDir) && !guideMap[cleanPath] {
				if content, ok := readToolGuide(cleanPath); ok {
					guides = append(guides, content)
					guideMap[cleanPath] = true
					logger.Info("Statistically predicted next tool", "from", lastTool, "predicted", nextTool)
				}
			}
		}
	}

	// C2. Global usage frequency: boost tools that are frequently used across all sessions
	if len(guides) < 3 {
		for _, tool := range GetFrequentTools(3) {
			if len(guides) >= 3 {
				break
			}
			cleanPath := filepath.Clean(filepath.Join(toolsDir, tool+".md"))
			if !isToolPathSafe(cleanPath, toolsDir) || guideMap[cleanPath] {
				continue
			}
			if content, ok := readToolGuide(cleanPath); ok {
				guides = append(guides, content)
				guideMap[cleanPath] = true
			}
		}
	}

	// D. Limit: explicit requests get boosted allowance, capped at maxTotalGuides.
	maxGuides := 3 + len(explicitTools)
	if maxGuides > maxTotalGuides {
		maxGuides = maxTotalGuides
	}
	if len(guides) > maxGuides {
		guides = guides[:maxGuides]
	}

	return guides
}

// GetCorePersonalityMeta loads and parses just the metadata for a specific core personality.
// Results are cached and invalidated when the personality file's ModTime changes.
func GetCorePersonalityMeta(promptsDir, corePersonality string) memory.PersonalityMeta {
	defaultMeta := memory.PersonalityMeta{
		Volatility:               1.0,
		EmpathyBias:              1.0,
		ConflictResponse:         "neutral",
		LonelinessSusceptibility: 1.0,
		TraitDecayRate:           1.0,
	}

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
	}

	data, err := os.ReadFile(profilePath)
	if err != nil {
		return defaultMeta
	}

	mod, err := parsePromptModule(string(data))
	if err != nil {
		return defaultMeta
	}

	// Apply defaults for fields that might be 0.0 in YAML if omitted, assuming 1.0 is intended default if totally empty.
	// But yaml parser sets 0.0 for unprovided floats. We should do a quick zero-check fallback for multipliers:
	m := mod.Metadata.Meta
	if m.Volatility == 0 {
		m.Volatility = 1.0
	}
	if m.EmpathyBias == 0 {
		m.EmpathyBias = 1.0
	}
	if m.ConflictResponse == "" {
		m.ConflictResponse = "neutral"
	}
	if m.LonelinessSusceptibility == 0 {
		m.LonelinessSusceptibility = 1.0
	}
	if m.TraitDecayRate == 0 {
		m.TraitDecayRate = 1.0
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
