package outputcompress

import "strings"

func compressTar(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")

	// Detect tar output type
	isListing := false
	hasProgress := false
	fileLines := []string{}
	summaryLines := []string{}
	errorLines := []string{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Error lines
		if strings.HasPrefix(trimmed, "tar:") && strings.Contains(trimmed, "Error") ||
			strings.Contains(trimmed, "Cannot open") ||
			strings.Contains(trimmed, "No such file") {
			errorLines = append(errorLines, trimmed)
			continue
		}

		// Summary lines (e.g., from tar -czf)
		if strings.HasPrefix(trimmed, "tar: ") && !strings.Contains(trimmed, "/") {
			summaryLines = append(summaryLines, trimmed)
			continue
		}

		// Progress/compression ratio lines
		if strings.Contains(trimmed, "%") && len(trimmed) < 30 {
			hasProgress = true
			continue
		}

		// File path lines (tar -tf, tar -xvf, tar -cvf)
		if !strings.HasPrefix(trimmed, "tar:") && !strings.HasPrefix(trimmed, "total ") {
			fileLines = append(fileLines, trimmed)
			isListing = true
		}
	}

	// Short output: return as-is
	if len(fileLines) <= 15 && len(errorLines) == 0 && len(summaryLines) == 0 {
		return result
	}

	var sb strings.Builder

	// Errors first
	for _, e := range errorLines {
		sb.WriteString(e + "\n")
	}

	// Summary lines
	for _, s := range summaryLines {
		sb.WriteString(s + "\n")
	}

	// File listing: group by directory
	if isListing && len(fileLines) > 15 {
		sb.WriteString(groupByDir(fileLines))
	} else if isListing {
		for _, f := range fileLines {
			sb.WriteString(f + "\n")
		}
	}

	if hasProgress {
		sb.WriteString("[compression progress omitted]\n")
	}

	return sb.String()
}

// compressZip handles zip/unzip outputs.
// Strategy: keep summary, group file paths, remove per-file compression ratios.
func compressZip(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")

	var sb strings.Builder
	fileLines := []string{}
	summaryLines := []string{}
	errorLines := []string{}
	headerWritten := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Error lines
		if strings.Contains(trimmed, "error:") || strings.Contains(trimmed, "Error:") ||
			strings.Contains(trimmed, "cannot find") || strings.Contains(trimmed, "bad zipfile") {
			errorLines = append(errorLines, trimmed)
			continue
		}

		// zip/unzip summary lines
		if strings.HasPrefix(trimmed, "adding:") || strings.HasPrefix(trimmed, "extracting:") ||
			strings.HasPrefix(trimmed, "inflating:") || strings.HasPrefix(trimmed, "deflated") ||
			strings.HasPrefix(trimmed, "stored") {
			// Extract just the filename from "adding: path/to/file (deflated 45%)"
			file := extractZipFile(trimmed)
			if file != "" {
				fileLines = append(fileLines, file)
			}
			continue
		}

		// unzip -l header lines
		if strings.HasPrefix(trimmed, "Archive:") || strings.HasPrefix(trimmed, "Length") ||
			strings.HasPrefix(trimmed, "---") {
			if !headerWritten {
				sb.WriteString(trimmed + "\n")
				headerWritten = true
			}
			continue
		}

		// unzip -l file entries: "  12345  2024-01-15 10:30   path/to/file"
		if isUnzipLEntry(trimmed) {
			parts := strings.Fields(trimmed)
			if len(parts) >= 4 {
				fileLines = append(fileLines, parts[len(parts)-1])
			}
			continue
		}

		// Summary lines (bytes, entries count)
		if strings.Contains(trimmed, "bytes") || strings.Contains(trimmed, "entries") ||
			strings.Contains(trimmed, "files") || strings.Contains(trimmed, "total") {
			summaryLines = append(summaryLines, trimmed)
			continue
		}

		// Other lines (keep)
		sb.WriteString(trimmed + "\n")
	}

	// Errors first
	for _, e := range errorLines {
		sb.WriteString(e + "\n")
	}

	// File listing: group by directory if large
	if len(fileLines) > 15 {
		sb.WriteString(groupByDir(fileLines))
	} else {
		for _, f := range fileLines {
			sb.WriteString(f + "\n")
		}
	}

	// Summary
	for _, s := range summaryLines {
		sb.WriteString(s + "\n")
	}

	output2 := sb.String()
	if output2 == "" {
		return result
	}
	return output2
}

// extractZipFile extracts the file path from a zip/unzip progress line.
// "  adding: dir/file.txt (deflated 45%)" → "dir/file.txt"
// "  extracting: dir/file.txt" → "dir/file.txt"
func extractZipFile(line string) string {
	// Remove common prefixes
	for _, prefix := range []string{"adding:", "extracting:", "inflating:", "deflating:", "copying:"} {
		if strings.Contains(line, prefix) {
			after := strings.SplitN(line, prefix, 2)[1]
			after = strings.TrimSpace(after)
			// Remove compression ratio suffix: "(deflated 45%)" or "(stored 0%)"
			if idx := strings.Index(after, " ("); idx > 0 {
				after = after[:idx]
			}
			return after
		}
	}
	return ""
}

// isUnzipLEntry detects lines from "unzip -l" output.
// Format: "  12345  2024-01-15 10:30   path/to/file"
func isUnzipLEntry(line string) bool {
	if len(line) < 20 {
		return false
	}
	// Must start with spaces and a digit (file size)
	trimmed := strings.TrimLeft(line, " ")
	if len(trimmed) == 0 || trimmed[0] < '0' || trimmed[0] > '9' {
		return false
	}
	// Must contain a date-like pattern
	return strings.Contains(line, ":") && strings.Contains(line, "/")
}

// compressRsync handles rsync output.
// Strategy: keep summary, remove per-file progress, deduplicate.
func compressRsync(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)
	lines := strings.Split(result, "\n")

	var sb strings.Builder
	fileLines := []string{}
	summaryLines := []string{}
	errorLines := []string{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Error lines
		if strings.HasPrefix(trimmed, "rsync:") || strings.HasPrefix(trimmed, "rsync error:") {
			errorLines = append(errorLines, trimmed)
			continue
		}

		// Progress lines: "<f++++++++x filename" or ">f.st...... filename"
		if (strings.HasPrefix(trimmed, "<") || strings.HasPrefix(trimmed, ">")) &&
			len(trimmed) > 11 && (trimmed[1] == 'f' || trimmed[1] == 'd') {
			// Extract filename after the progress indicator
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				fileLines = append(fileLines, parts[len(parts)-1])
			}
			continue
		}

		// "sending incremental file list" etc.
		if strings.HasPrefix(trimmed, "sending ") || strings.HasPrefix(trimmed, "sent ") ||
			strings.HasPrefix(trimmed, "total size ") || strings.HasPrefix(trimmed, "speedup ") {
			summaryLines = append(summaryLines, trimmed)
			continue
		}

		// Bytes transferred summary
		if strings.Contains(trimmed, "bytes received") || strings.Contains(trimmed, "bytes/sec") {
			summaryLines = append(summaryLines, trimmed)
			continue
		}

		// Per-file transfer progress lines like "     12,345 100%   12.34kB/s    0:00:01 (xfr#1, to-chk=5/10)"
		if strings.Contains(trimmed, "xfr#") || strings.Contains(trimmed, "to-chk=") {
			continue // skip per-file progress
		}

		// Regular file path lines (no progress prefix)
		if !strings.HasPrefix(trimmed, "rsync") && !strings.Contains(trimmed, "%") {
			fileLines = append(fileLines, trimmed)
		}
	}

	// Errors first
	for _, e := range errorLines {
		sb.WriteString(e + "\n")
	}

	// File listing: group by directory if large
	if len(fileLines) > 20 {
		sb.WriteString(groupByDir(fileLines))
	} else {
		for _, f := range fileLines {
			sb.WriteString(f + "\n")
		}
	}

	// Summary
	for _, s := range summaryLines {
		sb.WriteString(s + "\n")
	}

	compressed := sb.String()
	if compressed == "" {
		return result
	}
	return compressed
}
