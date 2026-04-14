package outputcompress

import (
	"fmt"
	"regexp"
	"strconv"
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
	// ─── Composite commands (must come before simple bin matches) ─────
	case bin == "docker" && sub == "compose":
		// commandSignature() truncates to 2 tokens, so extract 3rd from raw command
		composeSub := ""
		rawParts := strings.Fields(command)
		for i, p := range rawParts {
			if p == "compose" && i+1 < len(rawParts) {
				composeSub = rawParts[i+1]
				break
			}
		}
		return compressDockerCompose(composeSub, output)
	case bin == "docker-compose" || bin == "docker_compose":
		return compressDockerCompose(sub, output)

	// ─── V1–V5 routes (unchanged) ────────────────────────────────────
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
		return compressJsTest(output), "npm-test"
	case bin == "npx" && (sub == "vitest" || sub == "jest"):
		return compressJsTest(output), "js-test"
	case bin == "yarn" && (sub == "test" || sub == "jest"):
		return compressJsTest(output), "yarn-test"
	case bin == "pnpm" && (sub == "test" || sub == "run"):
		return compressJsTest(output), "pnpm-test"
	case bin == "eslint" || bin == "tsc" || bin == "ruff" || bin == "golangci-lint" || bin == "flake8" || bin == "pylint":
		return compressLint(output), "lint"
	case bin == "ls" || bin == "dir" || bin == "tree":
		return compressLsTree(output), "ls-tree"
	case bin == "find":
		return compressFind(output), "find"
	case bin == "grep" || bin == "rg" || bin == "ag" || bin == "ack":
		return compressGrep(output), "grep"
	case bin == "curl" || bin == "wget":
		return compressGeneric(output), "curl"
	case bin == "systemctl":
		return compressSystemctl(sub, output)
	case bin == "journalctl" || bin == "logcli" || strings.HasSuffix(bin, "log"):
		return compressLogs(output), "logs"
	case bin == "aws":
		return compressAws(sub, output)
	case bin == "ansible" || bin == "ansible-playbook":
		return compressAnsible(output), "ansible"

	// ─── V6: Home-Lab / Infra routes ─────────────────────────────────
	case bin == "helm":
		return compressHelm(sub, output)
	case bin == "terraform" || bin == "tf":
		return compressTerraform(sub, output)
	case bin == "df":
		return compressDiskFree(output), "df"
	case bin == "du":
		return compressDiskUsage(output), "du"
	case bin == "ps":
		return compressProcessList(output), "ps"
	case bin == "ss" || bin == "netstat":
		return compressNetworkConnections(output), "netstat"
	case bin == "ip" && (sub == "addr" || sub == "a" || sub == "address"):
		return compressIpAddr(output), "ip-addr"
	case bin == "ip" && (sub == "route" || sub == "r"):
		return compressIpRoute(output), "ip-route"
	case bin == "ip":
		return compressGeneric(output), "ip-generic"
	case bin == "free":
		return compressGeneric(output), "free"
	case bin == "uptime":
		return compressGeneric(output), "uptime"

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
		return compressK8sLogs(output), "k8s-logs"
	case "get":
		return compressK8sGet(output), "k8s-get"
	case "describe":
		return compressK8sDescribe(output), "k8s-describe"
	case "top":
		return compressGeneric(output), "k8s-top"
	default:
		return compressGeneric(output), "k8s-generic"
	}
}

// compressK8sLogs applies log-specific compression with level grouping.
func compressK8sLogs(output string) string {
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

// compressK8sGet summarises kubectl get output into status groups.
func compressK8sGet(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 8 {
		return result
	}

	running, pending, failed, other := 0, 0, 0, 0
	for _, line := range lines[1:] { // skip header
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "running"):
			running++
		case strings.Contains(lower, "pending") || strings.Contains(lower, "containercreating"):
			pending++
		case strings.Contains(lower, "error") || strings.Contains(lower, "crashloop") || strings.Contains(lower, "failed"):
			failed++
		default:
			other++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Status Summary: %d Running, %d Pending, %d Failed, %d Other\n", running, pending, failed, other))

	// Include failed/pending lines for context
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "crashloop") ||
			strings.Contains(lower, "failed") || strings.Contains(lower, "pending") ||
			strings.Contains(lower, "containercreating") {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}

// compressK8sDescribe extracts key information from kubectl describe output.
func compressK8sDescribe(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")

	var sb strings.Builder
	inEvents := false
	inConditions := false
	eventCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Always include Name, Status, and key labels
		if strings.HasPrefix(trimmed, "Name:") ||
			strings.HasPrefix(trimmed, "Status:") ||
			strings.HasPrefix(trimmed, "Node:") ||
			strings.HasPrefix(trimmed, "Labels:") {
			sb.WriteString(line + "\n")
			continue
		}

		// Track Events section
		if strings.HasPrefix(trimmed, "Events:") {
			inEvents = true
			inConditions = false
			sb.WriteString(line + "\n")
			continue
		}
		if strings.HasPrefix(trimmed, "Conditions:") {
			inConditions = true
			inEvents = false
			sb.WriteString(line + "\n")
			continue
		}

		// In Conditions section, include all lines
		if inConditions && (strings.HasPrefix(trimmed, "Type") || strings.HasPrefix(trimmed, "Ready") ||
			strings.HasPrefix(trimmed, "  ")) {
			sb.WriteString(line + "\n")
			continue
		}

		// In Events section, include warnings and last few events
		if inEvents {
			if strings.Contains(strings.ToLower(trimmed), "warning") || strings.Contains(strings.ToLower(trimmed), "error") {
				sb.WriteString(line + "\n")
			}
			eventCount++
		}
	}

	if eventCount > 10 && sb.Len() > 0 {
		// Add summary if too many events
		sb.WriteString(fmt.Sprintf("... and %d more events\n", eventCount-10))
	}

	compressed := sb.String()
	if len(compressed) < 50 {
		return result // too little extracted, return original
	}
	return compressed
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

// ─── Systemctl Filter ───────────────────────────────────────────────────────

// compressSystemctl handles systemctl status and related subcommands.
func compressSystemctl(sub, output string) (string, string) {
	switch sub {
	case "status":
		return compressSystemctlStatus(output), "systemctl-status"
	case "list-units", "list-unit-files":
		return compressSystemctlList(output), "systemctl-list"
	case "journalctl":
		return compressLogs(output), "journalctl"
	default:
		return compressGeneric(output), "systemctl-generic"
	}
}

// compressSystemctlStatus extracts key fields from systemctl status output.
func compressSystemctlStatus(output string) string {
	result := StripANSI(output)
	lines := strings.Split(result, "\n")

	var sb strings.Builder
	inLogs := false
	logLines := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Key fields to always include
		if strings.HasPrefix(trimmed, "●") || strings.HasPrefix(trimmed, "Active:") ||
			strings.HasPrefix(trimmed, "Main PID:") || strings.HasPrefix(trimmed, "Tasks:") ||
			strings.HasPrefix(trimmed, "Memory:") || strings.HasPrefix(trimmed, "CPU:") ||
			strings.HasPrefix(trimmed, "Loaded:") {
			sb.WriteString(line + "\n")
			continue
		}

		// Detect log section (indented lines after Process/Status)
		if strings.HasPrefix(line, "    ") && trimmed != "" {
			inLogs = true
		} else if !strings.HasPrefix(line, " ") && trimmed != "" {
			inLogs = false
		}

		if inLogs {
			// Include error/warning lines from logs
			lower := strings.ToLower(trimmed)
			if strings.Contains(lower, "error") || strings.Contains(lower, "fail") ||
				strings.Contains(lower, "warn") || strings.Contains(lower, "fatal") {
				sb.WriteString(line + "\n")
			}
			logLines++
		}
	}

	if logLines > 20 {
		sb.WriteString(fmt.Sprintf("... and %d more log lines\n", logLines-20))
	}

	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// compressSystemctlList summarises systemctl list-units output.
func compressSystemctlList(output string) string {
	result := StripANSI(output)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	running, failed, exited, other := 0, 0, 0, 0
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "running"):
			running++
		case strings.Contains(lower, "failed"):
			failed++
		case strings.Contains(lower, "exited"):
			exited++
		default:
			other++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Summary: %d Running, %d Failed, %d Exited, %d Other\n", running, failed, exited, other))

	// Include failed units
	for _, line := range lines[1:] {
		if strings.Contains(strings.ToLower(line), "failed") {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}

// ─── JS/Test Filter ─────────────────────────────────────────────────────────

// compressJsTest extracts failures and summary from JS test runner output.
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
func compressAws(sub, output string) (string, string) {
	switch sub {
	case "ec2":
		return compressAwsTable(output), "aws-ec2"
	case "s3":
		return compressAwsTable(output), "aws-s3"
	case "lambda":
		return compressAwsTable(output), "aws-lambda"
	default:
		return compressAwsTable(output), "aws-generic"
	}
}

// compressAwsTable summarises AWS CLI table/JSON output.
func compressAwsTable(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	// Try JSON compaction first
	if strings.HasPrefix(strings.TrimSpace(result), "{") || strings.HasPrefix(strings.TrimSpace(result), "[") {
		compacted, _ := compressAPIOutput(result)
		return compacted
	}

	lines := strings.Split(result, "\n")
	if len(lines) <= 8 {
		return result
	}

	// For table output, keep header + count summary + error lines
	var sb strings.Builder
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n") // header
	}

	errorCount := 0
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") ||
			strings.Contains(lower, "terminated") || strings.Contains(lower, "stopped") {
			sb.WriteString(line + "\n")
			errorCount++
		}
	}

	sb.WriteString(fmt.Sprintf("... %d total rows, %d with issues\n", len(lines)-1, errorCount))
	return sb.String()
}

// ─── Ansible Filter ─────────────────────────────────────────────────────────

// compressAnsible extracts task results and failures from Ansible output.
func compressAnsible(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	var sb strings.Builder
	changed, ok, failed, unreachable, skipped := 0, 0, 0, 0, 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		// PLAY/PLAYBOOK headers
		if strings.HasPrefix(trimmed, "PLAY") || strings.HasPrefix(trimmed, "PLAYBOOK") ||
			strings.HasPrefix(trimmed, "TASK") {
			sb.WriteString(line + "\n")
			continue
		}

		// Failed/error/unreachable tasks
		if strings.Contains(lower, "fatal") || strings.Contains(lower, "failed") ||
			strings.Contains(lower, "unreachable") || strings.Contains(lower, "error") {
			sb.WriteString(line + "\n")
			continue
		}

		// Changed notifications
		if strings.Contains(lower, "changed") {
			changed++
		}
		if strings.Contains(lower, "ok") && !strings.Contains(lower, "ok=") {
			ok++
		}
		if strings.Contains(lower, "failed") && !strings.Contains(lower, "failed=") {
			failed++
		}
		if strings.Contains(lower, "unreachable") {
			unreachable++
		}
		if strings.Contains(lower, "skipped") {
			skipped++
		}

		// PLAY RECAP section
		if strings.Contains(trimmed, "ok=") || strings.Contains(trimmed, "PLAY RECAP") {
			sb.WriteString(line + "\n")
		}
	}

	sb.WriteString(fmt.Sprintf("\nSummary: %d ok, %d changed, %d failed, %d unreachable, %d skipped\n",
		ok, changed, failed, unreachable, skipped))

	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// ─── Docker Compose Filters ─────────────────────────────────────────────────

// compressDockerCompose routes docker compose subcommands.
func compressDockerCompose(sub string, output string) (string, string) {
	switch sub {
	case "ps":
		return compressComposePs(output), "compose-ps"
	case "logs":
		return compressDockerLogs(output), "compose-logs"
	case "config":
		return compressComposeConfig(output), "compose-config"
	case "events":
		return compressComposeEvents(output), "compose-events"
	case "images":
		return compressComposePs(output), "compose-images" // similar table format
	case "top":
		return compressGeneric(output), "compose-top"
	default:
		return compressGeneric(output), "compose-generic"
	}
}

// compressComposePs summarises docker compose ps output.
func compressComposePs(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	running, stopped, other := 0, 0, 0
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "running") || strings.Contains(lower, "up"):
			running++
		case strings.Contains(lower, "stopped") || strings.Contains(lower, "exited") ||
			strings.Contains(lower, "down") || strings.Contains(lower, "dead"):
			stopped++
		default:
			other++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Services: %d Running, %d Stopped, %d Other\n", running, stopped, other))

	// Include header + stopped/error services
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n")
	}
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "stopped") || strings.Contains(lower, "exited") ||
			strings.Contains(lower, "down") || strings.Contains(lower, "dead") ||
			strings.Contains(lower, "error") || strings.Contains(lower, "restart") {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}

// compressComposeConfig summarises docker compose config output.
func compressComposeConfig(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 20 {
		return result
	}

	var sb strings.Builder
	serviceCount := 0
	networkCount := 0
	volumeCount := 0
	inServices := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "services:") {
			inServices = true
			sb.WriteString(line + "\n")
			continue
		}
		if inServices {
			// Detect individual service names (2-space indent + name + colon)
			if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":") {
				serviceCount++
				sb.WriteString(line + "\n")
			}
			if strings.HasPrefix(trimmed, "networks:") || strings.HasPrefix(trimmed, "volumes:") {
				inServices = false
			}
		}
		if strings.HasPrefix(trimmed, "networks:") {
			networkCount++
			sb.WriteString(line + "\n")
		}
		if strings.HasPrefix(trimmed, "volumes:") {
			volumeCount++
			sb.WriteString(line + "\n")
		}
	}

	sb.WriteString(fmt.Sprintf("\nConfig: %d services, %d networks, %d volumes (full config omitted)\n",
		serviceCount, networkCount, volumeCount))
	return sb.String()
}

// compressComposeEvents extracts key events from docker compose events output.
func compressComposeEvents(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	result = stripTimestamps(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	var sb strings.Builder
	eventCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "die") || strings.Contains(lower, "error") ||
			strings.Contains(lower, "kill") || strings.Contains(lower, "stop") ||
			strings.Contains(lower, "restart") || strings.Contains(lower, "health") {
			sb.WriteString(line + "\n")
		}
		eventCount++
	}

	sb.WriteString(fmt.Sprintf("... %d total events\n", eventCount))
	return sb.String()
}

// ─── Helm Filters ────────────────────────────────────────────────────────────

// compressHelm routes helm subcommands.
func compressHelm(sub string, output string) (string, string) {
	switch sub {
	case "list", "ls":
		return compressHelmList(output), "helm-list"
	case "status":
		return compressHelmStatus(output), "helm-status"
	case "history":
		return compressHelmHistory(output), "helm-history"
	case "get":
		return compressGeneric(output), "helm-get"
	case "repo":
		return compressGeneric(output), "helm-repo"
	default:
		return compressGeneric(output), "helm-generic"
	}
}

// compressHelmList summarises helm list output.
func compressHelmList(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 8 {
		return result
	}

	deployed, failed, other := 0, 0, 0
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "deployed"):
			deployed++
		case strings.Contains(lower, "failed"):
			failed++
		default:
			other++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Releases: %d Deployed, %d Failed, %d Other\n", deployed, failed, other))
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n")
	}
	// Include failed releases
	for _, line := range lines[1:] {
		if strings.Contains(strings.ToLower(line), "failed") {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}

// compressHelmStatus extracts key info from helm status output.
func compressHelmStatus(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")

	var sb strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "STATUS:") || strings.HasPrefix(trimmed, "REVISION:") ||
			strings.HasPrefix(trimmed, "CHART:") || strings.HasPrefix(trimmed, "NAMESPACE:") ||
			strings.HasPrefix(trimmed, "LAST DEPLOYED:") || strings.HasPrefix(trimmed, "NOTES:") {
			sb.WriteString(line + "\n")
			continue
		}
		// Resources section
		if strings.HasPrefix(trimmed, "==>") || strings.HasPrefix(trimmed, "NAME:") ||
			strings.HasPrefix(trimmed, "READY") {
			sb.WriteString(line + "\n")
		}
	}
	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// compressHelmHistory summarises helm history output.
func compressHelmHistory(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	// Keep header + failed/superseded revisions
	var sb strings.Builder
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n")
	}
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "failed") || strings.Contains(lower, "superseded") ||
			strings.Contains(lower, "pending") || strings.Contains(lower, "rollback") {
			sb.WriteString(line + "\n")
		}
	}
	sb.WriteString(fmt.Sprintf("... %d total revisions\n", len(lines)-1))
	return sb.String()
}

// ─── Terraform Filters ───────────────────────────────────────────────────────

// compressTerraform routes terraform subcommands.
func compressTerraform(sub string, output string) (string, string) {
	switch sub {
	case "plan":
		return compressTerraformPlan(output), "tf-plan"
	case "apply":
		return compressTerraformApply(output), "tf-apply"
	case "show":
		return compressTerraformShow(output), "tf-show"
	case "state":
		return compressTerraformStateList(output), "tf-state"
	case "output":
		return compressTerraformOutput(output), "tf-output"
	case "init":
		return compressGeneric(output), "tf-init"
	default:
		return compressGeneric(output), "tf-generic"
	}
}

// compressTerraformPlan extracts change summary from terraform plan output.
func compressTerraformPlan(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	var sb strings.Builder
	inChanges := false
	for _, line := range strings.Split(result, "\n") {
		trimmed := strings.TrimSpace(line)

		// Plan summary line
		if strings.Contains(trimmed, "Plan:") || strings.Contains(trimmed, "No changes") {
			sb.WriteString(line + "\n")
		}
		// Change markers
		if strings.Contains(trimmed, "will be created") || strings.Contains(trimmed, "will be destroyed") ||
			strings.Contains(trimmed, "will be updated") || strings.Contains(trimmed, "will be replaced") {
			sb.WriteString(line + "\n")
			inChanges = true
		}
		// Resource addresses (indented under change markers)
		if inChanges && (strings.HasPrefix(trimmed, "+ ") || strings.HasPrefix(trimmed, "- ") ||
			strings.HasPrefix(trimmed, "~ ") || strings.HasPrefix(trimmed, "-/+ ")) {
			sb.WriteString(line + "\n")
		}
		// Errors and warnings
		if strings.Contains(strings.ToLower(trimmed), "error") ||
			strings.Contains(strings.ToLower(trimmed), "warning") {
			sb.WriteString(line + "\n")
		}
	}

	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// compressTerraformApply extracts result summary from terraform apply output.
func compressTerraformApply(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	var sb strings.Builder
	for _, line := range strings.Split(result, "\n") {
		trimmed := strings.TrimSpace(line)

		// Apply result
		if strings.Contains(trimmed, "Apply complete!") || strings.Contains(trimmed, "Resources:") {
			sb.WriteString(line + "\n")
		}
		// Errors
		if strings.Contains(strings.ToLower(trimmed), "error") {
			sb.WriteString(line + "\n")
		}
		// Outputs section: "Outputs:" header or indented lines with " = "
		if strings.HasPrefix(trimmed, "Outputs:") {
			sb.WriteString(line + "\n")
		} else if strings.Contains(trimmed, " = ") && !strings.Contains(trimmed, "Apply") {
			sb.WriteString(line + "\n")
		}
	}

	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// compressTerraformShow summarises terraform show output.
func compressTerraformShow(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 15 {
		return result
	}

	var sb strings.Builder
	resourceCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Resource headers: resource "type" "name"
		if strings.Contains(trimmed, "resource ") && strings.Contains(trimmed, "\"") {
			sb.WriteString(line + "\n")
			resourceCount++
		}
		// Data sources
		if strings.Contains(trimmed, "data ") && strings.Contains(trimmed, "\"") {
			sb.WriteString(line + "\n")
		}
		// Outputs
		if strings.HasPrefix(trimmed, "output ") {
			sb.WriteString(line + "\n")
		}
	}
	sb.WriteString(fmt.Sprintf("\n... %d resources total\n", resourceCount))
	return sb.String()
}

// compressTerraformStateList summarises terraform state list output.
func compressTerraformStateList(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 15 {
		return result
	}

	// Group by resource type
	types := make(map[string]int)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: type.name or module.type.name
		parts := strings.Split(line, ".")
		var resType string
		if len(parts) >= 2 && parts[0] == "module" && len(parts) >= 3 {
			resType = parts[1]
		} else if len(parts) >= 2 {
			resType = parts[0]
		} else {
			resType = line
		}
		types[resType]++
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d resources in %d types:\n", len(lines), len(types)))
	for typ, count := range types {
		sb.WriteString(fmt.Sprintf("  %s: %d\n", typ, count))
	}
	return sb.String()
}

// compressTerraformOutput summarises terraform output output.
func compressTerraformOutput(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	// Keep output names and values, truncate long values
	var sb strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Output format: name = "value" or name = value
		if strings.Contains(trimmed, " = ") {
			parts := strings.SplitN(trimmed, " = ", 2)
			if len(parts) == 2 && len(parts[1]) > 200 {
				sb.WriteString(parts[0] + " = " + parts[1][:200] + "... (truncated)\n")
			} else {
				sb.WriteString(line + "\n")
			}
		} else {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}

// ─── SSH Diagnostic Filters ──────────────────────────────────────────────────

// compressDiskFree summarises df output, highlighting high-usage filesystems.
func compressDiskFree(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	// Always apply high-usage filter; only skip for trivially small output
	if len(lines) <= 2 {
		return result
	}

	var sb strings.Builder
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n") // header
	}

	highThreshold := 80.0
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Parse use% field (typically last or second-to-last column)
		fields := strings.Fields(line)
		for _, f := range fields {
			f = strings.TrimSuffix(f, "%")
			pct, err := parseFloat(f)
			if err == nil && pct >= highThreshold {
				sb.WriteString(line + "\n")
				break
			}
		}
	}

	if sb.Len() <= len(lines[0])+1 {
		sb.WriteString("All filesystems below 80% usage\n")
	}
	return sb.String()
}

// compressDiskUsage summarises du output, showing largest directories.
func compressDiskUsage(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	// Keep top 15 largest entries
	var sb strings.Builder
	limit := 15
	if len(lines) < limit {
		limit = len(lines)
	}
	for i := 0; i < limit; i++ {
		sb.WriteString(lines[i] + "\n")
	}
	if len(lines) > limit {
		sb.WriteString(fmt.Sprintf("... and %d more entries\n", len(lines)-limit))
	}
	return sb.String()
}

// compressProcessList summarises ps output, showing top processes.
func compressProcessList(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 15 {
		return result
	}

	var sb strings.Builder
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n") // header
	}

	// Keep first 20 processes + any with high CPU/memory
	showCount := 0
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		highResource := false
		for _, f := range fields {
			f = strings.TrimSuffix(f, "%")
			val, err := parseFloat(f)
			if err == nil && val > 50 {
				highResource = true
				break
			}
		}
		if showCount < 20 || highResource {
			sb.WriteString(line + "\n")
		}
		showCount++
	}
	if showCount > 20 {
		sb.WriteString(fmt.Sprintf("... %d total processes\n", showCount))
	}
	return sb.String()
}

// compressNetworkConnections summarises ss/netstat output.
func compressNetworkConnections(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result
	}

	var sb strings.Builder
	if len(lines) > 0 {
		sb.WriteString(lines[0] + "\n") // header
	}

	// Count by state
	states := make(map[string]int)
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		var state string
		switch {
		case strings.Contains(lower, "listen"):
			state = "LISTEN"
		case strings.Contains(lower, "established"):
			state = "ESTABLISHED"
		case strings.Contains(lower, "time-wait") || strings.Contains(lower, "time_wait"):
			state = "TIME-WAIT"
		case strings.Contains(lower, "close-wait") || strings.Contains(lower, "close_wait"):
			state = "CLOSE-WAIT"
		default:
			state = "OTHER"
		}
		states[state]++

		// Always include LISTEN lines
		if state == "LISTEN" {
			sb.WriteString(line + "\n")
		}
	}

	sb.WriteString(fmt.Sprintf("\nSummary: %d LISTEN, %d ESTABLISHED, %d TIME-WAIT, %d CLOSE-WAIT, %d OTHER\n",
		states["LISTEN"], states["ESTABLISHED"], states["TIME-WAIT"], states["CLOSE-WAIT"], states["OTHER"]))
	return sb.String()
}

// compressIpAddr summarises ip addr output.
func compressIpAddr(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")

	var sb strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Interface headers (e.g., "2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP>")
		if strings.Contains(line, ": ") && !strings.HasPrefix(trimmed, "link/") &&
			!strings.HasPrefix(trimmed, "inet") {
			sb.WriteString(line + "\n")
			continue
		}
		// IP addresses
		if strings.HasPrefix(trimmed, "inet ") || strings.HasPrefix(trimmed, "inet6 ") {
			sb.WriteString(line + "\n")
		}
		// State info
		if strings.Contains(strings.ToLower(trimmed), "state ") {
			sb.WriteString(line + "\n")
		}
	}
	compressed := sb.String()
	if len(compressed) < 50 {
		return result
	}
	return compressed
}

// compressIpRoute summarises ip route output.
func compressIpRoute(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")
	if len(lines) <= 3 {
		return result
	}

	var sb strings.Builder
	defaultRoutes := 0
	otherRoutes := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "default ") {
			sb.WriteString(line + "\n")
			defaultRoutes++
		} else {
			otherRoutes++
		}
	}
	sb.WriteString(fmt.Sprintf("... and %d other routes\n", otherRoutes))
	return sb.String()
}

// parseFloat is a helper to strictly parse a float64 from a string.
// Uses strconv.ParseFloat which rejects trailing characters like "200G".
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

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
