package outputcompress

import (
	"fmt"
	"regexp"
	"strings"
)

// commitHashRe matches short/full git commit hashes at the start of a line.
var commitHashRe = regexp.MustCompile(`^[0-9a-f]{6,40}\s`)

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
