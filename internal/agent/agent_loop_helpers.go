package agent

import (
	"encoding/json"
	"log/slog"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

var nowFunc = time.Now

func ShouldReloadCoreMemory(dirty bool, loadedAt time.Time, dbUpdatedAt, cachedUpdatedAt time.Time) bool {
	if dirty {
		return true
	}
	if loadedAt.IsZero() {
		return false
	}
	if !dbUpdatedAt.IsZero() && !cachedUpdatedAt.IsZero() && !dbUpdatedAt.Equal(cachedUpdatedAt) {
		return true
	}
	if nowFunc().Sub(loadedAt) > coreMemCacheTTL {
		return true
	}
	return false
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

var nonAlphaNumPattern = regexp.MustCompile(`[^a-z0-9]+`)
var adaptiveFamilySeedTools = map[string][]string{
	"files": {
		"filesystem", "smart_file_read", "file_search", "file_reader_advanced",
		"detect_file_type", "pdf_operations", "document_creator", "media_conversion", "send_document",
	},
	"shell": {
		"execute_shell", "execute_python", "execute_sandbox", "filesystem",
	},
	"coding": {
		"execute_python", "execute_sandbox", "execute_shell", "document_creator",
	},
	"web": {
		"web_scraper", "site_crawler", "web_capture", "document_creator",
	},
	"deployment": {
		"homepage", "netlify", "homepage_registry", "filesystem", "file_editor",
	},
	"network": {
		"network_ping", "dns_lookup", "port_scanner", "mdns_scan", "upnp_scan",
	},
	"infra": {
		"docker", "proxmox", "tailscale", "github", "ansible", "remote_execution",
	},
	"communication": {
		"fetch_email", "send_email", "send_document", "send_audio", "send_video",
	},
	"automation": {
		"cron_scheduler", "follow_up", "manage_missions", "co_agent",
	},
	"media": {
		"media_registry", "media_conversion", "send_document", "send_audio", "send_video", "send_image", "tts",
		"transcribe_audio", "generate_image", "generate_music", "generate_video", "chromecast",
	},
}

var adaptiveToolNeighbors = map[string][]string{
	// Web & Hosting
	"homepage":          {"netlify", "homepage_registry", "filesystem", "file_editor"},
	"netlify":           {"homepage", "homepage_registry", "filesystem", "file_editor"},
	"homepage_registry": {"homepage", "netlify"},

	// File System & Editing
	"filesystem":           {"file_search", "file_reader_advanced", "file_editor", "manage_memory"},
	"smart_file_read":      {"filesystem", "file_reader_advanced", "file_search"},
	"file_search":          {"filesystem", "file_reader_advanced"},
	"file_reader_advanced": {"filesystem", "file_search", "smart_file_read"},
	"file_editor":          {"filesystem", "json_editor", "yaml_editor", "xml_editor"},
	"json_editor":          {"filesystem", "file_editor"},
	"yaml_editor":          {"filesystem", "file_editor"},
	"xml_editor":           {"filesystem", "file_editor"},

	// Code & Execution
	"execute_shell":        {"filesystem", "file_editor", "file_search", "media_registry", "send_document", "document_creator"},
	"execute_python":       {"filesystem", "execute_sandbox", "media_registry", "send_document", "document_creator"},
	"execute_sandbox":      {"filesystem", "execute_python"},
	"ansible":              {"ssh_exec", "query_inventory", "execute_shell", "filesystem"},
	"remote_execution":     {"transfer_remote_file", "ssh_exec", "execute_shell"},
	"transfer_remote_file": {"remote_execution", "filesystem", "ssh_exec"},

	// Media & Documents
	"document_creator": {"media_registry", "media_conversion", "send_document", "filesystem"},
	"media_registry":   {"document_creator", "media_conversion", "send_document", "filesystem", "generate_image", "generate_video", "send_image", "send_video", "tts", "send_audio", "generate_music"},
	"media_conversion": {"media_registry", "document_creator", "image_processing", "transcribe_audio", "send_audio", "send_video", "send_image", "filesystem"},
	"send_document":    {"media_registry", "document_creator"},
	"send_image":       {"media_registry", "generate_image"},
	"send_audio":       {"media_registry", "tts", "generate_music"},
	"send_video":       {"media_registry", "generate_video", "media_conversion"},
	"generate_image":   {"media_registry", "send_image", "analyze_image"},
	"generate_music":   {"media_registry", "send_audio", "tts"},
	"generate_video":   {"media_registry", "media_conversion", "send_video"},
	"analyze_image":    {"generate_image", "media_registry"},
	"tts":              {"media_registry", "send_audio"},
	"transcribe_audio": {"filesystem", "media_registry"},
	"pdf_operations":   {"detect_file_type", "filesystem", "document_creator"},
	"image_processing": {"detect_file_type", "filesystem", "media_registry"},

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
	"list_skills":                {"execute_skill", "list_skill_templates"},
	"execute_skill":              {"list_skills"},
	"list_skill_templates":       {"create_skill_from_template", "list_skills"},
	"create_skill_from_template": {"list_skill_templates", "save_tool", "execute_skill"},
	"save_tool":                  {"execute_skill", "filesystem"},

	// Agents & Teams
	"co_agent":         {"invasion_control", "follow_up"},
	"invasion_control": {"co_agent", "register_device", "ssh_exec"},
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
//   - All SQLite database files (short-term, long-term, inventory, invasion) + WAL/SHM journals
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
	protected := []string{
		cfg.ConfigPath,
		vaultBase,
		vaultBase + ".lock",
		cfg.SQLite.ShortTermPath,
		cfg.SQLite.ShortTermPath + "-wal",
		cfg.SQLite.ShortTermPath + "-shm",
		cfg.SQLite.LongTermPath,
		cfg.SQLite.LongTermPath + "-wal",
		cfg.SQLite.LongTermPath + "-shm",
		cfg.SQLite.InventoryPath,
		cfg.SQLite.InventoryPath + "-wal",
		cfg.SQLite.InventoryPath + "-shm",
		cfg.SQLite.InvasionPath,
		cfg.SQLite.InvasionPath + "-wal",
		cfg.SQLite.InvasionPath + "-shm",
	}

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
	if family == "" {
		return nil
	}
	return adaptiveFamilySeedTools[family]
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
		add(name)
	}

	// MCP must stay callable once the user enabled it. Hiding the generic bridge
	// causes the model to improvise fake direct tool names like
	// "minimax_understand_image" instead of using mcp_call.
	if cfg.Agent.AllowMCP && cfg.MCP.Enabled {
		add("mcp_call")
	}

	return out
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
		paths, err := guideSearcher.SearchToolGuides(userQuery, 4)
		if err != nil {
			if logger != nil {
				logger.Debug("[AdaptiveTools] Semantic tool search unavailable", "error", err)
			}
		} else {
			for _, path := range paths {
				name := strings.TrimSuffix(filepath.Base(filepath.Clean(path)), filepath.Ext(path))
				add(name)
			}
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

// filterToolSchemas removes tools that are neither in the preferred-tools list
// nor in the always-include list, keeping at most maxTools schemas.
// alwaysInclude tools and skill__/tool__ prefixed tools are never dropped.
// maxTools is a hard cap applied to preferred tools; alwaysInclude tools do not
// count against the cap. maxTools=0 disables the cap entirely.
// This reduces token overhead for rarely-used tools without breaking any dispatch.
func filterToolSchemas(schemas []openai.Tool, frequentTools, alwaysInclude []string, maxTools int, logger *slog.Logger) []openai.Tool {
	keepAlways := make(map[string]bool, len(alwaysInclude))
	for _, t := range alwaysInclude {
		keepAlways[t] = true
	}
	preferredOrder := make([]string, 0, len(frequentTools))
	keepPreferred := make(map[string]bool, len(frequentTools))
	for _, t := range frequentTools {
		if t == "" || keepAlways[t] || keepPreferred[t] {
			continue
		}
		keepPreferred[t] = true
		preferredOrder = append(preferredOrder, t)
	}

	var keptAlways []openai.Tool
	schemaByName := make(map[string]openai.Tool, len(schemas))
	schemaOrder := make([]string, 0, len(schemas))
	for _, s := range schemas {
		if s.Function == nil {
			keptAlways = append(keptAlways, s)
			continue
		}
		name := s.Function.Name
		schemaByName[name] = s
		schemaOrder = append(schemaOrder, name)
		// alwaysInclude and skill__/tool__ prefixed tools are never filtered
		if keepAlways[name] || strings.HasPrefix(name, "skill__") || strings.HasPrefix(name, "tool__") {
			keptAlways = append(keptAlways, s)
		}
	}

	var keptFrequent, dropped []openai.Tool
	consumed := make(map[string]bool, len(preferredOrder))
	for _, name := range preferredOrder {
		schema, ok := schemaByName[name]
		if !ok || consumed[name] {
			continue
		}
		if keepAlways[name] || strings.HasPrefix(name, "skill__") || strings.HasPrefix(name, "tool__") {
			continue
		}
		keptFrequent = append(keptFrequent, schema)
		consumed[name] = true
	}
	for _, name := range schemaOrder {
		if consumed[name] || keepAlways[name] || strings.HasPrefix(name, "skill__") || strings.HasPrefix(name, "tool__") {
			continue
		}
		dropped = append(dropped, schemaByName[name])
	}

	// Enforce maxTools cap on frequent tools (alwaysInclude and skills are entirely exempt from the count)
	if maxTools > 0 {
		if len(keptFrequent) > maxTools {
			// Push excess frequent tools back to dropped (preserving order)
			dropped = append(keptFrequent[maxTools:], dropped...)
			keptFrequent = keptFrequent[:maxTools]
		}
	}

	kept := append(keptAlways, keptFrequent...)

	// Fill remaining slots from dropped tools (preserving original order)
	// Note: we only count keptFrequent towards maxTools, not keptAlways
	if maxTools > 0 {
		remaining := maxTools - len(keptFrequent)
		for i := 0; i < remaining && i < len(dropped); i++ {
			kept = append(kept, dropped[i])
		}
	} else {
		kept = append(kept, dropped...)
	}

	finalDropped := len(schemas) - len(kept)
	if logger != nil && finalDropped > 0 {
		var droppedNames []string
		keptSet := make(map[string]bool, len(kept))
		for _, k := range kept {
			if k.Function != nil {
				keptSet[k.Function.Name] = true
			}
		}
		for _, s := range schemas {
			if s.Function != nil && !keptSet[s.Function.Name] {
				droppedNames = append(droppedNames, s.Function.Name)
			}
		}
		logger.Info("[AdaptiveTools] Filtered tool schemas",
			"kept_always", len(keptAlways), "kept_adaptive", len(kept)-len(keptAlways), "dropped", finalDropped, "max_adaptive", maxTools,
			"dropped_tools", strings.Join(droppedNames, ", "))
	}
	return kept
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
	case "tts":
		var ttsRes struct {
			Status string `json:"status"`
			File   string `json:"file"`
		}
		if json.Unmarshal([]byte(raw), &ttsRes) == nil && ttsRes.Status == "success" {
			mimeType := "audio/mpeg"
			if strings.HasSuffix(ttsRes.File, ".wav") {
				mimeType = "audio/wav"
			}
			evtPayload, _ := json.Marshal(map[string]string{
				"path":      "/tts/" + ttsRes.File,
				"title":     "TTS Audio",
				"mime_type": mimeType,
				"filename":  ttsRes.File,
				"file_path": filepath.Join(dataDir, "tts", ttsRes.File),
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
			broker.Send("document", string(evtPayload))
		}
	}
}

func compactMemoryForPrompt(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	return strings.TrimSpace(text[:maxLen]) + "…"
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
	if _, dropped := SanitizeToolMessages(trimmed); dropped > 0 {
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
// slice. It ensures every role=tool message has a matching tool_call_id in a
// preceding assistant message's ToolCalls, and strips unmatched tool calls
// from assistant messages. This is the last line of defence before sending
// messages to the LLM provider.
//
// Returns the (possibly modified) message slice and the number of dropped
// messages.
func SanitizeToolMessages(msgs []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, int) {
	if len(msgs) == 0 {
		return msgs, 0
	}

	// Phase 1: collect all tool_call_ids from assistant messages and track
	// which assistant message index owns each ID.
	ownerOf := make(map[string]int) // tool_call_id → assistant message index
	for i, m := range msgs {
		if m.Role == openai.ChatMessageRoleAssistant {
			for _, tc := range m.ToolCalls {
				id := strings.TrimSpace(tc.ID)
				if id != "" {
					ownerOf[id] = i
				}
			}
		}
	}

	// Phase 2: validate every role=tool message. Track which tool_call_ids
	// are actually consumed by a tool result so we can detect orphaned calls.
	dropped := 0
	consumedIDs := make(map[string]bool)
	validated := make([]openai.ChatCompletionMessage, 0, len(msgs))

	for _, m := range msgs {
		if m.Role == openai.ChatMessageRoleTool {
			id := strings.TrimSpace(m.ToolCallID)
			if id == "" {
				// Empty tool_call_id — always invalid
				dropped++
				continue
			}
			if _, ok := ownerOf[id]; !ok {
				// No matching tool call in any assistant message
				dropped++
				continue
			}
			consumedIDs[id] = true
			validated = append(validated, m)
			continue
		}
		validated = append(validated, m)
	}

	// Phase 3: strip unmatched tool calls from assistant messages.
	// If an assistant message has ToolCalls whose IDs were never consumed by
	// a tool result, those calls are orphaned and will cause API errors.
	// Iterate backwards so removals don't shift unprocessed indices.
	for i := len(validated) - 1; i >= 0; i-- {
		m := validated[i]
		if m.Role != openai.ChatMessageRoleAssistant || len(m.ToolCalls) == 0 {
			continue
		}
		matched := make([]openai.ToolCall, 0, len(m.ToolCalls))
		for _, tc := range m.ToolCalls {
			id := strings.TrimSpace(tc.ID)
			if id != "" && consumedIDs[id] {
				matched = append(matched, tc)
			}
		}
		if len(matched) != len(m.ToolCalls) {
			if len(matched) == 0 && strings.TrimSpace(m.Content) == "" {
				// Entire assistant message is orphaned tool calls with no content — drop it
				validated = append(validated[:i], validated[i+1:]...)
				dropped++
			} else {
				validated[i].ToolCalls = matched
			}
		}
	}

	return validated, dropped
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
