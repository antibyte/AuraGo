package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

func looksLikeBinaryFile(path string, data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	contentType := http.DetectContentType(sample)
	switch contentType {
	case "application/json", "application/xml", "image/svg+xml", "application/javascript":
		return false
	}
	if strings.HasPrefix(contentType, "text/") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md", ".json", ".yaml", ".yml", ".xml", ".html", ".htm", ".css", ".js", ".ts", ".tsx", ".jsx", ".go", ".py", ".sh", ".ps1", ".sql", ".csv", ".svg":
		return false
	}
	return !utf8.Valid(sample)
}

func binaryReadResult(path string, size int64) FSResult {
	guidance := "Use a specialized tool instead of read_file."
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		guidance = "This looks like an image. Use analyze_image to inspect screenshots or photos."
	case ".pdf":
		guidance = "This looks like a PDF. Use pdf_extractor or pdf_operations instead."
	}
	return FSResult{
		Status:  "error",
		Message: fmt.Sprintf("'%s' appears to be a binary file and cannot be returned as text. %s", path, guidance),
		Data: map[string]interface{}{
			"path": path,
			"size": size,
			"kind": "binary",
		},
	}
}

func normalizeFilesystemOperation(operation string) string {
	switch strings.TrimSpace(operation) {
	case "read":
		return "read_file"
	case "write":
		return "write_file"
	default:
		return operation
	}
}

func filesystemUnknownOperationMessage(operation string) string {
	switch strings.TrimSpace(operation) {
	case "read":
		return "Unknown filesystem operation: 'read'. Use 'read_file' to read file contents. Valid: list_dir, create_dir, delete, read_file, write_file, copy, move, stat, copy_batch, move_batch, delete_batch, create_dir_batch"
	case "write":
		return "Unknown filesystem operation: 'write'. Use 'write_file' to create or overwrite a file. Valid: list_dir, create_dir, delete, read_file, write_file, copy, move, stat, copy_batch, move_batch, delete_batch, create_dir_batch"
	default:
		return fmt.Sprintf("Unknown filesystem operation: '%s'. Valid: list_dir, create_dir, delete, read_file, write_file, copy, move, stat, copy_batch, move_batch, delete_batch, create_dir_batch", operation)
	}
}

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

type filesystemBatchItem struct {
	FilePath    string
	Destination string
}

func filesystemRoots(workspaceDir string) (string, string) {
	absWorkdir, err := filepath.EvalSymlinks(workspaceDir)
	if err != nil {
		absWorkdir, err = filepath.Abs(workspaceDir)
		if err != nil {
			absWorkdir = workspaceDir
		}
	}
	projectRoot := detectFilesystemProjectRoot(absWorkdir)
	return absWorkdir, projectRoot
}

func detectFilesystemProjectRoot(absWorkdir string) string {
	current := filepath.Clean(absWorkdir)
	for {
		if filepath.Base(current) == "agent_workspace" {
			parent := filepath.Dir(current)
			if parent != "" && parent != current {
				return parent
			}
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	if filepath.Base(absWorkdir) == "workdir" {
		parent := filepath.Dir(absWorkdir)
		if parent != "" && parent != absWorkdir && parent != current {
			return parent
		}
	}

	return absWorkdir
}

func filesystemProjectRootHint(workspaceRoot, projectRoot string) string {
	if workspaceRoot == projectRoot {
		return "Paths are confined to the workspace root."
	}
	return "Use ../../ to reach project-root files from agent_workspace/workdir."
}

func filesystemErrorData(workspaceDir, requestedPath, resolvedPath string) map[string]interface{} {
	workspaceRoot, projectRoot := filesystemRoots(workspaceDir)
	data := map[string]interface{}{
		"requested_path":    requestedPath,
		"workspace_root":    workspaceRoot,
		"project_root":      projectRoot,
		"project_root_hint": filesystemProjectRootHint(workspaceRoot, projectRoot),
	}
	if resolvedPath != "" {
		data["resolved_path"] = resolvedPath
	}
	return data
}

func filesystemErrorResult(message, code, workspaceDir, requestedPath, resolvedPath string) FSResult {
	data := filesystemErrorData(workspaceDir, requestedPath, resolvedPath)
	if code != "" {
		data["error_code"] = code
	}
	return FSResult{
		Status:  "error",
		Message: message,
		Data:    data,
	}
}

func isReadOnlyFilesystemError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "read-only file system") || strings.Contains(lower, "readonly file system")
}

func filesystemWritableAlternatives() []string {
	return []string{
		".",
		"./tmp",
		"../../data",
	}
}

func filesystemWriteErrorResult(op, workspaceDir, requestedPath, resolvedPath string, err error) FSResult {
	if isReadOnlyFilesystemError(err) {
		data := filesystemErrorData(workspaceDir, requestedPath, resolvedPath)
		data["error_code"] = "read_only_filesystem"
		data["operation"] = op
		data["suggested_alternatives"] = filesystemWritableAlternatives()
		return FSResult{
			Status:  "error",
			Message: fmt.Sprintf("Failed to %s because the target location is mounted read-only. Try a writable path inside workdir or ../../data.", op),
			Data:    data,
		}
	}
	return filesystemErrorResult(fmt.Sprintf("Failed to %s: %v", op, err), "io_error", workspaceDir, requestedPath, resolvedPath)
}

func filesystemResolveErrorResult(workspaceDir, requestedPath string, err error) FSResult {
	workspaceRoot, projectRoot := filesystemRoots(workspaceDir)
	return filesystemErrorResult(
		fmt.Sprintf("%s %s", err.Error(), filesystemProjectRootHint(workspaceRoot, projectRoot)),
		"path_resolution_error",
		workspaceDir,
		requestedPath,
		"",
	)
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

	projectRoot := detectFilesystemProjectRoot(absWorkdir)

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
	absPath, err := secureResolveFinalPath(clean)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path %q: %w", userPath, err)
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

	return absPath, nil
}

func secureResolveFinalPath(clean string) (string, error) {
	current := clean
	var missing []string
	for {
		info, err := os.Lstat(current)
		if err == nil {
			resolved := current
			if info.Mode()&os.ModeSymlink != 0 {
				resolved, err = filepath.EvalSymlinks(current)
				if err != nil {
					return "", err
				}
			} else if eval, evalErr := filepath.EvalSymlinks(current); evalErr == nil {
				resolved = eval
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			resolved := current
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func filesystemBatchItemString(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

func filesystemCopyFile(srcResolved, dstResolved string) error {
	srcInfo, err := os.Stat(srcResolved)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("directory copy is not supported")
	}
	if err := os.MkdirAll(filepath.Dir(dstResolved), 0o755); err != nil {
		return err
	}
	srcFile, err := os.Open(srcResolved)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstResolved)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return os.Chmod(dstResolved, srcInfo.Mode())
}

func filesystemBatchResult(operation string, items []map[string]interface{}, workspaceDir string) FSResult {
	if len(items) == 0 {
		return FSResult{Status: "error", Message: "'items' is required for batch filesystem operations"}
	}

	results := make([]map[string]interface{}, 0, len(items))
	succeeded := 0
	failed := 0

	for idx, rawItem := range items {
		path := filesystemBatchItemString(rawItem, "file_path", "path")
		destination := filesystemBatchItemString(rawItem, "destination", "dest")
		itemResult := map[string]interface{}{
			"index":     idx,
			"file_path": path,
		}
		if destination != "" {
			itemResult["destination"] = destination
		}

		var single FSResult
		switch operation {
		case "copy_batch":
			if path == "" || destination == "" {
				single = FSResult{Status: "error", Message: "Each copy_batch item requires 'file_path' and 'destination'"}
			} else {
				single = executeFilesystemResult("copy", path, destination, "", nil, workspaceDir)
			}
		case "move_batch":
			if path == "" || destination == "" {
				single = FSResult{Status: "error", Message: "Each move_batch item requires 'file_path' and 'destination'"}
			} else {
				single = executeFilesystemResult("move", path, destination, "", nil, workspaceDir)
			}
		case "delete_batch":
			if path == "" {
				single = FSResult{Status: "error", Message: "Each delete_batch item requires 'file_path'"}
			} else {
				single = executeFilesystemResult("delete", path, "", "", nil, workspaceDir)
			}
		case "create_dir_batch":
			if path == "" {
				single = FSResult{Status: "error", Message: "Each create_dir_batch item requires 'file_path'"}
			} else {
				single = executeFilesystemResult("create_dir", path, "", "", nil, workspaceDir)
			}
		default:
			single = FSResult{Status: "error", Message: fmt.Sprintf("Unsupported batch operation: %s", operation)}
		}

		itemResult["status"] = single.Status
		itemResult["message"] = single.Message
		if data, ok := single.Data.(map[string]interface{}); ok {
			if errorCode, ok := data["error_code"]; ok {
				itemResult["error_code"] = errorCode
			}
			itemResult["details"] = data
		}

		if single.Status == "success" {
			succeeded++
		} else {
			failed++
		}
		results = append(results, itemResult)
	}

	status := "success"
	switch {
	case succeeded == 0:
		status = "error"
	case failed > 0:
		status = "partial"
	}

	return FSResult{
		Status:  status,
		Message: fmt.Sprintf("%s processed %d items (%d succeeded, %d failed)", operation, len(items), succeeded, failed),
		Data: map[string]interface{}{
			"summary": map[string]int{
				"requested": len(items),
				"succeeded": succeeded,
				"failed":    failed,
			},
			"results": results,
		},
	}
}

func executeFilesystemResult(operation, path, destination, content string, items []map[string]interface{}, workspaceDir string) FSResult {
	originalOperation := operation
	operation = normalizeFilesystemOperation(operation)

	switch operation {
	case "list_dir":
		resolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return filesystemResolveErrorResult(workspaceDir, path, err)
		}
		if path == "" || path == "." {
			resolved = workspaceDir
		}
		entries, err := os.ReadDir(resolved)
		if err != nil {
			return filesystemErrorResult(fmt.Sprintf("Failed to list directory: %v", err), "io_error", workspaceDir, path, resolved)
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
		return FSResult{Status: "success", Message: fmt.Sprintf("Listed %d entries", len(items)), Data: items}

	case "create_dir":
		if path == "" {
			return FSResult{Status: "error", Message: "'path' is required for create_dir"}
		}
		resolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return filesystemResolveErrorResult(workspaceDir, path, err)
		}
		if err := os.MkdirAll(resolved, 0755); err != nil {
			return filesystemWriteErrorResult("create directory", workspaceDir, path, resolved, err)
		}
		return FSResult{Status: "success", Message: fmt.Sprintf("Directory created: %s", path)}

	case "delete":
		if path == "" {
			return FSResult{Status: "error", Message: "'path' is required for delete"}
		}
		resolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return filesystemResolveErrorResult(workspaceDir, path, err)
		}
		if err := os.RemoveAll(resolved); err != nil {
			return filesystemWriteErrorResult("delete path", workspaceDir, path, resolved, err)
		}
		return FSResult{Status: "success", Message: fmt.Sprintf("Deleted: %s", path)}

	case "read_file":
		if path == "" {
			return FSResult{Status: "error", Message: "'path' is required for read_file"}
		}
		resolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return filesystemResolveErrorResult(workspaceDir, path, err)
		}

		// Check file size before reading to avoid OOM
		info, err := os.Stat(resolved)
		if err != nil {
			return filesystemErrorResult(fmt.Sprintf("Failed to stat file: %v", err), "io_error", workspaceDir, path, resolved)
		}

		// Cap file read at 32KB + a little padding for UTF-8.
		maxRead := 32*1024 + 2048
		if info.Size() > int64(maxRead) {
			// Read only the first maxRead bytes
			f, err := os.Open(resolved)
			if err != nil {
				return filesystemErrorResult(fmt.Sprintf("Failed to read file: %v", err), "io_error", workspaceDir, path, resolved)
			}
			defer f.Close()

			data := make([]byte, maxRead)
			n, err := io.ReadFull(f, data)
			if err != nil && err != io.ErrUnexpectedEOF {
				return filesystemErrorResult(fmt.Sprintf("Failed to read file: %v", err), "io_error", workspaceDir, path, resolved)
			}
			if looksLikeBinaryFile(path, data[:n]) {
				return binaryReadResult(path, info.Size())
			}
			text := string(data[:n])
			return FSResult{
				Status: "success",
				Message: fmt.Sprintf(
					"Read %d bytes (truncated, file has %d bytes total). For larger text files use smart_file_read (analyze/sample/summarize) or file_reader_advanced (head/tail/read_lines/search_context).",
					n, info.Size(),
				),
				Data: text + "\n\n[...truncated — use smart_file_read or file_reader_advanced for targeted follow-up reads...]",
			}
		}

		// Small file, read entirely
		data, err := os.ReadFile(resolved)
		if err != nil {
			return filesystemErrorResult(fmt.Sprintf("Failed to read file: %v", err), "io_error", workspaceDir, path, resolved)
		}
		if looksLikeBinaryFile(path, data) {
			return binaryReadResult(path, info.Size())
		}
		return FSResult{Status: "success", Message: fmt.Sprintf("Read %d bytes", len(data)), Data: string(data)}

	case "write_file":
		if path == "" || content == "" {
			return FSResult{Status: "error", Message: "'path' and 'content' are required for write_file"}
		}
		resolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return filesystemResolveErrorResult(workspaceDir, path, err)
		}
		// Ensure parent directories exist
		if err := os.MkdirAll(filepath.Dir(resolved), 0755); err != nil {
			return filesystemWriteErrorResult("create parent directory", workspaceDir, path, filepath.Dir(resolved), err)
		}
		if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
			return filesystemWriteErrorResult("write file", workspaceDir, path, resolved, err)
		}
		return FSResult{Status: "success", Message: fmt.Sprintf("Wrote %d bytes to %s", len(content), path)}

	case "copy":
		if path == "" || destination == "" {
			return FSResult{Status: "error", Message: "'path' and 'destination' are required for copy"}
		}
		srcResolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return filesystemResolveErrorResult(workspaceDir, path, err)
		}
		dstResolved, err := secureResolve(workspaceDir, destination)
		if err != nil {
			return filesystemResolveErrorResult(workspaceDir, destination, err)
		}
		if err := filesystemCopyFile(srcResolved, dstResolved); err != nil {
			return filesystemWriteErrorResult("copy path", workspaceDir, destination, dstResolved, err)
		}
		return FSResult{Status: "success", Message: fmt.Sprintf("Copied %s → %s", path, destination)}

	case "move":
		if path == "" || destination == "" {
			return FSResult{Status: "error", Message: "'path' and 'destination' are required for move"}
		}
		srcResolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return filesystemResolveErrorResult(workspaceDir, path, err)
		}
		dstResolved, err := secureResolve(workspaceDir, destination)
		if err != nil {
			return filesystemResolveErrorResult(workspaceDir, destination, err)
		}
		if err := os.MkdirAll(filepath.Dir(dstResolved), 0755); err != nil {
			return filesystemWriteErrorResult("create parent directory", workspaceDir, destination, filepath.Dir(dstResolved), err)
		}
		if err := os.Rename(srcResolved, dstResolved); err != nil {
			return filesystemWriteErrorResult("move path", workspaceDir, path, srcResolved, err)
		}
		return FSResult{Status: "success", Message: fmt.Sprintf("Moved %s → %s", path, destination)}

	case "copy_batch", "move_batch", "delete_batch", "create_dir_batch":
		return filesystemBatchResult(operation, items, workspaceDir)

	case "stat":
		if path == "" {
			return FSResult{Status: "error", Message: "'path' is required for stat"}
		}
		resolved, err := secureResolve(workspaceDir, path)
		if err != nil {
			return filesystemResolveErrorResult(workspaceDir, path, err)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return filesystemErrorResult(fmt.Sprintf("Failed to stat: %v", err), "io_error", workspaceDir, path, resolved)
		}
		return FSResult{Status: "success", Data: FileInfoEntry{
			Name:    info.Name(),
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
		}}

	default:
		return FSResult{Status: "error", Message: filesystemUnknownOperationMessage(originalOperation)}
	}
}

// ExecuteFilesystem handles all filesystem operations, sandboxed to workspaceDir.
func ExecuteFilesystem(operation, path, destination, content string, items []map[string]interface{}, workspaceDir string) string {
	result := executeFilesystemResult(operation, path, destination, content, items, workspaceDir)
	b, _ := json.Marshal(result)
	return string(b)
}
