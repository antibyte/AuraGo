package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"aurago/internal/remote"
	"aurago/internal/shellpolicy"
)

// Executor handles command execution on the remote device.
type Executor struct {
	logger           *slog.Logger
	mu               sync.RWMutex
	maxFileSizeBytes int64
	commandResults   map[string]*commandExecutionRecord
	commandOrder     []string
}

const maxCachedCommandResults = 1024

type commandExecutionRecord struct {
	done   chan struct{}
	result remote.ResultPayload
}

// NewExecutor creates a new command executor.
func NewExecutor(logger *slog.Logger, maxFileSizeMB int) *Executor {
	e := &Executor{
		logger:         logger,
		commandResults: make(map[string]*commandExecutionRecord),
	}
	e.SetMaxFileSizeMB(maxFileSizeMB)
	return e
}

// Execute runs a command and returns the result.
func (e *Executor) Execute(cmd remote.CommandPayload, readOnly bool, allowedPaths []string) remote.ResultPayload {
	commandID := strings.TrimSpace(cmd.CommandID)
	if commandID == "" {
		return e.executeOnce(cmd, readOnly, allowedPaths)
	}

	record, owner := e.claimCommandExecution(commandID)
	if !owner {
		<-record.done
		return record.result
	}

	result := e.executeOnce(cmd, readOnly, allowedPaths)
	e.finishCommandExecution(commandID, record, result)
	return result
}

func (e *Executor) claimCommandExecution(commandID string) (*commandExecutionRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.commandResults == nil {
		e.commandResults = make(map[string]*commandExecutionRecord)
	}
	if record, ok := e.commandResults[commandID]; ok {
		return record, false
	}
	record := &commandExecutionRecord{done: make(chan struct{})}
	e.commandResults[commandID] = record
	e.commandOrder = append(e.commandOrder, commandID)
	return record, true
}

func (e *Executor) finishCommandExecution(commandID string, record *commandExecutionRecord, result remote.ResultPayload) {
	e.mu.Lock()
	record.result = result
	close(record.done)
	e.trimCommandResultCacheLocked(commandID)
	e.mu.Unlock()
}

func (e *Executor) trimCommandResultCacheLocked(currentID string) {
	if len(e.commandOrder) <= maxCachedCommandResults {
		return
	}

	targetRemovals := len(e.commandOrder) - maxCachedCommandResults
	removed := 0
	kept := e.commandOrder[:0]
	for _, commandID := range e.commandOrder {
		if removed >= targetRemovals {
			kept = append(kept, commandID)
			continue
		}
		if commandID == currentID {
			kept = append(kept, commandID)
			continue
		}
		record := e.commandResults[commandID]
		if record == nil {
			removed++
			continue
		}
		select {
		case <-record.done:
			delete(e.commandResults, commandID)
			removed++
		default:
			kept = append(kept, commandID)
		}
	}
	e.commandOrder = kept
}

func (e *Executor) executeOnce(cmd remote.CommandPayload, readOnly bool, allowedPaths []string) remote.ResultPayload {
	result := remote.ResultPayload{
		CommandID: cmd.CommandID,
		Status:    "ok",
	}

	// Client-side read-only check
	if readOnly && !remote.ReadOnlySafe(cmd.Operation) {
		result.Status = "denied"
		result.Error = "operation blocked: device is in read-only mode"
		return result
	}

	switch cmd.Operation {
	case remote.OpSysinfo:
		info := e.CollectSysinfo()
		data, _ := json.Marshal(info)
		result.Output = string(data)

	case remote.OpFileRead:
		path, _ := cmd.Args["path"].(string)
		resolvedPath, err := e.resolveAllowedPath(path, allowedPaths)
		if err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		if err := e.validateFileReadSize(resolvedPath); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
		data, err := os.ReadFile(resolvedPath)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
		result.Output = base64.StdEncoding.EncodeToString(data)

	case remote.OpFileWrite:
		path, _ := cmd.Args["path"].(string)
		content, _ := cmd.Args["content"].(string) // base64
		modeRaw, _ := cmd.Args["mode"].(float64)
		mode := os.FileMode(0644)
		if modeRaw > 0 {
			mode = os.FileMode(int(modeRaw))
		}
		resolvedPath, err := e.resolveAllowedPath(path, allowedPaths)
		if err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		if err := e.validateEncodedWriteSize(content); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			result.Status = "error"
			result.Error = fmt.Sprintf("invalid base64 content: %v", err)
			return result
		}
		if err := e.validateDecodedWriteSize(len(decoded)); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(resolvedPath), 0755); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
		if err := os.WriteFile(resolvedPath, decoded, mode); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
		result.Output = fmt.Sprintf("wrote %d bytes to %s", len(decoded), resolvedPath)

	case remote.OpFileList:
		path, _ := cmd.Args["path"].(string)
		recursive, _ := cmd.Args["recursive"].(bool)
		resolvedPath, err := e.resolveAllowedPath(path, allowedPaths)
		if err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		entries, err := e.listDir(resolvedPath, recursive)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
		data, _ := json.Marshal(entries)
		result.Output = string(data)

	case remote.OpFileDelete:
		path, _ := cmd.Args["path"].(string)
		resolvedPath, err := e.resolveAllowedPath(path, allowedPaths)
		if err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		if err := os.Remove(resolvedPath); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
		result.Output = fmt.Sprintf("deleted %s", resolvedPath)

	case remote.OpShellExec:
		command, _ := cmd.Args["command"].(string)
		workDir, _ := cmd.Args["working_dir"].(string)
		timeout := time.Duration(cmd.TimeoutSec) * time.Second
		if timeout == 0 {
			timeout = 60 * time.Second
		}
		if strings.TrimSpace(workDir) != "" {
			resolvedWorkDir, err := e.resolveAllowedPath(workDir, allowedPaths)
			if err != nil {
				result.Status = "denied"
				result.Error = err.Error()
				return result
			}
			workDir = resolvedWorkDir
		}
		output, err := e.shellExec(command, workDir, timeout)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		result.Output = output

	case remote.OpFileEdit:
		path, _ := cmd.Args["path"].(string)
		resolvedPath, err := e.resolveAllowedPath(path, allowedPaths)
		if err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		op, _ := cmd.Args["operation"].(string)
		old, _ := cmd.Args["old"].(string)
		new_, _ := cmd.Args["new"].(string)
		marker, _ := cmd.Args["marker"].(string)
		content, _ := cmd.Args["content"].(string)
		startLine, _ := cmd.Args["start_line"].(float64)
		endLine, _ := cmd.Args["end_line"].(float64)
		output, err := e.fileEdit(resolvedPath, op, old, new_, marker, content, int(startLine), int(endLine))
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		result.Output = output

	case remote.OpJsonEdit:
		path, _ := cmd.Args["path"].(string)
		resolvedPath, err := e.resolveAllowedPath(path, allowedPaths)
		if err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		op, _ := cmd.Args["operation"].(string)
		jsonPath, _ := cmd.Args["json_path"].(string)
		setValue := cmd.Args["set_value"]
		output, err := e.jsonEdit(resolvedPath, op, jsonPath, setValue)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		result.Output = output

	case remote.OpYamlEdit:
		path, _ := cmd.Args["path"].(string)
		resolvedPath, err := e.resolveAllowedPath(path, allowedPaths)
		if err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		op, _ := cmd.Args["operation"].(string)
		jsonPath, _ := cmd.Args["json_path"].(string)
		setValue := cmd.Args["set_value"]
		output, err := e.yamlEdit(resolvedPath, op, jsonPath, setValue)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		result.Output = output

	case remote.OpXmlEdit:
		path, _ := cmd.Args["path"].(string)
		resolvedPath, err := e.resolveAllowedPath(path, allowedPaths)
		if err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		op, _ := cmd.Args["operation"].(string)
		xpath, _ := cmd.Args["xpath"].(string)
		setValue := cmd.Args["set_value"]
		output, err := e.xmlEdit(resolvedPath, op, xpath, setValue)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		result.Output = output

	case remote.OpFileSearch:
		op, _ := cmd.Args["operation"].(string)
		pattern, _ := cmd.Args["pattern"].(string)
		path, _ := cmd.Args["path"].(string)
		globPattern, _ := cmd.Args["glob"].(string)
		outputMode, _ := cmd.Args["output_mode"].(string)
		if strings.TrimSpace(path) == "" {
			result.Status = "denied"
			result.Error = "path is required for file_search operations"
			return result
		}
		resolvedPath, err := e.resolveAllowedPath(path, allowedPaths)
		if err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		output, err := e.fileSearch(op, pattern, resolvedPath, globPattern, outputMode, resolvedPath)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		result.Output = output

	case remote.OpFileReadAdv:
		path, _ := cmd.Args["path"].(string)
		resolvedPath, err := e.resolveAllowedPath(path, allowedPaths)
		if err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		op, _ := cmd.Args["operation"].(string)
		pattern, _ := cmd.Args["pattern"].(string)
		startLine, _ := cmd.Args["start_line"].(float64)
		endLine, _ := cmd.Args["end_line"].(float64)
		lineCount, _ := cmd.Args["line_count"].(float64)
		output, err := e.fileReadAdvanced(resolvedPath, op, pattern, int(startLine), int(endLine), int(lineCount))
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		result.Output = output

	default:
		result.Status = "error"
		result.Error = fmt.Sprintf("unknown operation: %s", cmd.Operation)
	}

	return result
}

func (e *Executor) SetMaxFileSizeMB(maxFileSizeMB int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.maxFileSizeBytes = int64(normalizeMaxFileSizeMB(maxFileSizeMB)) * 1024 * 1024
}

func (e *Executor) maxFileSizeBytesSnapshot() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.maxFileSizeBytes <= 0 {
		return int64(remote.DefaultMaxFileSizeMB) * 1024 * 1024
	}
	return e.maxFileSizeBytes
}

func normalizeMaxFileSizeMB(maxFileSizeMB int) int {
	if maxFileSizeMB <= 0 {
		return remote.DefaultMaxFileSizeMB
	}
	return maxFileSizeMB
}

func (e *Executor) validateFileReadSize(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() > e.maxFileSizeBytesSnapshot() {
		return fmt.Errorf("file exceeds configured max_file_size_mb limit")
	}
	return nil
}

func (e *Executor) validateEncodedWriteSize(content string) error {
	if base64.StdEncoding.DecodedLen(len(content)) > int(e.maxFileSizeBytesSnapshot()) {
		return fmt.Errorf("file exceeds configured max_file_size_mb limit")
	}
	return nil
}

func (e *Executor) validateDecodedWriteSize(size int) error {
	if int64(size) > e.maxFileSizeBytesSnapshot() {
		return fmt.Errorf("file exceeds configured max_file_size_mb limit")
	}
	return nil
}

// CollectSysinfo gathers system information for heartbeats.
func (e *Executor) CollectSysinfo() remote.HeartbeatPayload {
	hostname, _ := os.Hostname()
	return remote.HeartbeatPayload{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
	}
}

// validatePath checks that a path is within the allowed paths.
func (e *Executor) validatePath(path string, allowedPaths []string) error {
	_, err := e.resolveAllowedPath(path, allowedPaths)
	return err
}

// resolveAllowedPath returns the canonical path that should be used for the
// subsequent file operation. This avoids validating a symlink path and then
// operating on a different target if an existing symlink prefix is involved.
func (e *Executor) resolveAllowedPath(path string, allowedPaths []string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	if len(allowedPaths) == 0 {
		return "", fmt.Errorf("path operations are disabled until allowed_paths is configured")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	resolved, err := resolveExistingPrefix(absPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	for _, allowed := range allowedPaths {
		allowedAbs, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		allowedResolved, err := resolveExistingPrefix(allowedAbs)
		if err != nil {
			continue
		}
		if pathWithinDir(resolved, allowedResolved) {
			return resolved, nil
		}
	}

	return "", fmt.Errorf("path %q is outside allowed directories", path)
}

func resolveExistingPrefix(absPath string) (string, error) {
	cleanPath := filepath.Clean(absPath)
	if resolved, err := filepath.EvalSymlinks(cleanPath); err == nil {
		return filepath.Clean(resolved), nil
	}

	existing := cleanPath
	var suffix []string
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", fmt.Errorf("no existing path prefix for %q", absPath)
		}
		suffix = append([]string{filepath.Base(existing)}, suffix...)
		existing = parent
	}

	resolvedExisting, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", err
	}
	parts := append([]string{resolvedExisting}, suffix...)
	return filepath.Clean(filepath.Join(parts...)), nil
}

func pathWithinDir(path, allowedDir string) bool {
	cleanPath := filepath.Clean(path)
	cleanAllowed := filepath.Clean(allowedDir)
	if runtime.GOOS == "windows" {
		cleanPath = strings.ToLower(cleanPath)
		cleanAllowed = strings.ToLower(cleanAllowed)
	}
	if cleanPath == cleanAllowed {
		return true
	}
	rel, err := filepath.Rel(cleanAllowed, cleanPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

type fileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

func (e *Executor) listDir(path string, recursive bool) ([]fileEntry, error) {
	var entries []fileEntry

	if !recursive {
		dirEntries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, de := range dirEntries {
			info, err := de.Info()
			if err != nil {
				continue
			}
			entries = append(entries, fileEntry{
				Name:    de.Name(),
				Path:    filepath.Join(path, de.Name()),
				IsDir:   de.IsDir(),
				Size:    info.Size(),
				ModTime: info.ModTime().UTC().Format(time.RFC3339),
			})
		}
		return entries, nil
	}

	_ = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		entries = append(entries, fileEntry{
			Name:    info.Name(),
			Path:    p,
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
		})
		return nil
	})

	return entries, nil
}

func (e *Executor) shellExec(command, workDir string, timeout time.Duration) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	// Security: Check for dangerous commands before execution
	if err := shellpolicy.ValidateCommand(command); err != nil {
		e.logger.Warn("[shellExec] blocked shell command", "reason", err.Error(), "command", command)
		return "", err
	}

	// Log every command execution for audit trail
	e.logger.Info("[shellExec] executing remote command", "command", command, "workDir", workDir, "timeout", timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.WaitDelay = 250 * time.Millisecond

	if workDir != "" {
		cmd.Dir = workDir
	}

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out after %v", timeout)
	}
	return string(output), err
}

// fileEdit performs a file editing operation on the remote device.
func (e *Executor) fileEdit(path, operation, old, new_, marker, content string, startLine, endLine int) (string, error) {
	switch operation {
	case "str_replace":
		return e.fileStrReplace(path, old, new_, false)
	case "str_replace_all":
		return e.fileStrReplace(path, old, new_, true)
	case "insert_after":
		return e.fileInsertRelative(path, marker, content, true)
	case "insert_before":
		return e.fileInsertRelative(path, marker, content, false)
	case "append":
		return e.fileAppendPrepend(path, content, true)
	case "prepend":
		return e.fileAppendPrepend(path, content, false)
	case "delete_lines":
		return e.fileDeleteLines(path, startLine, endLine)
	default:
		return "", fmt.Errorf("unknown file_edit operation: %s", operation)
	}
}

func (e *Executor) fileStrReplace(path, old, new_ string, replaceAll bool) (string, error) {
	if old == "" {
		return "", fmt.Errorf("'old' text is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := string(data)
	count := strings.Count(text, old)
	if count == 0 {
		return "", fmt.Errorf("text not found in file")
	}
	if !replaceAll && count > 1 {
		return "", fmt.Errorf("text found %d times, must be unique for str_replace", count)
	}
	var result string
	if replaceAll {
		result = strings.ReplaceAll(text, old, new_)
	} else {
		result = strings.Replace(text, old, new_, 1)
	}
	if err := os.WriteFile(path, []byte(result), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("replaced %d occurrence(s)", count), nil
}

func (e *Executor) fileInsertRelative(path, marker, content string, after bool) (string, error) {
	if marker == "" {
		return "", fmt.Errorf("'marker' is required")
	}
	if content == "" {
		return "", fmt.Errorf("'content' is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	idx := -1
	for i, line := range lines {
		if strings.Contains(line, marker) {
			if idx >= 0 {
				return "", fmt.Errorf("marker found on multiple lines")
			}
			idx = i
		}
	}
	if idx < 0 {
		return "", fmt.Errorf("marker not found")
	}
	insertLines := strings.Split(content, "\n")
	insertIdx := idx
	if after {
		insertIdx = idx + 1
	}
	newLines := make([]string, 0, len(lines)+len(insertLines))
	newLines = append(newLines, lines[:insertIdx]...)
	newLines = append(newLines, insertLines...)
	newLines = append(newLines, lines[insertIdx:]...)
	if err := os.WriteFile(path, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
		return "", err
	}
	pos := "after"
	if !after {
		pos = "before"
	}
	return fmt.Sprintf("inserted %d line(s) %s marker", len(insertLines), pos), nil
}

func (e *Executor) fileAppendPrepend(path, content string, appendMode bool) (string, error) {
	if content == "" {
		return "", fmt.Errorf("'content' is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && appendMode {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return "", err
			}
			return "file created", nil
		}
		return "", err
	}
	text := string(data)
	var result string
	if appendMode {
		if len(text) > 0 && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		result = text + content
	} else {
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		result = content + text
	}
	if err := os.WriteFile(path, []byte(result), 0644); err != nil {
		return "", err
	}
	op := "appended"
	if !appendMode {
		op = "prepended"
	}
	return fmt.Sprintf("%s content to file", op), nil
}

func (e *Executor) fileDeleteLines(path string, startLine, endLine int) (string, error) {
	if startLine < 1 {
		return "", fmt.Errorf("start_line must be >= 1")
	}
	if endLine < startLine {
		return "", fmt.Errorf("end_line must be >= start_line")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if startLine > len(lines) {
		return "", fmt.Errorf("start_line %d exceeds file length (%d lines)", startLine, len(lines))
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	newLines := make([]string, 0, len(lines)-(endLine-startLine+1))
	newLines = append(newLines, lines[:startLine-1]...)
	newLines = append(newLines, lines[endLine:]...)
	if err := os.WriteFile(path, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("deleted %d line(s)", endLine-startLine+1), nil
}

// fileSearch performs file search operations on the remote device.
func (e *Executor) fileSearch(op, pattern, filePath, globPattern, outputMode, baseDir string) (string, error) {
	switch op {
	case "grep":
		if filePath == "" {
			return "", fmt.Errorf("path is required for grep")
		}
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return "", fmt.Errorf("invalid regex: %w", err)
		}
		matches, err := grepFileRemote(filePath, re, filePath)
		if err != nil {
			return "", err
		}
		if outputMode == "count" {
			out, _ := json.Marshal(map[string]interface{}{"count": len(matches)})
			return string(out), nil
		}
		out, _ := json.Marshal(matches)
		return string(out), nil

	case "grep_recursive":
		if globPattern == "" {
			globPattern = "*"
		}
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return "", fmt.Errorf("invalid regex: %w", err)
		}
		type match struct {
			File    string `json:"file"`
			Line    int    `json:"line"`
			Content string `json:"content"`
		}
		var allMatches []match
		maxResults := 500
		filepath.Walk(baseDir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil || info.IsDir() {
				if info != nil && info.IsDir() {
					base := info.Name()
					if base == ".git" || base == "node_modules" || base == "__pycache__" {
						return filepath.SkipDir
					}
				}
				return nil
			}
			if info.Size() > 10*1024*1024 {
				return nil
			}
			matched, _ := filepath.Match(globPattern, info.Name())
			if !matched {
				return nil
			}
			relPath, _ := filepath.Rel(baseDir, path)
			fileMatches, _ := grepFileRemote(path, re, filepath.ToSlash(relPath))
			for _, m := range fileMatches {
				allMatches = append(allMatches, match{File: m.File, Line: m.Line, Content: m.Content})
			}
			if len(allMatches) >= maxResults {
				return fmt.Errorf("max")
			}
			return nil
		})
		if outputMode == "count" {
			out, _ := json.Marshal(map[string]interface{}{"count": len(allMatches)})
			return string(out), nil
		}
		out, _ := json.Marshal(allMatches)
		return string(out), nil

	case "find":
		if globPattern == "" {
			return "", fmt.Errorf("glob pattern required for find (passed as 'pattern')")
		}
		var files []string
		filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				if info != nil && info.IsDir() {
					base := info.Name()
					if base == ".git" || base == "node_modules" || base == "__pycache__" {
						return filepath.SkipDir
					}
				}
				return nil
			}
			matched, _ := filepath.Match(globPattern, info.Name())
			if matched {
				relPath, _ := filepath.Rel(baseDir, path)
				files = append(files, filepath.ToSlash(relPath))
			}
			if len(files) >= 1000 {
				return fmt.Errorf("max")
			}
			return nil
		})
		out, _ := json.Marshal(map[string]interface{}{"count": len(files), "files": files})
		return string(out), nil

	default:
		return "", fmt.Errorf("unknown file_search operation: %s", op)
	}
}

type remoteGrepMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

func grepFileRemote(absPath string, re *regexp.Regexp, displayPath string) ([]remoteGrepMatch, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var matches []remoteGrepMatch
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, remoteGrepMatch{File: displayPath, Line: lineNum, Content: line})
		}
	}
	return matches, nil
}

// fileReadAdvanced performs advanced file reading on the remote device.
func (e *Executor) fileReadAdvanced(path, op, pattern string, startLine, endLine, lineCount int) (string, error) {
	switch op {
	case "read_lines":
		if startLine < 1 {
			startLine = 1
		}
		if endLine < startLine {
			return "", fmt.Errorf("end_line must be >= start_line")
		}
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		var lines []string
		scanner := bufio.NewScanner(f)
		num := 0
		for scanner.Scan() {
			num++
			if num >= startLine && num <= endLine {
				lines = append(lines, scanner.Text())
			}
			if num > endLine {
				break
			}
		}
		out, _ := json.Marshal(map[string]interface{}{
			"start_line": startLine,
			"end_line":   startLine + len(lines) - 1,
			"content":    strings.Join(lines, "\n"),
		})
		return string(out), nil

	case "head":
		if lineCount <= 0 {
			lineCount = 20
		}
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		var lines []string
		scanner := bufio.NewScanner(f)
		for scanner.Scan() && len(lines) < lineCount {
			lines = append(lines, scanner.Text())
		}
		out, _ := json.Marshal(map[string]interface{}{"content": strings.Join(lines, "\n"), "lines": len(lines)})
		return string(out), nil

	case "tail":
		if lineCount <= 0 {
			lineCount = 20
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		allLines := strings.Split(string(data), "\n")
		start := len(allLines) - lineCount
		if start < 0 {
			start = 0
		}
		lines := allLines[start:]
		out, _ := json.Marshal(map[string]interface{}{"content": strings.Join(lines, "\n"), "lines": len(lines)})
		return string(out), nil

	case "count_lines":
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		count := 0
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			count++
		}
		info, _ := os.Stat(path)
		var size int64
		if info != nil {
			size = info.Size()
		}
		out, _ := json.Marshal(map[string]interface{}{"lines": count, "bytes": size})
		return string(out), nil

	case "search_context":
		if pattern == "" {
			return "", fmt.Errorf("pattern is required for search_context")
		}
		if lineCount <= 0 {
			lineCount = 3
		}
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return "", fmt.Errorf("invalid regex: %w", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		allLines := strings.Split(string(data), "\n")
		type ctxResult struct {
			MatchLine int    `json:"match_line"`
			Content   string `json:"content"`
		}
		var results []ctxResult
		for i, line := range allLines {
			if re.MatchString(line) {
				start := i - lineCount
				if start < 0 {
					start = 0
				}
				end := i + lineCount
				if end >= len(allLines) {
					end = len(allLines) - 1
				}
				results = append(results, ctxResult{MatchLine: i + 1, Content: strings.Join(allLines[start:end+1], "\n")})
				if len(results) >= 50 {
					break
				}
			}
		}
		out, _ := json.Marshal(map[string]interface{}{"matches": results, "count": len(results)})
		return string(out), nil

	default:
		return "", fmt.Errorf("unknown file_reader_advanced operation: %s", op)
	}
}
