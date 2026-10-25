package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

// reinjectionFixture publishes a two-tool catalog in which only filesystem is
// active, so docker is a hidden native tool.
func reinjectionFixture(t *testing.T) (string, []openai.Tool) {
	t.Helper()
	sid := t.Name()
	schemas := []openai.Tool{testToolSchema("docker", "Docker containers"), testToolSchema("filesystem", "Files")}
	SetDiscoverToolsState(sid, schemas, schemas[1:], "")
	t.Cleanup(func() { ClearDiscoverToolsState(sid) })
	return sid, schemas
}

func TestGetToolInfoDirectOnlyWhenRunReinjects(t *testing.T) {
	sid, _ := reinjectionFixture(t)
	info := func(dc *DispatchContext, name string) DiscoverToolResult {
		t.Helper()
		out := handleDiscoverToolsContext(context.Background(), ToolCall{Params: map[string]interface{}{"operation": "get_tool_info", "tool_name": name}}, &config.Config{}, nil, sid, dc)
		var response DiscoverToolsResponse
		decodeToolOutputJSON(t, out, &response)
		if response.Tool == nil {
			t.Fatalf("no tool in %s", out)
		}
		return *response.Tool
	}

	hidden := info(&DispatchContext{SessionID: sid, discoveryReinjects: true}, "docker")
	if hidden.CallMethod != "direct" || !hidden.CallableNow || !strings.Contains(hidden.Instruction, "call docker directly") {
		t.Fatalf("re-injecting run: %+v", hidden)
	}
	// The queued schema can still be dropped (caps, budget, a finalizing step),
	// so the instruction promises a queue entry, not an attachment, and names
	// the fallback.
	if !strings.Contains(hidden.Instruction, "queued for your next request") || !strings.Contains(hidden.Instruction, "If it is not in your tool list then, call invoke_tool with tool_name=docker.") {
		t.Fatalf("direct instruction overpromises or lacks the fallback: %q", hidden.Instruction)
	}
	fixed := info(&DispatchContext{SessionID: sid}, "docker")
	if fixed.CallMethod != "invoke_tool" || !strings.Contains(fixed.Instruction, "invoke_tool") || strings.Contains(fixed.Instruction, "directly") {
		t.Fatalf("run with a fixed tool list: %+v", fixed)
	}
	// An out-of-scope run must not be told to call a tool it may not call.
	scoped := &DispatchContext{SessionID: sid, discoveryReinjects: true, ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet([]string{"filesystem"})}
	if got := info(scoped, "docker"); got.CallMethod == "direct" {
		t.Fatalf("out-of-scope tool advertised as direct: %+v", got)
	}
}

func TestDirectHiddenNativeCallRequestsReinjection(t *testing.T) {
	for _, test := range []struct {
		name   string
		dc     DispatchContext
		action string
		want   []string
	}{
		{"hidden native, re-injecting run", DispatchContext{discoveryReinjects: true}, "docker", []string{"docker"}},
		{"run with a fixed tool list", DispatchContext{}, "docker", nil},
		{"already active tool", DispatchContext{discoveryReinjects: true}, "filesystem", nil},
		{"tool outside the catalog", DispatchContext{discoveryReinjects: true}, "fixture_missing", nil},
		{"tool outside the scope", DispatchContext{discoveryReinjects: true, ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet([]string{"filesystem"})}, "docker", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			sid, _ := reinjectionFixture(t)
			dc := test.dc
			dc.SessionID = sid
			if got := prepareToolCall(ToolCall{Action: test.action}, &dc); got.Action != test.action || got.PreparationError != "" {
				t.Fatalf("preparation changed the call: %+v", got)
			}
			if got := GetDiscoverRequestedTools(sid); strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Fatalf("requested = %v, want %v", got, test.want)
			}
		})
	}
}
