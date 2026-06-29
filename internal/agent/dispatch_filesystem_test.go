package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/services"
	"aurago/internal/tools"
)

func testHashlineContent(content string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(content))
	return fmt.Sprintf("%08x", h.Sum32())
}

func TestDispatchFilesystemRejectsOutsideHostWriteCanary(t *testing.T) {
	t.Parallel()

	tempRoot := t.TempDir()
	repoRoot := filepath.Join(tempRoot, "repo")
	workspaceDir := filepath.Join(repoRoot, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	outsidePath := filepath.Join(tempRoot, "outside-host.txt")
	const original = "original host content"
	if err := os.WriteFile(outsidePath, []byte(original), 0o644); err != nil {
		t.Fatalf("create outside canary file: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Directories.WorkspaceDir = workspaceDir
	dc := &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	calls := []ToolCall{
		{
			Action:    "filesystem",
			Operation: "write_file",
			FilePath:  outsidePath,
			Content:   "mutated by filesystem",
		},
		{
			Action:    "file_editor",
			Operation: "str_replace",
			FilePath:  outsidePath,
			Params: map[string]interface{}{
				"old": "original",
				"new": "mutated by file editor",
			},
		},
	}

	for _, tc := range calls {
		output := dispatchFilesystem(context.Background(), tc, dc)
		if !strings.Contains(output, `"status":"error"`) {
			t.Fatalf("%s outside-host write did not return an error: %s", tc.Action, output)
		}
		if !strings.Contains(output, "absolute path outside the project root") {
			t.Fatalf("%s outside-host write returned the wrong error: %s", tc.Action, output)
		}

		got, err := os.ReadFile(outsidePath)
		if err != nil {
			t.Fatalf("read outside canary after %s: %v", tc.Action, err)
		}
		if string(got) != original {
			t.Fatalf("%s mutated outside-host file: got %q", tc.Action, string(got))
		}
	}
}

func TestDispatchFilesystemRoutesReadFileIncludeHashes(t *testing.T) {
	tempRoot := t.TempDir()
	workspaceDir := filepath.Join(tempRoot, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "notes.txt"), []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspaceDir
	dc := &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	output := dispatchFilesystem(context.Background(), ToolCall{
		Action:    "filesystem",
		Operation: "read_file",
		FilePath:  "notes.txt",
		Params: map[string]interface{}{
			"include_hashes": true,
		},
	}, dc)

	if !strings.Contains(output, `"format":"hashline"`) || !strings.Contains(output, "1#"+testHashlineContent("alpha")+":alpha") {
		t.Fatalf("read_file include_hashes was not routed to hashline output: %s", output)
	}
}

func TestDispatchFilesystemRoutesHashlineFileEditor(t *testing.T) {
	tempRoot := t.TempDir()
	workspaceDir := filepath.Join(tempRoot, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "notes.txt"), []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Directories.WorkspaceDir = workspaceDir
	dc := &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	output := dispatchFilesystem(context.Background(), ToolCall{
		Action:    "file_editor",
		Operation: "hashline_replace",
		FilePath:  "notes.txt",
		Params: map[string]interface{}{
			"old":         "beta",
			"new":         "changed",
			"anchor_line": float64(2),
			"anchor_hash": testHashlineContent("beta"),
		},
	}, dc)

	if !strings.Contains(output, `"status":"success"`) {
		t.Fatalf("hashline file_editor did not succeed: %s", output)
	}
	data, err := os.ReadFile(filepath.Join(workspaceDir, "notes.txt"))
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if string(data) != "alpha\nchanged\n" {
		t.Fatalf("file content = %q", string(data))
	}
}

func TestDispatchFilesystemRejectsVirtualDesktopPathsForFileEditor(t *testing.T) {
	t.Parallel()

	tempRoot := t.TempDir()
	workspaceDir := filepath.Join(tempRoot, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Directories.WorkspaceDir = workspaceDir
	dc := &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	output := dispatchFilesystem(context.Background(), ToolCall{
		Action:    "file_editor",
		Operation: "str_replace_all",
		FilePath:  "Apps/space-invaders/game.js",
		Params: map[string]interface{}{
			"old": "before",
			"new": "after",
		},
	}, dc)

	if !strings.Contains(output, `"status":"error"`) {
		t.Fatalf("desktop file_editor path should be rejected, got: %s", output)
	}
	if !strings.Contains(output, "virtual_desktop") || !strings.Contains(output, "Apps/space-invaders/game.js") {
		t.Fatalf("desktop file_editor rejection should point to virtual_desktop and preserve path, got: %s", output)
	}
}

func TestDispatchFilesystemRoutesTomlEditor(t *testing.T) {
	tempRoot := t.TempDir()
	workspaceDir := filepath.Join(tempRoot, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "config.toml"), []byte("[server]\nport = 8088\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Directories.WorkspaceDir = workspaceDir
	dc := &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	output := dispatchFilesystem(context.Background(), ToolCall{
		Action:    "toml_editor",
		Operation: "get",
		FilePath:  "config.toml",
		Params: map[string]interface{}{
			"toml_path": "server.port",
		},
	}, dc)

	if !strings.Contains(output, `"status":"success"`) || !strings.Contains(output, `"data":8088`) {
		t.Fatalf("toml_editor dispatch returned unexpected output: %s", output)
	}
}

func TestDispatchWorkspaceSearchExecutesIndexedGrep(t *testing.T) {
	tempRoot := t.TempDir()
	dataDir := filepath.Join(tempRoot, "data")
	agentWorkspaceDir := filepath.Join(tempRoot, "agent_workspace")
	workspaceDir := filepath.Join(agentWorkspaceDir, "workdir")
	toolsDir := filepath.Join(agentWorkspaceDir, "tools")
	if err := os.MkdirAll(filepath.Join(workspaceDir, "docs"), 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(toolsDir, "helpers"), 0o755); err != nil {
		t.Fatalf("create tools dir: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "docs", "readme.md"), []byte("needle line\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, "helpers", "tool.md"), []byte("needle tool\n"), 0o644); err != nil {
		t.Fatalf("write tool fixture: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.Directories.WorkspaceDir = workspaceDir
	cfg.Directories.ToolsDir = toolsDir
	cfg.WorkspaceSearch.Enabled = true
	cfg.WorkspaceSearch.MaxFileSizeMB = 2
	cfg.WorkspaceSearch.MaxIndexSizeMB = 64
	cfg.WorkspaceSearch.MaxResults = 20
	cfg.WorkspaceSearch.PollIntervalSeconds = 3600
	cfg.WorkspaceSearch.FuzzyThreshold = 0.25
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := services.NewWorkspaceSearchService(cfg, nil, logger)
	if err != nil {
		t.Fatalf("new workspace search service: %v", err)
	}
	t.Cleanup(func() {
		svc.Stop()
		if err := svc.Close(); err != nil {
			t.Fatalf("close workspace search service: %v", err)
		}
	})
	if err := svc.Rescan(context.Background()); err != nil {
		t.Fatalf("rescan: %v", err)
	}

	dc := &DispatchContext{
		Cfg:             cfg,
		Logger:          logger,
		WorkspaceSearch: svc,
	}

	output := dispatchFilesystem(context.Background(), ToolCall{
		Action:    "workspace_search",
		Operation: "grep",
		Params: map[string]interface{}{
			"query": "needle",
			"glob":  "**/*.md",
		},
	}, dc)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(output, "Tool Output: ")), &parsed); err != nil {
		t.Fatalf("unmarshal workspace_search output %q: %v", output, err)
	}
	if parsed["status"] != "success" {
		t.Fatalf("workspace_search status = %v, output: %s", parsed["status"], output)
	}
	data := parsed["data"].(map[string]interface{})
	matches := data["matches"].([]interface{})
	if len(matches) != 2 {
		t.Fatalf("matches = %#v, want 2", matches)
	}
	gotFiles := map[string]bool{}
	for _, rawMatch := range matches {
		match := rawMatch.(map[string]interface{})
		gotFiles[match["file"].(string)] = true
	}
	for _, want := range []string{"workdir/docs/readme.md", "tools/helpers/tool.md"} {
		if !gotFiles[want] {
			t.Fatalf("matches missing %s, got %#v", want, gotFiles)
		}
	}
}

func TestDispatchFileSearchFindDelegatesToWorkspaceSearchWithCompatibleShape(t *testing.T) {
	tempRoot := t.TempDir()
	dataDir := filepath.Join(tempRoot, "data")
	agentWorkspaceDir := filepath.Join(tempRoot, "agent_workspace")
	workspaceDir := filepath.Join(agentWorkspaceDir, "workdir")
	toolsDir := filepath.Join(agentWorkspaceDir, "tools")
	if err := os.MkdirAll(filepath.Join(workspaceDir, "docs"), 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(toolsDir, "helpers"), 0o755); err != nil {
		t.Fatalf("create tools dir: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "docs", "readme.md"), []byte("needle line\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, "helpers", "tool.md"), []byte("needle tool\n"), 0o644); err != nil {
		t.Fatalf("write tool fixture: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.Directories.WorkspaceDir = workspaceDir
	cfg.Directories.ToolsDir = toolsDir
	cfg.WorkspaceSearch.Enabled = true
	cfg.WorkspaceSearch.MaxFileSizeMB = 2
	cfg.WorkspaceSearch.MaxIndexSizeMB = 64
	cfg.WorkspaceSearch.MaxResults = 20
	cfg.WorkspaceSearch.PollIntervalSeconds = 3600
	cfg.WorkspaceSearch.FuzzyThreshold = 0.25
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := services.NewWorkspaceSearchService(cfg, nil, logger)
	if err != nil {
		t.Fatalf("new workspace search service: %v", err)
	}
	t.Cleanup(func() {
		svc.Stop()
		if err := svc.Close(); err != nil {
			t.Fatalf("close workspace search service: %v", err)
		}
	})
	if err := svc.Rescan(context.Background()); err != nil {
		t.Fatalf("rescan: %v", err)
	}

	dc := &DispatchContext{
		Cfg:             cfg,
		Logger:          logger,
		WorkspaceSearch: svc,
	}

	output := dispatchFilesystem(context.Background(), ToolCall{
		Action:    "file_search",
		Operation: "find",
		Params: map[string]interface{}{
			"glob": "**/*.md",
		},
	}, dc)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(output, "Tool Output: ")), &parsed); err != nil {
		t.Fatalf("unmarshal file_search output %q: %v", output, err)
	}
	if parsed["status"] != "success" {
		t.Fatalf("file_search status = %v, output: %s", parsed["status"], output)
	}
	data := parsed["data"].(map[string]interface{})
	files := data["files"].([]interface{})
	if len(files) != 1 || files[0] != "docs/readme.md" {
		t.Fatalf("files = %#v, want only workdir-relative docs/readme.md", files)
	}
}

func TestDispatchFilesystemAccessTrackingHooks(t *testing.T) {
	tempRoot := t.TempDir()
	dataDir := filepath.Join(tempRoot, "data")
	workspaceDir := filepath.Join(tempRoot, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "notes.txt"), []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Directories.DataDir = dataDir
	cfg.Directories.WorkspaceDir = workspaceDir
	cfg.WorkspaceSearch.Enabled = true
	cfg.WorkspaceSearch.MaxFileSizeMB = 2
	cfg.WorkspaceSearch.MaxIndexSizeMB = 64
	cfg.WorkspaceSearch.MaxResults = 20
	cfg.WorkspaceSearch.PollIntervalSeconds = 3600
	cfg.WorkspaceSearch.FuzzyThreshold = 0.25
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := services.NewWorkspaceSearchService(cfg, nil, logger)
	if err != nil {
		t.Fatalf("new workspace search service: %v", err)
	}
	t.Cleanup(func() {
		tools.SetFileAccessTracker(nil)
		svc.Stop()
		if err := svc.Close(); err != nil {
			t.Fatalf("close workspace search service: %v", err)
		}
	})
	if err := svc.Rescan(context.Background()); err != nil {
		t.Fatalf("rescan: %v", err)
	}
	tools.SetFileAccessTracker(func(workspaceDir, path, kind string) {
		_ = svc.TrackAccess(path, kind)
	})
	dc := &DispatchContext{
		Cfg:             cfg,
		Logger:          logger,
		WorkspaceSearch: svc,
	}

	readOutput := dispatchFilesystem(context.Background(), ToolCall{
		Action:    "filesystem",
		Operation: "read_file",
		FilePath:  "notes.txt",
	}, dc)
	if !strings.Contains(readOutput, `"status":"success"`) {
		t.Fatalf("read_file failed: %s", readOutput)
	}

	editOutput := dispatchFilesystem(context.Background(), ToolCall{
		Action:    "file_editor",
		Operation: "str_replace",
		FilePath:  "notes.txt",
		Params: map[string]interface{}{
			"old": "beta",
			"new": "changed",
		},
	}, dc)
	if !strings.Contains(editOutput, `"status":"success"`) {
		t.Fatalf("file_editor failed: %s", editOutput)
	}

	recent, err := svc.Recent(context.Background(), services.WorkspaceSearchRequest{Limit: 10})
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(recent) == 0 || recent[0].Path != "workdir/notes.txt" || recent[0].AccessCount < 2 {
		t.Fatalf("recent = %#v, want workdir/notes.txt with at least two tracked accesses", recent)
	}
}
