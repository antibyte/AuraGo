package tools

import (
	"encoding/json"
	"fmt"
	"os"
	slashpath "path"
	"path/filepath"
	"regexp"
	"strings"
)

// FileSearchResult is the JSON response returned for file_search operations.
type FileSearchResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// FileSearchMatch represents a single search match.
type FileSearchMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// ExecuteFileSearch handles file search operations, sandboxed to workspaceDir.
func ExecuteFileSearch(operation, pattern, filePath, glob, outputMode string, workspaceDir string) string {
	encode := func(r FileSearchResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	switch operation {
	case "grep":
		if pattern == "" {
			return encode(FileSearchResult{Status: "error", Message: "'pattern' is required for grep"})
		}
		return fileGrep(filePath, pattern, outputMode, workspaceDir, encode)
	case "grep_recursive":
		if pattern == "" {
			return encode(FileSearchResult{Status: "error", Message: "'pattern' is required for grep_recursive"})
		}
		return fileGrepRecursive(glob, pattern, outputMode, workspaceDir, encode)
	case "find":
		return fileFind(glob, workspaceDir, encode)
	default:
		return encode(FileSearchResult{Status: "error", Message: fmt.Sprintf("Unknown file_search operation '%s'. Valid: grep, grep_recursive, find", operation)})
	}
}

// fileGrep searches a single file for matches.
func fileGrep(filePath, pattern, outputMode, workspaceDir string, encode func(FileSearchResult) string) string {
	if filePath == "" {
		return encode(FileSearchResult{Status: "error", Message: "'file_path' is required for grep"})
	}

	resolved, err := secureResolve(workspaceDir, filePath)
	if err != nil {
		return encode(FileSearchResult{Status: "error", Message: err.Error()})
	}

	if len(pattern) > 256 {
		return encode(FileSearchResult{Status: "error", Message: "regex pattern too long (max 256 characters)"})
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return encode(FileSearchResult{Status: "error", Message: fmt.Sprintf("Invalid regex: %v", err)})
	}

	matches, err := grepFile(resolved, re, filePath)
	if err != nil {
		return encode(FileSearchResult{Status: "error", Message: err.Error()})
	}

	if outputMode == "count" {
		return encode(FileSearchResult{Status: "success", Data: map[string]interface{}{
			"count": len(matches),
			"file":  filePath,
		}})
	}

	return encode(FileSearchResult{Status: "success", Data: matches})
}

// fileGrepRecursive searches multiple files matching a glob pattern.
func fileGrepRecursive(glob, pattern, outputMode, workspaceDir string, encode func(FileSearchResult) string) string {
	patterns := normalizeSearchGlobs(glob)

	if len(pattern) > 256 {
		return encode(FileSearchResult{Status: "error", Message: "regex pattern too long (max 256 characters)"})
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return encode(FileSearchResult{Status: "error", Message: fmt.Sprintf("Invalid regex: %v", err)})
	}

	var allMatches []FileSearchMatch
	maxResults := 500

	err = filepath.Walk(workspaceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "node_modules" || base == "__pycache__" || base == "venv" || base == ".venv" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Size() > 10*1024*1024 {
			return nil // skip files > 10MB
		}

		relPath, _ := filepath.Rel(workspaceDir, path)
		relPath = filepath.ToSlash(relPath)
		if !matchesAnySearchGlob(patterns, relPath) {
			return nil
		}

		fileMatches, err := grepFile(path, re, relPath)
		if err != nil {
			return nil // skip unreadable files
		}
		allMatches = append(allMatches, fileMatches...)

		if len(allMatches) >= maxResults {
			return fmt.Errorf("max results reached")
		}
		return nil
	})

	if err != nil && err.Error() != "max results reached" {
		return encode(FileSearchResult{Status: "error", Message: err.Error()})
	}

	if len(allMatches) >= maxResults {
		allMatches = allMatches[:maxResults]
	}

	if outputMode == "count" {
		// Group counts by file
		fileCounts := map[string]int{}
		for _, m := range allMatches {
			fileCounts[m.File]++
		}
		return encode(FileSearchResult{Status: "success", Data: map[string]interface{}{
			"total":       len(allMatches),
			"files_count": len(fileCounts),
			"by_file":     fileCounts,
		}})
	}

	return encode(FileSearchResult{Status: "success", Data: allMatches})
}

// fileFind finds files matching a glob pattern (no content search).
func fileFind(glob, workspaceDir string, encode func(FileSearchResult) string) string {
	if glob == "" {
		return encode(FileSearchResult{Status: "error", Message: "'glob' is required for find (used as the pattern)"})
	}
	patterns := normalizeSearchGlobs(glob)

	var files []string
	maxResults := 1000

	err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "node_modules" || base == "__pycache__" || base == "venv" || base == ".venv" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, _ := filepath.Rel(workspaceDir, path)
		relPath = filepath.ToSlash(relPath)
		if !matchesAnySearchGlob(patterns, relPath) {
			return nil
		}
		files = append(files, relPath)

		if len(files) >= maxResults {
			return fmt.Errorf("max results reached")
		}
		return nil
	})
	if err != nil && err.Error() != "max results reached" {
		return encode(FileSearchResult{Status: "error", Message: err.Error()})
	}
	if len(files) >= maxResults {
		files = files[:maxResults]
	}

	return encode(FileSearchResult{Status: "success", Data: map[string]interface{}{
		"count": len(files),
		"files": files,
	}})
}

func normalizeSearchGlobs(glob string) []string {
	if strings.TrimSpace(glob) == "" {
		return []string{"*"}
	}
	parts := strings.FieldsFunc(glob, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	var patterns []string
	for _, part := range parts {
		part = strings.TrimSpace(filepath.ToSlash(part))
		if part != "" {
			patterns = append(patterns, part)
		}
	}
	if len(patterns) == 0 {
		return []string{"*"}
	}
	return patterns
}

func matchesAnySearchGlob(patterns []string, relPath string) bool {
	for _, pattern := range patterns {
		if matchSearchGlob(pattern, relPath) {
			return true
		}
	}
	return false
}

func matchSearchGlob(pattern, relPath string) bool {
	pattern = strings.TrimSpace(filepath.ToSlash(pattern))
	relPath = filepath.ToSlash(relPath)
	if pattern == "" {
		return false
	}
	if !strings.Contains(pattern, "/") && !strings.Contains(pattern, "**") {
		matched, err := slashpath.Match(pattern, slashpath.Base(relPath))
		return err == nil && matched
	}
	return matchSearchGlobSegments(splitSearchPattern(pattern), splitSearchPattern(relPath))
}

func splitSearchPattern(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(strings.Trim(value, "/"), "/")
}

func matchSearchGlobSegments(patternParts, pathParts []string) bool {
	if len(patternParts) == 0 {
		return len(pathParts) == 0
	}
	if patternParts[0] == "**" {
		if matchSearchGlobSegments(patternParts[1:], pathParts) {
			return true
		}
		return len(pathParts) > 0 && matchSearchGlobSegments(patternParts, pathParts[1:])
	}
	if len(pathParts) == 0 {
		return false
	}
	matched, err := slashpath.Match(patternParts[0], pathParts[0])
	if err != nil || !matched {
		return false
	}
	return matchSearchGlobSegments(patternParts[1:], pathParts[1:])
}

// grepFile searches a single file for regex matches, returning matches with line numbers.
func grepFile(absPath string, re *regexp.Regexp, displayPath string) ([]FileSearchMatch, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var matches []FileSearchMatch
	scanner := newLargeFileScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, FileSearchMatch{
				File:    displayPath,
				Line:    lineNum,
				Content: strings.TrimRight(line, "\r\n"),
			})
		}
	}
	return matches, nil
}
