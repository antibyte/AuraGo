package outputcompress

import (
	"fmt"
	"strings"
)

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
	return compressLogOutput(output)
}

// ─── Python Output Filter ───────────────────────────────────────────────────

// ─── Systemctl Filter ───────────────────────────────────────────────────────

// compressSystemctl handles systemctl status and related subcommands.
func isLogContent(output string) bool {
	lines := strings.Split(output, "\n")
	sample := 20
	if len(lines) < sample {
		sample = len(lines)
	}
	logMarkers := 0
	for i := 0; i < sample; i++ {
		line := lines[i]
		// Check for common log patterns (structured and unstructured)
		if strings.Contains(line, "ERROR") || strings.Contains(line, "WARN") ||
			strings.Contains(line, "INFO") || strings.Contains(line, "DEBUG") ||
			strings.Contains(line, " level=") || strings.Contains(line, " lvl=") ||
			strings.Contains(line, "msg=") || strings.Contains(line, "message=") ||
			strings.Contains(line, `"level"`) || strings.Contains(line, `"msg"`) ||
			strings.Contains(line, `"message"`) {
			logMarkers++
			continue
		}
		// Check for timestamp patterns at line start
		trimmed := strings.TrimSpace(line)
		if (len(trimmed) > 19 && (trimmed[4] == '-' && trimmed[7] == '-' &&
			(trimmed[10] == 'T' || trimmed[10] == ' '))) ||
			strings.HasPrefix(trimmed, "[20") {
			logMarkers++
		}
	}
	// If more than half the sampled lines look like logs, treat as log content
	return logMarkers > sample/2
}

// compressCatFile compresses cat/less/more output.
// For log-like content, applies log compression. For large output, uses tail focus.
func compressCatFile(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	if isLogContent(result) {
		return compressLogs(result)
	}

	lines := strings.Split(result, "\n")
	if len(lines) > 200 {
		return TailFocus(result, 20, 100, 5)
	}

	return compressGeneric(result)
}

// compressTailHead compresses tail/head output.
// These commands already limit output by -n flag, so we only apply
// log-specific compression for log-like content.
func compressTailHead(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	if isLogContent(result) {
		return compressLogs(result)
	}

	return compressGeneric(result)
}

// compressStat compresses stat output to a compact one-line-per-file summary.
// Input format (GNU stat):
//
//	File: somefile.txt
//	Size: 1234       Blocks: 8        IO Block: 4096   regular file
//	Access: (0644/-rw-r--r--)  Uid: (1000/user)   Gid: (1000/user)
//	Access: 2024-01-15 10:30:00.000000000 +0100
//	Modify: 2024-01-14 15:20:00.000000000 +0100
//	Change: 2024-01-14 15:20:00.000000000 +0100
//	 Birth: 2024-01-10 08:00:00.000000000 +0100
func compressStat(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	lines := strings.Split(result, "\n")
	if len(lines) <= 3 {
		return result // Short output, keep as-is
	}

	var sb strings.Builder
	var currentFile string
	var size, fileType, perms, modify string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// File name line
		if strings.HasPrefix(trimmed, "File:") {
			// Flush previous file
			if currentFile != "" {
				writeStatLine(&sb, currentFile, size, fileType, perms, modify)
			}
			currentFile = strings.TrimSpace(strings.TrimPrefix(trimmed, "File:"))
			size = ""
			fileType = ""
			perms = ""
			modify = ""
			continue
		}

		// Size and type line: "Size: 1234       Blocks: 8        IO Block: 4096   regular file"
		if strings.HasPrefix(trimmed, "Size:") {
			parts := strings.Fields(trimmed)
			for i, p := range parts {
				if p == "Size:" && i+1 < len(parts) {
					size = parts[i+1]
				}
			}
			// File type is the last field(s) after IO Block value
			// Find "IO Block:" and take what comes after its value
			for i, p := range parts {
				if p == "Block:" && i+2 < len(parts) {
					// Everything after the IO Block value is the file type
					fileType = strings.Join(parts[i+2:], " ")
					// Remove trailing comma or space
					fileType = strings.TrimRight(fileType, ", ")
				}
			}
			continue
		}

		// Permissions line: "Access: (0644/-rw-r--r--)  Uid: ..."
		if strings.HasPrefix(trimmed, "Access: (") {
			// Extract permission string from parentheses
			start := strings.Index(trimmed, "(")
			end := strings.Index(trimmed, ")")
			if start >= 0 && end > start {
				permStr := trimmed[start+1 : end]
				// Extract symbolic part after slash: "0644/-rw-r--r--" -> "-rw-r--r--"
				if idx := strings.Index(permStr, "/"); idx >= 0 {
					perms = permStr[idx+1:]
				} else {
					perms = permStr
				}
			}
			continue
		}

		// Modify time line: "Modify: 2024-01-14 15:20:00.000000000 +0100"
		if strings.HasPrefix(trimmed, "Modify:") {
			modify = strings.TrimSpace(strings.TrimPrefix(trimmed, "Modify:"))
			// Truncate nanoseconds: "2024-01-14 15:20:00.000000000 +0100" -> "2024-01-14 15:20:00"
			if dotIdx := strings.Index(modify, "."); dotIdx > 0 {
				modify = modify[:dotIdx]
			}
			continue
		}
	}

	// Flush last file
	if currentFile != "" {
		writeStatLine(&sb, currentFile, size, fileType, perms, modify)
	}

	compressed := sb.String()
	if compressed == "" {
		return result
	}
	return compressed
}

// writeStatLine writes a compact one-line stat summary.
func writeStatLine(sb *strings.Builder, file, size, fileType, perms, modify string) {
	parts := []string{file + ":"}
	if size != "" {
		parts = append(parts, size+"B")
	}
	if fileType != "" {
		parts = append(parts, fileType)
	}
	if perms != "" {
		parts = append(parts, perms)
	}
	if modify != "" {
		parts = append(parts, "modified "+modify)
	}
	sb.WriteString(strings.Join(parts, " ") + "\n")
}
