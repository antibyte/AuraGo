package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FSResult is the JSON response returned to the LLM.
type FSResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// FileInfo represents a single directory entry for listing.
type FileInfoEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modified"`
}

// secureResolve resolves a path relative to the workspace and ensures it stays within project bounds.
func secureResolve(workspaceDir, userPath string) (string, error) {
	// Resolve symlinks in workspaceDir first
	absWorkdir, err := filepath.EvalSymlinks(workspaceDir)
	if err != nil {
		absWorkdir, err = filepath.Abs(workspaceDir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve workspace path: %w", err)
		}
	}

	// Allow escaping to project root (2 levels up from workspace/workdir)
	projectRoot := filepath.Dir(filepath.Dir(absWorkdir))

	// Normalize: strip workspace-dir prefix if the LLM passed a project-root-relative path.
	// Example: workspaceDir = ".../agent_workspace/workdir", userPath = "agent_workspace/workdir/file.txt"
	// → the path is duplicated; strip the prefix so we resolve to the correct location.
	if wsRelFromRoot, relErr := filepath.Rel(projectRoot, absWorkdir); relErr == nil {
		wsRelSlash := filepath.ToSlash(wsRelFromRoot)
		userSlash := filepath.ToSlash(filepath.Clean(userPath))
		if strings.HasPrefix(userSlash, wsRelSlash+"/") {
			userPath = filepath.FromSlash(strings.TrimPrefix(userSlash, wsRelSlash+"/"))
		} else if userSlash == wsRelSlash {
			userPath = "."
		}
	}

	// Clean the user path
	full := filepath.Join(absWorkdir, userPath)
	clean := filepath.Clean(full)

	// Try to resolve symlinks in the target path
	absPath, err := filepath.EvalSymlinks(clean)
	if err != nil {
		// Path may not exist yet, use the clean path
		absPath = clean
	}

	// Use filepath.Rel for proper path comparison
	rel, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		return "", fmt.Errorf("path '%s' escapes the project root", userPath)
	}
	// Check if the relative path starts with ".." which means it's escaping
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path '%s' escapes the project root", userPath)
	}

	return clean, nil
}

// ExecuteFilesystem handles all filesystem operations, sandboxed to workspaceDir.
func ExecuteFilesystem(operation, path, destination, content string, workspaceDir string) string {
	encode := func(r FSResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	switch operation {
	case "list_dir":
		resolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return encode(FSResult{Status: "error", Message: err.Error()})
		}
		if path == "" || path == "." {
			resolved = workspaceDir
		}
		entries, err := os.ReadDir(resolved)
		if err != nil {
			return encode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to list directory: %v", err)})
		}
		var items []FileInfoEntry
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				// Skip entries we can't stat
				continue
			}
			mod := ""
			size := int64(0)
			if info != nil {
				mod = info.ModTime().Format(time.RFC3339)
				size = info.Size()
			}
			items = append(items, FileInfoEntry{
				Name:    e.Name(),
				IsDir:   e.IsDir(),
				Size:    size,
				ModTime: mod,
			})
		}
		return encode(FSResult{Status: "success", Message: fmt.Sprintf("Listed %d entries", len(items)), Data: items})

	case "create_dir":
		if path == "" {
			return encode(FSResult{Status: "error", Message: "'path' is required for create_dir"})
		}
		resolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return encode(FSResult{Status: "error", Message: err.Error()})
		}
		if err := os.MkdirAll(resolved, 0755); err != nil {
			return encode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to create directory: %v", err)})
		}
		return encode(FSResult{Status: "success", Message: fmt.Sprintf("Directory created: %s", path)})

	case "delete":
		if path == "" {
			return encode(FSResult{Status: "error", Message: "'path' is required for delete"})
		}
		resolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return encode(FSResult{Status: "error", Message: err.Error()})
		}
		if err := os.RemoveAll(resolved); err != nil {
			return encode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to delete: %v", err)})
		}
		return encode(FSResult{Status: "success", Message: fmt.Sprintf("Deleted: %s", path)})

	case "read_file":
		if path == "" {
			return encode(FSResult{Status: "error", Message: "'path' is required for read_file"})
		}
		resolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return encode(FSResult{Status: "error", Message: err.Error()})
		}

		// Check file size before reading to avoid OOM
		info, err := os.Stat(resolved)
		if err != nil {
			return encode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to stat file: %v", err)})
		}

		// Cap file read at 8KB + some padding for UTF-8
		maxRead := 8192 + 1024
		if info.Size() > int64(maxRead) {
			// Read only the first maxRead bytes
			f, err := os.Open(resolved)
			if err != nil {
				return encode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
			}
			defer f.Close()

			data := make([]byte, maxRead)
			n, err := io.ReadFull(f, data)
			if err != nil && err != io.ErrUnexpectedEOF {
				return encode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
			}
			text := string(data[:n])
			return encode(FSResult{Status: "success", Message: fmt.Sprintf("Read %d bytes (truncated, file has %d bytes total)", n, info.Size()), Data: text + "\n\n[...truncated]"})
		}

		// Small file, read entirely
		data, err := os.ReadFile(resolved)
		if err != nil {
			return encode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
		}
		return encode(FSResult{Status: "success", Message: fmt.Sprintf("Read %d bytes", len(data)), Data: string(data)})

	case "write_file":
		if path == "" || content == "" {
			return encode(FSResult{Status: "error", Message: "'path' and 'content' are required for write_file"})
		}
		resolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return encode(FSResult{Status: "error", Message: err.Error()})
		}
		// Ensure parent directories exist
		if err := os.MkdirAll(filepath.Dir(resolved), 0755); err != nil {
			return encode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to create parent dir: %v", err)})
		}
		if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
			return encode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
		}
		return encode(FSResult{Status: "success", Message: fmt.Sprintf("Wrote %d bytes to %s", len(content), path)})

	case "move":
		if path == "" || destination == "" {
			return encode(FSResult{Status: "error", Message: "'path' and 'destination' are required for move"})
		}
		srcResolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return encode(FSResult{Status: "error", Message: err.Error()})
		}
		dstResolved, err := secureResolve(workspaceDir, destination)
		if err != nil {
			return encode(FSResult{Status: "error", Message: err.Error()})
		}
		if err := os.Rename(srcResolved, dstResolved); err != nil {
			return encode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to move: %v", err)})
		}
		return encode(FSResult{Status: "success", Message: fmt.Sprintf("Moved %s → %s", path, destination)})

	case "stat":
		if path == "" {
			return encode(FSResult{Status: "error", Message: "'path' is required for stat"})
		}
		resolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return encode(FSResult{Status: "error", Message: err.Error()})
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return encode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to stat: %v", err)})
		}
		return encode(FSResult{Status: "success", Data: FileInfoEntry{
			Name:    info.Name(),
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
		}})

	default:
		return encode(FSResult{Status: "error", Message: fmt.Sprintf("Unknown filesystem operation: '%s'. Valid: list_dir, create_dir, delete, read_file, write_file, move, stat", operation)})
	}
}
