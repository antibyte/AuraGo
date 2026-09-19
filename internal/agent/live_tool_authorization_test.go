package agent

import (
	"aurago/internal/config"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestLiveAuthorizationIntersectsWithoutChangingRunSnapshot(t *testing.T) {
	initial := &config.Config{}
	initial.Agent.AllowShell = true
	initial.Docker.Enabled = true
	initial.LLM.Model = "frozen-model"
	initial.HuggingFace.AllowedRepos = []string{"org/one", "org/two"}
	current := *initial
	current.Agent.AllowShell = false
	current.Agent.AllowPython = true
	current.Docker.ReadOnly = true
	current.LLM.Model = "new-model"
	initial.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return initial, &current }
	// Delegated runs use deep copies. The authority must survive that boundary.
	delegated := deepClone(*initial)
	delegated.HuggingFace.AllowedRepos = []string{"org/one"}
	actual, ok := dispatchAuthorization(&delegated)
	if !ok || actual.Agent.AllowShell || actual.Agent.AllowPython || !actual.Docker.ReadOnly || !actual.Docker.Enabled {
		t.Fatalf("incorrect permission intersection: %+v", actual.Agent)
	}
	if actual.LLM.Model != "frozen-model" || !initial.Agent.AllowShell || initial.Docker.ReadOnly {
		t.Fatal("provider or original immutable snapshot changed")
	}
	if len(actual.HuggingFace.AllowedRepos) != 1 || actual.HuggingFace.AllowedRepos[0] != "org/one" {
		t.Fatal("delegated scope widened")
	}
	current.HuggingFace.AllowedRepos = []string{"new/repo"}
	if _, ok := dispatchAuthorization(initial); ok {
		t.Fatal("changed compound authorization accepted in existing run")
	}
}

func TestLiveAuthorizationRevocationAndGrantThroughDirectAndWrappedCalls(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		for _, grant := range []bool{false, true} {
			t.Run(map[bool]string{false: "direct", true: "wrapped"}[wrapped]+map[bool]string{false: "-revoke", true: "-grant"}[grant], func(t *testing.T) {
				cfg := &config.Config{}
				cfg.Directories.WorkspaceDir = t.TempDir()
				cfg.Agent.AllowFilesystemWrite = !grant
				current := *cfg
				current.Agent.AllowFilesystemWrite = grant
				cfg.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return cfg, &current }
				dc := &DispatchContext{Cfg: cfg, SessionID: t.Name(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
				path := filepath.Join(cfg.Directories.WorkspaceDir, "fixture.txt")
				call := ToolCall{Action: "filesystem", Operation: "write_file", FilePath: path, Content: "forbidden"}
				if wrapped {
					setRunDiscoverToolsState(dc, dispatchCatalogSchemas(dc), nil)
					t.Cleanup(func() { ClearDiscoverToolsState(t.Name()) })
					call = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "filesystem", "arguments": map[string]interface{}{"operation": "write_file", "file_path": path, "content": "forbidden"}}}
				}
				result := DispatchToolCallResult(context.Background(), &call, dc, "write fixture")
				if !result.IsError {
					t.Fatalf("permission bypass: %+v", result)
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("unexpected write: %v", err)
				}
				if err := os.WriteFile(path, []byte("readable"), 0600); err != nil {
					t.Fatal(err)
				}
				read := ToolCall{Action: "filesystem", Operation: "read_file", FilePath: path}
				if result := DispatchToolCallResult(context.Background(), &read, dc, "read fixture"); result.IsError {
					t.Fatalf("revocation blocked an allowed read: %+v", result)
				}
			})
		}
	}
}

func TestSecuritySpecialistFilesystemSchemaUsesPositiveReadAllowlist(t *testing.T) {
	for _, schema := range builtinToolSchemas(ToolFeatureFlags{AllowFilesystemWrite: true}) {
		if schema.Function == nil || schema.Function.Name != "filesystem" {
			continue
		}
		entry := &ToolCatalogEntry{Schema: schema}
		for _, op := range toolCatalogOperationField(entry, "operation") {
			wantAllowed := op == "read_file" || op == "list_dir" || op == "stat"
			for _, action := range []string{"filesystem", "filesystem_op"} {
				if allowed := checkSecurityRestriction(action, op) == ""; allowed != wantAllowed {
					t.Errorf("%s/%s: allowed=%v", action, op, allowed)
				}
			}
		}
	}
	for _, op := range []string{"", "append", "write", "write_file", "batch", "future_write"} {
		if checkSecurityRestriction("filesystem", op) == "" {
			t.Errorf("unrecognized/write operation %q allowed", op)
		}
	}
	if checkSecurityRestriction("file_editor", "replace") == "" {
		t.Fatal("file_editor bypasses security role")
	}
}

func TestLiveAuthorizationRevokesCatalogAndCapturedHooks(t *testing.T) {
	initial := &config.Config{}
	initial.Docker.Enabled = true
	current := *initial
	initial.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return initial, &current }
	called := false
	dc := &DispatchContext{Cfg: initial, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SessionID: t.Name(), ExecutionHooks: &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) { called = true; return `{"status":"success"}`, true }}}
	all := dispatchCatalogSchemas(dc)
	current.Docker.Enabled = false
	setRunDiscoverToolsState(dc, all, all)
	t.Cleanup(func() { ClearDiscoverToolsState(t.Name()) })
	entry, ok := GetToolCatalogState(t.Name()).Get("docker")
	if !ok || entry.Enabled || entry.Status != ToolStatusDisabled {
		t.Fatalf("stale enabled catalog entry: %+v", entry)
	}
	for _, action := range []string{"docker", "invoke_tool"} {
		call := ToolCall{Action: action, Operation: "list", Params: map[string]interface{}{"tool_name": "docker", "arguments": map[string]interface{}{"operation": "list"}}}
		result := DispatchToolCallResult(context.Background(), &call, dc, "list containers")
		if result.Status != ToolResultDenied || called {
			t.Fatalf("revoked hook executed: %+v called=%v", result, called)
		}
	}
	current.Docker.Enabled = true
	current.Docker.ReadOnly = true
	call := ToolCall{Action: "docker", Operation: "remove"}
	if result := DispatchToolCallResult(context.Background(), &call, dc, "remove container"); result.Status != ToolResultDenied || called {
		t.Fatalf("captured hook retained write permission: %+v", result)
	}
}
