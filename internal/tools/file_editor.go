package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

	resolved, err := secureResolve(workspaceDir, filePath)
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: err.Error()})
	}

	switch operation {
	case "str_replace":
		return fileStrReplace(resolved, old, new_, false, encode)
	case "str_replace_all":
		return fileStrReplace(resolved, old, new_, true, encode)
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
	default:
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Unknown file_editor operation '%s'. Valid: str_replace, str_replace_all, insert_after, insert_before, append, prepend, delete_lines", operation)})
	}
}

// fileStrReplace replaces text in a file. If replaceAll is false, the old text must appear exactly once.
func fileStrReplace(resolved, old, new_ string, replaceAll bool, encode func(FileEditorResult) string) string {
	if old == "" {
		return encode(FileEditorResult{Status: "error", Message: "'old' text is required for str_replace"})
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
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("The 'old' text was found %d times — must be unique for str_replace. Use str_replace_all to replace all occurrences, or provide more context to make the match unique.", count)})
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
