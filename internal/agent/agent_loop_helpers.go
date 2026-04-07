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
	finalized          bool
}

func (s *streamingAccountingState) recordProviderUsage(prompt, completion int) {
	s.providerPrompt = prompt
	s.providerCompletion = completion
	s.hasProviderUsage = true
}

type toolGuideSearcher interface {
	SearchToolGuides(query string, topK int) ([]string, error)
}

var nonAlphaNumPattern = regexp.MustCompile(`[^a-z0-9]+`)
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
	"document_creator": {"media_registry", "send_document", "filesystem"},
	"media_registry":   {"document_creator", "send_document", "filesystem", "generate_image", "send_image", "tts", "send_audio", "generate_music"},
	"send_document":    {"media_registry", "document_creator"},
	"send_image":       {"media_registry", "generate_image"},
	"send_audio":       {"media_registry", "tts", "generate_music"},
	"generate_image":   {"media_registry", "send_image", "analyze_image"},
	"generate_music":   {"media_registry", "send_audio", "tts"},
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
	"web_scraper":  {"site_crawler", "web_capture", "web_performance_audit", "document_creator"},
	"site_crawler": {"web_scraper", "web_capture"},
	"web_capture":  {"web_scraper", "site_crawler", "web_performance_audit"},
	"site_monitor": {"web_scraper", "network_ping"},

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

	for _, tool := range extractIntentMatchedTools(userQuery, available) {
		add(tool)
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
