package outputcompress

import (
	"fmt"
	"regexp"
	"strings"
)

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
func compactJSON(input string) string {
	// Simple approach: remove lines containing : null, : null,
	// and truncate arrays that are very long
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
	if len(lines) > 100 {
		result = TailFocus(result, 10, 50, 5)
	}

	return result
}
