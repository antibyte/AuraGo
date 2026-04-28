package main

import (
	"bufio"
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
	"aurago/internal/tools"

	"github.com/beevik/etree"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"gopkg.in/yaml.v3"
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
	for len(e.commandOrder) > maxCachedCommandResults {
		oldest := e.commandOrder[0]
		e.commandOrder = e.commandOrder[1:]
		if oldest == currentID {
			e.commandOrder = append(e.commandOrder, oldest)
			return
		}
		record := e.commandResults[oldest]
		if record == nil {
			continue
		}
		select {
		case <-record.done:
			delete(e.commandResults, oldest)
		default:
			e.commandOrder = append(e.commandOrder, oldest)
			return
		}
	}
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
		if err := e.validatePath(path, allowedPaths); err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		if err := e.validateFileReadSize(path); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
		data, err := os.ReadFile(path)
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
		if err := e.validatePath(path, allowedPaths); err != nil {
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
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
		if err := os.WriteFile(path, decoded, mode); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
		result.Output = fmt.Sprintf("wrote %d bytes to %s", len(decoded), path)

	case remote.OpFileList:
		path, _ := cmd.Args["path"].(string)
		recursive, _ := cmd.Args["recursive"].(bool)
		if err := e.validatePath(path, allowedPaths); err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		entries, err := e.listDir(path, recursive)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
		data, _ := json.Marshal(entries)
		result.Output = string(data)

	case remote.OpFileDelete:
		path, _ := cmd.Args["path"].(string)
		if err := e.validatePath(path, allowedPaths); err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		if err := os.Remove(path); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return result
		}
		result.Output = fmt.Sprintf("deleted %s", path)

	case remote.OpShellExec:
		command, _ := cmd.Args["command"].(string)
		workDir, _ := cmd.Args["working_dir"].(string)
		timeout := time.Duration(cmd.TimeoutSec) * time.Second
		if timeout == 0 {
			timeout = 60 * time.Second
		}
		if strings.TrimSpace(workDir) != "" {
			if err := e.validatePath(workDir, allowedPaths); err != nil {
				result.Status = "denied"
				result.Error = err.Error()
				return result
			}
		}
		output, err := e.shellExec(command, workDir, timeout)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		result.Output = output

	case remote.OpFileEdit:
		path, _ := cmd.Args["path"].(string)
		if err := e.validatePath(path, allowedPaths); err != nil {
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
		output, err := e.fileEdit(path, op, old, new_, marker, content, int(startLine), int(endLine))
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		result.Output = output

	case remote.OpJsonEdit:
		path, _ := cmd.Args["path"].(string)
		if err := e.validatePath(path, allowedPaths); err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		op, _ := cmd.Args["operation"].(string)
		jsonPath, _ := cmd.Args["json_path"].(string)
		setValue := cmd.Args["set_value"]
		output, err := e.jsonEdit(path, op, jsonPath, setValue)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		result.Output = output

	case remote.OpYamlEdit:
		path, _ := cmd.Args["path"].(string)
		if err := e.validatePath(path, allowedPaths); err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		op, _ := cmd.Args["operation"].(string)
		jsonPath, _ := cmd.Args["json_path"].(string)
		setValue := cmd.Args["set_value"]
		output, err := e.yamlEdit(path, op, jsonPath, setValue)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		result.Output = output

	case remote.OpXmlEdit:
		path, _ := cmd.Args["path"].(string)
		if err := e.validatePath(path, allowedPaths); err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		op, _ := cmd.Args["operation"].(string)
		xpath, _ := cmd.Args["xpath"].(string)
		setValue := cmd.Args["set_value"]
		output, err := e.xmlEdit(path, op, xpath, setValue)
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
		if err := e.validatePath(path, allowedPaths); err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		output, err := e.fileSearch(op, pattern, path, globPattern, outputMode, path)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		result.Output = output

	case remote.OpFileReadAdv:
		path, _ := cmd.Args["path"].(string)
		if err := e.validatePath(path, allowedPaths); err != nil {
			result.Status = "denied"
			result.Error = err.Error()
			return result
		}
		op, _ := cmd.Args["operation"].(string)
		pattern, _ := cmd.Args["pattern"].(string)
		startLine, _ := cmd.Args["start_line"].(float64)
		endLine, _ := cmd.Args["end_line"].(float64)
		lineCount, _ := cmd.Args["line_count"].(float64)
		output, err := e.fileReadAdvanced(path, op, pattern, int(startLine), int(endLine), int(lineCount))
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
	if path == "" {
		return fmt.Errorf("path is required")
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Resolve symlinks to prevent traversal
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If file doesn't exist yet (for writes), resolve parent
		resolved, err = filepath.EvalSymlinks(filepath.Dir(absPath))
		if err != nil {
			resolved = absPath
		} else {
			resolved = filepath.Join(resolved, filepath.Base(absPath))
		}
	}

	// Check against allowed paths
	if len(allowedPaths) == 0 {
		return fmt.Errorf("path operations are disabled until allowed_paths is configured")
	}

	for _, allowed := range allowedPaths {
		allowedAbs, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		// Normalize separators
		if strings.HasPrefix(resolved+string(filepath.Separator), allowedAbs+string(filepath.Separator)) || resolved == allowedAbs {
			return nil
		}
	}

	return fmt.Errorf("path %q is outside allowed directories", path)
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
	if err := tools.ValidateShellCommandPolicy(command); err != nil {
		e.logger.Warn("[shellExec] blocked shell command", "reason", err.Error(), "command", command)
		return "", err
	}

	// Log every command execution for audit trail
	e.logger.Info("[shellExec] executing remote command", "command", command, "workDir", workDir, "timeout", timeout)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}

	if workDir != "" {
		cmd.Dir = workDir
	}

	// Use a channel to enforce timeout
	done := make(chan struct{})
	var output []byte
	var cmdErr error

	go func() {
		output, cmdErr = cmd.CombinedOutput()
		close(done)
	}()

	select {
	case <-done:
		return string(output), cmdErr
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return "", fmt.Errorf("command timed out after %v", timeout)
	}
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

// jsonEdit performs JSON file operations on the remote device.
func (e *Executor) jsonEdit(path, op, jsonPath string, setValue interface{}) (string, error) {
	switch op {
	case "get":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if !gjson.ValidBytes(data) {
			return "", fmt.Errorf("file is not valid JSON")
		}
		if jsonPath == "" {
			return string(data), nil
		}
		result := gjson.GetBytes(data, jsonPath)
		if !result.Exists() {
			return "", fmt.Errorf("path '%s' not found", jsonPath)
		}
		return result.Raw, nil

	case "set":
		if jsonPath == "" {
			return "", fmt.Errorf("json_path is required for set")
		}
		var data []byte
		if _, err := os.Stat(path); err == nil {
			data, err = os.ReadFile(path)
			if err != nil {
				return "", err
			}
			if !gjson.ValidBytes(data) {
				return "", fmt.Errorf("file is not valid JSON")
			}
		} else {
			data = []byte("{}")
		}
		updated, err := sjson.SetBytes(data, jsonPath, setValue)
		if err != nil {
			return "", fmt.Errorf("set failed: %w", err)
		}
		var pretty json.RawMessage = updated
		formatted, err := json.MarshalIndent(pretty, "", "  ")
		if err != nil {
			formatted = updated
		}
		if err := os.WriteFile(path, append(formatted, '\n'), 0644); err != nil {
			return "", err
		}
		return fmt.Sprintf("set '%s' successfully", jsonPath), nil

	case "delete":
		if jsonPath == "" {
			return "", fmt.Errorf("json_path is required for delete")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if !gjson.ValidBytes(data) {
			return "", fmt.Errorf("file is not valid JSON")
		}
		updated, err := sjson.DeleteBytes(data, jsonPath)
		if err != nil {
			return "", fmt.Errorf("delete failed: %w", err)
		}
		var pretty json.RawMessage = updated
		formatted, err := json.MarshalIndent(pretty, "", "  ")
		if err != nil {
			formatted = updated
		}
		if err := os.WriteFile(path, append(formatted, '\n'), 0644); err != nil {
			return "", err
		}
		return fmt.Sprintf("deleted '%s' successfully", jsonPath), nil

	case "keys":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if !gjson.ValidBytes(data) {
			return "", fmt.Errorf("file is not valid JSON")
		}
		target := string(data)
		if jsonPath != "" {
			r := gjson.Get(target, jsonPath)
			if !r.Exists() {
				return "", fmt.Errorf("path '%s' not found", jsonPath)
			}
			target = r.Raw
		}
		var keys []string
		gjson.Parse(target).ForEach(func(key, _ gjson.Result) bool {
			keys = append(keys, key.String())
			return true
		})
		out, _ := json.Marshal(keys)
		return string(out), nil

	case "validate":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if gjson.ValidBytes(data) {
			return `{"valid":true}`, nil
		}
		return `{"valid":false}`, nil

	case "format":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if !gjson.ValidBytes(data) {
			return "", fmt.Errorf("file is not valid JSON")
		}
		var raw json.RawMessage = data
		formatted, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			return "", fmt.Errorf("format failed: %w", err)
		}
		if err := os.WriteFile(path, append(formatted, '\n'), 0644); err != nil {
			return "", err
		}
		return "formatted successfully", nil

	default:
		return "", fmt.Errorf("unknown json_edit operation: %s", op)
	}
}

// yamlEdit performs YAML file operations on the remote device.
func (e *Executor) yamlEdit(path, op, dotPath string, setValue interface{}) (string, error) {
	switch op {
	case "get":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		var doc interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return "", fmt.Errorf("invalid YAML: %w", err)
		}
		if dotPath == "" {
			return string(data), nil
		}
		val, err := yamlNavigateRemote(doc, dotPath)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(val)
		return string(out), nil

	case "set":
		if dotPath == "" {
			return "", fmt.Errorf("json_path is required for set")
		}
		var root yaml.Node
		data, _ := os.ReadFile(path)
		if len(data) > 0 {
			if err := yaml.Unmarshal(data, &root); err != nil {
				return "", fmt.Errorf("invalid YAML: %w", err)
			}
		} else {
			root.Kind = yaml.DocumentNode
			root.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
		}
		parts := strings.Split(dotPath, ".")
		if err := yamlNodeSetRemote(root.Content[0], parts, setValue); err != nil {
			return "", err
		}
		out, err := yaml.Marshal(&root)
		if err != nil {
			return "", fmt.Errorf("marshal failed: %w", err)
		}
		if err := os.WriteFile(path, out, 0644); err != nil {
			return "", err
		}
		return fmt.Sprintf("set '%s' successfully", dotPath), nil

	case "delete":
		if dotPath == "" {
			return "", fmt.Errorf("json_path is required for delete")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		var root yaml.Node
		if err := yaml.Unmarshal(data, &root); err != nil {
			return "", fmt.Errorf("invalid YAML: %w", err)
		}
		parts := strings.Split(dotPath, ".")
		if err := yamlNodeDeleteRemote(root.Content[0], parts); err != nil {
			return "", err
		}
		out, err := yaml.Marshal(&root)
		if err != nil {
			return "", fmt.Errorf("marshal failed: %w", err)
		}
		if err := os.WriteFile(path, out, 0644); err != nil {
			return "", err
		}
		return fmt.Sprintf("deleted '%s' successfully", dotPath), nil

	case "keys":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		var doc interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return "", fmt.Errorf("invalid YAML: %w", err)
		}
		target := doc
		if dotPath != "" {
			val, err := yamlNavigateRemote(doc, dotPath)
			if err != nil {
				return "", err
			}
			target = val
		}
		m, ok := target.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("value at path is not a mapping")
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		out, _ := json.Marshal(keys)
		return string(out), nil

	case "validate":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		var doc interface{}
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return `{"valid":false,"error":"` + err.Error() + `"}`, nil
		}
		return `{"valid":true}`, nil

	default:
		return "", fmt.Errorf("unknown yaml_edit operation: %s", op)
	}
}

// yamlNavigateRemote traverses a decoded YAML value by dot-path.
func yamlNavigateRemote(doc interface{}, dotPath string) (interface{}, error) {
	parts := strings.Split(dotPath, ".")
	current := doc
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("path '%s' not found", dotPath)
			}
			current = val
		default:
			return nil, fmt.Errorf("path '%s' not found (not a mapping)", dotPath)
		}
	}
	return current, nil
}

// yamlNodeSetRemote sets a value in a YAML node tree.
func yamlNodeSetRemote(node *yaml.Node, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}
	key := parts[0]
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node")
	}
	// Find existing key
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			if len(parts) == 1 {
				var valNode yaml.Node
				valNode.Encode(value)
				*node.Content[i+1] = valNode
				return nil
			}
			return yamlNodeSetRemote(node.Content[i+1], parts[1:], value)
		}
	}
	// Key not found — create intermediate mappings
	if len(parts) == 1 {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"}
		var valNode yaml.Node
		valNode.Encode(value)
		node.Content = append(node.Content, keyNode, &valNode)
		return nil
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"}
	newMapping := &yaml.Node{Kind: yaml.MappingNode}
	node.Content = append(node.Content, keyNode, newMapping)
	return yamlNodeSetRemote(newMapping, parts[1:], value)
}

// yamlNodeDeleteRemote deletes a key from a YAML node tree.
func yamlNodeDeleteRemote(node *yaml.Node, parts []string) error {
	if len(parts) == 0 || node.Kind != yaml.MappingNode {
		return fmt.Errorf("cannot delete: invalid path or node type")
	}
	key := parts[0]
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			if len(parts) == 1 {
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
				return nil
			}
			return yamlNodeDeleteRemote(node.Content[i+1], parts[1:])
		}
	}
	return fmt.Errorf("key '%s' not found", key)
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

// xmlEdit performs XML editing operations on the remote host.
func (e *Executor) xmlEdit(path, op, xpath string, setValue interface{}) (string, error) {
	switch op {
	case "get":
		if xpath == "" {
			return "", fmt.Errorf("xpath is required for get")
		}
		doc := etree.NewDocument()
		if err := doc.ReadFromFile(path); err != nil {
			return "", fmt.Errorf("failed to parse XML: %w", err)
		}
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", fmt.Errorf("no elements found for path '%s'", xpath)
		}
		var results []map[string]interface{}
		for _, el := range elements {
			entry := map[string]interface{}{"tag": el.Tag, "text": strings.TrimSpace(el.Text())}
			if len(el.Attr) > 0 {
				attrs := make(map[string]string)
				for _, a := range el.Attr {
					key := a.Key
					if a.Space != "" {
						key = a.Space + ":" + key
					}
					attrs[key] = a.Value
				}
				entry["attributes"] = attrs
			}
			results = append(results, entry)
		}
		if len(results) == 1 {
			out, _ := json.Marshal(results[0])
			return string(out), nil
		}
		out, _ := json.Marshal(results)
		return string(out), nil

	case "set_text":
		if xpath == "" {
			return "", fmt.Errorf("xpath is required for set_text")
		}
		text := fmt.Sprintf("%v", setValue)
		doc := etree.NewDocument()
		if err := doc.ReadFromFile(path); err != nil {
			return "", fmt.Errorf("failed to parse XML: %w", err)
		}
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", fmt.Errorf("no elements found for path '%s'", xpath)
		}
		for _, el := range elements {
			el.SetText(text)
		}
		doc.Indent(2)
		if err := doc.WriteToFile(path); err != nil {
			return "", fmt.Errorf("failed to write XML: %w", err)
		}
		out, _ := json.Marshal(map[string]interface{}{"updated": len(elements)})
		return string(out), nil

	case "set_attribute":
		if xpath == "" {
			return "", fmt.Errorf("xpath is required for set_attribute")
		}
		attrs, ok := setValue.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("set_value must be {name, value} for set_attribute")
		}
		attrName, _ := attrs["name"].(string)
		if attrName == "" {
			return "", fmt.Errorf("set_value.name is required for set_attribute")
		}
		attrValue := fmt.Sprintf("%v", attrs["value"])
		doc := etree.NewDocument()
		if err := doc.ReadFromFile(path); err != nil {
			return "", fmt.Errorf("failed to parse XML: %w", err)
		}
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", fmt.Errorf("no elements found for path '%s'", xpath)
		}
		for _, el := range elements {
			el.CreateAttr(attrName, attrValue)
		}
		doc.Indent(2)
		if err := doc.WriteToFile(path); err != nil {
			return "", fmt.Errorf("failed to write XML: %w", err)
		}
		out, _ := json.Marshal(map[string]interface{}{"updated": len(elements)})
		return string(out), nil

	case "add_element":
		if xpath == "" {
			return "", fmt.Errorf("xpath is required for add_element (selects parent)")
		}
		spec, ok := setValue.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("set_value must be {tag, text?, attributes?} for add_element")
		}
		tag, _ := spec["tag"].(string)
		if tag == "" {
			return "", fmt.Errorf("set_value.tag is required for add_element")
		}
		doc := etree.NewDocument()
		if err := doc.ReadFromFile(path); err != nil {
			return "", fmt.Errorf("failed to parse XML: %w", err)
		}
		parents := doc.FindElements(xpath)
		if len(parents) == 0 {
			return "", fmt.Errorf("no parent elements found for path '%s'", xpath)
		}
		for _, parent := range parents {
			child := parent.CreateElement(tag)
			if text, ok := spec["text"].(string); ok {
				child.SetText(text)
			}
			if childAttrs, ok := spec["attributes"].(map[string]interface{}); ok {
				for k, v := range childAttrs {
					child.CreateAttr(k, fmt.Sprintf("%v", v))
				}
			}
		}
		doc.Indent(2)
		if err := doc.WriteToFile(path); err != nil {
			return "", fmt.Errorf("failed to write XML: %w", err)
		}
		out, _ := json.Marshal(map[string]interface{}{"added_to": len(parents)})
		return string(out), nil

	case "delete":
		if xpath == "" {
			return "", fmt.Errorf("xpath is required for delete")
		}
		doc := etree.NewDocument()
		if err := doc.ReadFromFile(path); err != nil {
			return "", fmt.Errorf("failed to parse XML: %w", err)
		}
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", fmt.Errorf("no elements found for path '%s'", xpath)
		}
		count := 0
		for _, el := range elements {
			if p := el.Parent(); p != nil {
				p.RemoveChild(el)
				count++
			}
		}
		doc.Indent(2)
		if err := doc.WriteToFile(path); err != nil {
			return "", fmt.Errorf("failed to write XML: %w", err)
		}
		out, _ := json.Marshal(map[string]interface{}{"deleted": count})
		return string(out), nil

	case "validate":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		doc := etree.NewDocument()
		if err := doc.ReadFromBytes(data); err != nil {
			return "", fmt.Errorf("invalid XML: %w", err)
		}
		root := doc.Root()
		if root == nil {
			return "", fmt.Errorf("XML document has no root element")
		}
		out, _ := json.Marshal(map[string]interface{}{"valid": true, "root_tag": root.Tag})
		return string(out), nil

	case "format":
		doc := etree.NewDocument()
		if err := doc.ReadFromFile(path); err != nil {
			return "", fmt.Errorf("failed to parse XML: %w", err)
		}
		doc.Indent(2)
		if err := doc.WriteToFile(path); err != nil {
			return "", fmt.Errorf("failed to write XML: %w", err)
		}
		return `{"formatted":true}`, nil

	default:
		return "", fmt.Errorf("unknown xml_editor operation: %s", op)
	}
}
