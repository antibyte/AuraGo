package tools

import (
	"encoding/json"
	"fmt"
	"os"
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

	if pattern == "" {
		return encode(FileSearchResult{Status: "error", Message: "'pattern' is required"})
	}

	switch operation {
	case "grep":
		return fileGrep(filePath, pattern, outputMode, workspaceDir, encode)
	case "grep_recursive":
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
	if glob == "" {
		glob = "*"
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

		matched, _ := filepath.Match(glob, info.Name())
		if !matched {
			return nil
		}

		relPath, _ := filepath.Rel(workspaceDir, path)
		relPath = filepath.ToSlash(relPath)

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

	var files []string
	maxResults := 1000

	filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
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

		matched, _ := filepath.Match(glob, info.Name())
		if !matched {
			return nil
		}

		relPath, _ := filepath.Rel(workspaceDir, path)
		files = append(files, filepath.ToSlash(relPath))

		if len(files) >= maxResults {
			return fmt.Errorf("max results reached")
		}
		return nil
	})

	return encode(FileSearchResult{Status: "success", Data: map[string]interface{}{
		"count": len(files),
		"files": files,
	}})
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
