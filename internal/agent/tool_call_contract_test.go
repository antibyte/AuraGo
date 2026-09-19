package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestInvokeCoAgentPolicyParity(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	if err := stm.InitNotesTables(); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Tools.Notes.Enabled = true
	sid := t.Name()
	t.Cleanup(func() { ClearDiscoverToolsState(sid) })
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{NotesEnabled: true}, logger)
	SetDiscoverToolsState(sid, schemas, schemas, "")
	hookCalls := 0
	dc := &DispatchContext{Cfg: cfg, Logger: logger, ShortTermMem: stm, SessionID: sid, IsCoAgent: true,
		ExecutionHooks: &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) { hookCalls++; return `{"status":"success"}`, true }}}
	for _, hooks := range []*ExecutionHooks{nil, dc.ExecutionHooks} {
		dc.ExecutionHooks = hooks
		for _, call := range []ToolCall{
			{Action: "manage_notes", Operation: "add", Title: "fixture"},
			{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "manage_notes", "arguments": map[string]interface{}{"operation": "add", "title": "fixture"}}},
		} {
			result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
			if !result.IsError || !strings.Contains(result.Output, "Co-Agents cannot modify notes") {
				t.Fatalf("policy bypass: %+v", result)
			}
		}
	}
	notes, err := stm.ListNotes("", -1)
	if err != nil || len(notes) != 0 || hookCalls != 0 {
		t.Fatalf("mutation before policy: notes=%d hooks=%d err=%v", len(notes), hookCalls, err)
	}
}

func TestWrappedMutationSnapshotParity(t *testing.T) {
	for _, action := range []string{"manage_notes", "manage_plan", "manage_todos", "manage_memory"} {
		op := "add"
		if action == "manage_plan" {
			op = "create"
		}
		direct := ToolCall{Action: action, Operation: op}
		for _, args := range []map[string]interface{}{
			{"tool_name": action, "arguments": map[string]interface{}{"operation": op}},
			{"tool_name": action, "operation": op},
		} {
			wrapped := ToolCall{Action: "invoke_tool", Params: args}
			if got, want := turnSnapshotMutationCategories(wrapped), turnSnapshotMutationCategories(direct); got == 0 || got != want {
				t.Fatalf("%s: %v != %v", action, got, want)
			}
		}
	}
}

func TestAgentSkillScriptScopeBeforeManagerLookup(t *testing.T) {
	out := dispatchRunAgentSkillScript(context.Background(), ToolCall{Skill: "excluded", FilePath: "scripts/read.py"}, &DispatchContext{SkillScopeRestricted: true})
	if classifyLegacyToolResult(out) != ToolResultDenied {
		t.Fatal(out)
	}
}

func TestToolResultStatusSurvivesPresentation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, item := range []struct {
		output string
		status ToolResultStatus
	}{
		{`{"status":"policy_denied"}`, ToolResultDenied},
		{`{"status":"connect_required"}`, ToolResultNeedsSetup},
		{`{"success":false}`, ToolResultFailed},
		{"[Tool Output]\nTool Output: [PERMISSION DENIED] blocked", ToolResultDenied},
		{`{"status":"pending"}`, ToolResultDeferred},
		{`{"status":"success"}`, ToolResultSuccess},
		{"opaque legacy text", ToolResultUnknown},
	} {
		status := classifyLegacyToolResult(item.output)
		if status != item.status {
			t.Fatalf("%q: %s", item.output, status)
		}
		cfg := &config.Config{}
		cfg.Agent.ToolOutputLimit = 40
		result := finalizeToolExecution(context.Background(), ToolCall{Action: "fixture", DispatchStatus: status}, strings.Repeat("presentation ", 50), false, cfg, nil, t.Name(), nil, nil, logger, AgentTelemetryScope{}, "", 0, RunConfig{})
		if result.Status != status || result.Failed != status.IsError() {
			t.Fatalf("status changed: %+v", result)
		}
	}
}
