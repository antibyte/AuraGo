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

func TestSpecialistExecutionFamilies(t *testing.T) {
	calls := []struct {
		name    string
		call    ToolCall
		allowed string
	}{
		{"shell", ToolCall{Action: "execute_shell"}, "coder security"},
		{"sudo", ToolCall{Action: "execute_sudo"}, "coder security"},
		{"remote", ToolCall{Action: "execute_remote_shell"}, ""},
		{"python", ToolCall{Action: "execute_python"}, "researcher coder security"},
		{"saved python", ToolCall{Action: "run_tool"}, "researcher coder security"},
		{"sandbox default", ToolCall{Action: "execute_sandbox"}, "researcher coder security"},
		{"sandbox python", ToolCall{Action: "execute_sandbox", Params: map[string]interface{}{"language": "python"}}, "researcher coder security"},
		{"sandbox bash", ToolCall{Action: "execute_sandbox", Language: "bash"}, "coder security"},
		{"sandbox javascript", ToolCall{Action: "execute_sandbox", SandboxLang: "javascript"}, "coder security"},
		{"sandbox unknown", ToolCall{Action: "execute_sandbox", Language: "unknown"}, "coder security"},
		{"script python", ToolCall{Action: "run_agent_skill_script", Params: map[string]interface{}{"script": "scripts/check.py"}}, "researcher coder security"},
		{"script bash", ToolCall{Action: "run_agent_skill_script", FilePath: "scripts/check.sh"}, "coder security"},
		{"script javascript", ToolCall{Action: "run_agent_skill_script", Path: "scripts/check.js"}, "coder security"},
		{"script missing", ToolCall{Action: "run_agent_skill_script"}, ""},
		{"script unknown", ToolCall{Action: "run_agent_skill_script", Path: "scripts/check.exe"}, ""},
		{"registered skill", ToolCall{Action: "execute_skill", Skill: "ddg_search"}, "researcher coder designer security"},
		{"read", ToolCall{Action: "filesystem", Operation: "read"}, "researcher coder designer security"},
	}
	for _, role := range []string{"researcher", "coder", "designer", "security", "writer"} {
		for _, item := range calls {
			t.Run(role+"/"+item.name, func(t *testing.T) {
				dc := &DispatchContext{IsCoAgent: true, CoAgentSpecialist: role}
				denied := roleToolRestriction(item.call, dc)
				if (denied == "") != strings.Contains(item.allowed, role) {
					t.Fatalf("unexpected role decision: %s", denied)
				}
			})
		}
	}
}

func TestSpecialistDiscoveryMatchesExecutionCapabilities(t *testing.T) {
	for _, role := range []string{"researcher", "coder", "designer", "security", "writer"} {
		dc := &DispatchContext{Cfg: &config.Config{}, IsCoAgent: true, CoAgentSpecialist: role}
		var schemas []openai.Tool
		for _, name := range []string{"execute_sudo", "execute_remote_shell", "execute_python", "execute_sandbox", "run_tool", "run_agent_skill_script", "system_metrics", "tool__saved.py", "skill__ddg_search", "mcp_call"} {
			schemas = append(schemas, testToolSchema(name, name))
		}
		catalog := newToolCatalog(schemas, schemas, "", false)
		applyCatalogEntryPolicies(catalog, dc)
		for _, entry := range catalog.Entries() {
			call := ToolCall{Action: catalogEntryAction(entry), FilePath: "scripts/check.py"}
			if (roleToolRestriction(call, dc) == "") != entry.Enabled {
				t.Fatalf("%s/%s discovery diverged: %s", role, entry.Name, entry.HiddenReason)
			}
		}
	}
}

func TestSpecialistExecutionDenialsPrecedeHandlersAcrossWrappers(t *testing.T) {
	for _, role := range []string{"researcher", "designer", "writer"} {
		for _, form := range []string{"direct", "alias", "invoke", "skill"} {
			t.Run(role+"/"+form, func(t *testing.T) {
				cfg := &config.Config{}
				cfg.Agent.AllowShell, cfg.Agent.AllowPython, cfg.Agent.SudoEnabled = true, true, true
				cfg.Agent.AllowRemoteShell, cfg.Sandbox.Enabled = true, true
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				called := false
				dc := &DispatchContext{Cfg: cfg, Logger: logger, SessionID: t.Name(), IsCoAgent: true, CoAgentSpecialist: role,
					ExecutionHooks: &ExecutionHooks{BeforeTool: func(context.Context, ToolCall) error { called = true; return nil }, HandleTool: func(context.Context, ToolCall) (string, bool) { return "unexpected execution", true }},
				}
				call := ToolCall{Action: "execute_sudo", Command: "echo fixture"}
				switch form {
				case "alias":
					call = ToolCall{Action: "exec", Command: "echo fixture"}
				case "invoke":
					setRunDiscoverToolsState(dc, dispatchCatalogSchemas(dc), nil)
					defer ClearDiscoverToolsState(dc.SessionID)
					call = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "execute_sudo", "arguments": map[string]interface{}{"command": "echo fixture"}}}
				case "skill":
					call = ToolCall{Action: "execute_skill", Skill: "execute_sudo", SkillArgs: map[string]interface{}{"command": "echo fixture"}}
				}
				result := DispatchToolCallResult(t.Context(), &call, dc, "inspect")
				if called || !result.IsError {
					t.Fatalf("%s reached handler: %+v", form, result)
				}
			})
		}
	}
}

func TestSpecialistResearcherPythonDispatchAcrossWrappers(t *testing.T) {
	for _, form := range []string{"direct", "invoke", "skill"} {
		t.Run(form, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Agent.AllowPython = true
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			started := 0
			dc := &DispatchContext{Cfg: cfg, Logger: logger, SessionID: t.Name(), IsCoAgent: true, CoAgentSpecialist: "researcher",
				ExecutionHooks: &ExecutionHooks{BeforeTool: func(context.Context, ToolCall) error { started++; return nil }, HandleTool: func(_ context.Context, call ToolCall) (string, bool) {
					if call.Action != "execute_python" {
						t.Errorf("unexpected execution capability: %s", call.Action)
					}
					return "Tool Output: 1", true
				}},
			}
			call := ToolCall{Action: "execute_python", Code: "print(1)"}
			switch form {
			case "invoke":
				setRunDiscoverToolsState(dc, dispatchCatalogSchemas(dc), nil)
				defer ClearDiscoverToolsState(dc.SessionID)
				call = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "execute_python", "arguments": map[string]interface{}{"code": "print(1)"}}}
			case "skill":
				call = ToolCall{Action: "execute_skill", Skill: "execute_python", SkillArgs: map[string]interface{}{"code": "print(1)"}}
			}
			result := DispatchToolCallResult(t.Context(), &call, dc, "Process the supplied data")
			if started != 1 || result.IsError {
				t.Fatalf("Python permission was lost: starts=%d result=%+v", started, result)
			}
		})
	}
}
