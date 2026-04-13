package outputcompress

import (
	"fmt"
	"regexp"
	"strings"
)

// compressShellOutput analyses the command string and routes to the
// appropriate domain-specific filter.
func compressShellOutput(command, output string) (string, string) {
	sig := commandSignature(command)
	parts := strings.Fields(sig)
	if len(parts) == 0 {
		return compressGeneric(output), "generic"
	}

	bin := parts[0]
	sub := ""
	if len(parts) >= 2 {
		sub = parts[1]
	}

	switch {
	case bin == "git":
		return compressGit(sub, output)
	case bin == "docker" || bin == "podman":
		return compressContainer(sub, output)
	case bin == "kubectl" || bin == "k3s" || bin == "k9s":
		return compressK8s(sub, output)
	case bin == "go" && sub == "test":
		return compressGoTest(output), "go-test"
	case bin == "python" && (sub == "-m" || sub == "pytest"):
		return compressPytest(output), "pytest"
	case bin == "cargo" && sub == "test":
		return compressCargoTest(output), "cargo-test"
	case bin == "npm" && (sub == "test" || sub == "run"):
		return compressGeneric(output), "npm-test"
	case bin == "npx" && (sub == "vitest" || sub == "jest"):
		return compressGeneric(output), "js-test"
	case bin == "ls" || bin == "dir" || bin == "tree":
		return compressLsTree(output), "ls-tree"
	case bin == "find":
		return compressFind(output), "find"
	case bin == "grep" || bin == "rg" || bin == "ag" || bin == "ack":
		return compressGrep(output), "grep"
	case bin == "curl" || bin == "wget":
		return compressGeneric(output), "curl"
	case bin == "journalctl" || bin == "logcli" || strings.HasSuffix(bin, "log"):
		return compressLogs(output), "logs"
	default:
		return compressGeneric(output), "generic"
	}
}

// ─── Git Filters ────────────────────────────────────────────────────────────

func compressGit(sub, output string) (string, string) {
	switch sub {
	case "status":
		return compressGitStatus(output), "git-status"
	case "log":
		return compressGitLog(output), "git-log"
	case "diff":
		return compressGitDiff(output), "git-diff"
	case "branch":
		return compressGeneric(output), "git-branch"
	default:
		return compressGeneric(output), "git-generic"
	}
}

// compressGitStatus groups files by status category.
// Handles both `git status --short` format (XY path) and verbose format.
func compressGitStatus(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	// Detect format: short format lines start with status markers like "M ", "??", "A ", etc.
	isShortFormat := false
	for _, line := range strings.Split(result, "\n") {
		line = strings.TrimSpace(line)
		if len(line) >= 3 && isShortStatusMarker(line[:2]) {
			isShortFormat = true
			break
		}
	}

	if isShortFormat {
		return compressGitStatusShort(result)
	}
	return compressGitStatusVerbose(result)
}

// isShortStatusMarker checks if a two-char string is a valid git short status marker.
func isShortStatusMarker(s string) bool {
	if len(s) < 2 {
		return false
	}
	validChars := "MADRC?! "
	for _, c := range s {
		if !strings.ContainsRune(validChars, c) {
			return false
		}
	}
	return s != "  " // "  " means clean, not a marker
}

// compressGitStatusShort handles `git status --short` output.
func compressGitStatusShort(output string) string {
	var staged, unstaged, untracked []string

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 3 {
			continue
		}
		marker := line[:2]
		file := strings.TrimSpace(line[2:])
		if file == "" {
			continue
		}

		switch {
		case strings.Contains(marker, "?"):
			untracked = append(untracked, file)
		case marker[0] != ' ':
			// Index (staged) change
			staged = append(staged, string(marker[0])+" "+file)
		case marker[1] != ' ':
			// Working tree (unstaged) change
			unstaged = append(unstaged, string(marker[1])+" "+file)
		}
	}

	return buildGitStatusSummary(staged, unstaged, untracked)
}

// compressGitStatusVerbose handles verbose `git status` output.
func compressGitStatusVerbose(output string) string {
	var staged, unstaged, untracked []string
	section := ""

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(line, "Changes to be committed"):
			section = "staged"
		case strings.HasPrefix(line, "Changes not staged"):
			section = "unstaged"
		case strings.HasPrefix(line, "Untracked files"):
			section = "untracked"
		case strings.HasPrefix(line, "(") || line == "" ||
			strings.HasPrefix(line, "On branch") ||
			strings.HasPrefix(line, "nothing to") ||
			strings.HasPrefix(line, "no changes"):
			continue
		default:
			// Extract file path from verbose format lines like:
			// "new file:   path/to/file.go"
			// "modified:   path/to/file.go"
			// "deleted:    path/to/file.go"
			file := extractFileFromVerboseLine(line)
			if file == "" {
				continue
			}
			switch section {
			case "staged":
				staged = append(staged, file)
			case "unstaged":
				unstaged = append(unstaged, file)
			case "untracked":
				untracked = append(untracked, file)
			}
		}
	}

	result := buildGitStatusSummary(staged, unstaged, untracked)
	if result == "" {
		return "Clean working tree."
	}
	return result
}

// extractFileFromVerboseLine extracts the file path from a verbose git status line.
// Handles: "new file:   path", "modified:   path", "deleted:    path", "renamed:    old -> new"
func extractFileFromVerboseLine(line string) string {
	prefixes := []string{"new file:", "modified:", "deleted:", "renamed:", "typechange:", "copied:"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			rest := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			// Handle rename: "old -> new"
			if strings.Contains(rest, "->") {
				parts := strings.SplitN(rest, "->", 2)
				return strings.TrimSpace(parts[1])
			}
			return rest
		}
	}
	// If no prefix matched, it might be a plain file path (untracked files)
	if line != "" && !strings.HasPrefix(line, "(") {
		return line
	}
	return ""
}

// buildGitStatusSummary creates a compact summary from categorized file lists.
func buildGitStatusSummary(staged, unstaged, untracked []string) string {
	var sb strings.Builder
	if len(staged) > 0 {
		sb.WriteString("Staged:\n")
		sb.WriteString(groupByDir(staged))
	}
	if len(unstaged) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Unstaged:\n")
		sb.WriteString(groupByDir(unstaged))
	}
	if len(untracked) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Untracked:\n")
		sb.WriteString(groupByDir(untracked))
	}
	return sb.String()
}

// compressGitLog converts verbose git log to one-line format.
func compressGitLog(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	// If already in oneline format (hash + message per line), just dedup
	lines := strings.Split(result, "\n")
	commitHashRe := regexp.MustCompile(`^[0-9a-f]{6,40}\s`)

	// Check if output looks like standard verbose git log
	isVerbose := false
	for _, line := range lines {
		if strings.HasPrefix(line, "commit ") || strings.HasPrefix(line, "Author: ") ||
			strings.HasPrefix(line, "Date:   ") {
			isVerbose = true
			break
		}
	}

	if !isVerbose {
		// Already compact, just apply generic compression
		return compressGeneric(result)
	}

	// Extract commit hash + subject from verbose format
	var sb strings.Builder
	var currentHash, currentSubject string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "commit ") {
			if currentHash != "" && currentSubject != "" {
				sb.WriteString(currentHash + " " + currentSubject + "\n")
			}
			currentHash = line[7:17] // short hash
			currentSubject = ""
		} else if strings.HasPrefix(line, "Merge:") {
			currentSubject = line
		} else if commitHashRe.MatchString(line) {
			// Already oneline format mixed in
			sb.WriteString(line + "\n")
			currentHash = ""
			currentSubject = ""
		} else if currentHash != "" && currentSubject == "" && line != "" &&
			!strings.HasPrefix(line, "Author:") && !strings.HasPrefix(line, "Date:") {
			currentSubject = line
		}
	}
	if currentHash != "" && currentSubject != "" {
		sb.WriteString(currentHash + " " + currentSubject + "\n")
	}

	compressed := sb.String()
	if compressed == "" {
		return compressGeneric(result)
	}
	return compressed
}

// compressGitDiff collapses large diffs to hunk headers + stats.
func compressGitDiff(output string) string {
	result := StripANSI(output)
	lines := strings.Split(result, "\n")

	if len(lines) < 50 {
		return compressGeneric(result)
	}

	var sb strings.Builder
	hunkCount := 0
	added, removed := 0, 0
	var files []string
	currentFile := ""

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			parts := strings.SplitN(line, " ", 4)
			if len(parts) >= 4 {
				currentFile = parts[3]
				if strings.HasPrefix(currentFile, "b/") {
					currentFile = currentFile[2:]
				}
				files = append(files, currentFile)
			}
		case strings.HasPrefix(line, "@@"):
			hunkCount++
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			added++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			removed++
		}
	}

	// Summary format
	fmt.Fprintf(&sb, "Diff summary: %d files changed, %d hunks\n", len(files), hunkCount)
	fmt.Fprintf(&sb, "+%d / -%d lines\n", added, removed)
	if len(files) > 0 {
		sb.WriteString("Files:\n")
		limit := 30
		for i, f := range files {
			if i >= limit {
				fmt.Fprintf(&sb, "  ... and %d more files\n", len(files)-limit)
				break
			}
			sb.WriteString("  " + f + "\n")
		}
	}

	// Include first 3 hunks of context for LLM understanding
	sb.WriteString("\nFirst hunks:\n")
	hunkShown := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			if hunkShown >= 3 {
				break
			}
			sb.WriteString(line + "\n")
			hunkShown++
			continue
		}
		if hunkShown > 0 && hunkShown <= 3 {
			if strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "@@") {
				if strings.HasPrefix(line, "@@") {
					hunkShown++
					if hunkShown > 3 {
						break
					}
				}
				continue
			}
			sb.WriteString(line + "\n")
		}
	}

	return sb.String()
}

// ─── Container Filters ──────────────────────────────────────────────────────

func compressContainer(sub, output string) (string, string) {
	switch sub {
	case "ps":
		return compressDockerPS(output), "docker-ps"
	case "logs":
		return compressDockerLogs(output), "docker-logs"
	case "images":
		return compressGeneric(output), "docker-images"
	default:
		return compressGeneric(output), "docker-generic"
	}
}

// compressDockerPS strips container hashes and unnecessary columns.
func compressDockerPS(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	lines := strings.Split(result, "\n")
	if len(lines) <= 1 {
		return result
	}

	// Try to parse as table - keep Name, Status, Ports, Image
	var sb strings.Builder
	for i, line := range lines {
		if line == "" {
			continue
		}
		// Keep header
		if i == 0 {
			sb.WriteString(line + "\n")
			continue
		}
		// Strip container ID hash (first column, 12+ hex chars)
		fields := strings.Fields(line)
		if len(fields) >= 2 && isContainerID(fields[0]) {
			// Rebuild without the ID column
			sb.WriteString(strings.Join(fields[1:], " ") + "\n")
		} else {
			sb.WriteString(line + "\n")
		}
	}

	return sb.String()
}

// compressDockerLogs applies log-specific compression.
func compressDockerLogs(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	result = stripTimestamps(result)
	result = DeduplicateLines(result)

	// If still very long, apply tail focus
	lines := strings.Split(result, "\n")
	if len(lines) > 100 {
		result = TailFocus(result, 10, 50, 5)
	}

	return result
}

// ─── Kubernetes Filters ─────────────────────────────────────────────────────

func compressK8s(sub, output string) (string, string) {
	switch sub {
	case "logs":
		return compressDockerLogs(output), "k8s-logs" // same log compression
	case "get":
		return compressGeneric(output), "k8s-get"
	default:
		return compressGeneric(output), "k8s-generic"
	}
}

// ─── Test Runner Filters ────────────────────────────────────────────────────

// compressGoTest extracts failures and summary from go test output.
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
func compressLsTree(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	lines := strings.Split(result, "\n")
	if len(lines) < 20 {
		return result
	}

	// Group by directory
	dirs := make(map[string][]string)
	var order []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "." || line == "./" {
			continue
		}
		parts := strings.SplitN(line, "/", 2)
		dir := parts[0]
		if len(parts) > 1 {
			if _, exists := dirs[dir]; !exists {
				order = append(order, dir)
			}
			dirs[dir] = append(dirs[dir], parts[1])
		} else {
			if _, exists := dirs["."]; !exists {
				order = append(order, ".")
			}
			dirs["."] = append(dirs["."], line)
		}
	}

	var sb strings.Builder
	for _, dir := range order {
		files := dirs[dir]
		if dir == "." {
			sb.WriteString(fmt.Sprintf("%d files in root\n", len(files)))
		} else {
			sb.WriteString(fmt.Sprintf("%s/ (%d entries)\n", dir, len(files)))
		}
		if len(files) <= 10 {
			for _, f := range files {
				sb.WriteString("  " + f + "\n")
			}
		}
	}

	compressed := sb.String()
	if compressed == "" {
		return result
	}
	return compressed
}

// compressFind groups find results by directory.
func compressFind(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	lines := strings.Split(result, "\n")
	if len(lines) < 20 {
		return result
	}

	// Group by directory
	dirs := make(map[string]int)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lastSlash := strings.LastIndex(line, "/")
		var dir string
		if lastSlash > 0 {
			dir = line[:lastSlash]
		} else {
			dir = "."
		}
		dirs[dir]++
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d results in %d directories:\n", len(lines), len(dirs))
	for dir, count := range dirs {
		fmt.Fprintf(&sb, "  %s/ (%d files)\n", dir, count)
	}

	return sb.String()
}

// compressGrep groups grep results by file with match counts.
func compressGrep(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	lines := strings.Split(result, "\n")
	if len(lines) < 15 {
		return result
	}

	// Group by file
	fileMatches := make(map[string][]string)
	fileOrder := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Split on first colon (grep format: file:line:content)
		parts := strings.SplitN(line, ":", 3)
		if len(parts) >= 2 {
			file := parts[0]
			if _, exists := fileMatches[file]; !exists {
				fileOrder = append(fileOrder, file)
			}
			fileMatches[file] = append(fileMatches[file], line)
		} else {
			// Not standard grep format, keep as-is
			if _, exists := fileMatches[""]; !exists {
				fileOrder = append(fileOrder, "")
			}
			fileMatches[""] = append(fileMatches[""], line)
		}
	}

	var sb strings.Builder
	for _, file := range fileOrder {
		matches := fileMatches[file]
		if file == "" {
			for _, m := range matches {
				sb.WriteString(m + "\n")
			}
			continue
		}
		if len(matches) <= 5 {
			for _, m := range matches {
				sb.WriteString(m + "\n")
			}
		} else {
			fmt.Fprintf(&sb, "%s: %d matches (showing first 3)\n", file, len(matches))
			for i := 0; i < 3 && i < len(matches); i++ {
				sb.WriteString("  " + matches[i] + "\n")
			}
		}
	}

	return sb.String()
}

// compressLogs applies log-specific compression: strip timestamps, dedup, tail.
func compressLogs(output string) string {
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

// ─── Python Output Filter ───────────────────────────────────────────────────

// compressPythonOutput filters Python-specific noise.
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
func compressAPIOutput(output string) (string, string) {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	// Try JSON compaction
	if strings.HasPrefix(strings.TrimSpace(result), "{") ||
		strings.HasPrefix(strings.TrimSpace(result), "[") {
		result = compactJSON(result)
	}

	result = DeduplicateLines(result)

	lines := strings.Split(result, "\n")
	if len(lines) > 100 {
		result = TailFocus(result, 20, 50, 5)
	}

	return result, "api"
}

// ─── Helper Functions ───────────────────────────────────────────────────────

// isContainerID checks if a string looks like a Docker container ID.
func isContainerID(s string) bool {
	if len(s) < 12 {
		return false
	}
	for _, c := range s[:12] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// stripTimestamps removes common log timestamp prefixes.
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
