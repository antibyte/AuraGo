package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/desktop"
)

func virtualDesktopLargeReadPayload(entry desktop.FileEntry, content string) map[string]interface{} {
	head, tail := splitLargeVirtualDesktopContent(content)
	return map[string]interface{}{
		"entry":             entry,
		"content":           head + "\n\n[... large desktop file truncated; use virtual_desktop.patch_file for exact edits instead of reading the whole file ...]\n\n" + tail,
		"content_truncated": true,
		"original_size":     len(content),
		"shown_size":        len(head) + len(tail),
		"suggested_tools": []string{
			"virtual_desktop.search_file",
			"virtual_desktop.read_file_excerpt",
			"virtual_desktop.patch_file",
			"virtual_desktop.write_file",
			"virtual_desktop.open_in_app",
			"text_diff",
		},
		"editing_hint": "This desktop file is larger than 8 KB. Do not ask the user for anchors. Use virtual_desktop.search_file and read_file_excerpt to locate the relevant block yourself, then use virtual_desktop.patch_file with exact replacements, prepend_text, or append_text; use write_file only when replacing the whole file intentionally, then open_in_app to show the result.",
	}
}

func splitLargeVirtualDesktopContent(content string) (string, string) {
	const headLimit = 4096
	const tailLimit = 2048
	runes := []rune(content)
	if len(runes) <= headLimit+tailLimit {
		return content, ""
	}
	return string(runes[:headLimit]), string(runes[len(runes)-tailLimit:])
}

type virtualDesktopSearchMatch struct {
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	ByteOffset int    `json:"byte_offset"`
	Preview    string `json:"preview"`
	Context    string `json:"context"`
}

func virtualDesktopSearchText(content, query string, caseSensitive bool, maxMatches, contextLines int) []virtualDesktopSearchMatch {
	if maxMatches <= 0 || maxMatches > 20 {
		maxMatches = 8
	}
	if contextLines < 0 {
		contextLines = 0
	}
	if contextLines > 10 {
		contextLines = 10
	}
	haystack := content
	needle := query
	if !caseSensitive {
		haystack = strings.ToLower(haystack)
		needle = strings.ToLower(needle)
	}
	lines := strings.Split(content, "\n")
	searchLines := strings.Split(haystack, "\n")
	matches := make([]virtualDesktopSearchMatch, 0, maxMatches)
	byteOffset := 0
	for i, line := range searchLines {
		column := strings.Index(line, needle)
		if column >= 0 {
			contextStart := virtualDesktopMaxInt(0, i-contextLines)
			contextEnd := virtualDesktopMinInt(len(lines), i+contextLines+1)
			matches = append(matches, virtualDesktopSearchMatch{
				Line:       i + 1,
				Column:     column + 1,
				ByteOffset: byteOffset + column,
				Preview:    strings.TrimSpace(lines[i]),
				Context:    strings.Join(lines[contextStart:contextEnd], "\n"),
			})
			if len(matches) >= maxMatches {
				break
			}
		}
		byteOffset += len(lines[i]) + 1
	}
	return matches
}

func virtualDesktopLineExcerpt(content string, lineStart, lineCount int) (string, int, int) {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	if lineStart < 1 {
		lineStart = 1
	}
	if lineStart > totalLines {
		return "", totalLines, totalLines
	}
	if lineCount <= 0 || lineCount > 240 {
		lineCount = 80
	}
	startIdx := lineStart - 1
	endIdx := virtualDesktopMinInt(totalLines, startIdx+lineCount)
	return strings.Join(lines[startIdx:endIdx], "\n"), endIdx, totalLines
}

func virtualDesktopMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func virtualDesktopMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type virtualDesktopReplacement struct {
	Find    string `json:"find"`
	Replace string `json:"replace"`
}

const virtualDesktopReplacementShape = `replacements must be an array of objects like [{"find":"old text","replace":"new text"}]`

func virtualDesktopApplyTextPatch(content string, args map[string]interface{}) (string, int, int, error) {
	next := content
	replacementCount := 0
	appliedOperations := 0
	var replacements []virtualDesktopReplacement
	if raw, ok := args["replacements"]; ok {
		parsed, err := virtualDesktopParseReplacements(raw)
		if err != nil {
			return "", 0, 0, fmt.Errorf("invalid replacements: %w", err)
		}
		replacements = parsed
	}
	for _, repl := range replacements {
		if repl.Find == "" {
			return "", 0, 0, fmt.Errorf("replacement find text is required")
		}
		count := strings.Count(next, repl.Find)
		if count == 0 {
			return "", 0, 0, fmt.Errorf("replacement text not found")
		}
		next = strings.ReplaceAll(next, repl.Find, repl.Replace)
		replacementCount += count
		appliedOperations++
	}
	if prepend := virtualDesktopString(args, "prepend_text"); prepend != "" {
		next = prepend + next
		appliedOperations++
	}
	if appendText := virtualDesktopString(args, "append_text"); appendText != "" {
		next += appendText
		appliedOperations++
	}
	return next, replacementCount, appliedOperations, nil
}

func virtualDesktopParseReplacements(raw interface{}) ([]virtualDesktopReplacement, error) {
	if raw == nil {
		return nil, nil
	}
	switch typed := raw.(type) {
	case string:
		return virtualDesktopParseReplacementJSON(typed)
	case map[string]interface{}, map[string]string:
		return virtualDesktopParseReplacementItem(typed)
	case []interface{}:
		replacements := make([]virtualDesktopReplacement, 0, len(typed))
		for _, item := range typed {
			parsed, err := virtualDesktopParseReplacementItem(item)
			if err != nil {
				return nil, err
			}
			replacements = append(replacements, parsed...)
		}
		return replacements, nil
	}

	var replacements []virtualDesktopReplacement
	if err := mapToStruct(raw, &replacements); err != nil {
		return nil, fmt.Errorf("%s", virtualDesktopReplacementShape)
	}
	return replacements, nil
}

func virtualDesktopParseReplacementItem(raw interface{}) ([]virtualDesktopReplacement, error) {
	switch typed := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			return virtualDesktopParseReplacementJSON(trimmed)
		}
		return nil, fmt.Errorf("%s", virtualDesktopReplacementShape)
	}
	var replacement virtualDesktopReplacement
	if err := mapToStruct(raw, &replacement); err != nil {
		return nil, fmt.Errorf("%s", virtualDesktopReplacementShape)
	}
	return []virtualDesktopReplacement{replacement}, nil
}

func virtualDesktopParseReplacementJSON(raw string) ([]virtualDesktopReplacement, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	var replacements []virtualDesktopReplacement
	if err := json.Unmarshal([]byte(trimmed), &replacements); err == nil {
		return replacements, nil
	}
	var replacement virtualDesktopReplacement
	if err := json.Unmarshal([]byte(trimmed), &replacement); err == nil {
		return []virtualDesktopReplacement{replacement}, nil
	}
	return nil, fmt.Errorf("%s", virtualDesktopReplacementShape)
}
