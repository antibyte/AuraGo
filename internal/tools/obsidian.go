package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/obsidian"
	"aurago/internal/security"
)

const defaultObsidianRequestTimeout = 30 * time.Second
const maxObsidianContentSize = 50 * 1024 // 50KB

func obsidianRequestContext(cfg config.ObsidianConfig) (context.Context, context.CancelFunc) {
	timeout := time.Duration(cfg.RequestTimeout) * time.Second
	if timeout <= 0 {
		timeout = defaultObsidianRequestTimeout
	}
	return context.WithTimeout(context.Background(), timeout)
}

// DispatchObsidianTool routes Obsidian tool calls by operation name.
func DispatchObsidianTool(operation string, params map[string]string, cfg *config.Config, vault *security.Vault, logger *slog.Logger) string {
	if !cfg.Obsidian.Enabled {
		return errJSON("Obsidian integration is disabled")
	}

	switch operation {
	case "health":
		return ObsidianHealth(cfg.Obsidian, vault, logger)
	case "list_files":
		dir := getString(params, "directory", "")
		return ObsidianListFiles(cfg.Obsidian, vault, dir, logger)
	case "read_note":
		path := getString(params, "path")
		targetType := getString(params, "target_type", "")
		target := getString(params, "target", "")
		return ObsidianReadNote(cfg.Obsidian, vault, path, targetType, target, logger)
	case "create_note":
		path := getString(params, "path")
		content := getString(params, "content")
		return ObsidianCreateNote(cfg.Obsidian, vault, path, content, logger)
	case "update_note":
		path := getString(params, "path")
		content := getString(params, "content")
		return ObsidianUpdateNote(cfg.Obsidian, vault, path, content, logger)
	case "patch_note":
		path := getString(params, "path")
		content := getString(params, "content")
		targetType := getString(params, "target_type", "")
		target := getString(params, "target", "")
		patchOp := getString(params, "patch_op", "append")
		return ObsidianPatchNote(cfg.Obsidian, vault, path, content, targetType, target, patchOp, logger)
	case "delete_note":
		path := getString(params, "path")
		return ObsidianDeleteNote(cfg.Obsidian, vault, path, logger)
	case "search":
		query := getString(params, "query")
		contextLength := getInt(params, "context_length", 100)
		return ObsidianSearch(cfg.Obsidian, vault, query, contextLength, logger)
	case "search_dataview":
		query := getString(params, "query")
		return ObsidianSearchDataview(cfg.Obsidian, vault, query, logger)
	case "list_tags":
		return ObsidianListTags(cfg.Obsidian, vault, logger)
	case "daily_note", "periodic_note":
		period := getString(params, "period", "daily")
		content := getString(params, "content", "")
		return ObsidianPeriodicNote(cfg.Obsidian, vault, period, content, logger)
	case "list_commands":
		return ObsidianListCommands(cfg.Obsidian, vault, logger)
	case "execute_command":
		commandID := getString(params, "command_id")
		return ObsidianExecuteCommand(cfg.Obsidian, vault, commandID, logger)
	case "document_map":
		path := getString(params, "path")
		return ObsidianDocumentMap(cfg.Obsidian, vault, path, logger)
	case "open_in_obsidian":
		path := getString(params, "path")
		return ObsidianOpenInApp(cfg.Obsidian, vault, path, logger)
	default:
		return errJSON("Unknown Obsidian operation: %s", operation)
	}
}

func newObsidianClient(cfg config.ObsidianConfig, vault *security.Vault) (*obsidian.Client, error) {
	return obsidian.NewClient(cfg, vault)
}

func wrapExternalContent(content string) string {
	if len(content) > maxObsidianContentSize {
		content = content[:maxObsidianContentSize] + "\n... (truncated)"
	}
	return "<external_data>" + content + "</external_data>"
}

// ObsidianHealth checks connectivity to the Obsidian REST API.
func ObsidianHealth(cfg config.ObsidianConfig, vault *security.Vault, logger *slog.Logger) string {
	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	status, err := client.Ping(ctx)
	if err != nil {
		return errJSON("Obsidian health check failed: %v", err)
	}

	result := map[string]interface{}{
		"status":           "ok",
		"authenticated":    status.Authenticated,
		"api_version":      status.Versions["self"],
		"obsidian_version": status.Versions["obsidian"],
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianListFiles lists files and directories in the vault.
func ObsidianListFiles(cfg config.ObsidianConfig, vault *security.Vault, directory string, logger *slog.Logger) string {
	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	files, err := client.ListFiles(ctx, directory)
	if err != nil {
		return errJSON("Failed to list files: %v", err)
	}

	result := map[string]interface{}{
		"status":    "ok",
		"directory": directory,
		"files":     files,
		"count":     len(files),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianReadNote reads a note from the vault.
func ObsidianReadNote(cfg config.ObsidianConfig, vault *security.Vault, path, targetType, target string, logger *slog.Logger) string {
	if path == "" {
		return errJSON("path is required")
	}

	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	var note *obsidian.NoteJSON
	if targetType != "" && target != "" {
		note, err = client.ReadNoteSection(ctx, path, targetType, target)
	} else {
		note, err = client.ReadNote(ctx, path)
	}
	if err != nil {
		return errJSON("Failed to read note: %v", err)
	}

	result := map[string]interface{}{
		"status":  "ok",
		"path":    path,
		"content": wrapExternalContent(note.Content),
	}
	if len(note.Tags) > 0 {
		result["tags"] = note.Tags
	}
	if len(note.Frontmatter) > 0 {
		result["frontmatter"] = note.Frontmatter
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianCreateNote creates a new note in the vault.
func ObsidianCreateNote(cfg config.ObsidianConfig, vault *security.Vault, path, content string, logger *slog.Logger) string {
	if path == "" {
		return errJSON("path is required")
	}
	if content == "" {
		return errJSON("content is required")
	}

	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	if err := client.CreateNote(ctx, path, content); err != nil {
		return errJSON("Failed to create note: %v", err)
	}

	result := map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Note created at %s", path),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianUpdateNote replaces the entire content of a note.
func ObsidianUpdateNote(cfg config.ObsidianConfig, vault *security.Vault, path, content string, logger *slog.Logger) string {
	if path == "" {
		return errJSON("path is required")
	}

	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	if err := client.UpdateNote(ctx, path, content); err != nil {
		return errJSON("Failed to update note: %v", err)
	}

	result := map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Note updated at %s", path),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianPatchNote appends, prepends, or replaces content in a note.
func ObsidianPatchNote(cfg config.ObsidianConfig, vault *security.Vault, path, content, targetType, target, patchOp string, logger *slog.Logger) string {
	if path == "" {
		return errJSON("path is required")
	}
	if content == "" {
		return errJSON("content is required")
	}

	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	if err := client.PatchNote(ctx, path, content, targetType, target, patchOp); err != nil {
		return errJSON("Failed to patch note: %v", err)
	}

	result := map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Note patched at %s (%s)", path, patchOp),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianDeleteNote deletes a note from the vault.
func ObsidianDeleteNote(cfg config.ObsidianConfig, vault *security.Vault, path string, logger *slog.Logger) string {
	if path == "" {
		return errJSON("path is required")
	}

	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	if err := client.DeleteNote(ctx, path); err != nil {
		return errJSON("Failed to delete note: %v", err)
	}

	result := map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Note deleted: %s", path),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianSearch performs a full-text search in the vault.
func ObsidianSearch(cfg config.ObsidianConfig, vault *security.Vault, query string, contextLength int, logger *slog.Logger) string {
	if query == "" {
		return errJSON("query is required")
	}

	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	results, err := client.SearchSimple(ctx, query, contextLength)
	if err != nil {
		return errJSON("Search failed: %v", err)
	}

	// Wrap search results in external_data for prompt injection protection
	wrapped := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		entry := map[string]interface{}{
			"filename": r.Filename,
			"score":    r.Score,
		}
		if len(r.Matches) > 0 {
			snippets := make([]string, 0, len(r.Matches))
			for _, m := range r.Matches {
				snippets = append(snippets, m.Context)
			}
			entry["matches"] = wrapExternalContent(strings.Join(snippets, "\n---\n"))
		}
		wrapped = append(wrapped, entry)
	}

	result := map[string]interface{}{
		"status":  "ok",
		"query":   query,
		"results": wrapped,
		"count":   len(results),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianSearchDataview executes a Dataview DQL query.
func ObsidianSearchDataview(cfg config.ObsidianConfig, vault *security.Vault, query string, logger *slog.Logger) string {
	if query == "" {
		return errJSON("query is required")
	}

	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	results, err := client.SearchDataview(ctx, query)
	if err != nil {
		return errJSON("Dataview query failed: %v", err)
	}

	result := map[string]interface{}{
		"status":  "ok",
		"results": results,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianListTags returns all tags in the vault.
func ObsidianListTags(cfg config.ObsidianConfig, vault *security.Vault, logger *slog.Logger) string {
	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	tags, err := client.ListTags(ctx)
	if err != nil {
		return errJSON("Failed to list tags: %v", err)
	}

	result := map[string]interface{}{
		"status": "ok",
		"tags":   tags,
		"count":  len(tags),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianPeriodicNote reads or creates a periodic note.
func ObsidianPeriodicNote(cfg config.ObsidianConfig, vault *security.Vault, period, content string, logger *slog.Logger) string {
	if period == "" {
		period = "daily"
	}

	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	if content == "" {
		// Read the current periodic note
		note, err := client.ReadPeriodicNote(ctx, period)
		if err != nil {
			return errJSON("Failed to read periodic note: %v", err)
		}
		result := map[string]interface{}{
			"status":  "ok",
			"period":  period,
			"content": wrapExternalContent(note.Content),
		}
		if len(note.Tags) > 0 {
			result["tags"] = note.Tags
		}
		data, _ := json.Marshal(result)
		return string(data)
	}

	// Create/update the periodic note
	if err := client.PatchPeriodicNote(ctx, period, content, "append"); err != nil {
		return errJSON("Failed to update periodic note: %v", err)
	}
	result := map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Appended content to %s note", period),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianListCommands lists all available Obsidian commands.
func ObsidianListCommands(cfg config.ObsidianConfig, vault *security.Vault, logger *slog.Logger) string {
	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	commands, err := client.ListCommands(ctx)
	if err != nil {
		return errJSON("Failed to list commands: %v", err)
	}

	result := map[string]interface{}{
		"status":   "ok",
		"commands": commands,
		"count":    len(commands),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianExecuteCommand executes an Obsidian command by ID.
func ObsidianExecuteCommand(cfg config.ObsidianConfig, vault *security.Vault, commandID string, logger *slog.Logger) string {
	if commandID == "" {
		return errJSON("command_id is required")
	}

	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	if err := client.ExecuteCommand(ctx, commandID); err != nil {
		return errJSON("Failed to execute command: %v", err)
	}

	result := map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Command executed: %s", commandID),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianDocumentMap returns the heading/block structure of a note.
func ObsidianDocumentMap(cfg config.ObsidianConfig, vault *security.Vault, path string, logger *slog.Logger) string {
	if path == "" {
		return errJSON("path is required")
	}

	client, err := newObsidianClient(cfg, vault)
	if err != nil {
		return errJSON("Obsidian connection failed: %v", err)
	}
	defer client.Close()

	ctx, cancel := obsidianRequestContext(cfg)
	defer cancel()

	docMap, err := client.GetDocumentMap(ctx, path)
	if err != nil {
		return errJSON("Failed to get document map: %v", err)
	}

	result := map[string]interface{}{
		"status":       "ok",
		"path":         path,
		"document_map": docMap,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// ObsidianOpenInApp opens a note in the Obsidian desktop app via command execution.
func ObsidianOpenInApp(cfg config.ObsidianConfig, vault *security.Vault, path string, logger *slog.Logger) string {
	if path == "" {
		return errJSON("path is required")
	}

	// Use obsidian URI scheme to open the note
	result := map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("To open in Obsidian, use URI: obsidian://open?path=%s", path),
		"uri":     fmt.Sprintf("obsidian://open?path=%s", path),
	}
	data, _ := json.Marshal(result)
	return string(data)
}
