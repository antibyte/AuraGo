package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// HashlineEditorRequest holds parameters for hashline-validated edits.
type HashlineEditorRequest struct {
	Operation  string
	FilePath   string
	Old        string
	New        string
	Marker     string
	Content    string
	AnchorLine int
	AnchorHash string
	StartLine  int
	EndLine    int
}

// ExecuteHashlineEditor handles hashline-validated file edits without changing
// the legacy ExecuteFileEditor behavior.
func ExecuteHashlineEditor(req HashlineEditorRequest, workspaceDir string) string {
	encode := func(r FileEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if err := requireFilesystemWritePermission(); err != nil {
		return encode(FileEditorResult{Status: "error", Message: err.Error()})
	}
	if req.FilePath == "" {
		return encode(FileEditorResult{Status: "error", Message: "'file_path' is required"})
	}

	resolved, err := secureResolve(workspaceDir, req.FilePath)
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: err.Error()})
	}

	switch req.Operation {
	case "hashline_replace":
		return fileHashlineReplace(resolved, req.Old, req.New, req.AnchorLine, req.AnchorHash, encode)
	case "hashline_insert_after":
		return fileHashlineInsert(resolved, req.Marker, req.Content, req.AnchorLine, req.AnchorHash, true, encode)
	case "hashline_insert_before":
		return fileHashlineInsert(resolved, req.Marker, req.Content, req.AnchorLine, req.AnchorHash, false, encode)
	case "hashline_delete":
		return fileHashlineDelete(resolved, req.StartLine, req.EndLine, req.AnchorLine, req.AnchorHash, encode)
	default:
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Unknown hashline operation '%s'", req.Operation)})
	}
}

func fileHashlineReplace(resolved, old, new_ string, anchorLine int, anchorHash string, encode func(FileEditorResult) string) string {
	if old == "" {
		return encode(FileEditorResult{Status: "error", Message: "'old' text is required for hashline_replace"})
	}
	data, entries, errResult := readHashlineEditableFile(resolved, anchorLine, anchorHash, encode)
	if errResult != "" {
		return errResult
	}

	text := string(data)
	lineStart, lineEnd, err := hashlineLineBounds(text, anchorLine, len(entries))
	if err != nil {
		return encode(FileEditorResult{Status: "error", Message: err.Error()})
	}

	var matches []int
	for searchStart := 0; ; {
		idx := strings.Index(text[searchStart:], old)
		if idx < 0 {
			break
		}
		pos := searchStart + idx
		if pos >= lineStart && pos <= lineEnd {
			matches = append(matches, pos)
		}
		searchStart = pos + len(old)
	}

	if len(matches) == 0 {
		return encode(FileEditorResult{Status: "error", Message: "The 'old' text was not found starting on the validated anchor line"})
	}
	if len(matches) > 1 {
		return encode(FileEditorResult{Status: "error", Message: "The 'old' text appears multiple times on the validated anchor line; include more surrounding context"})
	}

	pos := matches[0]
	result := text[:pos] + new_ + text[pos+len(old):]
	if err := writeFileAtomic(resolved, []byte(result)); err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}

	return encode(FileEditorResult{
		Status:       "success",
		Message:      "Replaced text on validated hashline anchor",
		LinesChanged: strings.Count(old, "\n") + 1,
		TotalLines:   len(strings.Split(result, "\n")),
	})
}

func fileHashlineInsert(resolved, marker, content string, anchorLine int, anchorHash string, after bool, encode func(FileEditorResult) string) string {
	if marker == "" {
		return encode(FileEditorResult{Status: "error", Message: "'marker' text is required for hashline_insert_before/hashline_insert_after"})
	}
	if content == "" {
		return encode(FileEditorResult{Status: "error", Message: "'content' is required for hashline_insert_before/hashline_insert_after"})
	}
	data, entries, errResult := readHashlineEditableFile(resolved, anchorLine, anchorHash, encode)
	if errResult != "" {
		return errResult
	}

	anchorContent := entries[anchorLine-1].Content
	if !strings.Contains(anchorContent, marker) {
		return encode(FileEditorResult{Status: "error", Message: "Marker text was not found on the validated anchor line"})
	}

	lines := strings.Split(string(data), "\n")
	insertLines := strings.Split(content, "\n")
	insertIdx := anchorLine - 1
	if after {
		insertIdx = anchorLine
	}

	newLines := make([]string, 0, len(lines)+len(insertLines))
	newLines = append(newLines, lines[:insertIdx]...)
	newLines = append(newLines, insertLines...)
	newLines = append(newLines, lines[insertIdx:]...)

	result := strings.Join(newLines, "\n")
	if err := writeFileAtomic(resolved, []byte(result)); err != nil {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}

	where := "before"
	if after {
		where = "after"
	}
	return encode(FileEditorResult{
		Status:       "success",
		Message:      fmt.Sprintf("Inserted %d line(s) %s validated hashline anchor", len(insertLines), where),
		LinesChanged: len(insertLines),
		TotalLines:   len(newLines),
	})
}

func fileHashlineDelete(resolved string, startLine, endLine, anchorLine int, anchorHash string, encode func(FileEditorResult) string) string {
	if startLine < 1 {
		return encode(FileEditorResult{Status: "error", Message: "'start_line' must be >= 1"})
	}
	if endLine < startLine {
		return encode(FileEditorResult{Status: "error", Message: "'end_line' must be >= start_line"})
	}
	data, entries, errResult := readHashlineEditableFile(resolved, anchorLine, anchorHash, encode)
	if errResult != "" {
		return errResult
	}
	if startLine > len(entries) {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("start_line %d exceeds file length (%d lines)", startLine, len(entries))})
	}
	if endLine > len(entries) {
		return encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("end_line %d exceeds file length (%d lines)", endLine, len(entries))})
	}
	if anchorLine < startLine || anchorLine > endLine {
		return encode(FileEditorResult{Status: "error", Message: "anchor_line must be within the delete range for hashline_delete"})
	}

	lines := strings.Split(string(data), "\n")
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
		Message:      fmt.Sprintf("Deleted %d line(s) from validated hashline range", deleted),
		LinesChanged: deleted,
		TotalLines:   len(newLines),
	})
}

func readHashlineEditableFile(resolved string, anchorLine int, anchorHash string, encode func(FileEditorResult) string) ([]byte, []HashlineEntry, string) {
	if err := checkEditSizeLimit(resolved); err != nil {
		return nil, nil, encode(FileEditorResult{Status: "error", Message: err.Error()})
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, nil, encode(FileEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}
	entries := buildHashlineEntries(data)
	if err := validateHashlineAnchor(entries, anchorLine, anchorHash); err != nil {
		return nil, nil, encode(FileEditorResult{Status: "error", Message: err.Error()})
	}
	return data, entries, ""
}

func hashlineLineBounds(text string, lineNum, lineCount int) (int, int, error) {
	if lineNum < 1 || lineNum > lineCount {
		return 0, 0, fmt.Errorf("anchor_line %d is out of range (file has %d lines)", lineNum, lineCount)
	}
	start := 0
	for currentLine := 1; currentLine < lineNum; currentLine++ {
		next := strings.IndexByte(text[start:], '\n')
		if next < 0 {
			return 0, 0, fmt.Errorf("anchor_line %d is out of range", lineNum)
		}
		start += next + 1
	}
	end := strings.IndexByte(text[start:], '\n')
	if end < 0 {
		return start, len(text), nil
	}
	return start, start + end, nil
}
