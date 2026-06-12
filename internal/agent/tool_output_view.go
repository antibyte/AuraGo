package agent

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"aurago/internal/memory"
)

type toolOutputViewRequest struct {
	View      string
	Query     string
	StartLine int
	EndLine   int
	MaxLines  int
	MaxChars  int
	Reason    string
}

func renderToolOutputView(out *memory.CompressedToolOutput, req toolOutputViewRequest) (string, bool, error) {
	if out == nil {
		return "", false, fmt.Errorf("output is nil")
	}
	view := strings.ToLower(strings.TrimSpace(req.View))
	if view == "" {
		view = "summary"
	}
	var content string
	switch view {
	case "summary":
		content = firstNonEmptyToolString(out.SummaryContent, out.ViewContent, out.CompressedContent)
	case "head":
		content = selectHeadLines(out.OriginalContent, req.MaxLines)
	case "tail":
		content = selectTailLines(out.OriginalContent, req.MaxLines)
	case "range":
		content = selectLineRange(out.OriginalContent, req.StartLine, req.EndLine)
	case "grep":
		content = selectGrepLines(out.OriginalContent, req.Query)
	case "jsonpath":
		value, err := selectJSONPath(out.OriginalContent, req.Query)
		if err != nil {
			return "", false, err
		}
		content = value
	case "full":
		content = out.OriginalContent
	default:
		return "", false, fmt.Errorf("unknown view %q", req.View)
	}
	return capToolOutputView(content, req.MaxChars)
}

func selectHeadLines(content string, maxLines int) string {
	lines := splitOutputLines(content)
	if maxLines <= 0 {
		maxLines = 40
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}

func selectTailLines(content string, maxLines int) string {
	lines := splitOutputLines(content)
	if maxLines <= 0 {
		maxLines = 40
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}

func selectLineRange(content string, startLine, endLine int) string {
	lines := splitOutputLines(content)
	if startLine <= 0 {
		startLine = 1
	}
	if endLine <= 0 || endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > endLine || startLine > len(lines) {
		return ""
	}
	return strings.Join(lines[startLine-1:endLine], "\n")
}

func selectGrepLines(content, query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	needle := strings.ToLower(query)
	var matches []string
	for _, line := range splitOutputLines(content) {
		if strings.Contains(strings.ToLower(line), needle) {
			matches = append(matches, line)
		}
	}
	return strings.Join(matches, "\n")
}

func splitOutputLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func capToolOutputView(content string, maxChars int) (string, bool, error) {
	if maxChars <= 0 || len(content) <= maxChars {
		return content, false, nil
	}
	return content[:maxChars] + fmt.Sprintf("\n[TRUNCATED: view was %d chars, returned first %d]", len(content), maxChars), true, nil
}

func selectJSONPath(content, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("query is required for jsonpath view")
	}
	if !strings.HasPrefix(query, "$") {
		return "", fmt.Errorf("jsonpath query must start with $")
	}
	var value interface{}
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		return "", fmt.Errorf("jsonpath input is not valid JSON: %w", err)
	}
	remaining := strings.TrimPrefix(query, "$")
	for remaining != "" {
		if strings.HasPrefix(remaining, ".") {
			remaining = remaining[1:]
			key, rest := nextJSONPathKey(remaining)
			obj, ok := value.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("jsonpath %q expected object before %q", query, key)
			}
			var exists bool
			value, exists = obj[key]
			if !exists {
				return "", fmt.Errorf("jsonpath key %q not found", key)
			}
			remaining = rest
			continue
		}
		if strings.HasPrefix(remaining, "[") {
			end := strings.Index(remaining, "]")
			if end < 0 {
				return "", fmt.Errorf("jsonpath index is missing closing bracket")
			}
			index, err := strconv.Atoi(remaining[1:end])
			if err != nil {
				return "", fmt.Errorf("jsonpath index %q is invalid", remaining[1:end])
			}
			arr, ok := value.([]interface{})
			if !ok {
				return "", fmt.Errorf("jsonpath expected array before index %d", index)
			}
			if index < 0 || index >= len(arr) {
				return "", fmt.Errorf("jsonpath index %d out of range", index)
			}
			value = arr[index]
			remaining = remaining[end+1:]
			continue
		}
		return "", fmt.Errorf("unsupported jsonpath segment %q", remaining)
	}
	b, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("jsonpath marshal: %w", err)
	}
	return string(b), nil
}

func nextJSONPathKey(path string) (string, string) {
	for i, r := range path {
		if r == '.' || r == '[' {
			return path[:i], path[i:]
		}
	}
	return path, ""
}
