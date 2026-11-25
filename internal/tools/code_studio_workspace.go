package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
)

// code_studio_workspace is the agent's file and command tool for saved Code
// Studio SSH workspaces. The server registers the provider
// (internal/server/code_studio_agent_tool.go). Paths are workspace-relative
// for the agent and /workspace/... towards the provider, like every Code
// Studio path. The local container workspace stays on virtual_desktop_files.

var (
	// ErrCodeStudioWorkspaceNotFound: the id names no saved workspace.
	ErrCodeStudioWorkspaceNotFound = errors.New("code studio workspace not found")
	// ErrCodeStudioWorkspaceLocal: the id names the local container workspace.
	ErrCodeStudioWorkspaceLocal = errors.New("code studio workspace is local")
)

const (
	codeStudioWorkspaceRoot            = "/workspace"
	codeStudioWorkspaceReadInlineBytes = 48 << 10
	codeStudioWorkspaceDefaultTimeout  = 60  // seconds
	codeStudioWorkspaceMaxTimeout      = 120 // seconds
	codeStudioWorkspaceOutputHead      = 16 << 10
	codeStudioWorkspaceOutputTail      = 32 << 10
	codeStudioWorkspaceDefaultMatches  = 50
	codeStudioWorkspaceMaxMatches      = 200
)

type CodeStudioWorkspaceInfo struct {
	ID, Label, Host, Root string
}

type CodeStudioWorkspaceEntry struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Type     string    `json:"type"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

type CodeStudioWorkspaceSearch struct {
	Query, Path                     string
	CaseSensitive, Regex, WholeWord bool
	Include, Exclude                string
}

type CodeStudioWorkspaceMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Preview string `json:"preview"`
}

type CodeStudioWorkspaceExecResult struct {
	ExitCode int
	Output   string
}

type CodeStudioWorkspaceLimits struct {
	MaxFileBytes int64
	ReadOnly     bool // live desktop read-only flag, checked with cfg.VirtualDesktop.ReadOnly
}

// CodeStudioWorkspace is one saved SSH workspace. Paths are virtual
// (/workspace/...); command runs `/bin/sh -c command` in vcwd.
type CodeStudioWorkspace interface {
	Info() CodeStudioWorkspaceInfo
	List(ctx context.Context, vdir string) ([]CodeStudioWorkspaceEntry, error)
	Read(ctx context.Context, vpath string, max int64) ([]byte, error)
	Write(ctx context.Context, vpath string, content []byte) error
	Search(ctx context.Context, opts CodeStudioWorkspaceSearch) ([]CodeStudioWorkspaceMatch, bool, error)
	Exec(ctx context.Context, command, vcwd string, timeout time.Duration) (CodeStudioWorkspaceExecResult, error)
}

// CodeStudioWorkspaceProvider resolves workspace ids and receives change notices.
type CodeStudioWorkspaceProvider interface {
	// Workspace returns ErrCodeStudioWorkspaceLocal for the local workspace
	// and ErrCodeStudioWorkspaceNotFound for an unknown id.
	Workspace(ctx context.Context, id string) (CodeStudioWorkspace, error)
	// Changed runs after every agent write; no paths means a command ran.
	Changed(id string, vpaths ...string)
	Limits() CodeStudioWorkspaceLimits
}

var (
	codeStudioWorkspaceMu       sync.RWMutex
	codeStudioWorkspaceProvider CodeStudioWorkspaceProvider
)

// SetCodeStudioWorkspaceProvider registers the server's provider; nil withdraws it.
func SetCodeStudioWorkspaceProvider(p CodeStudioWorkspaceProvider) {
	codeStudioWorkspaceMu.Lock()
	defer codeStudioWorkspaceMu.Unlock()
	codeStudioWorkspaceProvider = p
}

func currentCodeStudioWorkspaceProvider() CodeStudioWorkspaceProvider {
	codeStudioWorkspaceMu.RLock()
	defer codeStudioWorkspaceMu.RUnlock()
	return codeStudioWorkspaceProvider
}

// codeStudioWorkspacePath maps a workspace-relative path (or one under
// /workspace) to the virtual Code Studio path. Host paths, ~, drive letters
// and .. are refused.
func codeStudioWorkspacePath(raw string, allowRoot bool) (string, error) {
	p := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if strings.ContainsRune(p, 0) {
		return "", errors.New("path contains a null byte")
	}
	switch {
	case p == "" || p == "." || p == codeStudioWorkspaceRoot || p == codeStudioWorkspaceRoot+"/":
		p = ""
	case strings.HasPrefix(p, codeStudioWorkspaceRoot+"/"):
		p = strings.TrimPrefix(p, codeStudioWorkspaceRoot+"/")
	case strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~") || (len(p) > 1 && p[1] == ':'):
		return "", fmt.Errorf("path %q must be relative to the workspace root, for example src/main.go", raw)
	}
	var parts []string
	for _, segment := range strings.Split(p, "/") {
		switch segment {
		case "", ".":
			continue
		case "..":
			return "", fmt.Errorf("path %q leaves the workspace", raw)
		}
		parts = append(parts, segment)
	}
	if len(parts) == 0 {
		if !allowRoot {
			return "", errors.New("path is required")
		}
		return codeStudioWorkspaceRoot, nil
	}
	return codeStudioWorkspaceRoot + "/" + strings.Join(parts, "/"), nil
}

// codeStudioWorkspaceRel is the agent-facing form of a virtual path.
func codeStudioWorkspaceRel(vpath string) string {
	rel := strings.TrimPrefix(strings.TrimPrefix(vpath, codeStudioWorkspaceRoot), "/")
	if rel == "" {
		return "."
	}
	return rel
}

// codeStudioWorkspaceChangedEvent tells Code Studio to re-read clean tabs of
// the workspace: path is the virtual path, or "" after a command.
func codeStudioWorkspaceChangedEvent(id, vpath string) *desktop.Event {
	return virtualDesktopEvent("code_studio_file_changed", map[string]interface{}{"workspace_id": id, "path": vpath})
}

var (
	codeStudioWorkspaceOperations = map[string]bool{"list_files": true, "read_file": true, "read_file_excerpt": true, "search": true, "write_file": true, "patch_file": true, "run_command": true}
	codeStudioWorkspaceMutations  = map[string]bool{"write_file": true, "patch_file": true, "run_command": true}
)

// ExecuteCodeStudioWorkspace performs one agent operation in a saved Code
// Studio SSH workspace. The gates are virtual_desktop_files' own: desktop
// enabled, agent control, the desktop tool switch, read-only. The agent acts
// for the signed-in chat user (the chat endpoint is authenticated and
// permission-checked), so the HTTP remote-caller gate of the Code Studio API
// does not apply here.
func ExecuteCodeStudioWorkspace(ctx context.Context, cfg *config.Config, args map[string]interface{}) VirtualDesktopExecution {
	if cfg == nil {
		return codeStudioWorkspaceFail("configuration is unavailable")
	}
	if !cfg.VirtualDesktop.Enabled {
		return codeStudioWorkspaceFail("virtual desktop is disabled in config")
	}
	if !cfg.VirtualDesktop.AllowAgentControl || !cfg.Tools.VirtualDesktop.Enabled {
		return codeStudioWorkspaceFail("agent control for the virtual desktop is disabled in config")
	}
	provider := currentCodeStudioWorkspaceProvider()
	if provider == nil {
		return codeStudioWorkspaceFail("Code Studio workspaces are unavailable")
	}
	op := strings.ToLower(virtualDesktopString(args, "operation"))
	if !codeStudioWorkspaceOperations[op] {
		return codeStudioWorkspaceFail(fmt.Sprintf("unsupported operation %q; use list_files, read_file, read_file_excerpt, search, write_file, patch_file or run_command", op))
	}
	limits := provider.Limits()
	if codeStudioWorkspaceMutations[op] && (cfg.VirtualDesktop.ReadOnly || limits.ReadOnly) {
		return codeStudioWorkspaceFail("virtual desktop is read-only: code_studio_workspace may only list, read and search")
	}
	id := virtualDesktopString(args, "workspace_id")
	if id == "" {
		return codeStudioWorkspaceFail("workspace_id is required; use the Code Studio workspace id from the chat context")
	}
	ws, err := provider.Workspace(ctx, id)
	switch {
	case errors.Is(err, ErrCodeStudioWorkspaceLocal):
		return codeStudioWorkspaceFail(fmt.Sprintf("workspace %q is the local Code Studio workspace; use virtual_desktop_files for its files", id))
	case errors.Is(err, ErrCodeStudioWorkspaceNotFound):
		return codeStudioWorkspaceFail(fmt.Sprintf("Code Studio workspace %q does not exist; ask the user to reopen the folder in Code Studio", id))
	case err != nil:
		return codeStudioWorkspaceFail(err.Error())
	}
	maxBytes := limits.MaxFileBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	call := codeStudioWorkspaceCall{ctx: ctx, args: args, id: id, ws: ws, provider: provider, maxBytes: maxBytes}
	switch op {
	case "list_files":
		return call.listFiles()
	case "read_file":
		return call.readFile()
	case "read_file_excerpt":
		return call.readExcerpt()
	case "search":
		return call.search()
	case "write_file":
		return call.writeFile()
	case "patch_file":
		return call.patchFile()
	default:
		return call.runCommand()
	}
}

func codeStudioWorkspaceFail(message string) VirtualDesktopExecution {
	return virtualDesktopJSON("error", message, nil, nil)
}

type codeStudioWorkspaceCall struct {
	ctx      context.Context
	args     map[string]interface{}
	id       string
	ws       CodeStudioWorkspace
	provider CodeStudioWorkspaceProvider
	maxBytes int64
}

func (c codeStudioWorkspaceCall) path(allowRoot bool, keys ...string) (string, error) {
	return codeStudioWorkspacePath(virtualDesktopString(c.args, keys...), allowRoot)
}

func (c codeStudioWorkspaceCall) listFiles() VirtualDesktopExecution {
	vdir, err := c.path(true, "path", "file_path")
	if err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	entries, err := c.ws.List(c.ctx, vdir)
	if err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	for i := range entries {
		entries[i].Path = codeStudioWorkspaceRel(entries[i].Path)
	}
	return virtualDesktopJSON("ok", "workspace files listed", map[string]interface{}{"workspace_id": c.id, "path": codeStudioWorkspaceRel(vdir), "files": entries}, nil)
}

func (c codeStudioWorkspaceCall) readText(vpath string) (string, error) {
	data, err := c.ws.Read(c.ctx, vpath, c.maxBytes)
	if err != nil {
		return "", err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", fmt.Errorf("%s is a binary file; code_studio_workspace reads text files only", codeStudioWorkspaceRel(vpath))
	}
	return string(data), nil
}

func (c codeStudioWorkspaceCall) readFile() VirtualDesktopExecution {
	vpath, err := c.path(false, "path", "file_path")
	if err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	content, err := c.readText(vpath)
	if err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	payload := map[string]interface{}{"workspace_id": c.id, "path": codeStudioWorkspaceRel(vpath), "size": len(content), "total_lines": strings.Count(content, "\n") + 1}
	if len(content) > codeStudioWorkspaceReadInlineBytes {
		payload["content"] = strings.ToValidUTF8(content[:codeStudioWorkspaceReadInlineBytes], "")
		payload["truncated"] = true
		payload["next_action"] = "Read the rest with read_file_excerpt (line_start, line_count) or locate it with search."
	} else {
		payload["content"] = content
	}
	return virtualDesktopJSON("ok", "workspace file read", payload, nil)
}

func (c codeStudioWorkspaceCall) readExcerpt() VirtualDesktopExecution {
	vpath, err := c.path(false, "path", "file_path")
	if err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	content, err := c.readText(vpath)
	if err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	lineStart := virtualDesktopInt(c.args, 1, "line_start", "start_line")
	if lineStart < 1 {
		lineStart = 1
	}
	excerpt, lineEnd, totalLines := virtualDesktopLineExcerpt(content, lineStart, virtualDesktopInt(c.args, 80, "line_count", "lines"))
	return virtualDesktopJSON("ok", "workspace file excerpt read", map[string]interface{}{
		"workspace_id": c.id, "path": codeStudioWorkspaceRel(vpath), "line_start": lineStart, "line_end": lineEnd, "total_lines": totalLines, "excerpt": excerpt,
	}, nil)
}

func (c codeStudioWorkspaceCall) search() VirtualDesktopExecution {
	query, _ := virtualDesktopRawString(c.args, "query")
	if strings.TrimSpace(query) == "" {
		query = virtualDesktopString(c.args, "pattern")
	}
	if strings.TrimSpace(query) == "" {
		return codeStudioWorkspaceFail("query is required")
	}
	vdir, err := c.path(true, "path", "file_path")
	if err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	matches, truncated, err := c.ws.Search(c.ctx, CodeStudioWorkspaceSearch{
		Query: query, Path: vdir,
		CaseSensitive: virtualDesktopBool(c.args, "case_sensitive"), Regex: virtualDesktopBool(c.args, "regex"), WholeWord: virtualDesktopBool(c.args, "whole_word"),
		Include: virtualDesktopString(c.args, "include"), Exclude: virtualDesktopString(c.args, "exclude"),
	})
	if err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	limit := virtualDesktopInt(c.args, codeStudioWorkspaceDefaultMatches, "max_matches")
	if limit < 1 {
		limit = codeStudioWorkspaceDefaultMatches
	}
	if limit > codeStudioWorkspaceMaxMatches {
		limit = codeStudioWorkspaceMaxMatches
	}
	if len(matches) > limit {
		matches, truncated = matches[:limit], true
	}
	for i := range matches {
		matches[i].Path = codeStudioWorkspaceRel(matches[i].Path)
	}
	return virtualDesktopJSON("ok", "workspace searched", map[string]interface{}{
		"workspace_id": c.id, "query": query, "path": codeStudioWorkspaceRel(vdir), "match_count": len(matches), "matches": matches, "truncated": truncated,
	}, nil)
}

func (c codeStudioWorkspaceCall) writeFile() VirtualDesktopExecution {
	vpath, err := c.path(false, "path", "file_path")
	if err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	content, has := virtualDesktopRawString(c.args, "content")
	if !has || (strings.TrimSpace(content) == "" && !virtualDesktopBool(c.args, "allow_empty")) {
		return codeStudioWorkspaceFail("content is required for write_file; send the complete file, or use patch_file for edits (allow_empty=true writes an intentionally empty file)")
	}
	if int64(len(content)) > c.maxBytes {
		return codeStudioWorkspaceFail(fmt.Sprintf("content exceeds the %d byte file limit", c.maxBytes))
	}
	if err := c.ws.Write(c.ctx, vpath, []byte(content)); err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	return c.changed("workspace file written", vpath, map[string]interface{}{"size": len(content)})
}

func (c codeStudioWorkspaceCall) patchFile() VirtualDesktopExecution {
	vpath, err := c.path(false, "path", "file_path")
	if err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	replacements, err := codeStudioWorkspaceReplacements(c.args)
	if err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	if len(replacements) == 0 {
		return codeStudioWorkspaceFail(`patch_file needs find and replace, or replacements=[{"find":"...","replace":"..."}]`)
	}
	content, err := c.readText(vpath)
	if err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	rel := codeStudioWorkspaceRel(vpath)
	next := content
	for i, r := range replacements {
		if r.Find == "" {
			return codeStudioWorkspaceFail(fmt.Sprintf("replacement %d has an empty find text", i+1))
		}
		switch n := strings.Count(next, r.Find); {
		case n == 0:
			return codeStudioWorkspaceFail(fmt.Sprintf("replacement %d: find text not found in %s; read the file again and copy the exact text", i+1, rel))
		case n > 1:
			return codeStudioWorkspaceFail(fmt.Sprintf("replacement %d: find text occurs %d times in %s; add surrounding lines so it occurs once", i+1, n, rel))
		}
		next = strings.Replace(next, r.Find, r.Replace, 1)
	}
	if int64(len(next)) > c.maxBytes {
		return codeStudioWorkspaceFail(fmt.Sprintf("patched file exceeds the %d byte file limit", c.maxBytes))
	}
	if err := c.ws.Write(c.ctx, vpath, []byte(next)); err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	return c.changed("workspace file patched", vpath, map[string]interface{}{"replacements": len(replacements), "size": len(next)})
}

// codeStudioWorkspaceReplacements reads find/replace and/or replacements[].
func codeStudioWorkspaceReplacements(args map[string]interface{}) ([]virtualDesktopReplacement, error) {
	var out []virtualDesktopReplacement
	if find, ok := virtualDesktopRawString(args, "find"); ok && find != "" {
		replace, _ := virtualDesktopRawString(args, "replace")
		out = append(out, virtualDesktopReplacement{Find: find, Replace: replace})
	}
	if raw, ok := args["replacements"]; ok {
		parsed, err := virtualDesktopParseReplacements(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid replacements: %w", err)
		}
		out = append(out, parsed...)
	}
	return out, nil
}

func (c codeStudioWorkspaceCall) changed(message, vpath string, extra map[string]interface{}) VirtualDesktopExecution {
	c.provider.Changed(c.id, vpath)
	payload := map[string]interface{}{"workspace_id": c.id, "path": codeStudioWorkspaceRel(vpath)}
	for key, value := range extra {
		payload[key] = value
	}
	return virtualDesktopJSON("ok", message, payload, codeStudioWorkspaceChangedEvent(c.id, vpath))
}

func (c codeStudioWorkspaceCall) runCommand() VirtualDesktopExecution {
	command, _ := virtualDesktopRawString(c.args, "command")
	if strings.TrimSpace(command) == "" {
		return codeStudioWorkspaceFail("command is required")
	}
	cwd, err := c.path(true, "cwd")
	if err != nil {
		return codeStudioWorkspaceFail(err.Error())
	}
	seconds := virtualDesktopInt(c.args, codeStudioWorkspaceDefaultTimeout, "timeout_seconds", "timeout")
	if seconds < 1 {
		seconds = codeStudioWorkspaceDefaultTimeout
	}
	if seconds > codeStudioWorkspaceMaxTimeout {
		seconds = codeStudioWorkspaceMaxTimeout
	}
	result, err := c.ws.Exec(c.ctx, command, cwd, time.Duration(seconds)*time.Second)
	// A command may change any file, even when it fails: refresh either way.
	c.provider.Changed(c.id)
	event := codeStudioWorkspaceChangedEvent(c.id, "")
	if err != nil {
		return virtualDesktopJSON("error", err.Error(), map[string]interface{}{"workspace_id": c.id}, event)
	}
	output, truncated := codeStudioWorkspaceCapOutput(result.Output)
	message := "command finished"
	if result.ExitCode != 0 {
		message = fmt.Sprintf("command exited with code %d", result.ExitCode)
	}
	return virtualDesktopJSON("ok", message, map[string]interface{}{
		"workspace_id": c.id, "cwd": codeStudioWorkspaceRel(cwd), "exit_code": result.ExitCode, "timeout_seconds": seconds, "output": output, "output_truncated": truncated,
	}, event)
}

// codeStudioWorkspaceCapOutput keeps the head and the tail of a long output.
func codeStudioWorkspaceCapOutput(out string) (string, bool) {
	if len(out) <= codeStudioWorkspaceOutputHead+codeStudioWorkspaceOutputTail {
		return out, false
	}
	head := strings.ToValidUTF8(out[:codeStudioWorkspaceOutputHead], "")
	tail := strings.ToValidUTF8(out[len(out)-codeStudioWorkspaceOutputTail:], "")
	return head + fmt.Sprintf("\n... [%d bytes omitted] ...\n", len(out)-len(head)-len(tail)) + tail, true
}
