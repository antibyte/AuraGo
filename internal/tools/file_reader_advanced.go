package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
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
		b, _ := json.Marshal(r)
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
	scanner := bufio.NewScanner(f)
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

	if len(lines) == 0 {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("No lines in range %d-%d (file has %d lines)", start, end, lineNum)})
	}

	return encode(FileReaderResult{Status: "success", Data: map[string]interface{}{
		"start_line": start,
		"end_line":   start + len(lines) - 1,
		"total_read": len(lines),
		"content":    strings.Join(lines, "\n"),
	}})
}

// readTail reads the last N lines of a file.
func readTail(resolved string, n int, encode func(FileReaderResult) string) string {
	f, err := os.Open(resolved)
	if err != nil {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Failed to open file: %v", err)})
	}
	defer f.Close()

	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	total := len(allLines)
	start := total - n
	if start < 0 {
		start = 0
	}

	lines := allLines[start:]
	return encode(FileReaderResult{Status: "success", Data: map[string]interface{}{
		"start_line":  start + 1,
		"end_line":    total,
		"total_lines": total,
		"total_read":  len(lines),
		"content":     strings.Join(lines, "\n"),
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
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
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

	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Invalid regex: %v", err)})
	}

	f, err := os.Open(resolved)
	if err != nil {
		return encode(FileReaderResult{Status: "error", Message: fmt.Sprintf("Failed to open file: %v", err)})
	}
	defer f.Close()

	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	type contextResult struct {
		MatchLine int    `json:"match_line"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
		Content   string `json:"content"`
	}

	var results []contextResult
	maxResults := 50

	for i, line := range allLines {
		if re.MatchString(line) {
			start := i - contextLines
			if start < 0 {
				start = 0
			}
			end := i + contextLines
			if end >= len(allLines) {
				end = len(allLines) - 1
			}

			results = append(results, contextResult{
				MatchLine: i + 1,
				StartLine: start + 1,
				EndLine:   end + 1,
				Content:   strings.Join(allLines[start:end+1], "\n"),
			})

			if len(results) >= maxResults {
				break
			}
		}
	}

	return encode(FileReaderResult{Status: "success", Data: map[string]interface{}{
		"matches": results,
		"count":   len(results),
	}})
}
