package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// maxEditFileSize limits file sizes for in-memory edit operations to prevent OOM.
const maxEditFileSize int64 = 10 * 1024 * 1024 // 10 MB

// checkEditSizeLimit returns an error if the file exceeds the edit size limit.
func checkEditSizeLimit(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return nil // let os.ReadFile report the real error
	}
	if info.Size() > maxEditFileSize {
		return fmt.Errorf("file exceeds the %d MB edit size limit", maxEditFileSize/(1024*1024))
	}
	return nil
}

// FileEditorResult is the JSON response returned for file_editor operations.
type FileEditorResult struct {
	Status       string `json:"status"`
	Message      string `json:"message,omitempty"`
	LinesChanged int    `json:"lines_changed,omitempty"`
	TotalLines   int    `json:"total_lines,omitempty"`
}

// ExecuteFileEditor handles precise file editing operations, sandboxed to workspaceDir.
func ExecuteFileEditor(operation, filePath, old, new_, marker, content string, startLine, endLine, lineCount int, workspaceDir string) string {
	encode := func(r FileEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if filePath == "" {
		return encode(FileEditorResult{Status: "error", Message: "'file_path' is required"})
	}

	// str_replace_glob uses filePath as a glob pattern — secureResolve rejects wildcards on Windows.
	// The function performs its own workspace-boundary check for each matched file.
	if operation == "str_replace_glob" {
		return fileStrReplaceGlob(workspaceDir, filePath, old, new_, encode)
	}

	resolved, err := secureResolve(workspaceDir, filePath)
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: err.Error()})
	}

	switch operation {
	case "str_replace":
		return fileStrReplace(resolved, old, new_, false, encode)
	case "str_replace_all":
		return fileStrReplace(resolved, old, new_, true, encode)
	case "str_replace_regex":
		return fileStrReplaceRegex(resolved, old, new_, encode)
	case "insert_after":
		return fileInsertRelative(resolved, marker, content, true, encode)
	case "insert_before":
		return fileInsertRelative(resolved, marker, content, false, encode)
	case "append":
		return fileAppendPrepend(resolved, content, true, encode)
	case "prepend":
		return fileAppendPrepend(resolved, content, false, encode)
	case "delete_lines":
		return fileDeleteLines(resolved, startLine, endLine, encode)
	case "apply_patch":
		return fileApplyPatch(resolved, content, workspaceDir, encode)
	default:
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Unknown file_editor operation '%s'. Valid: str_replace, str_replace_all, str_replace_regex, str_replace_glob, insert_after, insert_before, append, prepend, delete_lines, apply_patch", operation)})
	}
}

// fileStrReplace replaces text in a file. If replaceAll is false, the old text must appear exactly once.
func fileStrReplace(resolved, old, new_ string, replaceAll bool, encode func(FileEditorResult) string) string {
	if old == "" {
		return encode(FileEditorResult{Status: "error", Message: "'old' text is required for str_replace"})
	}

	if err := checkEditSizeLimit(resolved); err != nil {
		return encode(FileEditorResult{Status: "error", Message: err.Error()})
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}
	text := string(data)

	count := strings.Count(text, old)
	if count == 0 {
		return encode(FileEditorResult{Status: "error", Message: "The 'old' text was not found in the file"})
	}
	if !replaceAll && count > 1 {
		// Collect the first line of each occurrence to help the LLM add enough context.
		var occurrences []string
		for i, part := range strings.SplitAfter(text, old) {
			if i == 0 || i >= count {
				continue
			}
			// Find the line in the original text where match (i-1) starts.
			before := strings.Join(strings.SplitAfter(text, old)[:i], "")
			lineStart := strings.LastIndex(before[:len(before)-len(old)], "\n")
			if lineStart < 0 {
				lineStart = 0
			} else {
				lineStart++
			}
			lineEnd := strings.Index(before[lineStart:], "\n")
			if lineEnd < 0 {
				lineEnd = len(before) - lineStart
			}
			line := strings.TrimSpace(before[lineStart : lineStart+lineEnd])
			if len(line) > 80 {
				line = line[:80] + "…"
			}
			occurrences = append(occurrences, fmt.Sprintf("  match %d: …%s…", i, line))
			_ = part
		}
		hint := strings.Join(occurrences, "\n")
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf(
			"The 'old' text was found %d times — must be unique for str_replace. Include more surrounding context in 'old' to disambiguate, or use str_replace_all to replace all occurrences.\n%s",
			count, hint)})
	}

	var result string
	if replaceAll {
		result = strings.ReplaceAll(text, old, new_)
	} else {
		result = strings.Replace(text, old, new_, 1)
	}

	if err := writeFileAtomic(resolved, []byte(result)); err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}

	newLines := strings.Count(result, "\n")
	return encode(FileEditorResult{
		Status:       "success",
		Message:      fmt.Sprintf("Replaced %d occurrence(s)", count),
		LinesChanged: count,
		TotalLines:   newLines + 1,
	})
}

// fileInsertRelative inserts content after or before a marker line.
func fileInsertRelative(resolved, marker, content string, after bool, encode func(FileEditorResult) string) string {
	if marker == "" {
		return encode(FileEditorResult{Status: "error", Message: "'marker' text is required for insert_after/insert_before"})
	}
	if content == "" {
		return encode(FileEditorResult{Status: "error", Message: "'content' is required for insert_after/insert_before"})
	}

	if err := checkEditSizeLimit(resolved); err != nil {
		return encode(FileEditorResult{Status: "error", Message: err.Error()})
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	lines := strings.Split(string(data), "\n")
	markerIdx := -1
	for i, line := range lines {
		if strings.Contains(line, marker) {
			if markerIdx >= 0 {
				return encode(FileEditorResult{Status: "error", Message: "Marker text found on multiple lines — provide a more specific marker"})
			}
			markerIdx = i
		}
	}
	if markerIdx < 0 {
		return encode(FileEditorResult{Status: "error", Message: "Marker text not found in the file"})
	}

	insertLines := strings.Split(content, "\n")
	insertIdx := markerIdx
	if after {
		insertIdx = markerIdx + 1
	}

	newLines := make([]string, 0, len(lines)+len(insertLines))
	newLines = append(newLines, lines[:insertIdx]...)
	newLines = append(newLines, insertLines...)
	newLines = append(newLines, lines[insertIdx:]...)

	result := strings.Join(newLines, "\n")
	if err := writeFileAtomic(resolved, []byte(result)); err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}

	return encode(FileEditorResult{
		Status:       "success",
		Message:      fmt.Sprintf("Inserted %d line(s) %s marker", len(insertLines), map[bool]string{true: "after", false: "before"}[after]),
		LinesChanged: len(insertLines),
		TotalLines:   len(newLines),
	})
}

// fileAppendPrepend appends or prepends content to a file.
func fileAppendPrepend(resolved, content string, appendMode bool, encode func(FileEditorResult) string) string {
	if content == "" {
		return encode(FileEditorResult{Status: "error", Message: "'content' is required for append/prepend"})
	}

	if err := checkEditSizeLimit(resolved); err != nil {
		return encode(FileEditorResult{Status: "error", Message: err.Error()})
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) && appendMode {
			// For append, create file if it doesn't exist
			if err := writeFileAtomic(resolved, []byte(content)); err != nil {
				return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to create file: %v", err)})
			}
			lines := strings.Count(content, "\n") + 1
			return encode(FileEditorResult{
				Status:     "success",
				Message:    "File created with content",
				TotalLines: lines,
			})
		}
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	var result string
	if appendMode {
		text := string(data)
		if len(text) > 0 && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		result = text + content
	} else {
		text := string(data)
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		result = content + text
	}

	if err := writeFileAtomic(resolved, []byte(result)); err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}

	totalLines := strings.Count(result, "\n") + 1
	op := "Appended"
	if !appendMode {
		op = "Prepended"
	}
	return encode(FileEditorResult{
		Status:     "success",
		Message:    fmt.Sprintf("%s content to file", op),
		TotalLines: totalLines,
	})
}

// fileDeleteLines deletes a range of lines (1-indexed, inclusive).
func fileDeleteLines(resolved string, startLine, endLine int, encode func(FileEditorResult) string) string {
	if startLine < 1 {
		return encode(FileEditorResult{Status: "error", Message: "'start_line' must be >= 1"})
	}
	if endLine < startLine {
		return encode(FileEditorResult{Status: "error", Message: "'end_line' must be >= start_line"})
	}

	if err := checkEditSizeLimit(resolved); err != nil {
		return encode(FileEditorResult{Status: "error", Message: err.Error()})
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	lines := strings.Split(string(data), "\n")
	if startLine > len(lines) {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("start_line %d exceeds file length (%d lines)", startLine, len(lines))})
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Convert to 0-indexed
	newLines := make([]string, 0, len(lines)-(endLine-startLine+1))
	newLines = append(newLines, lines[:startLine-1]...)
	newLines = append(newLines, lines[endLine:]...)

	result := strings.Join(newLines, "\n")
	if err := writeFileAtomic(resolved, []byte(result)); err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}

	deleted := endLine - startLine + 1
	return encode(FileEditorResult{
		Status:       "success",
		Message:      fmt.Sprintf("Deleted %d line(s) (%d–%d)", deleted, startLine, endLine),
		LinesChanged: deleted,
		TotalLines:   len(newLines),
	})
}

// writeFileAtomic writes data to a file atomically using a temporary file and rename.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create parent dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".aurago_edit_*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Preserve original permissions if file exists
	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode()
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Chmod(tmpName, mode); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	return nil
}

// fileStrReplaceRegex performs a regex-based replacement in a single file.
// The 'old' field is the regex pattern; 'new' is the replacement string (supports $1, $2 capture groups).
func fileStrReplaceRegex(resolved, pattern, replacement string, encode func(FileEditorResult) string) string {
	if pattern == "" {
		return encode(FileEditorResult{Status: "error", Message: "'old' (regex pattern) is required for str_replace_regex"})
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Invalid regex pattern: %v", err)})
	}
	if err := checkEditSizeLimit(resolved); err != nil {
		return encode(FileEditorResult{Status: "error", Message: err.Error()})
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}
	text := string(data)
	count := len(re.FindAllString(text, -1))
	if count == 0 {
		return encode(FileEditorResult{Status: "error", Message: "Pattern matched 0 occurrences in the file"})
	}
	result := re.ReplaceAllString(text, replacement)
	if err := writeFileAtomic(resolved, []byte(result)); err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}
	return encode(FileEditorResult{
		Status:       "success",
		Message:      fmt.Sprintf("Replaced %d match(es) using regex", count),
		LinesChanged: count,
		TotalLines:   strings.Count(result, "\n") + 1,
	})
}

// fileStrReplaceGlob applies str_replace_all across all files matching a glob pattern.
// 'filePath' is interpreted as a glob (e.g. "src/**/*.ts"). 'old' and 'new' are literal strings.
func fileStrReplaceGlob(workspaceDir, globPattern, old, new_ string, encode func(FileEditorResult) string) string {
	if globPattern == "" {
		return encode(FileEditorResult{Status: "error", Message: "'file_path' (glob pattern) is required for str_replace_glob"})
	}
	if old == "" {
		return encode(FileEditorResult{Status: "error", Message: "'old' text is required for str_replace_glob"})
	}
	if err := validateRelativeGlobPattern(globPattern); err != nil {
		return encode(FileEditorResult{Status: "error", Message: err.Error()})
	}
	// Resolve glob relative to workspace dir
	absGlob := filepath.Join(workspaceDir, filepath.Clean(globPattern))
	matches, err := filepath.Glob(absGlob)
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Invalid glob pattern: %v", err)})
	}
	if len(matches) == 0 {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Glob pattern matched 0 files: %s", globPattern)})
	}

	// Resolve the workspace root for security checks
	absWorkspace, wsErr := filepath.Abs(workspaceDir)
	if wsErr != nil {
		absWorkspace = workspaceDir
	}

	filesChanged := 0
	totalReplacements := 0
	var skipped []string
	for _, path := range matches {
		// Security: ensure file is within workspace
		absPath, err := filepath.Abs(path)
		if err != nil {
			skipped = append(skipped, filepath.Base(path)+" (path error)")
			continue
		}
		rel, relErr := filepath.Rel(absWorkspace, absPath)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			skipped = append(skipped, filepath.Base(path)+" (outside workspace)")
			continue
		}

		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Size() > maxEditFileSize {
			skipped = append(skipped, filepath.Base(path)+" (too large)")
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			skipped = append(skipped, filepath.Base(path)+" (read error)")
			continue
		}
		text := string(data)
		count := strings.Count(text, old)
		if count == 0 {
			continue
		}
		result := strings.ReplaceAll(text, old, new_)
		if err := writeFileAtomic(path, []byte(result)); err != nil {
			skipped = append(skipped, filepath.Base(path)+" (write error)")
			continue
		}
		filesChanged++
		totalReplacements += count
	}

	msg := fmt.Sprintf("Replaced %d occurrence(s) across %d file(s) (of %d matching)", totalReplacements, filesChanged, len(matches))
	if len(skipped) > 0 {
		msg += fmt.Sprintf("; skipped: %s", strings.Join(skipped, ", "))
	}
	return encode(FileEditorResult{
		Status:       "success",
		Message:      msg,
		LinesChanged: totalReplacements,
		TotalLines:   filesChanged,
	})
}

func validateRelativeGlobPattern(pattern string) error {
	if filepath.IsAbs(pattern) || filepath.VolumeName(pattern) != "" {
		return fmt.Errorf("glob pattern must be relative to the workspace")
	}
	clean := filepath.Clean(pattern)
	if clean == "." || clean == "" {
		return fmt.Errorf("glob pattern must reference files relative to the workspace")
	}
	for _, part := range strings.Split(clean, string(os.PathSeparator)) {
		if part == ".." {
			return fmt.Errorf("glob pattern must be relative to the workspace")
		}
	}
	return nil
}
