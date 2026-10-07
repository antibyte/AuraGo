package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestRunToolAvailabilityFollowsPythonGate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, enabled := range []bool{false, true} {
		cfg := &config.Config{}
		cfg.Agent.AllowPython = enabled
		cfg.Directories.SkillsDir = t.TempDir()
		schemas := BuildNativeToolSchemaSnapshot(cfg.Directories.SkillsDir, nil, ToolFeatureFlags{AllowPython: enabled}, logger)
		selected := filterToolSchemasWithReport(schemas.FullSchemas(), toolSchemaFilterOptions{
			HardAlwaysTools: hardAlwaysToolNames(cfg), SoftAlwaysTools: []string{"run_tool"},
			PreferredTools: []string{"run_tool"}, MaxAdaptiveTools: 4, MaxTotalTools: 10,
		}, logger).Tools
		for group, definitions := range map[string][]openai.Tool{"full": schemas.FullSchemas(), "strict": schemas.StrictSchemas(), "adaptive": selected, "looper": GetLooperToolSchemas(cfg)} {
			found, skill := false, false
			for _, definition := range definitions {
				if definition.Function != nil {
					found = found || definition.Function.Name == "run_tool"
					skill = skill || definition.Function.Name == "execute_skill"
				}
			}
			if found != enabled || !skill {
				t.Fatalf("python=%v %s: run_tool=%v execute_skill=%v", enabled, group, found, skill)
			}
		}
		catalog := BuildToolCatalog(schemas.FullSchemas(), selected, "")
		entry, ok := catalog.Get("run_tool")
		if !ok {
			t.Fatal("disabled run_tool vanished from discovery")
		}
		result := discoverResultFromEntry(entry, t.Name(), false)
		if result.CallableNow != enabled || entry.Enabled != enabled {
			t.Fatalf("python=%v: %+v", enabled, result)
		}
		if !enabled && (entry.Active || entry.Status != ToolStatusDisabled || result.CallMethod != "disabled") {
			t.Fatalf("misleading disabled catalog: %+v", result)
		}
		if !enabled {
			dc := &DispatchContext{Cfg: cfg, Logger: logger, SessionID: t.Name()}
			SetDiscoverToolsState(dc.SessionID, schemas.FullSchemas(), selected, "", dc)
			t.Cleanup(func() { ClearDiscoverToolsState(dc.SessionID) })
			for _, action := range []string{"run_tool", "invoke_tool"} {
				call := ToolCall{Action: action, Name: "fixture.py", Params: map[string]interface{}{"tool_name": "run_tool", "arguments": map[string]interface{}{"name": "fixture.py"}}}
				if got := DispatchToolCallResult(context.Background(), &call, dc, "run fixture"); got.Status != ToolResultDenied {
					t.Fatalf("disabled %s executed: %+v", action, got)
				}
			}
		}
	}
}

func TestRunToolRevocationBlocksCatalogAndExecution(t *testing.T) {
	initial := &config.Config{}
	initial.Agent.AllowPython = true
	initial.Directories.SkillsDir = t.TempDir()
	current := *initial
	initial.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return initial, &current }
	called := 0
	dc := &DispatchContext{Cfg: initial, SessionID: t.Name(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), ExecutionHooks: &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) { called++; return `{"status":"success"}`, true }}}
	all := dispatchCatalogSchemas(dc)
	setRunDiscoverToolsState(dc, all, all)
	t.Cleanup(func() { ClearDiscoverToolsState(dc.SessionID) })
	call := ToolCall{Action: "run_tool", Name: "fixture.py"}
	if result := DispatchToolCallResult(context.Background(), &call, dc, "run fixture"); result.Status != ToolResultSuccess || called != 1 {
		t.Fatalf("enabled hook not reached: %+v calls=%d", result, called)
	}
	current.Agent.AllowPython = false
	setRunDiscoverToolsState(dc, all, all) // stale captured schemas must not grant execution
	entry, _ := GetToolCatalogState(dc.SessionID).Get("run_tool")
	if entry.Enabled || entry.Active || entry.Status != ToolStatusDisabled || discoverResultFromEntry(entry, dc.SessionID, false).CallMethod != "disabled" {
		t.Fatalf("revoked catalog: %+v", entry)
	}
	for _, action := range []string{"run_tool", "invoke_tool"} {
		call := ToolCall{Action: action, Name: "fixture.py", Params: map[string]interface{}{"tool_name": "run_tool", "arguments": map[string]interface{}{"name": "fixture.py"}}}
		if result := DispatchToolCallResult(context.Background(), &call, dc, "run fixture"); result.Status != ToolResultDenied || called != 1 {
			t.Fatalf("revoked %s reached hook: %+v calls=%d", action, result, called)
		}
	}
}

func TestExecuteSkillGoBuiltinStillWorksWithoutPython(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.PDFExtractor.Enabled = true
	cfg.Directories.SkillsDir = t.TempDir()
	dc := &DispatchContext{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	out, handled := dispatchComm(context.Background(), ToolCall{Action: "execute_skill", Skill: "pdf_extractor", SkillArgs: map[string]interface{}{"filepath": "fixture.txt"}}, dc)
	if !handled || !strings.Contains(out, "not a PDF") {
		t.Fatalf("Go builtin blocked by Python gate: %s", out)
	}
}
