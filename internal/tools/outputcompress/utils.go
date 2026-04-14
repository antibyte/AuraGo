package outputcompress

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ─── JSON Helper Functions (zentralisiert) ───────────────────────────────────

// jsonString extracts a string value from raw JSON bytes.
func jsonString(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// jsonInt extracts an int value from raw JSON bytes.
func jsonInt(raw json.RawMessage) int {
	if raw == nil {
		return 0
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0
	}
	return n
}

// jsonBool extracts a bool value from raw JSON bytes.
func jsonBool(raw json.RawMessage) bool {
	if raw == nil {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return false
	}
	return b
}

// rawToString converts a map of raw JSON back to a string.
func rawToString(raw map[string]json.RawMessage) string {
	b, err := json.Marshal(raw)
	if err != nil {
		return ""
	}
	return string(b)
}

// formatFileSize formats bytes into human-readable size.
func formatFileSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// ─── Timestamp & Log Processing ──────────────────────────────────────────────

func stripTimestamps(input string) string {
	// Common timestamp patterns at the start of lines
	tsPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:?\d{2})?\s*`),
		regexp.MustCompile(`^\[\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}[^\]]*\]\s*`),
		regexp.MustCompile(`^[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}\s*`),
	}

	lines := strings.Split(input, "\n")
	var sb strings.Builder
	for _, line := range lines {
		modified := line
		for _, pat := range tsPatterns {
			modified = pat.ReplaceAllString(modified, "")
		}
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(modified)
	}
	return sb.String()
}

// isUserCode determines if a traceback File line refers to user code.
func groupByDir(files []string) string {
	dirs := make(map[string][]string)
	var order []string

	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		// Remove status markers
		parts := strings.SplitN(f, " ", 2)
		var marker, path string
		if len(parts) == 2 {
			marker = parts[0]
			path = parts[1]
		} else {
			path = f
		}

		lastSlash := strings.LastIndex(path, "/")
		var dir string
		if lastSlash > 0 {
			dir = path[:lastSlash]
		} else {
			dir = "."
		}

		if _, exists := dirs[dir]; !exists {
			order = append(order, dir)
		}

		entry := path
		if lastSlash > 0 {
			entry = path[lastSlash+1:]
		}
		if marker != "" {
			entry = marker + " " + entry
		}
		dirs[dir] = append(dirs[dir], entry)
	}

	var sb strings.Builder
	for _, dir := range order {
		files := dirs[dir]
		if dir == "." {
			sb.WriteString(fmt.Sprintf("  (%d files): %s\n", len(files), strings.Join(files, ", ")))
		} else {
			sb.WriteString(fmt.Sprintf("  %s/ (%d): %s\n", dir, len(files), strings.Join(files, ", ")))
		}
	}
	return sb.String()
}

// compactJSON removes null fields and truncates long arrays.
// Uses proper JSON parsing instead of line-based manipulation.
func compactJSON(input string) string {
	// Parse JSON properly
	var data interface{}
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		// Not valid JSON, fall back to line-based approach
		return compactJSONFallback(input)
	}

	// Remove empty values recursively
	data = removeEmptyValues(data)

	// Marshal back with indentation
	result, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return input
	}

	return string(result)
}

// removeEmptyValues recursively removes null, empty strings, empty arrays,
// and empty objects from JSON data.
func removeEmptyValues(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			cleaned := removeEmptyValues(val)
			// Skip null, empty strings, empty arrays, empty objects
			if cleaned == nil || cleaned == "" {
				continue
			}
			// Check for empty slice
			if arr, ok := cleaned.([]interface{}); ok && len(arr) == 0 {
				continue
			}
			// Check for empty map
			if m, ok := cleaned.(map[string]interface{}); ok && len(m) == 0 {
				continue
			}
			result[key] = cleaned
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0)
		for _, item := range v {
			cleaned := removeEmptyValues(item)
			// Skip null, empty strings, empty arrays, empty objects
			if cleaned == nil || cleaned == "" {
				continue
			}
			// Check for empty slice
			if arr, ok := cleaned.([]interface{}); ok && len(arr) == 0 {
				continue
			}
			// Check for empty map
			if m, ok := cleaned.(map[string]interface{}); ok && len(m) == 0 {
				continue
			}
			result = append(result, cleaned)
		}
		if len(result) == 0 {
			return nil
		}
		return result
	default:
		return v
	}
}

// compactJSONFallback is the fallback for invalid JSON.
func compactJSONFallback(input string) string {
	lines := strings.Split(input, "\n")
	if len(lines) < 20 {
		return input
	}

	var sb strings.Builder
	skipped := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, ": null") || strings.Contains(trimmed, ":null") ||
			strings.Contains(trimmed, ": []") || strings.Contains(trimmed, ":[]") ||
			strings.Contains(trimmed, ": \"\"") || strings.Contains(trimmed, ":\"\"") {
			skipped++
			continue
		}
		sb.WriteString(line + "\n")
	}

	if skipped > 0 {
		fmt.Fprintf(&sb, "  [%d empty/null fields omitted]\n", skipped)
	}

	return sb.String()
}

// compressLogOutput applies the standard log compression pipeline:
// Strip ANSI, collapse whitespace, strip timestamps, deduplicate lines,
// and tail-focus if the output is still very long.
func compressLogOutput(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	result = stripTimestamps(result)
	result = DeduplicateLines(result)

	lines := strings.Split(result, "\n")
	if len(lines) > tailFocusLogsHead+tailFocusLogsTail+tailFocusLogsMinGap {
		result = TailFocus(result, tailFocusLogsHead, tailFocusLogsTail, tailFocusLogsMinGap)
	}

	return result
}
