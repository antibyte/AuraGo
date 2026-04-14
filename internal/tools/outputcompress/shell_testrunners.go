package outputcompress

import "strings"

func compressGoTest(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	var sb strings.Builder
	var failures []string
	var summary string

	for _, line := range strings.Split(result, "\n") {
		switch {
		case strings.Contains(line, "FAIL"):
			failures = append(failures, line)
		case strings.Contains(line, "PASS") && strings.Contains(line, "ok"):
			// Passing package summary
			sb.WriteString(line + "\n")
		case strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "FAIL\t"):
			sb.WriteString(line + "\n")
		case strings.Contains(line, "=== RUN") || strings.Contains(line, "--- FAIL") ||
			strings.Contains(line, "--- PASS"):
			sb.WriteString(line + "\n")
		case strings.HasPrefix(line, "panic:"):
			failures = append(failures, line)
		}
		// Capture final summary line
		if strings.Contains(line, "FAIL") && (strings.Contains(line, "fail") || strings.Contains(line, "package")) {
			summary = line
		}
	}

	if len(failures) > 0 {
		sb.WriteString("\nFailures:\n")
		for _, f := range failures {
			sb.WriteString("  " + f + "\n")
		}
	}

	if summary != "" {
		sb.WriteString("\nSummary: " + summary + "\n")
	}

	compressed := sb.String()
	if compressed == "" {
		return compressGeneric(result)
	}
	return compressed
}

// compressPytest extracts failures and summary from pytest output.
func compressPytest(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	var sb strings.Builder
	var inFailure bool

	for _, line := range strings.Split(result, "\n") {
		switch {
		case strings.HasPrefix(line, "FAILED"):
			sb.WriteString(line + "\n")
		case strings.HasPrefix(line, "ERROR"):
			sb.WriteString(line + "\n")
		case strings.HasPrefix(line, "=== ") && strings.Contains(line, "failed"):
			sb.WriteString(line + "\n")
			inFailure = false
		case strings.HasPrefix(line, "=== ") && strings.Contains(line, "FAILURES"):
			inFailure = true
			sb.WriteString(line + "\n")
		case inFailure && line == "":
			inFailure = false
		case inFailure:
			sb.WriteString(line + "\n")
		case strings.HasPrefix(line, "PASSED"):
			// Skip passing tests to save tokens
		}
	}

	compressed := sb.String()
	if compressed == "" {
		return compressGeneric(result)
	}
	return compressed
}

// compressCargoTest extracts failures and summary from cargo test output.
func compressCargoTest(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	var sb strings.Builder

	for _, line := range strings.Split(result, "\n") {
		switch {
		case strings.Contains(line, "FAILED"):
			sb.WriteString(line + "\n")
		case strings.Contains(line, "test result:"):
			sb.WriteString(line + "\n")
		case strings.HasPrefix(line, "failures:"):
			sb.WriteString(line + "\n")
		case strings.Contains(line, "---- ") && strings.Contains(line, "stdout ----"):
			sb.WriteString(line + "\n")
		case strings.HasPrefix(line, "thread ") && strings.Contains(line, "panicked"):
			sb.WriteString(line + "\n")
		}
	}

	compressed := sb.String()
	if compressed == "" {
		return compressGeneric(result)
	}
	return compressed
}

// ─── File Listing Filters ───────────────────────────────────────────────────

// compressLsTree groups files by directory for ls/tree output.
func compressJsTest(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")

	var sb strings.Builder
	failMode := false
	testCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		// Detect FAIL sections
		if strings.Contains(lower, "fail") && (strings.Contains(lower, "test") ||
			strings.Contains(lower, "suite") || strings.Contains(lower, "●") ||
			strings.Contains(lower, "✕") || strings.Contains(lower, "×")) {
			failMode = true
			sb.WriteString(line + "\n")
			continue
		}

		// Continue FAIL context
		if failMode {
			if trimmed == "" || strings.HasPrefix(trimmed, "PASS") ||
				strings.HasPrefix(lower, "test suite") || strings.Contains(lower, "test files") {
				failMode = false
			} else {
				sb.WriteString(line + "\n")
				continue
			}
		}

		// Summary lines
		if strings.Contains(lower, "test") && (strings.Contains(lower, "passed") ||
			strings.Contains(lower, "failed") || strings.Contains(lower, "total") ||
			strings.Contains(lower, "skipped") || strings.Contains(lower, "suites")) {
			sb.WriteString(line + "\n")
		}

		// Error/stack traces
		if strings.Contains(lower, "error") || strings.Contains(lower, "expected") ||
			strings.Contains(lower, "assert") || strings.Contains(lower, "thrown") {
			sb.WriteString(line + "\n")
		}

		testCount++
	}

	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// ─── Lint Filter ────────────────────────────────────────────────────────────

// compressLint groups lint output by file/rule and extracts key findings.
