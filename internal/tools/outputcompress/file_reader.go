// Package outputcompress – file_reader_advanced output compressors.
//
// file_reader_advanced returns JSON with operation-specific data:
//   - count_lines: {"status":"success","data":{"lines":N,"bytes":M}}
//   - head/tail/read_lines: {"status":"success","data":{"start_line":N,"end_line":M,"content":"...","truncated":bool}}
//   - search_context: {"status":"success","data":{"pattern":"...","matches":[{...}],"total_matches":N}}
//
// Strategy:
//   - count_lines: already compact, pass through
//   - head/tail/read_lines: PRESERVE content, compact wrapper metadata
//   - search_context: compact match list with line ranges, limit displayed matches
package outputcompress

import (
	"encoding/json"
	"fmt"
	"strings"
)

// compressFileReaderOutput routes file_reader_advanced output to sub-compressors.
func compressFileReaderOutput(output string) (string, string) {
	clean := strings.TrimSpace(output)

	if !strings.HasPrefix(clean, "{") {
		return compressGeneric(output), "fr-nonjson"
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(clean), &raw); err != nil {
		return compressGeneric(output), "fr-parse-err"
	}

	// Error responses: return as-is
	if statusStr := jsonString(raw["status"]); statusStr == "error" {
		return clean, "fr-error"
	}

	data := raw["data"]
	if data == nil {
		return clean, "fr-no-data"
	}

	var dataObj map[string]json.RawMessage
	if err := json.Unmarshal(data, &dataObj); err != nil {
		// Data is not an object – pass through
		return clean, "fr-simple"
	}

	// Detect operation by data fields
	switch {
	case dataObj["lines"] != nil && dataObj["bytes"] != nil && dataObj["content"] == nil:
		// count_lines: already compact
		return clean, "fr-count-lines"
	case dataObj["content"] != nil:
		// head/tail/read_lines: preserve content, compact wrapper
		return compressFRContent(raw, dataObj), "fr-content"
	case dataObj["matches"] != nil:
		// search_context: compact match list
		return compressFRSearchContext(raw, dataObj), "fr-search"
	default:
		return clean, "fr-generic"
	}
}

// compressFRContent preserves file content but compacts the JSON wrapper.
// From: {"status":"success","data":{"start_line":1,"end_line":100,"total_read":100,"content":"...","truncated":false}}
// To:   "Lines 1-100 (100 lines):\n<content>"
func compressFRContent(raw map[string]json.RawMessage, dataObj map[string]json.RawMessage) string {
	startLine := jsonInt(dataObj["start_line"])
	endLine := jsonInt(dataObj["end_line"])
	totalRead := jsonInt(dataObj["total_read"])
	truncated := jsonBool(dataObj["truncated"])
	content := jsonString(dataObj["content"])

	var sb strings.Builder
	if totalRead > 0 {
		fmt.Fprintf(&sb, "Lines %d-%d (%d lines)", startLine, endLine, totalRead)
	} else {
		fmt.Fprintf(&sb, "Lines %d-%d", startLine, endLine)
	}
	if truncated {
		sb.WriteString(" [truncated]")
	}
	sb.WriteString(":\n")
	sb.WriteString(content)

	return sb.String()
}

// compressFRSearchContext compacts search_context match results.
// From: {"status":"success","data":{"pattern":"error","total_matches":50,"matches":[{"match_line":10,"start_line":8,"end_line":12,"content":"..."},...]}}
// To:   "50 matches for 'error':\n  L8-12: context...\n  L20-24: context...\n  + 40 more"
func compressFRSearchContext(raw map[string]json.RawMessage, dataObj map[string]json.RawMessage) string {
	pattern := jsonString(dataObj["pattern"])
	totalMatches := jsonInt(dataObj["total_matches"])

	type match struct {
		MatchLine int    `json:"match_line"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
		Content   string `json:"content"`
	}

	var matches []match
	if err := json.Unmarshal(dataObj["matches"], &matches); err != nil {
		return rawToString(raw)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d matches for '%s':\n", totalMatches, pattern)

	limit := 15
	if len(matches) < limit {
		limit = len(matches)
	}

	for i := 0; i < limit; i++ {
		m := matches[i]
		content := m.Content
		// Truncate long match content
		if len(content) > 200 {
			content = content[:197] + "..."
		}
		// Collapse whitespace in content
		content = strings.ReplaceAll(content, "\n", " ")
		content = CollapseWhitespace(content)
		fmt.Fprintf(&sb, "  L%d-%d: %s\n", m.StartLine, m.EndLine, content)
	}

	if len(matches) > limit {
		fmt.Fprintf(&sb, "  + %d more matches\n", len(matches)-limit)
	}

	return sb.String()
}
