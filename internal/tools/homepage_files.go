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
		return homepageListFilesLocal(cfg, path)
	}

	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Warm up the kernel directory entry cache inside the container before listing.
	// When files are written to the bind-mounted workspace by the host process, the
	// container's view of that directory can occasionally lag until an inode lookup is
	// forced — stat on the target path triggers that lookup reliably.
	DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("stat /workspace/%s > /dev/null 2>&1 || true", path), "")
	return DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("find /workspace/%s -maxdepth 2 -not -path '*/node_modules/*' -not -path '*/.next/*' -not -path '*/.git/*' | head -200", path), "")
}

// homepageListFilesLocal performs the local (non-Docker) directory listing.
// It checks cfg.WorkspacePath first, then falls back to cfg.AgentWorkspaceDir when
// the two directories differ — this covers the case where files were written by the
// filesystem tool (which operates on AgentWorkspaceDir) while the homepage workspace
// is configured separately.
func homepageListFilesLocal(cfg HomepageConfig, path string) string {
	scanDirs := homepageResolveScanDirs(cfg, path)
	if len(scanDirs) == 0 {
		return homepageWorkspacePathNotConfiguredJSON()
	}

	seen := make(map[string]struct{})
	var files []string
	primaryWorkspace := scanDirs[0].workspace

	for _, sd := range scanDirs {
		_ = filepath.Walk(sd.base, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(sd.workspace, p)
			slashRel := filepath.ToSlash(rel)
			if strings.Contains(slashRel, "/node_modules") || strings.Contains(slashRel, "/.next") || strings.Contains(slashRel, "/.git") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			// Limit depth to 3 segments below workspace root
			parts := strings.Split(slashRel, "/")
			if len(parts) > 4 {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if _, dup := seen[slashRel]; !dup && len(files) < 200 {
				seen[slashRel] = struct{}{}
				files = append(files, slashRel)
			}
			return nil
		})
	}
	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "mode": "local", "workspace": primaryWorkspace, "files": files})
	return string(out)
}

type homepageScanDir struct {
	base      string // absolute path to scan
	workspace string // root to use for relative-path computation
}

// homepageResolveScanDirs returns the ordered list of base directories to scan for
// local (non-Docker) homepage file operations. cfg.WorkspacePath is primary; when it
// differs from cfg.AgentWorkspaceDir (the agent's own workdir where filesystem tool
// writes files), the latter is appended so that files created by filesystem write_file
// are also visible through homepage tools.
func homepageResolveScanDirs(cfg HomepageConfig, subPath string) []homepageScanDir {
	var dirs []homepageScanDir

	addDir := func(workspace string) {
		if workspace == "" {
			return
		}
		var base string
		if subPath == "." || subPath == "" {
			base = workspace
		} else {
			resolvedBase, err := resolveHomepagePath(workspace, subPath)
			if err != nil {
				return
			}
			base = resolvedBase
		}
		// Only add if the directory exists
		if _, err := os.Stat(base); err != nil {
			return
		}
		dirs = append(dirs, homepageScanDir{base: base, workspace: workspace})
	}

	addDir(cfg.WorkspacePath)
	// Add agent workspace dir only when it is set and is a different path from WorkspacePath.
	if cfg.AgentWorkspaceDir != "" {
		abs1, _ := filepath.Abs(cfg.WorkspacePath)
		abs2, _ := filepath.Abs(cfg.AgentWorkspaceDir)
		if abs1 != abs2 {
			addDir(cfg.AgentWorkspaceDir)
		}
	}
	return dirs
}

// HomepageReadFile reads a file from the container.
func HomepageReadFile(cfg HomepageConfig, path string, logger *slog.Logger) string {
	if err := validateHomepageRelativePathArg(path, "path"); err != nil {
		return errJSON("%v", err)
	}
	logger.Info("[Homepage] ReadFile", "path", path)

	if !checkDockerAvailable(cfg.DockerHost) {
		return homepageReadFileLocal(cfg, path)
	}

	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	return DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cat /workspace/%s", path), "")
}

// homepageReadFileLocal reads a file from the host filesystem in the local (non-Docker)
// fallback mode. It checks cfg.WorkspacePath first, then falls back to cfg.AgentWorkspaceDir
// when the two directories differ — covering the case where the file was written by the
// filesystem tool (which operates on AgentWorkspaceDir).
func homepageReadFileLocal(cfg HomepageConfig, path string) string {
	// Try WorkspacePath first
	if cfg.WorkspacePath != "" {
		fullPath, err := resolveHomepagePath(cfg.WorkspacePath, path)
		if err == nil {
			if data, readErr := os.ReadFile(fullPath); readErr == nil {
				out, _ := json.Marshal(map[string]interface{}{"status": "ok", "content": string(data)})
				return string(out)
			}
		}
	}
	// Fallback: try AgentWorkspaceDir when it differs from WorkspacePath
	if cfg.AgentWorkspaceDir != "" {
		abs1, _ := filepath.Abs(cfg.WorkspacePath)
		abs2, _ := filepath.Abs(cfg.AgentWorkspaceDir)
		if abs1 != abs2 {
			fullPath, err := resolveHomepagePath(cfg.AgentWorkspaceDir, path)
			if err == nil {
				if data, readErr := os.ReadFile(fullPath); readErr == nil {
					out, _ := json.Marshal(map[string]interface{}{"status": "ok", "content": string(data), "source": "agent_workspace"})
					return string(out)
				}
			}
		}
	}
	if cfg.WorkspacePath == "" && cfg.AgentWorkspaceDir == "" {
		return homepageWorkspacePathNotConfiguredJSON()
	}
	return errJSON("File not found: %s", path)
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
	result := DockerExec(dockerCfg, homepageContainerName, cmd, "")
	// Check for DockerExec errors via JSON status field.
	var execResult map[string]interface{}
	if err := json.Unmarshal([]byte(result), &execResult); err == nil {
		if status, ok := execResult["status"].(string); ok && status == "error" {
			errMsg, _ := execResult["error"].(string)
			return errJSON("write file failed for %s: %s", path, errMsg)
		}
	}
	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "path": path, "size": len(content)})
	return string(out)
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

	// Write back via base64 and verify success.
	encoded := base64.StdEncoding.EncodeToString([]byte(edited))
	dir := filepath.Dir(path)
	writeCmd := fmt.Sprintf("mkdir -p /workspace/%s && echo '%s' | base64 -d > /workspace/%s", dir, encoded, path)
	result := DockerExec(dockerCfg, homepageContainerName, writeCmd, "")
	// Check for DockerExec errors via JSON status field.
	var execResult map[string]interface{}
	if err := json.Unmarshal([]byte(result), &execResult); err == nil {
		if status, ok := execResult["status"].(string); ok && status == "error" {
			errMsg, _ := execResult["error"].(string)
			return errJSON("edit file write failed for %s: %s", path, errMsg)
		}
	}
	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "path": path, "operation": operation})
	return string(out)
}

// collapseSpaces replaces consecutive whitespace with a single space.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// whitespaceAwareReplace finds old in text with whitespace tolerance.
// It tries exact match first, then falls back to a whitespace-collapsed match
// where all runs of whitespace are normalized to single spaces.
// Returns the edited text and whether whitespace normalization was used.
func whitespaceAwareReplace(text, old, new_ string) (string, bool, error) {
	// Fast path: exact match
	if strings.Contains(text, old) {
		count := strings.Count(text, old)
		if count > 1 {
			return "", false, fmt.Errorf("'old' text found %d times — must be unique for str_replace", count)
		}
		return strings.Replace(text, old, new_, 1), false, nil
	}

	// Slow path: whitespace-tolerant match
	normText := collapseSpaces(text)
	normOld := collapseSpaces(old)
	normIdx := strings.Index(normText, normOld)
	if normIdx < 0 {
		return "", false, fmt.Errorf("'old' text not found in file")
	}

	// Find the corresponding position in the original text by scanning
	// character-by-character through both normalized and original.
	// This maps the whitespace-collapsed match position back to the real text.
	origIdx := 0
	normPos := 0
	textRunes := []rune(text)
	normRunes := []rune(normText)

	for origIdx < len(textRunes) && normPos < normIdx {
		r := textRunes[origIdx]
		if r == '\n' {
			// Lines in the normalized text are separated by single spaces;
			// newlines in original collapse to a single space in normalized.
			// Skip to next non-newline, consume the newline itself.
			for origIdx < len(textRunes) && textRunes[origIdx] == '\n' {
				origIdx++
			}
			// In normalized text, there is a single space between lines.
			// Advance normPos past any space.
			for normPos < len(normRunes) && normRunes[normPos] == ' ' {
				normPos++
			}
			continue
		}
		// Skip consecutive whitespace in original — it becomes single space in normalized
		if isSpace(r) {
			for origIdx < len(textRunes) && isSpace(textRunes[origIdx]) && textRunes[origIdx] != '\n' {
				origIdx++
			}
			// This run of whitespace (excluding newlines) → single space in normalized
			normPos++
			// In original text, skip all whitespace chars we already consumed
			continue
		}
		// Non-whitespace char: must match
		if normPos < len(normRunes) && textRunes[origIdx] == normRunes[normPos] {
			origIdx++
			normPos++
		} else {
			// Mismatch — this shouldn't happen if the algorithm is correct
			break
		}
	}

	// Now normIdx points into normText; find where that ends in the original.
	// We need the END of the match in original text.
	normEnd := normIdx + len(normOld)
	origEnd := origIdx

	// Advance origEnd in the original while the corresponding position in
	// normalized is still within the match region.
	normPos = normIdx
	for origEnd < len(textRunes) && normPos < normEnd {
		r := textRunes[origEnd]
		if r == '\n' {
			origEnd++
			continue
		}
		if isSpace(r) {
			for origEnd < len(textRunes) && isSpace(textRunes[origEnd]) && textRunes[origEnd] != '\n' {
				origEnd++
			}
			normPos++
			continue
		}
		if normPos < len(normRunes) && textRunes[origEnd] == normRunes[normPos] {
			origEnd++
			normPos++
		} else {
			break
		}
	}

	// Do the replacement using original character indices
	before := text[:origIdx]
	after := text[origEnd:]
	return before + new_ + after, true, nil
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r'
}

// applyHomepageEdit applies an editing operation to file content in memory.
// Returns the edited content and an error JSON string (empty if success).
func applyHomepageEdit(text, operation, old, new_, marker, content string, startLine, endLine int) (string, string) {
	switch operation {
	case "str_replace":
		if old == "" {
			return "", errJSON("'old' text is required for str_replace")
		}
		edited, usedNorm, err := whitespaceAwareReplace(text, old, new_)
		if err != nil {
			return "", errJSON("%s", err.Error())
		}
		if usedNorm {
			logger := slog.Default()
			logger.Info("[Homepage] str_replace: used whitespace-tolerant match (old text had whitespace differences)")
		}
		return edited, ""

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
