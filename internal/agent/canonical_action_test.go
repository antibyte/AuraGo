package agent

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestCanonicalVirusTotalWrapperCannotBypassUploadApproval(t *testing.T) {
	for _, shape := range []string{"direct", "skill args", "nested params", "native shortcut", "shortcut-only catalog", "invoke wrapper"} {
		t.Run(shape, func(t *testing.T) {
			calls := 0
			dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
			dc.Cfg.VirusTotal.Enabled = true
			args := map[string]interface{}{"mode": "upload", "file_path": "fixture-upload.bin"}
			call := toolCallFromInvokeArgs("virustotal_scan", args)
			switch shape {
			case "skill args":
				call = ToolCall{Action: "execute_skill", Skill: "virustotal_scan.py", SkillArgs: args}
			case "nested params":
				call = ToolCall{Action: "execute_skill", Params: map[string]interface{}{"skill_name": "virustotal_scan", "skill_args": args}}
			case "native shortcut":
				call = NativeToolCallToToolCall(openai.ToolCall{ID: "fixture-native-call", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "skill__virustotal_scan", Arguments: `{"skill_args":{"mode":"upload","file_path":"fixture-upload.bin"}}`}}, dc.Logger)
			case "shortcut-only catalog":
				call = ToolCall{Action: "execute_skill", Skill: "virustotal_scan", SkillArgs: args}
				setRunDiscoverToolsState(dc, []openai.Tool{testToolSchema("skill__virustotal_scan", "fixture")}, nil)
				t.Cleanup(func() { ClearDiscoverToolsState(dc.discoveryKey()) })
			case "invoke wrapper":
				call = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "execute_skill", "arguments": map[string]interface{}{"skill": "virustotal_scan", "skill_args": args}}}
				// The wrapper is intentionally present in this run's complete catalog,
				// while the leaf is absent from the adaptive presentation snapshot.
				dc.Cfg.Agent.AllowPython = true
				all := dispatchCatalogSchemas(dc)
				setRunDiscoverToolsState(dc, all, nil)
				t.Cleanup(func() { ClearDiscoverToolsState(dc.discoveryKey()) })
			}
			call.NativeCallID = "fixture-native-call"
			approval := queueSecurityApproval(t, dc, &call)
			if calls != 0 || call.Action != "virustotal_scan" || approval.Action.Tool != "virustotal_scan" || !approval.Action.HighRisk || call.NativeCallID != "fixture-native-call" {
				t.Fatalf("wrapper bypassed canonical leaf policy: action=%s approval=%+v calls=%d", call.Action, approval.Action, calls)
			}
			if shape == "invoke wrapper" && call.TransportAction != "invoke_tool" {
				t.Fatal("outer invocation scope was lost")
			}
			if again := prepareToolCall(call, dc); !reflect.DeepEqual(again, call) {
				t.Fatal("canonical preparation is not idempotent")
			}
		})
	}
}

func TestCanonicalNativeSkillWrapperRequiresWrapperAndLeafScope(t *testing.T) {
	for _, allowed := range [][]string{{"execute_skill"}, {"virustotal_scan"}, {"execute_skill", "virustotal_scan"}} {
		t.Run(strings.Join(allowed, "+"), func(t *testing.T) {
			calls := 0
			dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
			dc.Cfg.VirusTotal.Enabled = true
			dc.ToolScopeRestricted, dc.AllowedTools = true, normalizedAllowedToolSet(allowed)
			call := ToolCall{Action: "execute_skill", Skill: "virustotal_scan", SkillArgs: map[string]interface{}{"mode": "hash", "resource": "fixture-hash"}}
			result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
			if len(allowed) == 2 {
				if result.Status != ToolResultSuccess || calls != 1 || call.Action != "virustotal_scan" {
					t.Fatalf("valid native wrapper failed: %+v calls=%d", result, calls)
				}
			} else if result.Status != ToolResultDenied || calls != 0 {
				t.Fatalf("wrapper widened scope: %+v calls=%d", result, calls)
			}
		})
	}
}

func TestCanonicalNativeWrapperRevalidatesDisabledIntegration(t *testing.T) {
	for _, invoked := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct wrapper", true: "invoke wrapper"}[invoked], func(t *testing.T) {
			calls := 0
			dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
			dc.Cfg.VirusTotal.Enabled, dc.Cfg.Agent.AllowPython = true, true
			initial := dc.Cfg
			current := *initial
			initial.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return initial, &current }
			setRunDiscoverToolsState(dc, dispatchCatalogSchemas(dc), nil)
			t.Cleanup(func() { ClearDiscoverToolsState(dc.discoveryKey()) })
			current.VirusTotal.Enabled = false
			call := ToolCall{Action: "execute_skill", Skill: "virustotal_scan", SkillArgs: map[string]interface{}{"mode": "hash", "resource": "fixture-hash"}}
			if invoked {
				call = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "execute_skill", "arguments": map[string]interface{}{"skill": "virustotal_scan", "skill_args": call.SkillArgs}}}
			}
			result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
			if result.Status != ToolResultDenied || calls != 0 {
				t.Fatalf("revoked wrapper executed: %+v calls=%d", result, calls)
			}
		})
	}
}

func TestCanonicalAliasesReachLeafBeforeHooksAndScopes(t *testing.T) {
	for _, name := range []string{"register_server", "remote_execution", "get_system_metrics"} {
		t.Run(name, func(t *testing.T) {
			var executed ToolCall
			dc := actionSecurityFixture(t, func(_ context.Context, call ToolCall) (string, bool) {
				executed = call
				return `{"status":"success"}`, true
			})
			dc.Cfg.Tools.Inventory.Enabled, dc.Cfg.Agent.AllowRemoteShell = true, true
			leaf := canonicalNativeAlias(name)
			dc.Cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: leaf, Operation: "*", Target: "*"}}
			dc.ToolScopeRestricted, dc.AllowedTools = true, normalizedAllowedToolSet([]string{leaf})
			call := ToolCall{Action: name, NativeCallID: "fixture-call", Params: map[string]interface{}{"command": "fixture-command", "server_id": "fixture-server"}}
			if name == "remote_execution" {
				call = NativeToolCallToToolCall(openai.ToolCall{ID: "fixture-call", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: name, Arguments: `{"command":"fixture-command","server_id":"fixture-server"}`}}, dc.Logger)
			}
			result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
			if result.Status != ToolResultSuccess || executed.Action != leaf || executed.NativeCallID != "fixture-call" {
				t.Fatalf("alias missed leaf: %+v executed=%s", result, executed.Action)
			}
		})
	}
}

func TestCanonicalGitAliasesHaveFixedCommandsAndNoBroadScopeGrant(t *testing.T) {
	for name, command := range map[string]string{"git_status": "git status", "git_log": "git log --oneline -10", "git_diff": "git diff", "git_push": "git push", "git_pull": "git pull", "git_rollback": "git reset --hard HEAD~1"} {
		t.Run(name, func(t *testing.T) {
			call := prepareToolCall(ToolCall{Action: name, Params: map[string]interface{}{"command": "attacker command", "operation": "logs"}}, nil)
			if call.Action != "execute_shell" || call.Command != command || call.Params["command"] != command || call.TransportAction != name {
				t.Fatalf("bad fixed alias: %+v", call)
			}
			dc := &DispatchContext{ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet([]string{name})}
			if dispatchToolAllowed(dc, "execute_shell") {
				t.Fatal("narrow git alias scope granted arbitrary shell")
			}
			invoked := prepareToolCall(ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": name, "arguments": map[string]interface{}{}}}, &DispatchContext{})
			if invoked.PreparationError == "" {
				t.Fatal("non-catalog git alias accepted through invoke_tool")
			}
		})
	}
	call := prepareToolCall(ToolCall{Action: "git_commit", Message: "fixture ' message", Path: "fixture ' directory"}, nil)
	if !strings.Contains(call.Command, `cd 'fixture '"'"' directory' && git add -A && git commit -m 'fixture '"'"' message'`) {
		t.Fatalf("git arguments lost quoting: %s", call.Command)
	}
}

func TestCanonicalPreparationPreservesRealSkillsAndAmbiguousFileNames(t *testing.T) {
	dc := actionSecurityFixture(t, nil)
	for _, name := range []string{"list_files", "read_file", "write_file"} {
		call := prepareToolCall(ToolCall{Action: name, FilePath: "fixture-file"}, dc)
		if call.Action != name {
			t.Fatalf("ambiguous file action %q redirected to %q", name, call.Action)
		}
	}
	call := ToolCall{Action: "execute_skill", Skill: "fixture_python_skill", SkillArgs: map[string]interface{}{"query": "fixture"}}
	if got := prepareToolCall(call, dc); !reflect.DeepEqual(got, call) {
		t.Fatal("ordinary Python skill was rewritten")
	}
	call = ToolCall{Action: "execute_skill", Skill: "pdf_extractor", SkillArgs: map[string]interface{}{"filepath": "fixture.pdf"}}
	if got := prepareToolCall(call, dc); !reflect.DeepEqual(got, call) {
		t.Fatal("skill-only PDF builtin was rewritten to a nonexistent native leaf")
	}
}

func TestCanonicalInvokeUsesFullEnabledCatalogDespiteAdaptiveSubset(t *testing.T) {
	calls := 0
	dc := actionSecurityFixture(t, func(_ context.Context, call ToolCall) (string, bool) {
		calls++
		if call.Action != "docker" {
			t.Errorf("hook action = %s", call.Action)
		}
		return `{"status":"success"}`, true
	})
	// Simulate an older/minimal adaptive publisher that stored only the visible
	// subset as allSchemas. The actual enabled catalog still contains Docker.
	setRunDiscoverToolsState(dc, []openai.Tool{testToolSchema("invoke_tool", "fixture")}, nil)
	t.Cleanup(func() { ClearDiscoverToolsState(dc.discoveryKey()) })
	call := ToolCall{Action: "invoke_tool", NativeCallID: "fixture-invoke", Params: map[string]interface{}{"tool_name": "docker", "arguments": map[string]interface{}{"operation": "list"}}}
	result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
	if result.Status != ToolResultSuccess || calls != 1 || call.NativeCallID != "fixture-invoke" || call.TransportAction != "invoke_tool" {
		t.Fatalf("adaptive subset hid enabled tool: %+v calls=%d", result, calls)
	}
}

func TestCanonicalVirusTotalAutoAndOmittedModeStayReadOnly(t *testing.T) {
	for _, mode := range []string{"", "auto", "hash"} {
		t.Run("mode="+mode, func(t *testing.T) {
			calls := 0
			dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
			dc.Cfg.VirusTotal.Enabled = true
			call := ToolCall{Action: "execute_skill", Skill: "virustotal_scan", SkillArgs: map[string]interface{}{"mode": mode, "file_path": "fixture.bin"}}
			result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
			action := DescribeAction(call, dc)
			if result.Status != ToolResultSuccess || calls != 1 || action.Mutating || action.HighRisk || len(PendingActionApprovals(dc.SessionID)) != 0 {
				t.Fatalf("read-only VT lookup required approval: %+v action=%+v", result, action)
			}
		})
	}
	for _, schema := range builtinToolSchemas(ToolFeatureFlags{VirusTotalEnabled: true}) {
		if schema.Function == nil || schema.Function.Name != "virustotal_scan" {
			continue
		}
		props, _ := schemaParameters(schema)["properties"].(map[string]interface{})
		mode, _ := props["mode"].(map[string]interface{})
		description, _ := mode["description"].(string)
		if !strings.Contains(description, "never submit") || strings.Contains(description, "upload if unknown") {
			t.Fatalf("VT schema advertises an implicit upload: %s", description)
		}
		return
	}
	t.Fatal("VirusTotal schema missing")
}

func TestCanonicalInventoryAndBrowserWrappersMatchNativeScope(t *testing.T) {
	for _, leaf := range []string{"register_device", "browser_automation"} {
		t.Run(leaf, func(t *testing.T) {
			calls := 0
			dc := actionSecurityFixture(t, func(_ context.Context, call ToolCall) (string, bool) {
				calls++
				if call.Action != leaf {
					t.Errorf("wrong leaf %s", call.Action)
				}
				return `{"status":"success"}`, true
			})
			dc.Cfg.Tools.Inventory.Enabled = true
			dc.Cfg.BrowserAutomation.Enabled, dc.Cfg.Tools.BrowserAutomation.Enabled = true, true
			dc.Cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: leaf, Operation: "*", Target: "*"}}
			dc.ToolScopeRestricted, dc.AllowedTools = true, normalizedAllowedToolSet([]string{"execute_skill", leaf})
			call := ToolCall{Action: "execute_skill", Skill: leaf, SkillArgs: map[string]interface{}{"operation": "navigate", "url": "https://example.invalid/fixture", "hostname": "fixture-device"}}
			result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
			if result.Status != ToolResultSuccess || calls != 1 {
				t.Fatalf("native wrapper failed: %+v calls=%d", result, calls)
			}
			dc.AllowedTools = normalizedAllowedToolSet([]string{"execute_skill"})
			call = ToolCall{Action: "execute_skill", Skill: leaf, SkillArgs: map[string]interface{}{"operation": "navigate"}}
			result = DispatchToolCallResult(context.Background(), &call, dc, "fixture")
			if result.Status != ToolResultDenied || calls != 1 {
				t.Fatalf("wrapper escaped leaf scope: %+v calls=%d", result, calls)
			}
		})
	}
}

func TestCanonicalWrapperPreservesNamesDeclaredByLeafSchema(t *testing.T) {
	var executed ToolCall
	dc := actionSecurityFixture(t, func(_ context.Context, call ToolCall) (string, bool) {
		executed = call
		return `{"status":"success"}`, true
	})
	call := ToolCall{Action: "execute_skill", Skill: "docker", SkillArgs: map[string]interface{}{"operation": "create", "name": "fixture-container"}}
	result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
	if result.Status != ToolResultSuccess || executed.Action != "docker" || executed.Params["name"] != "fixture-container" {
		t.Fatalf("wrapper stripped a declared native argument: %+v params=%v", result, executed.Params)
	}
}

func TestCanonicalPaperlessWrapperCannotOverrideReadonly(t *testing.T) {
	for _, route := range []string{"direct", "alias", "wrapper", "invoke"} {
		for _, operation := range []string{"upload", "search"} {
			t.Run(route+"/"+operation, func(t *testing.T) {
				calls := 0
				dc := actionSecurityFixture(t, func(context.Context, ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
				dc.Cfg.PaperlessNGX.Enabled, dc.Cfg.PaperlessNGX.ReadOnly = true, true
				dc.Cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: "paperless_ngx", Operation: "*", Target: "*"}}
				args := map[string]interface{}{"operation": operation, "content": "fixture-document"}
				call := ToolCall{Action: "paperless_ngx", Params: args}
				switch route {
				case "alias":
					call.Action = "paperless"
				case "wrapper":
					call = ToolCall{Action: "execute_skill", Skill: "paperless", SkillArgs: args}
				case "invoke":
					call = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "paperless_ngx", "arguments": args}}
				}
				result := DispatchToolCallResult(context.Background(), &call, dc, "fixture")
				wantStatus, wantCalls := ToolResultDenied, 0
				if operation == "search" {
					wantStatus, wantCalls = ToolResultSuccess, 1
				}
				if result.Status != wantStatus || calls != wantCalls || call.Action != "paperless_ngx" {
					t.Fatalf("paperless readonly contract: %+v action=%s calls=%d, want %s/%d", result, call.Action, calls, wantStatus, wantCalls)
				}
			})
		}
	}
}
