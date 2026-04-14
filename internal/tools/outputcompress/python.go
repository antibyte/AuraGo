package outputcompress

import (
	"fmt"
	"strings"
)


// ─── Python Output Filter ───────────────────────────────────────────────────
func compressPythonOutput(output string) (string, string) {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	result = filterPythonTraceback(result)
	result = DeduplicateLines(result)

	lines := strings.Split(result, "\n")
	if len(lines) > 100 {
		result = TailFocus(result, 20, 50, 5)
	}

	return result, "python"
}

// filterPythonTraceback keeps only user-code frames in tracebacks.
func filterPythonTraceback(output string) string {
	if !strings.Contains(output, "Traceback (most recent call last)") {
		return output
	}

	var sb strings.Builder
	lines := strings.Split(output, "\n")
	inTraceback := false
	systemFrames := 0
	userFrames := []string{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(line, "Traceback (most recent call last)") {
			inTraceback = true
			sb.WriteString(line + "\n")
			continue
		}

		if inTraceback {
			if strings.HasPrefix(trimmed, "File ") {
				// Distinguish user code from library code
				if isUserCode(trimmed) {
					userFrames = append(userFrames, line)
					systemFrames = 0
				} else {
					systemFrames++
				}
			} else if strings.HasPrefix(trimmed, "Error") ||
				strings.HasPrefix(trimmed, "Exception") ||
				strings.HasPrefix(trimmed, "raise") ||
				strings.Contains(trimmed, "Error:") ||
				strings.Contains(trimmed, "Exception:") {
				// Always keep error type
				if systemFrames > 3 {
					sb.WriteString(fmt.Sprintf("  [... %d library frames omitted ...]\n", systemFrames))
				}
				for _, f := range userFrames {
					sb.WriteString(f + "\n")
				}
				userFrames = nil
				sb.WriteString(line + "\n")
				inTraceback = false
				continue
			}
		} else {
			sb.WriteString(line + "\n")
		}
	}

	// Flush remaining user frames
	if len(userFrames) > 0 {
		if systemFrames > 3 {
			sb.WriteString(fmt.Sprintf("  [... %d library frames omitted ...]\n", systemFrames))
		}
		for _, f := range userFrames {
			sb.WriteString(f + "\n")
		}
	}

	result := sb.String()
	if result == "" {
		return output
	}
	return result
}

// ─── API Output Filter ──────────────────────────────────────────────────────

// compressAPIOutput applies JSON compaction for API tool outputs.
// For Home Assistant, GitHub, SQL, filesystem, file_reader_advanced, and
// smart_file_read tools, it routes to domain-specific compressors.
func isUserCode(fileLine string) bool {
	// User code is typically in the workspace, not in site-packages or stdlib
	nonUserCode := []string{
		"site-packages/",
		"/usr/lib/python",
		"/usr/local/lib/python",
		"lib/python3.",
		"<frozen",
		"__pycache__",
		"/opt/homebrew/",
	}
	for _, pattern := range nonUserCode {
		if strings.Contains(fileLine, pattern) {
			return false
		}
	}
	return true
}

// groupByDir groups file paths by their parent directory.
