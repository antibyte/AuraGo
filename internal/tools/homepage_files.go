package tools

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// ─── File Operations ──────────────────────────────────────────────────────

// HomepageListFiles lists files in a directory inside the container.
func HomepageListFiles(cfg HomepageConfig, path string, logger *slog.Logger) string {
	if path == "" {
		path = "."
	}
	if path != "." {
		if err := validateHomepageRelativePathArg(path, "path"); err != nil {
			return errJSON("%v", err)
		}
	}
	logger.Info("[Homepage] ListFiles", "path", path)

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		var base string
		if path == "." {
			base = cfg.WorkspacePath
		} else {
			var err error
			base, err = resolveHomepagePath(cfg.WorkspacePath, path)
			if err != nil {
				return errJSON("%v", err)
			}
		}
		var files []string
		_ = filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			// Return paths relative to WorkspacePath so they are usable with read_file / write_file
			rel, _ := filepath.Rel(cfg.WorkspacePath, p)
			slashRel := filepath.ToSlash(rel)
			if strings.Contains(slashRel, "/node_modules") || strings.Contains(slashRel, "/.next") || strings.Contains(slashRel, "/.git") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			// Limit depth to 3 segments below WorkspacePath
			parts := strings.Split(slashRel, "/")
			if len(parts) > 4 {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if len(files) < 200 {
				files = append(files, slashRel)
			}
			return nil
		})
		out, _ := json.Marshal(map[string]interface{}{"status": "ok", "mode": "local", "workspace": cfg.WorkspacePath, "files": files})
		return string(out)
	}

	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	return DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("find /workspace/%s -maxdepth 2 -not -path '*/node_modules/*' -not -path '*/.next/*' -not -path '*/.git/*' | head -200", path), "")
}

// HomepageReadFile reads a file from the container.
func HomepageReadFile(cfg HomepageConfig, path string, logger *slog.Logger) string {
	if err := validateHomepageRelativePathArg(path, "path"); err != nil {
		return errJSON("%v", err)
	}
	logger.Info("[Homepage] ReadFile", "path", path)

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		fullPath, err := resolveHomepagePath(cfg.WorkspacePath, path)
		if err != nil {
			return errJSON("%v", err)
		}
		data, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			return errJSON("Failed to read file: %v", readErr)
		}
		out, _ := json.Marshal(map[string]interface{}{"status": "ok", "content": string(data)})
		return string(out)
	}

	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	return DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cat /workspace/%s", path), "")
}

// maxHomepageWriteFileSize is the maximum content size for HomepageWriteFile (2 MB).
const maxHomepageWriteFileSize = 2 * 1024 * 1024

// HomepageWriteFile writes content to a file inside the container.
func HomepageWriteFile(cfg HomepageConfig, path, content string, logger *slog.Logger) string {
	if err := validateHomepageRelativePathArg(path, "path"); err != nil {
		return errJSON("%v", err)
	}
	if len(content) > maxHomepageWriteFileSize {
		return errJSON("content too large: %d bytes exceeds maximum of %d bytes", len(content), maxHomepageWriteFileSize)
	}
	logger.Info("[Homepage] WriteFile", "path", path, "size", len(content))

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		fullPath, err := resolveHomepagePath(cfg.WorkspacePath, path)
		if err != nil {
			return errJSON("%v", err)
		}
		if mkErr := os.MkdirAll(filepath.Dir(fullPath), 0755); mkErr != nil {
			return errJSON("Failed to create directory: %v", mkErr)
		}
		if writeErr := os.WriteFile(fullPath, []byte(content), 0644); writeErr != nil {
			return errJSON("Failed to write file: %v", writeErr)
		}
		out, _ := json.Marshal(map[string]interface{}{"status": "ok", "path": path, "size": len(content)})
		return string(out)
	}

	// Use base64 to safely pass content through shell
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	cmd := fmt.Sprintf("mkdir -p /workspace/%s && echo '%s' | base64 -d > /workspace/%s", dir, encoded, path)
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}

// HomepageEditFile performs precise file editing inside the container (or locally).
// It reads the file, applies the edit in Go, then writes back.
func HomepageEditFile(cfg HomepageConfig, path, operation, old, new_, marker, content string, startLine, endLine int, logger *slog.Logger) string {
	if err := validateHomepageRelativePathArg(path, "path"); err != nil {
		return errJSON("%v", err)
	}
	logger.Info("[Homepage] EditFile", "path", path, "op", operation)

	if !checkDockerAvailable(cfg.DockerHost) {
		// Local fallback: use file_editor directly on workspace path
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		fullPath, err := resolveHomepagePath(cfg.WorkspacePath, path)
		if err != nil {
			return errJSON("%v", err)
		}
		return ExecuteFileEditor(operation, path, old, new_, marker, content, startLine, endLine, 0, filepath.Dir(fullPath))
	}

	// Docker: read file from container, apply edit in Go, write back
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	readResult := DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cat /workspace/%s", path), "")

	// DockerExec returns JSON; try to detect errors
	var readResp map[string]interface{}
	if err := json.Unmarshal([]byte(readResult), &readResp); err == nil {
		if status, ok := readResp["status"].(string); ok && status == "error" {
			return readResult
		}
		// If there's an "output" field, use that
		if output, ok := readResp["output"].(string); ok {
			readResult = output
		}
	}

	// Apply the edit operation on the content
	edited, editErr := applyHomepageEdit(readResult, operation, old, new_, marker, content, startLine, endLine)
	if editErr != "" {
		return editErr
	}

	// Write back via base64
	encoded := base64.StdEncoding.EncodeToString([]byte(edited))
	dir := filepath.Dir(path)
	writeCmd := fmt.Sprintf("mkdir -p /workspace/%s && echo '%s' | base64 -d > /workspace/%s", dir, encoded, path)
	return DockerExec(dockerCfg, homepageContainerName, writeCmd, "")
}

// applyHomepageEdit applies an editing operation to file content in memory.
// Returns the edited content and an error JSON string (empty if success).
func applyHomepageEdit(text, operation, old, new_, marker, content string, startLine, endLine int) (string, string) {
	switch operation {
	case "str_replace":
		if old == "" {
			return "", errJSON("'old' text is required for str_replace")
		}
		count := strings.Count(text, old)
		if count == 0 {
			return "", errJSON("'old' text not found in file")
		}
		if count > 1 {
			return "", errJSON("'old' text found %d times — must be unique for str_replace", count)
		}
		return strings.Replace(text, old, new_, 1), ""

	case "str_replace_all":
		if old == "" {
			return "", errJSON("'old' text is required")
		}
		if !strings.Contains(text, old) {
			return "", errJSON("'old' text not found in file")
		}
		return strings.ReplaceAll(text, old, new_), ""

	case "insert_after", "insert_before":
		if marker == "" {
			return "", errJSON("'marker' is required")
		}
		if content == "" {
			return "", errJSON("'content' is required")
		}
		lines := strings.Split(text, "\n")
		idx := -1
		for i, line := range lines {
			if strings.Contains(line, marker) {
				if idx >= 0 {
					return "", errJSON("marker found on multiple lines")
				}
				idx = i
			}
		}
		if idx < 0 {
			return "", errJSON("marker not found")
		}
		insertLines := strings.Split(content, "\n")
		insertIdx := idx
		if operation == "insert_after" {
			insertIdx = idx + 1
		}
		newLines := make([]string, 0, len(lines)+len(insertLines))
		newLines = append(newLines, lines[:insertIdx]...)
		newLines = append(newLines, insertLines...)
		newLines = append(newLines, lines[insertIdx:]...)
		return strings.Join(newLines, "\n"), ""

	case "append":
		if content == "" {
			return "", errJSON("'content' is required")
		}
		if len(text) > 0 && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return text + content, ""

	case "prepend":
		if content == "" {
			return "", errJSON("'content' is required")
		}
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		return content + text, ""

	case "delete_lines":
		if startLine < 1 {
			return "", errJSON("start_line must be >= 1")
		}
		if endLine < startLine {
			return "", errJSON("end_line must be >= start_line")
		}
		lines := strings.Split(text, "\n")
		if startLine > len(lines) {
			return "", errJSON("start_line exceeds file length")
		}
		if endLine > len(lines) {
			endLine = len(lines)
		}
		newLines := make([]string, 0, len(lines)-(endLine-startLine+1))
		newLines = append(newLines, lines[:startLine-1]...)
		newLines = append(newLines, lines[endLine:]...)
		return strings.Join(newLines, "\n"), ""

	default:
		return "", errJSON("unknown edit operation: %s", operation)
	}
}
