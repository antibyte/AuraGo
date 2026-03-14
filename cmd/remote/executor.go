package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aurago/internal/remote"
)

// Executor handles command execution on the remote device.
type Executor struct {
	logger *slog.Logger
}

// NewExecutor creates a new command executor.
func NewExecutor(logger *slog.Logger) *Executor {
	return &Executor{logger: logger}
}

// Execute runs a command and returns the result.
func (e *Executor) Execute(cmd remote.CommandPayload, readOnly bool, allowedPaths []string) remote.ResultPayload {
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
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			result.Status = "error"
			result.Error = fmt.Sprintf("invalid base64 content: %v", err)
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
		output, err := e.shellExec(command, workDir, timeout)
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
		return nil // no restrictions
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
