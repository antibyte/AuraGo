package agent

import (
	"encoding/json"
	"log/slog"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

type toolGuideSearcher interface {
	SearchToolGuides(query string, topK int) ([]string, error)
}

var nonAlphaNumPattern = regexp.MustCompile(`[^a-z0-9]+`)
var adaptiveToolNeighbors = map[string][]string{
	"homepage":             {"netlify", "homepage_registry", "filesystem"},
	"netlify":              {"homepage", "homepage_registry", "filesystem"},
	"homepage_registry":    {"homepage", "netlify"},
	"filesystem":           {"file_search", "file_reader_advanced", "file_editor"},
	"smart_file_read":      {"filesystem", "file_reader_advanced", "file_search"},
	"file_search":          {"filesystem", "file_reader_advanced"},
	"file_reader_advanced": {"filesystem", "file_search", "smart_file_read"},
	"file_editor":          {"filesystem", "json_editor", "yaml_editor", "xml_editor"},
	"json_editor":          {"filesystem", "file_editor"},
	"yaml_editor":          {"filesystem", "file_editor"},
	"xml_editor":           {"filesystem", "file_editor"},
	"execute_shell":        {"filesystem", "file_search"},
	"execute_python":       {"filesystem", "execute_sandbox"},
	"execute_sandbox":      {"filesystem", "execute_python"},
	"manage_memory":        {"query_memory", "remember"},
	"query_memory":         {"manage_memory", "remember"},
	"remember":             {"manage_memory", "query_memory"},
	"network_ping":         {"dns_lookup", "port_scanner", "mdns_scan"},
	"dns_lookup":           {"network_ping", "whois_lookup"},
	"port_scanner":         {"network_ping"},
	"web_scraper":          {"site_crawler", "web_capture", "web_performance_audit"},
	"site_crawler":         {"web_scraper", "web_capture"},
	"web_capture":          {"web_scraper", "site_crawler", "web_performance_audit"},
	"pdf_operations":       {"detect_file_type", "filesystem"},
	"image_processing":     {"detect_file_type", "filesystem"},
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
		return resultContent[:200]
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

	// Enforce maxTools cap on frequent tools (alwaysInclude tools are exempt)
	if maxTools > 0 {
		remaining := maxTools - len(keptAlways)
		if remaining < 0 {
			remaining = 0
		}
		if len(keptFrequent) > remaining {
			// Push excess frequent tools back to dropped (preserving order)
			dropped = append(keptFrequent[remaining:], dropped...)
			keptFrequent = keptFrequent[:remaining]
		}
	}

	kept := append(keptAlways, keptFrequent...)

	// If at or over the limit (or no limit), done
	if maxTools <= 0 || len(kept) >= maxTools {
		if logger != nil && len(dropped) > 0 {
			logger.Info("[AdaptiveTools] Filtered tool schemas",
				"kept", len(kept), "dropped", len(dropped), "max", maxTools)
		}
		return kept
	}

	// Fill remaining slots from dropped tools (preserving original order)
	remaining := maxTools - len(kept)
	for i := 0; i < remaining && i < len(dropped); i++ {
		kept = append(kept, dropped[i])
	}

	finalDropped := len(schemas) - len(kept)
	if logger != nil && finalDropped > 0 {
		logger.Info("[AdaptiveTools] Filtered tool schemas",
			"kept", len(kept), "dropped", finalDropped, "max", maxTools)
	}
	return kept
}
