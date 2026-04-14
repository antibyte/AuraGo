package outputcompress

import (
	"fmt"
	"strings"
)

func compressLint(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	// Group by file
	fileGroups := make(map[string][]string)
	var order []string
	var summary []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Summary lines
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "problem") || strings.Contains(lower, "error") ||
			strings.Contains(lower, "warning") || strings.Contains(lower, "issue") ||
			strings.Contains(lower, "found") || strings.Contains(lower, "total") {
			summary = append(summary, trimmed)
			continue
		}

		// Try to extract file path
		file := extractLintFile(line)
		if file != "" {
			if _, exists := fileGroups[file]; !exists {
				order = append(order, file)
			}
			fileGroups[file] = append(fileGroups[file], trimmed)
		}
	}

	var sb strings.Builder

	// Summary first
	for _, s := range summary {
		sb.WriteString(s + "\n")
	}

	if len(summary) > 0 && len(fileGroups) > 0 {
		sb.WriteString("\n")
	}

	// Per-file: count + first 3 issues
	for _, file := range order {
		issues := fileGroups[file]
		sb.WriteString(fmt.Sprintf("%s (%d issues)\n", file, len(issues)))
		limit := 3
		if len(issues) < limit {
			limit = len(issues)
		}
		for i := 0; i < limit; i++ {
			sb.WriteString("  " + issues[i] + "\n")
		}
		if len(issues) > 3 {
			sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(issues)-3))
		}
	}

	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// extractLintFile tries to extract a file path from a lint output line.
func extractLintFile(line string) string {
	// Common patterns: "path/to/file.js:line:col" or "path/to/file.py:42"
	for _, sep := range []string{":", " "} {
		parts := strings.SplitN(line, sep, 2)
		if len(parts) > 0 {
			candidate := parts[0]
			// Check if it looks like a file path
			if strings.Contains(candidate, "/") || strings.Contains(candidate, "\\") ||
				strings.Contains(candidate, ".") {
				// Strip leading whitespace and common prefixes
				candidate = strings.TrimSpace(candidate)
				for _, prefix := range []string{"./", ".\\"} {
					if strings.HasPrefix(candidate, prefix) {
						return candidate
					}
				}
				if strings.Contains(candidate, ".") && len(candidate) > 3 {
					return candidate
				}
			}
		}
	}
	return ""
}

// ─── AWS Filter ─────────────────────────────────────────────────────────────

// compressAws handles AWS CLI output by subcommand.
