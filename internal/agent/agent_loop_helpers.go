package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

var nowFunc = time.Now

const memoryDedupeSessionMaxIDs = 500

var memoryDedupeSessions = struct {
	sync.Mutex
	bySession map[string]map[string]int
}{
	bySession: make(map[string]map[string]int),
}

func ShouldReloadCoreMemory(dirty bool, loadedAt time.Time, dbUpdatedAt, cachedUpdatedAt time.Time) bool {
	if dirty {
		return true
	}
	if loadedAt.IsZero() {
		return false
	}
	if dbUpdatedAt.IsZero() && !cachedUpdatedAt.IsZero() {
		return true
	}
	if !dbUpdatedAt.IsZero() && !cachedUpdatedAt.IsZero() && !dbUpdatedAt.Equal(cachedUpdatedAt) {
		return true
	}
	if nowFunc().Sub(loadedAt) > coreMemCacheTTL {
		return true
	}
	return false
}

func countMemoryPromptTelemetryTokens(flags prompts.ContextFlags, model string) int {
	sections := []string{
		flags.RetrievedMemories,
		flags.AvailableMemoryContextIndex,
		flags.AvailableKnowledgeContextIndex,
		flags.PredictedMemories,
		flags.KnowledgeContext,
		flags.ErrorPatternContext,
		flags.LearnedRulesContext,
	}
	total := 0
	for _, section := range sections {
		if strings.TrimSpace(section) == "" {
			continue
		}
		total += prompts.CountTokensForModel(section, model)
	}
	return total
}

func mergeStreamToolCallChunk(streamToolCalls map[int]*openai.ToolCall, tc openai.ToolCall) {
	idx := 0
	if tc.Index != nil {
		idx = *tc.Index
	}
	existing, ok := streamToolCalls[idx]
	if !ok {
		clone := openai.ToolCall{
			Index: tc.Index,
			ID:    tc.ID,
			Type:  tc.Type,
			Function: openai.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
		streamToolCalls[idx] = &clone
		return
	}
	if tc.ID != "" {
		existing.ID = tc.ID
	}
	if tc.Function.Name != "" {
		existing.Function.Name += tc.Function.Name
	}
	existing.Function.Arguments += tc.Function.Arguments
}

func assembleSortedStreamToolCalls(streamToolCalls map[int]*openai.ToolCall) []openai.ToolCall {
	if len(streamToolCalls) == 0 {
		return nil
	}
	keys := make([]int, 0, len(streamToolCalls))
	for idx := range streamToolCalls {
		keys = append(keys, idx)
	}
	sort.Ints(keys)
	assembledToolCalls := make([]openai.ToolCall, 0, len(keys))
	for _, idx := range keys {
		if tc := streamToolCalls[idx]; tc != nil {
			assembledToolCalls = append(assembledToolCalls, *tc)
		}
	}
	return assembledToolCalls
}

type StreamToolCallAssembler struct {
	calls map[int]*openai.ToolCall
}

func NewStreamToolCallAssembler() *StreamToolCallAssembler {
	return &StreamToolCallAssembler{calls: make(map[int]*openai.ToolCall)}
}

func (a *StreamToolCallAssembler) Merge(tc openai.ToolCall) {
	idx := 0
	if tc.Index != nil {
		idx = *tc.Index
	}
	existing, ok := a.calls[idx]
	if !ok {
		clone := openai.ToolCall{
			Index: tc.Index,
			ID:    tc.ID,
			Type:  tc.Type,
			Function: openai.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
		a.calls[idx] = &clone
		return
	}
	if tc.ID != "" {
		existing.ID = tc.ID
	}
	if tc.Function.Name != "" {
		existing.Function.Name += tc.Function.Name
	}
	existing.Function.Arguments += tc.Function.Arguments
}

func (a *StreamToolCallAssembler) Assemble() []openai.ToolCall {
	if len(a.calls) == 0 {
		return nil
	}
	keys := make([]int, 0, len(a.calls))
	for idx := range a.calls {
		keys = append(keys, idx)
	}
	sort.Ints(keys)
	result := make([]openai.ToolCall, 0, len(keys))
	for _, idx := range keys {
		if tc := a.calls[idx]; tc != nil {
			result = append(result, *tc)
		}
	}
	return result
}

type streamingAccountingState struct {
	hasProviderUsage   bool
	providerPrompt     int
	providerCompletion int
	providerCached     int
	finalized          bool
}

func (s *streamingAccountingState) recordProviderUsage(prompt, completion, cached int) {
	// Providers may send usage across multiple chunks (e.g. prompt in one chunk,
	// completion in another). Only overwrite non-zero values so earlier
	// measurements are preserved.
	if prompt > 0 {
		s.providerPrompt = prompt
	}
	if completion > 0 {
		s.providerCompletion = completion
	}
	if cached > 0 {
		s.providerCached = cached
	}
	if prompt > 0 || completion > 0 || cached > 0 {
		s.hasProviderUsage = true
	}
}

type toolGuideSearcher interface {
	SearchToolGuides(query string, topK int) ([]string, error)
}

type contextToolGuideSearcher interface {
	SearchToolGuidesContext(ctx context.Context, query string, topK int) ([]string, error)
}

var nonAlphaNumPattern = regexp.MustCompile(`[^a-z0-9]+`)
var adaptiveFamilySeedTools = map[string][]string{
	"files": {
		"filesystem", "workspace_search", "smart_file_read", "file_search", "file_reader_advanced",
		"detect_file_type", "pdf_operations", "document_creator", "media_conversion", "send_document",
	},
	"shell": {
		"execute_shell", "execute_python", "execute_sandbox", "filesystem",
		"system_metrics", "process_analyzer", "process_management",
	},
	"coding": {
		"execute_python", "execute_sandbox", "execute_shell", "document_creator",
	},
	"web": {
		"ddg_search", "api_request", "web_scraper", "site_crawler", "web_capture", "document_creator",
	},
	"deployment": {
		"homepage_deploy", "homepage_project", "homepage_file", "homepage_quality", "homepage_git", "homepage_registry", "netlify",
	},
	"network": {
		"network_ping", "dns_lookup", "port_scanner", "mdns_scan", "upnp_scan",
	},
	"infra": {
		"docker", "proxmox", "tailscale", "github", "ansible", "remote_execution",
		"invasion_nests", "invasion_tasks", "invasion_artifacts", "execute_shell", "system_metrics", "process_analyzer",
	},
	"communication": {
		"fetch_email", "send_email", "send_document", "send_audio", "send_video", "send_youtube_video",
	},
	"automation": {
		"cron_scheduler", "follow_up", "manage_missions", "co_agent",
	},
	"media": {
		"media_registry", "media_conversion", "send_document", "send_audio", "send_video", "send_youtube_video", "send_image", "tts",
		"transcribe_audio", "generate_image", "generate_music", "generate_video", "chromecast",
	},
}

var adaptiveToolNeighbors = map[string][]string{
	// Web & Hosting
	"homepage_project":  {"homepage_file", "homepage_quality", "homepage_registry"},
	"homepage_file":     {"homepage_project", "homepage_quality", "homepage_git"},
	"homepage_quality":  {"homepage_file", "homepage_deploy"},
	"homepage_deploy":   {"homepage_quality", "netlify", "homepage_registry"},
	"homepage_git":      {"homepage_file", "homepage_registry"},
	"netlify":           {"homepage_deploy", "homepage_registry"},
	"homepage_registry": {"homepage_project", "homepage_deploy", "homepage_git"},

	// File System & Editing
	"filesystem":           {"workspace_search", "file_search", "file_reader_advanced", "file_editor", "manage_memory"},
	"workspace_search":     {"filesystem", "file_search", "file_reader_advanced", "smart_file_read"},
	"smart_file_read":      {"filesystem", "workspace_search", "file_reader_advanced", "file_search"},
	"file_search":          {"filesystem", "workspace_search", "file_reader_advanced"},
	"file_reader_advanced": {"filesystem", "workspace_search", "file_search", "smart_file_read"},
	"file_editor":          {"filesystem", "json_editor", "yaml_editor", "toml_editor", "xml_editor"},
	"json_editor":          {"filesystem", "file_editor"},
	"yaml_editor":          {"filesystem", "file_editor"},
	"toml_editor":          {"filesystem", "file_editor"},
	"xml_editor":           {"filesystem", "file_editor"},

	// Code & Execution
	"certificate_manager":  {"filesystem", "api_request"},
	"execute_shell":        {"filesystem", "file_editor", "workspace_search", "file_search", "media_registry", "send_document", "document_creator"},
	"execute_python":       {"filesystem", "execute_sandbox", "media_registry", "send_document", "document_creator"},
	"execute_sandbox":      {"filesystem", "execute_python"},
	"ansible":              {"ssh_exec", "query_inventory", "execute_shell", "filesystem"},
	"remote_execution":     {"transfer_remote_file", "ssh_exec", "execute_shell"},
	"transfer_remote_file": {"remote_execution", "filesystem", "ssh_exec"},

	// Media & Documents
	"document_creator":   {"media_registry", "media_conversion", "send_document", "filesystem"},
	"media_registry":     {"document_creator", "media_conversion", "send_document", "filesystem", "generate_image", "generate_video", "send_image", "send_video", "tts", "send_audio", "generate_music"},
	"media_conversion":   {"media_registry", "document_creator", "image_processing", "transcribe_audio", "send_audio", "send_video", "send_image", "filesystem"},
	"send_document":      {"media_registry", "document_creator"},
	"send_image":         {"media_registry", "generate_image"},
	"send_audio":         {"media_registry", "tts", "generate_music"},
	"send_video":         {"media_registry", "generate_video", "media_conversion"},
	"send_youtube_video": {"send_video"},
	"generate_image":     {"media_registry", "send_image", "analyze_image"},
	"generate_music":     {"media_registry", "send_audio", "tts"},
	"generate_video":     {"media_registry", "media_conversion", "send_video"},
	"analyze_image":      {"generate_image", "media_registry"},
	"tts":                {"media_registry", "send_audio"},
	"transcribe_audio":   {"filesystem", "media_registry"},
	"pdf_operations":     {"detect_file_type", "filesystem", "document_creator"},
	"image_processing":   {"detect_file_type", "filesystem", "media_registry"},

	// Docker & Infrastructure
	"docker":          {"docker_compose", "execute_shell", "filesystem"},
	"docker_compose":  {"docker", "execute_shell", "filesystem"},
	"query_inventory": {"ssh_exec", "register_device", "network_ping"},
	"register_device": {"query_inventory", "network_ping", "ssh_exec"},
	"ssh_exec":        {"query_inventory", "execute_shell", "filesystem", "ansible"},
	"proxmox":         {"ssh_exec", "query_inventory", "network_ping"},
	"truenas":         {"ssh_exec", "query_inventory", "network_ping"},

	// Memory & Knowledge
	"manage_memory":   {"query_memory", "remember", "knowledge_graph", "cheatsheet"},
	"query_memory":    {"manage_memory", "remember", "knowledge_graph"},
	"remember":        {"manage_memory", "query_memory", "knowledge_graph"},
	"knowledge_graph": {"manage_memory", "query_memory", "remember"},
	"cheatsheet":      {"manage_memory", "query_memory", "remember"},

	// Network & Scanning
	"network_ping":      {"dns_lookup", "port_scanner", "mdns_scan", "query_inventory", "mac_lookup"},
	"dns_lookup":        {"network_ping", "whois_lookup"},
	"port_scanner":      {"network_ping", "query_inventory"},
	"mdns_scan":         {"network_ping", "upnp_scan"},
	"mac_lookup":        {"network_ping", "mdns_scan"},
	"whois_lookup":      {"dns_lookup", "network_ping"},
	"upnp_scan":         {"mdns_scan", "network_ping"},
	"tailscale":         {"network_ping", "query_inventory"},
	"cloudflare_tunnel": {"network_ping", "dns_lookup"},

	// Web Scraping & QA
	"web_scraper":        {"site_crawler", "web_capture", "web_performance_audit", "document_creator"},
	"site_crawler":       {"web_scraper", "web_capture"},
	"web_capture":        {"web_scraper", "site_crawler", "web_performance_audit"},
	"browser_automation": {"web_capture", "form_automation", "analyze_image", "filesystem"},
	"site_monitor":       {"web_scraper", "network_ping"},

	// SQL & Databases
	"sql_query":              {"manage_sql_connections", "filesystem"},
	"manage_sql_connections": {"sql_query"},

	// Communication & Messaging
	"fetch_email":           {"send_email", "list_email_accounts"},
	"send_email":            {"fetch_email", "list_email_accounts"},
	"list_email_accounts":   {"fetch_email", "send_email"},
	"fetch_discord":         {"send_discord", "list_discord_channels"},
	"send_discord":          {"fetch_discord", "list_discord_channels"},
	"list_discord_channels": {"fetch_discord", "send_discord"},

	// Telephony & SMS
	"telnyx_sms":    {"telnyx_call", "telnyx_manage"},
	"telnyx_call":   {"telnyx_sms", "telnyx_manage"},
	"telnyx_manage": {"telnyx_sms", "telnyx_call"},

	// MQTT & Smart Home
	"mqtt_publish":       {"mqtt_subscribe", "mqtt_get_messages"},
	"mqtt_subscribe":     {"mqtt_publish", "mqtt_unsubscribe", "mqtt_get_messages"},
	"mqtt_unsubscribe":   {"mqtt_subscribe", "mqtt_get_messages"},
	"mqtt_get_messages":  {"mqtt_publish", "mqtt_subscribe"},
	"home_assistant":     {"fritzbox_smarthome", "chromecast"},
	"fritzbox_smarthome": {"home_assistant"},

	// Fritzbox Suite
	"fritzbox_system":  {"fritzbox_network", "fritzbox_storage", "fritzbox_telephony"},
	"fritzbox_network": {"fritzbox_system", "network_ping"},
	"fritzbox_storage": {"fritzbox_system", "filesystem"},

	// Webhooks
	"call_webhook":             {"manage_outgoing_webhooks", "manage_webhooks"},
	"manage_outgoing_webhooks": {"call_webhook"},
	"manage_webhooks":          {"call_webhook"},

	// Cloud Storage
	"google_workspace": {"filesystem", "document_creator"},
	"onedrive":         {"filesystem", "document_creator"},
	"koofr":            {"filesystem"},

	// System & OS
	"process_analyzer":   {"process_management", "system_metrics"},
	"process_management": {"process_analyzer", "system_metrics"},
	"system_metrics":     {"process_analyzer"},

	// Skills Management
	"list_skills":                {"execute_skill", "list_skill_templates", "get_skill_documentation"},
	"execute_skill":              {"list_skills", "get_skill_documentation"},
	"list_skill_templates":       {"create_skill_from_template", "list_skills"},
	"create_skill_from_template": {"list_skill_templates", "save_tool", "execute_skill", "set_skill_documentation"},
	"get_skill_documentation":    {"execute_skill", "list_skills"},
	"set_skill_documentation":    {"get_skill_documentation", "execute_skill"},
	"save_tool":                  {"execute_skill", "filesystem"},

	// Agents & Teams
	"co_agent":           {"invasion_tasks", "follow_up"},
	"invasion_nests":     {"invasion_tasks", "register_device"},
	"invasion_tasks":     {"invasion_nests", "invasion_artifacts", "co_agent"},
	"invasion_artifacts": {"invasion_tasks"},
}

// splitCSV splits a comma-separated value string into a trimmed, non-empty slice.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// isSystemSecret returns true if the given vault key belongs to a system/tool handler
// and therefore must not be readable by the agent via the secrets_vault tool.
// Single source of truth: delegates to tools.IsPythonAccessibleSecret so that the
// agent-read block list and the Python-injection block list are always identical.
func isSystemSecret(key string) bool {
	return !tools.IsPythonAccessibleSecret(key)
}

// isProtectedSystemPath returns true when the given path refers to a system-sensitive
// file that the agent must never read or write via the filesystem tool:
//   - The active config.yaml
//   - The vault file (vault.bin) and its lock
//   - All configured SQLite database files + WAL/SHM journals
//   - Any file named .env or ending in .env
//
// rawPath may be absolute or relative; relative paths are resolved against workspaceDir.
func isProtectedSystemPath(rawPath, workspaceDir string, cfg *config.Config) bool {
	if rawPath == "" {
		return false
	}

	// Resolve to absolute path
	var abs string
	if filepath.IsAbs(rawPath) {
		abs = filepath.Clean(rawPath)
	} else {
		abs = filepath.Clean(filepath.Join(workspaceDir, rawPath))
	}

	// Block .env files by name regardless of location
	base := strings.ToLower(filepath.Base(abs))
	if base == ".env" || strings.HasSuffix(base, ".env") {
		return true
	}

	// Build list of protected absolute paths from config
	vaultBase := filepath.Join(cfg.Directories.DataDir, "vault.bin")
	protected := append([]string{
		cfg.ConfigPath,
		vaultBase,
		vaultBase + ".lock",
	}, config.SQLiteProtectedPaths(cfg)...)

	for _, p := range protected {
		if p == "" {
			continue
		}
		cleanP := filepath.Clean(p)
		if abs == cleanP {
			return true
		}
		// Resolve symlinks on the stored path (covers Linux /proc/ or mount aliases)
		if resolved, err := filepath.EvalSymlinks(cleanP); err == nil {
			if abs == filepath.Clean(resolved) {
				return true
			}
		}
	}
	return false
}

func normalizeAdaptiveIntentText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphaNumPattern.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

func collectRecentUserIntentText(messages []openai.ChatCompletionMessage, maxMessages, maxChars int) string {
	if maxMessages <= 0 {
		maxMessages = 4
	}
	if maxChars <= 0 {
		maxChars = 800
	}

	parts := make([]string, 0, maxMessages)
	for i := len(messages) - 1; i >= 0 && len(parts) < maxMessages; i-- {
		if messages[i].Role != openai.ChatMessageRoleUser {
			continue
		}
		text := strings.TrimSpace(messageText(messages[i]))
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}

	joined := strings.TrimSpace(strings.Join(parts, "\n"))
	if len(joined) <= maxChars {
		return joined
	}
	return joined[len(joined)-maxChars:]
}

func extractIntentMatchedTools(userQuery string, availableTools []string) []string {
	normalizedQuery := normalizeAdaptiveIntentText(userQuery)
	if normalizedQuery == "" || len(availableTools) == 0 {
		return nil
	}

	type candidate struct {
		name     string
		priority int
	}

	matches := make([]candidate, 0)
	seen := make(map[string]bool, len(availableTools))

	for _, tool := range availableTools {
		if tool == "" || seen[tool] {
			continue
		}
		normalizedTool := normalizeAdaptiveIntentText(tool)
		if normalizedTool == "" {
			continue
		}

		priority := -1
		switch {
		case strings.Contains(normalizedQuery, normalizedTool):
			priority = 0
		default:
			tokens := strings.Fields(normalizedTool)
			if len(tokens) == 1 {
				if len(tokens[0]) >= 4 && strings.Contains(normalizedQuery, tokens[0]) {
					priority = 1
				}
			} else {
				matched := 0
				for _, token := range tokens {
					if len(token) >= 3 && strings.Contains(normalizedQuery, token) {
						matched++
					}
				}
				if matched == len(tokens) {
					priority = 1
				} else if matched >= 2 {
					priority = 2
				}
			}
		}

		if priority >= 0 {
			seen[tool] = true
			matches = append(matches, candidate{name: tool, priority: priority})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].priority != matches[j].priority {
			return matches[i].priority < matches[j].priority
		}
		return matches[i].name < matches[j].name
	})

	result := make([]string, 0, len(matches))
	for _, match := range matches {
		result = append(result, match.name)
	}
	return result
}

func adaptiveFamilySeedsForQuery(userQuery string) []string {
	family := inferToolFamilyFromQuery(userQuery)
	out := make([]string, 0, 8)
	seen := make(map[string]bool)
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	if family != "" {
		for _, seed := range adaptiveFamilySeedTools[family] {
			add(seed)
		}
	}

	q := normalizeAdaptiveIntentText(userQuery)
	if isResourceUsageIntent(q) {
		add("system_metrics")
		add("process_analyzer")
		add("execute_shell")
	}
	if isContainerIntent(q) {
		add("docker")
		add("execute_shell")
	}
	if isMCPIntent(q) {
		add("mcp_call")
	}
	if isComposioIntent(q) {
		add("composio_call")
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func isResourceUsageIntent(normalizedQuery string) bool {
	if normalizedQuery == "" {
		return false
	}
	resourceTerms := []string{
		"ram", "cpu", "memory usage", "mem usage", "resource usage",
		"resources", "ressourcen", "verbrauch", "auslastung",
		"speicherverbrauch", "arbeitsspeicher", "speicher auslastung",
		"wieviel speicher", "wie viel speicher", "wieviel ram", "wie viel ram",
	}
	for _, term := range resourceTerms {
		if strings.Contains(normalizedQuery, term) {
			return true
		}
	}
	return false
}

func isContainerIntent(normalizedQuery string) bool {
	if normalizedQuery == "" {
		return false
	}
	containerTerms := []string{"docker", "container", "containers", "compose"}
	for _, term := range containerTerms {
		if strings.Contains(normalizedQuery, term) {
			return true
		}
	}
	return false
}

func isMCPIntent(normalizedQuery string) bool {
	if normalizedQuery == "" {
		return false
	}
	mcpTerms := []string{"mcp", "model context protocol"}
	for _, term := range mcpTerms {
		if strings.Contains(normalizedQuery, term) {
			return true
		}
	}
	return false
}

func isComposioIntent(normalizedQuery string) bool {
	if normalizedQuery == "" {
		return false
	}
	composioTerms := []string{"composio", "integration", "integrations", "gmail", "google mail", "googlemail", "g mail"}
	for _, term := range composioTerms {
		if strings.Contains(normalizedQuery, term) {
			return true
		}
	}
	return false
}

func cacheAwareAdaptiveAlwaysInclude(userQuery string, alwaysInclude []string, schemas []openai.Tool) []string {
	seeds := adaptiveFamilySeedsForQuery(userQuery)
	if len(seeds) == 0 {
		return alwaysInclude
	}

	available := make(map[string]bool, len(schemas))
	for _, schema := range schemas {
		if schema.Function != nil && schema.Function.Name != "" {
			available[schema.Function.Name] = true
		}
	}

	out := make([]string, 0, len(alwaysInclude)+len(seeds))
	out = append(out, alwaysInclude...)
	for _, seed := range seeds {
		if available[seed] {
			out = append(out, seed)
		}
	}
	return out
}

func outputRefAdaptiveAlwaysInclude(messages []openai.ChatCompletionMessage, alwaysInclude []string) []string {
	for _, name := range alwaysInclude {
		if name == "read_tool_output" {
			return alwaysInclude
		}
	}
	for _, msg := range messages {
		text := messageText(msg)
		if strings.Contains(text, "output_ref") || strings.Contains(text, "toolout_") {
			out := make([]string, 0, len(alwaysInclude)+1)
			out = append(out, alwaysInclude...)
			out = append(out, "read_tool_output")
			return out
		}
	}
	return alwaysInclude
}

func expandAdaptiveAlwaysInclude(cfg *config.Config, alwaysInclude []string) []string {
	if cfg == nil {
		return alwaysInclude
	}

	seen := make(map[string]bool, len(alwaysInclude)+2)
	out := make([]string, 0, len(alwaysInclude)+2)
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}

	for _, name := range alwaysInclude {
		for _, expanded := range expandAdaptiveAlwaysIncludeAlias(name) {
			add(expanded)
		}
	}
	for _, name := range adaptiveHardAlwaysInclude(cfg) {
		add(name)
	}

	return out
}

func channelAdaptiveAlwaysInclude(runCfg RunConfig, alwaysInclude []string, ff ToolFeatureFlags) []string {
	out := make([]string, 0, len(alwaysInclude)+4)
	out = append(out, alwaysInclude...)
	messageSource := strings.ToLower(strings.TrimSpace(runCfg.MessageSource))
	if messageSource == "homepage_studio" {
		out = append(out, "homepage_project", "homepage_file", "homepage_quality", "homepage_deploy", "homepage_git", "homepage_registry")
		return out
	}
	if messageSource != "virtual_desktop_chat" {
		return out
	}
	out = append(out, "question_user")
	if ff.VirtualDesktopEnabled {
		out = append(out, "virtual_desktop_files", "virtual_desktop_apps", "virtual_desktop_widgets")
	}
	if ff.OfficeDocumentEnabled {
		out = append(out, "office_document")
	}
	if ff.OfficeWorkbookEnabled {
		out = append(out, "office_workbook")
	}
	return out
}

func adaptiveHardAlwaysInclude(cfg *config.Config) []string {
	return hardAlwaysToolNames(cfg)
}

func expandAdaptiveAlwaysIncludeAlias(name string) []string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch normalized {
	case "":
		return nil
	case "shell":
		return []string{"execute_shell"}
	case "python":
		return []string{"execute_python"}
	default:
		return []string{strings.TrimSpace(name)}
	}
}

// searchToolGuidesWithTimeout calls SearchToolGuides with a hard outer timeout.
// SearchToolGuides has a 30-second internal timeout, which can block the first
// agent turn during cold-start embedding computation. This wrapper limits the
// wait and falls back to intent/usage-based ordering on timeout.
func searchToolGuidesWithTimeout(gs toolGuideSearcher, query string, limit int, timeout time.Duration, logger *slog.Logger) []string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if contextSearcher, ok := gs.(contextToolGuideSearcher); ok {
		paths, err := contextSearcher.SearchToolGuidesContext(ctx, query, limit)
		if err != nil {
			if logger != nil {
				logger.Debug("[AdaptiveTools] Semantic tool search unavailable", "error", err)
			}
			return nil
		}
		return paths
	}

	type result struct {
		paths []string
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		paths, err := gs.SearchToolGuides(query, limit)
		ch <- result{paths, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			if logger != nil {
				logger.Debug("[AdaptiveTools] Semantic tool search unavailable", "error", r.err)
			}
			return nil
		}
		return r.paths
	case <-time.After(timeout):
		if logger != nil {
			logger.Debug("[AdaptiveTools] Semantic tool search timed out, falling back to intent/usage ordering")
		}
		return nil
	}
}

func buildAdaptiveToolPriority(schemas []openai.Tool, weightedUsage []string, userQuery string, guideSearcher toolGuideSearcher, logger *slog.Logger) []string {
	available := make([]string, 0, len(schemas))
	availableSet := make(map[string]bool, len(schemas))
	for _, schema := range schemas {
		if schema.Function == nil || schema.Function.Name == "" {
			continue
		}
		name := schema.Function.Name
		if !availableSet[name] {
			available = append(available, name)
			availableSet[name] = true
		}
	}

	prioritized := make([]string, 0, len(weightedUsage)+8)
	seen := make(map[string]bool, len(availableSet))
	add := func(name string) {
		if name == "" || seen[name] || !availableSet[name] {
			return
		}
		seen[name] = true
		prioritized = append(prioritized, name)
	}

	if guideSearcher != nil && strings.TrimSpace(userQuery) != "" {
		paths := searchToolGuidesWithTimeout(guideSearcher, userQuery, 4, 5*time.Second, logger)
		for _, path := range paths {
			name := strings.TrimSuffix(filepath.Base(filepath.Clean(path)), filepath.Ext(path))
			add(name)
		}
	}

	for _, tool := range adaptiveFamilySeedsForQuery(userQuery) {
		add(tool)
	}

	for _, tool := range extractIntentMatchedTools(userQuery, available) {
		add(tool)
	}

	seedCount := len(prioritized)
	for i := 0; i < seedCount; i++ {
		for _, neighbor := range adaptiveToolNeighbors[prioritized[i]] {
			add(neighbor)
		}
	}

	for _, tool := range weightedUsage {
		add(tool)
	}

	return prioritized
}

// isToolError returns true if the tool result content indicates an error.
// Used for tool usage tracking to distinguish successes from failures.
func isToolError(resultContent string) bool {
	if strings.Contains(resultContent, `"status": "error"`) ||
		strings.Contains(resultContent, `"status":"error"`) ||
		strings.Contains(resultContent, `[EXECUTION ERROR]`) {
		return true
	}
	// Sandbox/shell failures with non-zero exit code
	if strings.Contains(resultContent, `"exit_code":`) &&
		!strings.Contains(resultContent, `"exit_code": 0`) &&
		!strings.Contains(resultContent, `"exit_code":0`) {
		return true
	}
	return false
}

// toolResultFollowUpContent returns the message that should be fed back into the
// LLM after a non-native tool call. Most tools can reuse the raw output, but TTS
// in voice mode is special: feeding the raw success payload back as a user turn
// makes the model think a new user event happened, which can trigger another TTS
// call and create a self-sustaining voice loop.
func toolResultFollowUpContent(tc ToolCall, resultContent string, voiceModeActive bool) string {
	if tc.Action != "tts" || !voiceModeActive || isToolError(resultContent) {
		return resultContent
	}

	return "Internal note: the previous `tts` call succeeded and the audio has already been played to the user. This is not a new user message. Do not call `tts` again for the same reply. If your spoken reply is complete, finish with `<done/>`. If you still need on-screen text, provide it once and end with `<done/>`."
}

// extractErrorMessage pulls the error message from a tool output for error learning.
func extractErrorMessage(resultContent string) string {
	// Try to extract from JSON "message" field
	prefix := strings.TrimPrefix(resultContent, "Tool Output: ")
	prefix = strings.TrimPrefix(prefix, "[Tool Output]\n")
	var parsed struct {
		Message string `json:"message"`
	}
	if json.Unmarshal([]byte(prefix), &parsed) == nil && parsed.Message != "" {
		return parsed.Message
	}
	if idx := strings.Index(prefix, `"message":"`); idx >= 0 {
		start := idx + len(`"message":"`)
		end := start
		escaped := false
		for end < len(prefix) {
			ch := prefix[end]
			if ch == '\\' && !escaped {
				escaped = true
				end++
				continue
			}
			if ch == '"' && !escaped {
				break
			}
			escaped = false
			end++
		}
		if end > start {
			raw := prefix[start:end]
			raw = strings.ReplaceAll(raw, `\"`, `"`)
			raw = strings.ReplaceAll(raw, `\\`, `\`)
			return raw
		}
	}
	// Fall back to the first 200 chars of the raw output
	if len(resultContent) > 200 {
		return truncateUTF8Prefix(resultContent, 200)
	}
	return resultContent
}

type toolSchemaFilterOptions struct {
	PreferredTools   []string
	HardAlwaysTools  []string
	SoftAlwaysTools  []string
	MaxAdaptiveTools int
	MaxTotalTools    int
	MaxSchemaTokens  int
}

type toolSchemaFilterReport struct {
	OriginalToolCount          int                   `json:"original_tool_count"`
	FinalToolCount             int                   `json:"final_tool_count"`
	OriginalSchemaBytes        int                   `json:"original_schema_bytes"`
	FinalSchemaBytes           int                   `json:"final_schema_bytes"`
	OriginalSchemaTokens       int                   `json:"original_schema_tokens"`
	FinalSchemaTokens          int                   `json:"final_schema_tokens"`
	LargestSchemas             []toolSchemaSizeEntry `json:"largest_schemas,omitempty"`
	KeptHardAlways             int                   `json:"kept_hard_always"`
	KeptSoftAlways             int                   `json:"kept_soft_always"`
	KeptAdaptive               int                   `json:"kept_adaptive"`
	Dropped                    int                   `json:"dropped"`
	MaxAdaptive                int                   `json:"max_adaptive"`
	MaxTotal                   int                   `json:"max_total"`
	MaxSchemaTokens            int                   `json:"max_schema_tokens,omitempty"`
	DroppedTools               []string              `json:"dropped_tools,omitempty"`
	HardAlwaysExceededTotalCap bool                  `json:"hard_always_exceeded_total_cap,omitempty"`
	HardAlwaysExceededTokenCap bool                  `json:"hard_always_exceeded_token_cap,omitempty"`
}

type toolSchemaSizeEntry struct {
	Name        string `json:"name"`
	Bytes       int    `json:"bytes"`
	RoughTokens int    `json:"rough_tokens"`
}

type toolSchemaFilterResult struct {
	Tools  []openai.Tool
	Report toolSchemaFilterReport
}

// filterToolSchemas removes tools that are neither in the preferred-tools list
// nor in the always-include list, keeping at most maxTools schemas.
// alwaysInclude tools are never dropped. maxTools is a hard cap applied to
// preferred tools; alwaysInclude tools do not count against the cap.
// maxTools=0 disables the cap entirely.
// This reduces token overhead for rarely-used tools without breaking any dispatch.
func filterToolSchemas(schemas []openai.Tool, frequentTools, alwaysInclude []string, maxTools int, logger *slog.Logger) []openai.Tool {
	result := filterToolSchemasWithReport(schemas, toolSchemaFilterOptions{
		PreferredTools:   frequentTools,
		SoftAlwaysTools:  alwaysInclude,
		MaxAdaptiveTools: maxTools,
		MaxTotalTools:    0,
	}, logger)
	return result.Tools
}

func filterToolSchemasWithReport(schemas []openai.Tool, opts toolSchemaFilterOptions, logger *slog.Logger) toolSchemaFilterResult {
	originalBytes, originalLargest := measureToolSchemaSizes(schemas, 5)
	report := toolSchemaFilterReport{
		OriginalToolCount:    len(schemas),
		OriginalSchemaBytes:  originalBytes,
		OriginalSchemaTokens: roughTokenEstimate(originalBytes),
		LargestSchemas:       originalLargest,
		MaxAdaptive:          opts.MaxAdaptiveTools,
		MaxTotal:             opts.MaxTotalTools,
		MaxSchemaTokens:      opts.MaxSchemaTokens,
	}
	hardSet := stringSet(opts.HardAlwaysTools)
	softSet := stringSet(opts.SoftAlwaysTools)

	preferredOrder := make([]string, 0, len(opts.PreferredTools))
	preferredSet := make(map[string]bool, len(opts.PreferredTools))
	for _, t := range opts.PreferredTools {
		t = strings.TrimSpace(t)
		if t == "" || hardSet[t] || softSet[t] || preferredSet[t] {
			continue
		}
		preferredSet[t] = true
		preferredOrder = append(preferredOrder, t)
	}

	schemaByName := make(map[string]openai.Tool, len(schemas))
	schemaOrder := make([]string, 0, len(schemas))
	var kept []openai.Tool
	consumed := make(map[string]bool, len(schemas))
	schemaTokens := make(map[string]int, len(schemas))
	keptSchemaTokens := 0
	add := func(schema openai.Tool, class string) bool {
		tokens := estimateSingleToolSchemaTokens(schema)
		if schema.Function == nil {
			kept = append(kept, schema)
			report.KeptHardAlways++
			keptSchemaTokens += tokens
			return true
		}
		name := schema.Function.Name
		if consumed[name] {
			return false
		}
		if class != "hard" && opts.MaxTotalTools > 0 && len(kept) >= opts.MaxTotalTools {
			return false
		}
		if class != "hard" && opts.MaxSchemaTokens > 0 && keptSchemaTokens+tokens > opts.MaxSchemaTokens {
			return false
		}
		consumed[name] = true
		kept = append(kept, schema)
		keptSchemaTokens += tokens
		switch class {
		case "hard":
			report.KeptHardAlways++
		case "soft":
			report.KeptSoftAlways++
		default:
			report.KeptAdaptive++
		}
		return true
	}

	for _, s := range schemas {
		if s.Function == nil {
			add(s, "hard")
			continue
		}
		name := s.Function.Name
		schemaByName[name] = s
		schemaOrder = append(schemaOrder, name)
		schemaTokens[name] = estimateSingleToolSchemaTokens(s)
	}

	for _, name := range schemaOrder {
		if hardSet[name] {
			add(schemaByName[name], "hard")
		}
	}
	if opts.MaxTotalTools > 0 && report.KeptHardAlways > opts.MaxTotalTools {
		report.HardAlwaysExceededTotalCap = true
		if logger != nil {
			logger.Warn("[AdaptiveTools] Hard-required tools exceed total cap",
				"kept_hard_always", report.KeptHardAlways,
				"max_total", opts.MaxTotalTools)
		}
	}
	if opts.MaxSchemaTokens > 0 && keptSchemaTokens > opts.MaxSchemaTokens {
		report.HardAlwaysExceededTokenCap = true
		if logger != nil {
			logger.Warn("[AdaptiveTools] Hard-required tools exceed schema token budget",
				"kept_hard_always", report.KeptHardAlways,
				"schema_tokens", keptSchemaTokens,
				"max_schema_tokens", opts.MaxSchemaTokens)
		}
	}
	for _, name := range schemaOrder {
		if softSet[name] && !hardSet[name] {
			add(schemaByName[name], "soft")
		}
	}

	canAddAdaptive := func() bool {
		return opts.MaxAdaptiveTools <= 0 || report.KeptAdaptive < opts.MaxAdaptiveTools
	}
	adaptiveOrder := buildAdaptiveKnapsackOrder(schemaOrder, preferredOrder, consumed, hardSet, softSet, schemaTokens, opts.MaxSchemaTokens > 0)
	for _, name := range adaptiveOrder {
		schema, ok := schemaByName[name]
		if !ok || consumed[name] || !canAddAdaptive() {
			continue
		}
		add(schema, "adaptive")
	}

	finalDropped := len(schemas) - len(kept)
	report.FinalToolCount = len(kept)
	report.FinalSchemaBytes, report.LargestSchemas = measureToolSchemaSizes(kept, 5)
	report.FinalSchemaTokens = roughTokenEstimate(report.FinalSchemaBytes)
	report.Dropped = finalDropped
	keptSet := make(map[string]bool, len(kept))
	for _, k := range kept {
		if k.Function != nil {
			keptSet[k.Function.Name] = true
		}
	}
	for _, s := range schemas {
		if s.Function != nil && !keptSet[s.Function.Name] {
			report.DroppedTools = append(report.DroppedTools, s.Function.Name)
		}
	}
	if logger != nil && finalDropped > 0 {
		logger.Info("[AdaptiveTools] Filtered tool schemas",
			"kept_hard_always", report.KeptHardAlways,
			"kept_soft_always", report.KeptSoftAlways,
			"kept_adaptive", report.KeptAdaptive,
			"dropped", finalDropped,
			"max_adaptive", opts.MaxAdaptiveTools,
			"max_total", opts.MaxTotalTools,
			"dropped_tools", strings.Join(report.DroppedTools, ", "))
	}
	return toolSchemaFilterResult{Tools: kept, Report: report}
}

func estimateSingleToolSchemaTokens(schema openai.Tool) int {
	b, err := json.Marshal(schema)
	if err != nil {
		return 0
	}
	return roughTokenEstimate(len(b))
}

func buildAdaptiveKnapsackOrder(schemaOrder, preferredOrder []string, consumed map[string]bool, hardSet, softSet map[string]bool, schemaTokens map[string]int, tokenAware bool) []string {
	preferredRank := make(map[string]int, len(preferredOrder))
	for i, name := range preferredOrder {
		preferredRank[name] = len(preferredOrder) - i
	}
	type candidate struct {
		name         string
		utility      int
		tokens       int
		originalRank int
	}
	candidates := make([]candidate, 0, len(schemaOrder))
	for i, name := range schemaOrder {
		if consumed[name] || hardSet[name] || softSet[name] {
			continue
		}
		utility := preferredRank[name]
		if utility <= 0 {
			utility = 1
		}
		tokens := schemaTokens[name]
		if tokens <= 0 {
			tokens = 1
		}
		candidates = append(candidates, candidate{
			name:         name,
			utility:      utility,
			tokens:       tokens,
			originalRank: i,
		})
	}
	if tokenAware {
		sort.SliceStable(candidates, func(i, j int) bool {
			left := candidates[i]
			right := candidates[j]
			leftScore := left.utility * right.tokens
			rightScore := right.utility * left.tokens
			if leftScore != rightScore {
				return leftScore > rightScore
			}
			if left.utility != right.utility {
				return left.utility > right.utility
			}
			return left.originalRank < right.originalRank
		})
	} else {
		sort.SliceStable(candidates, func(i, j int) bool {
			left := candidates[i]
			right := candidates[j]
			leftPreferred := preferredRank[left.name] > 0
			rightPreferred := preferredRank[right.name] > 0
			if leftPreferred != rightPreferred {
				return leftPreferred
			}
			if leftPreferred && left.utility != right.utility {
				return left.utility > right.utility
			}
			return left.originalRank < right.originalRank
		})
	}
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate.name)
	}
	return out
}

func measureToolSchemaSizes(schemas []openai.Tool, limit int) (int, []toolSchemaSizeEntry) {
	if limit <= 0 {
		limit = 5
	}
	total := 0
	entries := make([]toolSchemaSizeEntry, 0, len(schemas))
	for _, schema := range schemas {
		if schema.Function == nil {
			continue
		}
		encoded, err := json.Marshal(schema)
		if err != nil {
			continue
		}
		size := len(encoded)
		total += size
		entries = append(entries, toolSchemaSizeEntry{
			Name:        schema.Function.Name,
			Bytes:       size,
			RoughTokens: roughTokenEstimate(size),
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Bytes != entries[j].Bytes {
			return entries[i].Bytes > entries[j].Bytes
		}
		return entries[i].Name < entries[j].Name
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return total, entries
}

func roughTokenEstimate(bytes int) int {
	if bytes <= 0 {
		return 0
	}
	return (bytes + 3) / 4
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = true
		}
	}
	return set
}

func toolSchemaNames(schemas []openai.Tool) []string {
	names := make([]string, 0, len(schemas))
	seen := make(map[string]bool, len(schemas))
	for _, schema := range schemas {
		if schema.Function == nil {
			continue
		}
		name := strings.TrimSpace(schema.Function.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func recentNativeToolNamesFromMessages(messages []openai.ChatCompletionMessage, turns int) []string {
	if turns <= 0 {
		turns = len(messages)
	}
	seenTurns := 0
	seen := map[string]bool{}
	outReversed := make([]string, 0)
	for i := len(messages) - 1; i >= 0 && seenTurns < turns; i-- {
		msg := messages[i]
		if msg.Role == openai.ChatMessageRoleUser {
			seenTurns++
			continue
		}
		if msg.Role != openai.ChatMessageRoleAssistant {
			continue
		}
		for _, tc := range msg.ToolCalls {
			name := strings.TrimSpace(tc.Function.Name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			outReversed = append(outReversed, name)
		}
	}
	names := make([]string, 0, len(outReversed))
	for i := len(outReversed) - 1; i >= 0; i-- {
		names = append(names, outReversed[i])
	}
	return names
}

func emitMediaSSEEvents(broker FeedbackBroker, action, resultContent string, dataDir string) {
	raw := strings.TrimPrefix(resultContent, "[Tool Output]\n")
	raw = strings.TrimPrefix(raw, "Tool Output: ")

	switch action {
	case "send_image":
		var imgRes struct {
			Status  string `json:"status"`
			WebPath string `json:"web_path"`
			Caption string `json:"caption"`
		}
		if json.Unmarshal([]byte(raw), &imgRes) == nil && imgRes.Status == "success" {
			evtPayload, _ := json.Marshal(map[string]string{"path": imgRes.WebPath, "caption": imgRes.Caption})
			broker.Send("image", string(evtPayload))
		}
	case "send_audio":
		var audioRes struct {
			Status   string `json:"status"`
			WebPath  string `json:"web_path"`
			Title    string `json:"title"`
			MimeType string `json:"mime_type"`
			Filename string `json:"filename"`
		}
		if json.Unmarshal([]byte(raw), &audioRes) == nil && audioRes.Status == "success" {
			evtPayload, _ := json.Marshal(map[string]string{
				"path":      audioRes.WebPath,
				"title":     audioRes.Title,
				"mime_type": audioRes.MimeType,
				"filename":  audioRes.Filename,
			})
			broker.Send("audio", string(evtPayload))
		}
	case "send_video":
		var videoRes struct {
			Status   string `json:"status"`
			WebPath  string `json:"web_path"`
			Title    string `json:"title"`
			MimeType string `json:"mime_type"`
			Filename string `json:"filename"`
			Format   string `json:"format"`
		}
		if json.Unmarshal([]byte(raw), &videoRes) == nil && videoRes.Status == "success" {
			evtPayload, _ := json.Marshal(map[string]string{
				"path":      videoRes.WebPath,
				"title":     videoRes.Title,
				"mime_type": videoRes.MimeType,
				"filename":  videoRes.Filename,
				"format":    videoRes.Format,
			})
			broker.Send("video", string(evtPayload))
		}
	case "send_stl":
		var stlRes struct {
			Status   string `json:"status"`
			WebPath  string `json:"web_path"`
			Title    string `json:"title"`
			MimeType string `json:"mime_type"`
			Filename string `json:"filename"`
			Format   string `json:"format"`
		}
		if json.Unmarshal([]byte(raw), &stlRes) == nil && stlRes.Status == "success" {
			evtPayload, _ := json.Marshal(map[string]string{
				"path":      stlRes.WebPath,
				"title":     stlRes.Title,
				"mime_type": stlRes.MimeType,
				"filename":  stlRes.Filename,
				"format":    stlRes.Format,
			})
			broker.Send("stl", string(evtPayload))
		}
	case "send_youtube_video":
		var videoRes struct {
			Status       string `json:"status"`
			URL          string `json:"url"`
			EmbedURL     string `json:"embed_url"`
			VideoID      string `json:"video_id"`
			Title        string `json:"title"`
			StartSeconds int    `json:"start_seconds"`
		}
		if json.Unmarshal([]byte(raw), &videoRes) == nil && videoRes.Status == "success" {
			evtPayload, _ := json.Marshal(map[string]interface{}{
				"url":           videoRes.URL,
				"embed_url":     videoRes.EmbedURL,
				"video_id":      videoRes.VideoID,
				"title":         videoRes.Title,
				"start_seconds": videoRes.StartSeconds,
				"provider":      "youtube",
			})
			broker.Send("youtube_video", string(evtPayload))
		}
	case "generate_video":
		var videoRes struct {
			Status     string `json:"status"`
			WebPath    string `json:"web_path"`
			Filename   string `json:"filename"`
			Format     string `json:"format"`
			Provider   string `json:"provider"`
			Model      string `json:"model"`
			DurationMs int64  `json:"duration_ms"`
		}
		if json.Unmarshal([]byte(raw), &videoRes) == nil && videoRes.Status == "ok" {
			mimeType := videoMIMEType(videoRes.Filename)
			evtPayload, _ := json.Marshal(map[string]string{
				"path":      videoRes.WebPath,
				"title":     strings.TrimSuffix(videoRes.Filename, filepath.Ext(videoRes.Filename)),
				"mime_type": mimeType,
				"filename":  videoRes.Filename,
				"format":    videoRes.Format,
				"provider":  videoRes.Provider,
				"model":     videoRes.Model,
			})
			broker.Send("video", string(evtPayload))
		}
	case "generate_music":
		var musicRes struct {
			Status     string `json:"status"`
			WebPath    string `json:"web_path"`
			Filename   string `json:"filename"`
			Title      string `json:"title"`
			Format     string `json:"format"`
			Provider   string `json:"provider"`
			Model      string `json:"model"`
			DurationMs int64  `json:"duration_ms"`
		}
		if json.Unmarshal([]byte(raw), &musicRes) == nil && musicRes.Status == "ok" {
			title := strings.TrimSpace(musicRes.Title)
			if title == "" {
				title = strings.TrimSuffix(musicRes.Filename, filepath.Ext(musicRes.Filename))
			}
			evtPayload, _ := json.Marshal(map[string]interface{}{
				"path":        musicRes.WebPath,
				"title":       title,
				"mime_type":   audioMIMEType(musicRes.Filename),
				"filename":    musicRes.Filename,
				"format":      musicRes.Format,
				"provider":    musicRes.Provider,
				"model":       musicRes.Model,
				"duration_ms": musicRes.DurationMs,
				"media_type":  "music",
			})
			broker.Send("audio", string(evtPayload))
		}
	case "tts":
		var ttsRes struct {
			Status  string `json:"status"`
			File    string `json:"file"`
			WebPath string `json:"web_path"`
		}
		if json.Unmarshal([]byte(raw), &ttsRes) == nil && ttsRes.Status == "success" && strings.TrimSpace(ttsRes.File) != "" {
			webPath := strings.TrimSpace(ttsRes.WebPath)
			if webPath == "" {
				webPath = "/tts/" + ttsRes.File
			}
			evtPayload, _ := json.Marshal(map[string]interface{}{
				"path":        webPath,
				"title":       "TTS Audio",
				"mime_type":   audioMIMEType(ttsRes.File),
				"filename":    ttsRes.File,
				"file_path":   filepath.Join(dataDir, "tts", ttsRes.File),
				"autoplay":    true,
				"show_player": true,
			})
			broker.Send("audio", string(evtPayload))
		}
	case "send_document":
		var docRes struct {
			Status     string `json:"status"`
			WebPath    string `json:"web_path"`
			PreviewURL string `json:"preview_url"`
			Title      string `json:"title"`
			MimeType   string `json:"mime_type"`
			Filename   string `json:"filename"`
			Format     string `json:"format"`
		}
		if json.Unmarshal([]byte(raw), &docRes) == nil && docRes.Status == "success" {
			evtPayload, _ := json.Marshal(map[string]string{
				"path":        docRes.WebPath,
				"preview_url": docRes.PreviewURL,
				"title":       docRes.Title,
				"mime_type":   docRes.MimeType,
				"filename":    docRes.Filename,
				"format":      docRes.Format,
			})
			if strings.EqualFold(docRes.Format, "stl") || strings.HasSuffix(strings.ToLower(docRes.Filename), ".stl") {
				broker.Send("stl", string(evtPayload))
				break
			}
			broker.Send("document", string(evtPayload))
		}
	}
}

func compactMemoryForPrompt(text string, maxLen int) string {
	text = sanitizeMemoryForPrompt(text)
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	runes := []rune(text)
	maxRunes := 0
	for byteLen, idx := 0, 0; idx < len(runes); idx++ {
		nextLen := byteLen + len(string(runes[idx]))
		if nextLen > maxLen {
			break
		}
		byteLen = nextLen
		maxRunes = idx + 1
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "…"
}

const (
	aggressiveRetrievedMemoriesChars = 1500
	aggressiveAutoRAGMemoryChars     = 320
	aggressiveDetailedRAGMemoryChars = 700
	aggressivePredictedMemoriesChars = 260
	aggressiveRecentOverviewChars    = 700
	aggressiveKnowledgeContextChars  = 800
	aggressiveErrorPatternChars      = 700
	aggressiveLearnedRulesChars      = 480
	aggressiveReuseContextChars      = 700
	aggressiveAvailableContextChars  = 1600
	runtimeOperationalIssueChars     = 600
)

type promptMemoryEntry struct {
	docID string
	text  string
}

func buildAggressiveRAGPromptEntries(served []rankedMemory, wantsDeepDetails bool, getFull func(string) (string, error)) []promptMemoryEntry {
	if len(served) == 0 {
		return nil
	}
	top := served[0]
	if wantsDeepDetails && getFull != nil && top.docID != "" {
		if full, err := getFull(top.docID); err == nil && full != "" && shouldServeRAGMemory(full) {
			return []promptMemoryEntry{{
				docID: top.docID,
				text:  "[Detailed Memory]\n" + compactMemoryForPrompt(full, aggressiveDetailedRAGMemoryChars),
			}}
		}
	}
	return []promptMemoryEntry{{
		docID: top.docID,
		text:  compactMemoryForPrompt(top.text, aggressiveAutoRAGMemoryChars),
	}}
}

func selectRAGMemoriesForOnDemand(ranked []rankedMemory, cfg config.MemoryOnDemandRetrievalConfig, logger *slog.Logger) ([]rankedMemory, []rankedMemory) {
	if len(ranked) == 0 {
		return nil, nil
	}
	essentialLimit := cfg.MaxEssentialMemories
	if essentialLimit <= 0 {
		essentialLimit = 1
	}
	if !cfg.Enabled {
		return selectServedRAGMemories(ranked, 1, logger), nil
	}
	availableLimit := cfg.MaxAvailableMemories
	if availableLimit < 0 {
		availableLimit = 0
	}
	served := selectServedRAGMemories(ranked, essentialLimit+availableLimit, logger)
	if len(served) <= essentialLimit {
		return served, nil
	}
	essential := append([]rankedMemory(nil), served[:essentialLimit]...)
	available := append([]rankedMemory(nil), served[essentialLimit:]...)
	return essential, available
}

func buildAvailableMemoryIndex(available []rankedMemory, maxChars int) string {
	if len(available) == 0 || maxChars <= 0 {
		return ""
	}
	var sb strings.Builder
	for _, item := range available {
		if strings.TrimSpace(item.docID) == "" || strings.TrimSpace(item.text) == "" {
			continue
		}
		teaser := compactMemoryForPrompt(item.text, 96)
		line := "- [memory:" + item.docID + "] source=ltm score=" + formatScore(item.score) + " - " + teaser
		nextLen := len([]rune(line))
		if sb.Len() > 0 {
			nextLen += 1
		}
		if len([]rune(sb.String()))+nextLen > maxChars {
			if sb.Len() == 0 {
				return compactMemoryForPrompt(line, maxChars)
			}
			break
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(line)
	}
	return sb.String()
}

func appendAvailableContextIndex(existing string, addition string, maxChars int) string {
	existing = strings.TrimSpace(existing)
	addition = strings.TrimSpace(addition)
	if addition == "" {
		return existing
	}
	if existing == "" {
		return compactMemoryForPrompt(addition, maxChars)
	}
	return compactMemoryForPrompt(existing+"\n"+addition, maxChars)
}

func formatScore(score float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", score), "0"), ".")
}

func memoryDedupeMapForScope(scope, sessionID string, turnMap map[string]int) map[string]int {
	if turnMap == nil {
		turnMap = make(map[string]int)
	}
	if normalizeMemoryDedupeScope(scope) != "session" || strings.TrimSpace(sessionID) == "" {
		return turnMap
	}

	memoryDedupeSessions.Lock()
	defer memoryDedupeSessions.Unlock()
	return cloneMemoryDedupeMap(memoryDedupeSessions.bySession[sessionID])
}

func persistMemoryDedupeMapForScope(scope, sessionID string, ids map[string]int) {
	if normalizeMemoryDedupeScope(scope) != "session" || strings.TrimSpace(sessionID) == "" {
		return
	}

	memoryDedupeSessions.Lock()
	defer memoryDedupeSessions.Unlock()
	if len(ids) == 0 {
		delete(memoryDedupeSessions.bySession, sessionID)
		return
	}
	existing := memoryDedupeSessions.bySession[sessionID]
	if existing == nil {
		existing = make(map[string]int, len(ids))
		memoryDedupeSessions.bySession[sessionID] = existing
	}
	for id, count := range ids {
		if strings.TrimSpace(id) == "" || count <= 0 {
			continue
		}
		if count > existing[id] {
			existing[id] = count
		}
	}
	if len(existing) > memoryDedupeSessionMaxIDs {
		delete(memoryDedupeSessions.bySession, sessionID)
	}
}

func normalizeMemoryDedupeScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "session":
		return "session"
	default:
		return "turn"
	}
}

func cloneMemoryDedupeMap(src map[string]int) map[string]int {
	dst := make(map[string]int, len(src))
	for id, count := range src {
		if strings.TrimSpace(id) == "" || count <= 0 {
			continue
		}
		dst[id] = count
	}
	return dst
}

func applyAggressivePromptContextBudgets(flags *prompts.ContextFlags) {
	if flags == nil {
		return
	}
	flags.RetrievedMemories = compactMemoryForPrompt(flags.RetrievedMemories, aggressiveRetrievedMemoriesChars)
	flags.AvailableMemoryContextIndex = compactMemoryForPrompt(flags.AvailableMemoryContextIndex, aggressiveAvailableContextChars)
	flags.AvailableKnowledgeContextIndex = compactMemoryForPrompt(flags.AvailableKnowledgeContextIndex, aggressiveAvailableContextChars)
	flags.PredictedMemories = compactMemoryForPrompt(flags.PredictedMemories, aggressivePredictedMemoriesChars)
	flags.RecentActivityOverview = compactMemoryForPrompt(flags.RecentActivityOverview, aggressiveRecentOverviewChars)
	flags.KnowledgeContext = compactMemoryForPrompt(flags.KnowledgeContext, aggressiveKnowledgeContextChars)
	flags.ErrorPatternContext = compactMemoryForPrompt(flags.ErrorPatternContext, aggressiveErrorPatternChars)
	flags.LearnedRulesContext = compactMemoryForPrompt(flags.LearnedRulesContext, aggressiveLearnedRulesChars)
	flags.ReuseContext = compactMemoryForPrompt(flags.ReuseContext, aggressiveReuseContextChars)
}

type runtimePromptContextOptions struct {
	UserText         string
	MessageSource    string
	RecentTools      []string
	SessionUsedTools map[string]bool
	ReuseLookup      ReuseLookupResult
}

func applyRuntimePromptContextPolicy(flags *prompts.ContextFlags, opts runtimePromptContextOptions) {
	if flags == nil {
		return
	}
	flags.CapabilityCreationIntent = flags.CapabilityCreationIntent || shouldInjectCapabilityCreationPrompt(opts.UserText, opts.RecentTools, opts.SessionUsedTools)
	flags.DaemonSkillsIntent = flags.DaemonSkillsIntent || shouldInjectDaemonSkillsPrompt(opts.UserText, opts.RecentTools, opts.SessionUsedTools)
	if flags.InternetExposed && !shouldInjectInternetExposureWarning(opts.UserText, opts.RecentTools, opts.SessionUsedTools, flags) {
		flags.InternetExposed = false
	}
	if !shouldInjectReachableChatChannelsContext(opts.UserText, opts.MessageSource, opts.RecentTools, opts.SessionUsedTools) {
		flags.ChatChannelsContext = ""
	}
	if !shouldInjectSpaceAgentRuntimePrompt(opts.UserText, opts.RecentTools, opts.SessionUsedTools) {
		flags.SpaceAgentEnabled = false
		flags.SpaceAgentPublicURL = ""
	}
	if !shouldInjectUserProfilingPrompt(opts.UserText) {
		flags.UserProfilingEnabled = false
		flags.UserProfileSummary = ""
	}
	if !shouldInjectReuseLookupPrompt(opts.UserText, opts.ReuseLookup) {
		flags.ReuseContext = ""
	}
	applyRuntimePromptContextBudgets(flags)
}

func shouldInjectCapabilityCreationPrompt(userText string, recentTools []string, sessionUsed map[string]bool) bool {
	text := normalizeAdaptiveIntentText(userText)
	cues := []string{
		"create skill", "erstelle skill", "python skill", "agent skill", "agent-skill",
		"skill.md", "skill md", "agentskills", "agentskills io", "codex skill", "claude skill", "skill package",
		"create tool", "erstelle tool", "new tool", "neues tool", "tool bridge",
		"internal_tools", "list_skill_templates", "create_skill_from_template",
		"reusable capability", "wiederverwendbare f higkeit", "wiederverwendbare faehigkeit",
		"create capability", "new capability", "capability creation", "skill template", "tool template",
	}
	if containsAnyRuntimeCue(text, cues) {
		return true
	}
	for _, tool := range recentTools {
		if isCapabilityCreationTool(tool) {
			return true
		}
	}
	for tool := range sessionUsed {
		if isCapabilityCreationTool(tool) {
			return true
		}
	}
	return false
}

func shouldInjectDaemonSkillsPrompt(userText string, recentTools []string, sessionUsed map[string]bool) bool {
	text := normalizeAdaptiveIntentText(userText)
	cues := []string{
		"daemon", "daemon skill", "daemon skills", "manage_daemon", "long-running",
		"long running", "background watcher", "background listener", "background monitor",
		"watcher", "listener", "monitor daemon", "dauerhaft", "hintergrunddienst",
		"hintergrund watcher", "hintergrund monitor", "wake_agent",
	}
	if containsAnyRuntimeCue(text, cues) {
		return true
	}
	for _, tool := range recentTools {
		if isDaemonSkillTool(tool) {
			return true
		}
	}
	for tool := range sessionUsed {
		if isDaemonSkillTool(tool) {
			return true
		}
	}
	return false
}

func shouldInjectInternetExposureWarning(userText string, recentTools []string, sessionUsed map[string]bool, flags *prompts.ContextFlags) bool {
	text := normalizeAdaptiveIntentText(userText)
	cues := []string{
		"homepage", "web deploy", "website", "deploy", "deployment", "caddy",
		"reverse proxy", "reverse-proxy", "public url", "publicly", "internet",
		"expose", "exposed", "domain", "https", "tls", "port", "ports",
		"docker", "container", "network", "netzwerk", "tailscale", "cloudflare",
		"tunnel", "web capture", "screenshot url", "scrape", "crawler",
	}
	if containsAnyRuntimeCue(text, cues) {
		return true
	}
	for _, tool := range recentTools {
		if isInternetExposureTool(tool) {
			return true
		}
	}
	for tool := range sessionUsed {
		if isInternetExposureTool(tool) {
			return true
		}
	}
	if flags != nil {
		for _, tool := range flags.ActiveNativeTools {
			if isInternetExposureTool(tool) {
				return true
			}
		}
	}
	return false
}

func applyRuntimePromptContextBudgets(flags *prompts.ContextFlags) {
	if flags == nil {
		return
	}
	flags.OperationalIssueReminder = compactMemoryForPrompt(flags.OperationalIssueReminder, runtimeOperationalIssueChars)
}

func shouldInjectReachableChatChannelsContext(userText, messageSource string, recentTools []string, sessionUsed map[string]bool) bool {
	source := strings.TrimSpace(strings.ToLower(messageSource))
	if source != "" && source != "web_chat" {
		return true
	}
	text := normalizeAdaptiveIntentText(userText)
	cues := []string{
		"send", "sende", "sent", "message", "nachricht", "benachrichtig", "notify",
		"notification", "telegram", "discord", "ntfy", "pushover", "sms", "rocketchat",
		"rocket chat", "agodesk", "kontakt", "contact", "chat channel", "kanal",
	}
	if containsAnyRuntimeCue(text, cues) {
		return true
	}
	for _, tool := range recentTools {
		if isChatChannelTool(tool) {
			return true
		}
	}
	for tool := range sessionUsed {
		if isChatChannelTool(tool) {
			return true
		}
	}
	return false
}

func shouldInjectSpaceAgentRuntimePrompt(userText string, recentTools []string, sessionUsed map[string]bool) bool {
	text := normalizeAdaptiveIntentText(userText)
	cues := []string{
		"space agent", "space_agent", "sidecar", "delegier", "delegate", "delegation",
		"workspace handoff", "workspace übergabe", "workspace uebergabe", "arbeitsbereich",
		"parallel", "subagent", "co agent", "coagent",
	}
	if containsAnyRuntimeCue(text, cues) {
		return true
	}
	for _, tool := range recentTools {
		if strings.EqualFold(strings.TrimSpace(tool), "space_agent") {
			return true
		}
	}
	for tool := range sessionUsed {
		if strings.EqualFold(strings.TrimSpace(tool), "space_agent") {
			return true
		}
	}
	return false
}

func shouldInjectUserProfilingPrompt(userText string) bool {
	text := normalizeAdaptiveIntentText(userText)
	cues := []string{
		"profile", "profil", "präferenz", "praeferenz", "präferenzen", "praeferenzen",
		"pr ferenz", "pr ferenzen",
		"preference", "preferences", "personalisiert", "personalisiere", "personalisierung",
		"about me", "über mich", "ueber mich", "ber mich", "was weisst du", "was wei t du",
		"kennst du über mich", "kennst du ueber mich", "kennst du ber mich", "user profile", "nutzerprofil",
	}
	return containsAnyRuntimeCue(text, cues)
}

func shouldInjectOperationalIssueReminderForTurn(userText, reminder string, debugOrError bool) bool {
	if strings.TrimSpace(reminder) == "" {
		return false
	}
	text := normalizeAdaptiveIntentText(userText)
	if debugOrError || containsAnyRuntimeCue(text, []string{
		"debug", "fehler", "error", "failed", "failure", "problem", "probleme",
		"log", "logs", "kaputt", "hängt", "haengt", "trace", "stacktrace",
	}) {
		return true
	}
	reminderTokens := runtimeCueTokenSet(normalizeAdaptiveIntentText(reminder))
	for token := range runtimeCueTokenSet(text) {
		if reminderTokens[token] {
			return true
		}
	}
	return false
}

func isChatChannelTool(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	cues := []string{
		"telegram", "discord", "ntfy", "pushover", "sms", "rocketchat", "rocket_chat",
		"send_notification", "send_message", "send_sms", "send_image", "send_video",
	}
	return containsAnyRuntimeCue(name, cues)
}

func isCapabilityCreationTool(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	cues := []string{
		"create_skill_from_template", "list_skill_templates", "skills_engine",
		"execute_skill", "list_agent_skills", "activate_agent_skill",
		"run_agent_skill_script", "run_tool", "tool_bridge",
	}
	return containsAnyRuntimeCue(name, cues)
}

func isDaemonSkillTool(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	return strings.Contains(name, "daemon") || strings.EqualFold(name, "manage_daemon")
}

func isInternetExposureTool(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	cues := []string{
		"homepage", "docker", "container", "network", "network_ping", "network_scan",
		"upnp", "fritzbox", "tailscale", "cloudflare", "tunnel", "web_capture",
		"web_scraper", "browser_automation", "api_request", "http", "netlify",
		"vercel", "s3", "caddy",
	}
	return containsAnyRuntimeCue(name, cues)
}

func containsAnyRuntimeCue(text string, cues []string) bool {
	for _, cue := range cues {
		if strings.Contains(text, cue) {
			return true
		}
	}
	return false
}

func runtimeCueTokenSet(text string) map[string]bool {
	tokens := make(map[string]bool)
	for _, token := range strings.Fields(text) {
		token = strings.Trim(token, ".,:;!?()[]{}\"'")
		if len(token) >= 4 {
			tokens[token] = true
		}
	}
	return tokens
}

func shouldInjectSpecialistAwareness(userText, suggestion string) bool {
	text := normalizeAdaptiveIntentText(userText + " " + suggestion)
	if text == "" {
		return false
	}
	cues := []string{
		"co agent", "coagent", "co agents", "specialist", "specialists",
		"spezialist", "spezialisten", "delegier", "delegat", "parallel",
		"subagent", "researcher", "reviewer", "security expert",
		"expert agent", "worker agent",
	}
	for _, cue := range cues {
		if strings.Contains(text, cue) {
			return true
		}
	}
	return false
}

func shouldExposeRuntimePaths(userText string, recentTools []string, sessionUsed map[string]bool) bool {
	text := normalizeAdaptiveIntentText(userText)
	cues := []string{
		"skill", "skills", "agent skill", "skill md", "agentskills",
		"create skill", "python skill", "custom tool", "create tool",
		"save tool", "run tool", "tool erstellen", "werkzeug erstellen",
		"skript", "script", "tool bridge",
	}
	for _, cue := range cues {
		if strings.Contains(text, cue) {
			return true
		}
	}
	for _, name := range recentTools {
		if isRuntimePathTool(name) {
			return true
		}
	}
	for name := range sessionUsed {
		if isRuntimePathTool(name) {
			return true
		}
	}
	return false
}

func isRuntimePathTool(name string) bool {
	switch strings.TrimSpace(name) {
	case "execute_skill", "run_tool", "create_skill_from_template", "save_tool",
		"list_skills", "list_agent_skills", "activate_agent_skill", "run_agent_skill_script",
		"execute_python", "file_editor":
		return true
	default:
		return false
	}
}

func sanitizeMemoryForPrompt(text string) string {
	text = strings.TrimSpace(text)
	for i := 0; i < 2; i++ {
		text = security.StripThinkingTags(text)
		unescaped := html.UnescapeString(text)
		if unescaped == text {
			break
		}
		text = unescaped
	}
	return strings.TrimSpace(security.StripThinkingTags(text))
}

func wantsDetailedMemory(query string) bool {
	q := strings.ToLower(query)
	cues := []string{
		"detail", "details", "exact", "specific", "precise",
		"genau", "details", "welche", "wann", "konkret",
	}
	for _, cue := range cues {
		if strings.Contains(q, cue) {
			return true
		}
	}
	return false
}

func selectServedRAGMemories(ranked []rankedMemory, topK int, logger *slog.Logger) []rankedMemory {
	if topK <= 0 || len(ranked) == 0 {
		return nil
	}
	served := make([]rankedMemory, 0, topK)
	for _, item := range ranked {
		if !shouldServeRAGMemory(item.text) {
			if logger != nil {
				logger.Debug("[RAG] Dropped stale transient memory", "preview", Truncate(item.text, 80))
			}
			continue
		}
		served = append(served, item)
		if len(served) >= topK {
			break
		}
	}
	return served
}

func trim422Messages(msgs []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	toolResponseIDs := make(map[string]bool, len(msgs))
	for _, m := range msgs {
		if m.Role == openai.ChatMessageRoleTool && m.ToolCallID != "" {
			toolResponseIDs[m.ToolCallID] = true
		}
	}

	type toolRound struct {
		assistantIdx int
		assistant    openai.ChatCompletionMessage
		toolResults  []openai.ChatCompletionMessage
	}
	var rounds []toolRound
	var nonToolMsgs []openai.ChatCompletionMessage

	for i := 0; i < len(msgs); i++ {
		m := msgs[i]
		switch m.Role {
		case openai.ChatMessageRoleTool:
			continue
		case openai.ChatMessageRoleAssistant:
			if len(m.ToolCalls) > 0 {
				round := toolRound{assistantIdx: len(nonToolMsgs), assistant: m}
				for j := i + 1; j < len(msgs); j++ {
					if msgs[j].Role == openai.ChatMessageRoleTool {
						for _, tc := range m.ToolCalls {
							// Require both IDs to be non-empty — "" == "" is a false match
							// that causes the API to return 400 "tool_call_id  is not found".
							if tc.ID != "" && msgs[j].ToolCallID != "" && msgs[j].ToolCallID == tc.ID {
								round.toolResults = append(round.toolResults, msgs[j])
								break
							}
						}
						if len(round.toolResults) == len(m.ToolCalls) {
							break
						}
					} else {
						break
					}
				}
				rounds = append(rounds, round)
				for j := i + 1; j < len(msgs) && msgs[j].Role == openai.ChatMessageRoleTool; j++ {
					i = j
				}
				continue
			}
			nonToolMsgs = append(nonToolMsgs, m)
		default:
			nonToolMsgs = append(nonToolMsgs, m)
		}
	}

	lastCompleteRound := -1
	hasAnyEmptyID := false
	for ri := len(rounds) - 1; ri >= 0; ri-- {
		r := rounds[ri]
		if len(r.toolResults) != len(r.assistant.ToolCalls) {
			continue
		}
		// Exclude rounds where any ToolCall has an empty ID — empty IDs cause
		// the API to return 400 "tool_call_id  is not found".
		validIDs := true
		for _, tc := range r.assistant.ToolCalls {
			if tc.ID == "" {
				validIDs = false
				hasAnyEmptyID = true
				break
			}
		}
		if validIDs {
			lastCompleteRound = ri
			break
		}
	}

	sysEnd := 0
	for sysEnd < len(nonToolMsgs) && nonToolMsgs[sysEnd].Role == openai.ChatMessageRoleSystem {
		sysEnd++
	}

	// Nuclear option: if ALL rounds have empty/mismatched IDs, the LLM is
	// consistently generating broken tool_call_ids. Strip ALL tool call history
	// and give it a clean slate with only system + last user message, so it
	// can't produce the same 400 error in an infinite loop.
	if hasAnyEmptyID && lastCompleteRound < 0 {
		// Keep only system messages + the last user message
		var lastUserMsg *openai.ChatCompletionMessage
		for i := len(nonToolMsgs) - 1; i >= 0; i-- {
			if nonToolMsgs[i].Role == openai.ChatMessageRoleUser {
				lastUserMsg = &nonToolMsgs[i]
				break
			}
		}
		trimmed := make([]openai.ChatCompletionMessage, 0, sysEnd+1)
		trimmed = append(trimmed, nonToolMsgs[:sysEnd]...)
		if lastUserMsg != nil {
			trimmed = append(trimmed, *lastUserMsg)
		}
		trimmed = append(trimmed, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "[The previous tool call history was cleared because all tool calls had empty IDs. Please summarise what you were doing and continue from the last user request.]",
		})
		return trimmed
	}

	conversation := nonToolMsgs[sysEnd:]
	if len(conversation) > 4 {
		conversation = conversation[len(conversation)-4:]
	}
	for len(conversation) > 0 && conversation[0].Role != openai.ChatMessageRoleUser {
		conversation = conversation[1:]
	}

	trimmed := make([]openai.ChatCompletionMessage, 0, sysEnd+len(conversation)+1)
	trimmed = append(trimmed, nonToolMsgs[:sysEnd]...)

	if lastCompleteRound >= 0 {
		r := rounds[lastCompleteRound]
		trimmed = append(trimmed, r.assistant)
		trimmed = append(trimmed, r.toolResults...)
	}

	trimmed = append(trimmed, conversation...)
	trimmed = append(trimmed, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "[The previous tool call history was trimmed due to a provider error. Please summarise what you were doing and continue.]",
	})

	// Final pass: run the shared sanitizer to catch any remaining orphaned
	// tool results that the round-based trimming above may have missed.
	// If the sanitizer still finds problems, fall back to the nuclear option
	// (strip ALL tool messages and keep only system + last user message).
	if sanitized, dropped := SanitizeToolMessages(trimmed); dropped > 0 {
		// Nuclear fallback: strip everything except system + last user message
		var lastUserMsg *openai.ChatCompletionMessage
		for i := len(nonToolMsgs) - 1; i >= 0; i-- {
			if nonToolMsgs[i].Role == openai.ChatMessageRoleUser {
				lastUserMsg = &nonToolMsgs[i]
				break
			}
		}
		nuclear := make([]openai.ChatCompletionMessage, 0, sysEnd+2)
		nuclear = append(nuclear, nonToolMsgs[:sysEnd]...)
		if lastUserMsg != nil {
			nuclear = append(nuclear, *lastUserMsg)
		}
		nuclear = append(nuclear, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "[The previous tool call history was cleared due to persistent provider errors. Please summarise what you were doing and continue from the last user request.]",
		})
		return nuclear
	} else {
		trimmed = sanitized
	}

	return trimmed
}

// queuePendingToolCalls appends newly discovered pending tool calls to the
// current loop-local queue and immediately mirrors the result back into the
// shared loop state. Native multi-tool responses are executed from
// agentLoopState.pendingTCs inside executeAgentToolTurn, so keeping only the
// local slice up to date can drop later tool results and break provider-side
// tool_call_id matching.
func queuePendingToolCalls(state *agentLoopState, existing []ToolCall, newCalls []ToolCall) []ToolCall {
	if len(newCalls) == 0 {
		return existing
	}
	queued := append(existing, newCalls...)
	if state != nil {
		state.pendingTCs = queued
	}
	return queued
}

// SanitizeToolMessages validates and repairs tool-call integrity in a message
// slice. It ensures every role=tool message directly follows the assistant
// message that declared its tool_call_id, and strips unmatched tool calls from
// assistant messages. This is the last line of defence before sending messages
// to the LLM provider.
//
// Returns the (possibly modified) message slice and the number of dropped
// messages.
func SanitizeToolMessages(msgs []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, int) {
	if len(msgs) == 0 {
		return msgs, 0
	}

	dropped := 0
	repaired := make([]openai.ChatCompletionMessage, 0, len(msgs))

	for i := 0; i < len(msgs); i++ {
		m := msgs[i]
		switch {
		case m.Role == openai.ChatMessageRoleTool:
			dropped++
			continue
		case m.Role != openai.ChatMessageRoleAssistant || len(m.ToolCalls) == 0:
			repaired = append(repaired, m)
			continue
		}

		expectedIDs := make(map[string]bool, len(m.ToolCalls))
		for _, tc := range m.ToolCalls {
			id := strings.TrimSpace(tc.ID)
			if id != "" {
				expectedIDs[id] = true
			}
		}
		if len(expectedIDs) == 0 {
			m.ToolCalls = nil
			if strings.TrimSpace(m.Content) == "" {
				dropped++
				continue
			}
			repaired = append(repaired, m)
			continue
		}

		consumedIDs := make(map[string]bool, len(expectedIDs))
		toolResults := make([]openai.ChatCompletionMessage, 0, len(expectedIDs))
		j := i + 1
		for j < len(msgs) && msgs[j].Role == openai.ChatMessageRoleTool {
			toolMsg := msgs[j]
			id := strings.TrimSpace(toolMsg.ToolCallID)
			if id == "" || !expectedIDs[id] || consumedIDs[id] {
				dropped++
				j++
				continue
			}
			consumedIDs[id] = true
			toolResults = append(toolResults, toolMsg)
			j++
		}

		matched := make([]openai.ToolCall, 0, len(m.ToolCalls))
		for _, tc := range m.ToolCalls {
			id := strings.TrimSpace(tc.ID)
			if id != "" && consumedIDs[id] {
				matched = append(matched, tc)
			}
		}
		if len(matched) == 0 {
			if strings.TrimSpace(m.Content) == "" {
				dropped++
			} else {
				m.ToolCalls = nil
				repaired = append(repaired, m)
			}
			i = j - 1
			continue
		}

		m.ToolCalls = matched
		repaired = append(repaired, m)
		repaired = append(repaired, toolResults...)
		i = j - 1
	}

	return repaired, dropped
}

var todoCheckboxLinePrefixes = []string{
	"- [ ] ",
	"- [x] ",
	"- [X] ",
	"* [ ] ",
	"* [x] ",
	"* [X] ",
}

func stripLeakedTodoList(content string) string {
	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	removed := 0
	for _, line := range lines {
		isTodo := false
		trimmed := strings.TrimLeft(line, " \t")
		for _, prefix := range todoCheckboxLinePrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				isTodo = true
				break
			}
		}
		if isTodo {
			removed++
			continue
		}
		filtered = append(filtered, line)
	}
	result := strings.Join(filtered, "\n")
	result = strings.TrimSpace(result)
	if removed > 0 && result == "" {
		return content
	}
	return result
}
