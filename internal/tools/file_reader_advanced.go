package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// fileReaderAdvancedMaxChars limits the character length of returned content
	// to prevent excessive token usage in LLM context.
	fileReaderAdvancedMaxChars = 24000

	// searchContextMaxMatches caps the number of match results returned by searchContext
	// to bound processing time and output size.
	searchContextMaxMatches = 50

	// searchContextMaxFileSize rejects files larger than this before opening them,
	// preventing OOM and unbounded scan time in searchContext.
	// 50 MB provides a generous boundary while protecting against pathological cases.
	searchContextMaxFileSize = 50 * 1024 * 1024

	// searchContextMaxContextLines caps the context window around each match.
	searchContextMaxContextLines = 100
)

// FileReaderResult is the JSON response returned for file_reader_advanced operations.
type FileReaderResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ExecuteFileReaderAdvanced handles advanced file reading operations.
func ExecuteFileReaderAdvanced(operation, filePath, pattern string, startLine, endLine, lineCount int, workspaceDir string) string {
	encode := func(r FileReaderResult) string {
		b, err := json.Marshal(r)
		if err != nil {
			return `{"status":"error","message":"internal: result serialization failed"}`
		}
		return string(b)
	}

	if filePath == "" {
		return encode(FileReaderResult{Status: "error", Message: "'file_path' is required"})
	}

	resolved, err := secureResolve(workspaceDir, filePath)
	if err != nil {
		return encode(FileReaderResult{Status: "error", Message: err.Error()})
	}

	switch operation {
	case "read_lines":
		return readLines(resolved, startLine, endLine, encode)
	case "head":
		if lineCount <= 0 {
			lineCount = 20
		}
		return readLines(resolved, 1, lineCount, encode)
	case "tail":
		if lineCount <= 0 {
			lineCount = 20
		}
		return readTail(resolved, lineCount, encode)
	case "count_lines":
		return countLines(resolved, encode)
	case "search_context":
		return searchContext(resolved, pattern, lineCount, encode)
	default:
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Unknown file_reader_advanced operation '%s'. Valid: read_lines, head, tail, count_lines, search_context", operation)})
	}
}

// readLines reads a range of lines from a file.
func readLines(resolved string, start, end int, encode func(FileReaderResult) string) string {
	if start < 1 {
		start = 1
	}
	if end < start {
		return encode(FileReaderResult{Status: "error", Message: "end_line must be >= start_line"})
	}

	f, err := os.Open(resolved)
	if err != nil {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Failed to open file: %v", err)})
	}
	defer f.Close()

	var lines []string
	scanner := newLargeFileScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum >= start && lineNum <= end {
			lines = append(lines, scanner.Text())
		}
		if lineNum > end {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	if len(lines) == 0 {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("No lines in range %d-%d (file has %d lines)", start, end, lineNum)})
	}

	content, truncated := clampFileReaderContent(strings.Join(lines, "\n"))

	return encode(FileReaderResult{Status: "success", Data: map[string]interface{}{
		"start_line": start,
		"end_line":   start + len(lines) - 1,
		"total_read": len(lines),
		"content":    content,
		"truncated":  truncated,
	}})
}

// readTail reads the last N lines of a file.
func readTail(resolved string, n int, encode func(FileReaderResult) string) string {
	f, err := os.Open(resolved)
	if err != nil {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Failed to open file: %v", err)})
	}
	defer f.Close()

	if n <= 0 {
		n = 1
	}
	// Ring buffer: avoids O(n) copy per line when buffer is full
	buf := make([]string, n)
	scanner := newLargeFileScanner(f)
	total := 0
	pos := 0
	for scanner.Scan() {
		buf[pos%n] = scanner.Text()
		pos++
		total++
	}
	if err := scanner.Err(); err != nil {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	filled := pos
	if filled > n {
		filled = n
	}
	lines := make([]string, filled)
	start := pos - filled
	for i := 0; i < filled; i++ {
		lines[i] = buf[(start+i)%n]
	}

	startLine := total - filled + 1
	if total == 0 {
		startLine = 0
	}
	content, truncated := clampFileReaderContent(strings.Join(lines, "\n"))
	return encode(FileReaderResult{Status: "success", Data: map[string]interface{}{
		"start_line":  startLine,
		"end_line":    total,
		"total_lines": total,
		"total_read":  filled,
		"content":     content,
		"truncated":   truncated,
	}})
}

// countLines counts the total number of lines in a file.
func countLines(resolved string, encode func(FileReaderResult) string) string {
	f, err := os.Open(resolved)
	if err != nil {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Failed to open file: %v", err)})
	}
	defer f.Close()

	count := 0
	scanner := newLargeFileScanner(f)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	info, _ := os.Stat(resolved)
	var size int64
	if info != nil {
		size = info.Size()
	}

	return encode(FileReaderResult{Status: "success", Data: map[string]interface{}{
		"lines": count,
		"bytes": size,
	}})
}

// searchContext finds matches in a file and returns surrounding context lines.
func searchContext(resolved, pattern string, contextLines int, encode func(FileReaderResult) string) string {
	if pattern == "" {
		return encode(FileReaderResult{Status: "error", Message: "'pattern' is required for search_context"})
	}
	if contextLines <= 0 {
		contextLines = 3
	}
	if contextLines > searchContextMaxContextLines {
		contextLines = searchContextMaxContextLines
	}

	if len(pattern) > 256 {
		return encode(FileReaderResult{Status: "error", Message: "regex pattern too long (max 256 characters)"})
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Invalid regex: %v", err)})
	}

	// Check file size before opening to prevent OOM on very large files
	info, err := os.Stat(resolved)
	if err != nil {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Failed to stat file: %v", err)})
	}
	if info.Size() > searchContextMaxFileSize {
		return encode(FileReaderResult{
			Status:  "error",
			Message: fmt.Sprintf("file too large for search_context (%d MB > %d MB limit). Use smart_file_read sample/head/tail to inspect large files, or narrow the file set.", info.Size()/(1024*1024), searchContextMaxFileSize/(1024*1024)),
			Data: map[string]interface{}{
				"size_bytes":    info.Size(),
				"max_file_size": searchContextMaxFileSize,
				"limit":         "file_size",
			},
		})
	}

	f, err := os.Open(resolved)
	if err != nil {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Failed to open file: %v", err)})
	}
	defer f.Close()

	type contextResult struct {
		MatchLine int    `json:"match_line"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
		Content   string `json:"content"`
	}
	type bufferedLine struct {
		lineNum int
		text    string
	}
	type activeContext struct {
		matchLine      int
		startLine      int
		lines          []string
		remainingAfter int
	}

	var results []contextResult
	var active []activeContext
	before := make([]bufferedLine, 0, contextLines)
	lineNum := 0
	scanner := newLargeFileScanner(f)
	flushActive := func() {
		for _, pending := range active {
			results = append(results, contextResult{
				MatchLine: pending.matchLine,
				StartLine: pending.startLine,
				EndLine:   pending.startLine + len(pending.lines) - 1,
				Content:   mustClampFileReaderContent(strings.Join(pending.lines, "\n")),
			})
		}
		active = nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		for index := range active {
			active[index].lines = append(active[index].lines, line)
			active[index].remainingAfter--
		}

		remaining := active[:0]
		for _, pending := range active {
			if pending.remainingAfter <= 0 {
				results = append(results, contextResult{
					MatchLine: pending.matchLine,
					StartLine: pending.startLine,
					EndLine:   pending.startLine + len(pending.lines) - 1,
					Content:   mustClampFileReaderContent(strings.Join(pending.lines, "\n")),
				})
				if len(results) >= searchContextMaxMatches {
					return encode(FileReaderResult{Status: "success", Data: map[string]interface{}{
						"matches":     results,
						"count":       len(results),
						"truncated":   true,
						"limit":       "max_matches",
						"limit_value": searchContextMaxMatches,
					}})
				}
				continue
			}
			remaining = append(remaining, pending)
		}
		active = remaining

		if re.MatchString(line) {
			lines := make([]string, 0, len(before)+1+contextLines)
			startLine := lineNum
			if len(before) > 0 {
				startLine = before[0].lineNum
				for _, prev := range before {
					lines = append(lines, prev.text)
				}
			}
			lines = append(lines, line)
			pending := activeContext{
				matchLine:      lineNum,
				startLine:      startLine,
				lines:          lines,
				remainingAfter: contextLines,
			}
			if contextLines == 0 {
				results = append(results, contextResult{
					MatchLine: pending.matchLine,
					StartLine: pending.startLine,
					EndLine:   pending.startLine + len(pending.lines) - 1,
					Content:   mustClampFileReaderContent(strings.Join(pending.lines, "\n")),
				})
				if len(results) >= searchContextMaxMatches {
					return encode(FileReaderResult{Status: "success", Data: map[string]interface{}{
						"matches":     results,
						"count":       len(results),
						"truncated":   true,
						"limit":       "max_matches",
						"limit_value": searchContextMaxMatches,
					}})
				}
			} else {
				active = append(active, pending)
			}
		}

		if contextLines > 0 {
			if len(before) == contextLines {
				copy(before, before[1:])
				before[len(before)-1] = bufferedLine{lineNum: lineNum, text: line}
			} else {
				before = append(before, bufferedLine{lineNum: lineNum, text: line})
			}
		}
	}
	flushActive()
	if err := scanner.Err(); err != nil {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	return encode(FileReaderResult{Status: "success", Data: map[string]interface{}{
		"matches":   results,
		"count":     len(results),
		"truncated": false,
		"limit":     "none",
	}})
}

func clampFileReaderContent(content string) (string, bool) {
	if len(content) <= fileReaderAdvancedMaxChars {
		return content, false
	}
	notice := "\n\n[...truncated for prompt safety — narrow the range or use smart_file_read summarize/sample...]"
	return truncateUTF8ToLimit(content, fileReaderAdvancedMaxChars+len(notice), notice), true
}

func mustClampFileReaderContent(content string) string {
	clamped, _ := clampFileReaderContent(content)
	return clamped
}

func truncateUTF8Prefix(content string, maxBytes int) string {
	if maxBytes <= 0 || content == "" {
		return ""
	}
	if len(content) <= maxBytes {
		return content
	}

	cut := maxBytes
	for cut > 0 && cut < len(content) && !utf8.RuneStart(content[cut]) {
		cut--
	}
	for cut > 0 && !utf8.ValidString(content[:cut]) {
		cut--
		for cut > 0 && cut < len(content) && !utf8.RuneStart(content[cut]) {
			cut--
		}
	}
	return content[:cut]
}

func truncateUTF8ToLimit(content string, limit int, suffix string) string {
	if limit <= 0 {
		return ""
	}
	if len(content) <= limit {
		return content
	}
	if suffix == "" {
		return truncateUTF8Prefix(content, limit)
	}
	if len(suffix) >= limit {
		return truncateUTF8Prefix(suffix, limit)
	}
	return truncateUTF8Prefix(content, limit-len(suffix)) + suffix
}
