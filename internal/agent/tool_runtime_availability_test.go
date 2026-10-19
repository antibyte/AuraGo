package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
)

func TestMissingDependencyAgreesAcrossDiscoveryAndDispatch(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		for _, policyEnabled := range []bool{true, false} {
			for _, warmCatalog := range []bool{true, false} {
				t.Run(fmt.Sprintf("enabled=%t/policy=%t/catalog=%t", enabled, policyEnabled, warmCatalog), func(t *testing.T) {
					cfg := &config.Config{}
					cfg.SQLConnections.Enabled = enabled
					cfg.Agent.ActionPolicy.Enabled = policyEnabled
					cfg.Directories.SkillsDir = t.TempDir()
					hookCalls := 0
					dc := &DispatchContext{Cfg: cfg, SessionID: t.Name(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), ExecutionHooks: &ExecutionHooks{
						BeforeTool: func(context.Context, ToolCall) error { hookCalls++; return nil },
					}}
					// Use the actual dispatch catalog without fabricated database/pool
					// handles or an administrative-only schema snapshot.
					schemas := dispatchCatalogSchemas(dc)
					SetDiscoverToolsState(dc.SessionID, schemas, schemas, "", dc)
					t.Cleanup(func() { ClearDiscoverToolsState(dc.SessionID) })
					entry, ok := GetToolCatalogState(dc.SessionID).Get("sql_query")
					if !ok {
						t.Fatal("SQL tool missing from discovery catalog")
					}
					discovery := discoverResultFromEntry(entry, dc.SessionID, false)
					wantStatus, wantCode := ToolResultDenied, "tool_disabled"
					wantDiscovery := "disabled"
					if enabled {
						wantStatus, wantCode, wantDiscovery = ToolResultNeedsSetup, "runtime_dependency_unavailable", "needs_setup"
					}
					if discovery.CallableNow || discovery.ToolStatus != wantDiscovery || enabled && discovery.CallMethod != "needs_setup" {
						t.Fatalf("misleading discovery: %+v", discovery)
					}
					if !warmCatalog {
						ClearDiscoverToolsState(dc.SessionID)
					}
					for _, action := range []string{"sql_query", "manage_sql_connections"} {
						for _, transport := range []string{"direct", "invoke"} {
							// Missing setup must block before even a destructive
							// operation can enter the approval queue.
							call := ToolCall{Action: action, Params: map[string]interface{}{"query": "SELECT 1"}}
							if action == "manage_sql_connections" {
								call.Operation, call.Params = "delete", map[string]interface{}{"operation": "delete"}
							}
							if transport == "invoke" {
								call = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": action, "arguments": call.Params}}
							}
							if _, decision := PreviewActionPolicy(call, dc); decision != "denied" {
								t.Fatalf("%s %s preview = %s", transport, action, decision)
							}
							result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
							var refusal struct {
								Status ToolResultStatus `json:"status"`
								Code   string           `json:"code"`
							}
							decodeToolOutputJSON(t, result.Output, &refusal)
							if result.Status != wantStatus || !result.IsError || classifyLegacyToolResult(result.Output) != wantStatus || (enabled || transport == "direct") && (refusal.Status != wantStatus || refusal.Code != wantCode) {
								t.Fatalf("%s %s: result=%+v refusal=%+v want %s/%s", transport, action, result, refusal, wantStatus, wantCode)
							}
						}
						if output := dispatchInner(context.Background(), ToolCall{Action: action}, dc); classifyLegacyToolResult(output) != wantStatus {
							t.Fatalf("raw %s: %s", action, output)
						}
					}
					if hookCalls != 0 || len(PendingActionApprovals(dc.SessionID)) != 0 {
						t.Fatalf("unavailable tool reached execution/approval: hooks=%d", hookCalls)
					}
				})
			}
		}
	}
}

func TestMissingSQLRuntimeCannotBeReplacedByExecutionHook(t *testing.T) {
	cfg := &config.Config{}
	cfg.SQLConnections.Enabled, cfg.Agent.ActionPolicy.Enabled = true, true
	cfg.Directories.SkillsDir = t.TempDir()
	calls := 0
	dc := &DispatchContext{Cfg: cfg, SessionID: t.Name(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), ExecutionHooks: &ExecutionHooks{
		HandleTool: func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true },
	}}
	for _, transport := range []string{"direct", "invoke"} {
		call := ToolCall{Action: "sql_query"}
		if transport == "invoke" {
			call = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "sql_query", "arguments": map[string]interface{}{"query": "SELECT 1"}}}
		}
		result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
		if result.Status != ToolResultNeedsSetup || !result.IsError || calls != 0 {
			t.Fatalf("%s missing SQL runtime reached its hook: %+v calls=%d", transport, result, calls)
		}
	}
}
