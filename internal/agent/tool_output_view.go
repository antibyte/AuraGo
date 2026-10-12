package agent

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"aurago/internal/memory"
)

type toolOutputViewRequest struct {
	View      string
	Query     string
	StartLine int
	EndLine   int
	MaxLines  int
	MaxChars  int
	Offset    int
	Reason    string
}

// renderToolOutputView selects a view of payload, the readable archived
// output (vaultReadablePayload): line numbers, grep and jsonpath see the
// tool's own text without presentation prefixes or isolation escaping. The
// caller pages and bounds the result.
func renderToolOutputView(out *memory.CompressedToolOutput, payload string, req toolOutputViewRequest) (string, error) {
	if out == nil {
		return "", fmt.Errorf("output is nil")
	}
	switch strings.ToLower(strings.TrimSpace(req.View)) {
	case "summary":
		return firstNonEmptyToolString(out.SummaryContent, out.ViewContent, out.CompressedContent), nil
	case "head":
		return selectHeadLines(payload, req.MaxLines), nil
	case "tail":
		return selectTailLines(payload, req.MaxLines), nil
	case "range":
		return selectLineRange(payload, req.StartLine, req.EndLine), nil
	case "grep":
		return selectGrepLines(payload, req.Query), nil
	case "jsonpath":
		return selectJSONPath(payload, req.Query)
	case "", "full":
		return payload, nil
	default:
		return "", fmt.Errorf("unknown view %q", req.View)
	}
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

// grepContextBytes is the context kept on each side of a match in a long
// line; shorter lines match whole.
const grepContextBytes = 200

// selectGrepLines returns the lines containing query (case-insensitive). A
// long line (minified JSON or HTML) yields a window around each match,
// labelled with its line and byte position.
func selectGrepLines(content, query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	needle := strings.ToLower(query)
	var matches []string
	for i, line := range splitOutputLines(content) {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, needle) {
			continue
		}
		if len(line) <= 4*grepContextBytes || len(lower) != len(line) {
			matches = append(matches, line)
			continue
		}
		for from := 0; from < len(lower); {
			at := strings.Index(lower[from:], needle)
			if at < 0 {
				break
			}
			at += from
			start, end := max(at-grepContextBytes, 0), min(at+len(needle)+grepContextBytes, len(line))
			for start > 0 && !utf8.RuneStart(line[start]) {
				start--
			}
			for end < len(line) && !utf8.RuneStart(line[end]) {
				end++
			}
			matches = append(matches, fmt.Sprintf("[line %d, byte %d] %s", i+1, at, line[start:end]))
			from = end
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
