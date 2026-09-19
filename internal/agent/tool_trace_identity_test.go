package agent

import "testing"

func TestToolTraceIdentityPreservesDynamicTargets(t *testing.T) {
	calls := []ToolCall{
		{Action: "send_email"}, {Action: "fetch_email"},
		{Action: "mcp_call", Params: map[string]interface{}{"server": "a", "tool_name": "read"}},
		{Action: "mcp_call", Params: map[string]interface{}{"server": "b", "tool_name": "read"}},
		{Action: "mcp_call", Params: map[string]interface{}{"server": "a", "tool_name": "write"}},
		{Action: "execute_skill", Skill: "one"}, {Action: "execute_skill", Skill: "two"},
		{Action: "run_tool", Name: "one.py"}, {Action: "run_tool", Name: "two.py"},
	}
	seen := map[string]bool{}
	for _, call := range calls {
		id := toolTraceActionIdentity(call)
		if id == "" || seen[id] {
			t.Fatalf("target identity collapsed: %+v => %q", call, id)
		}
		seen[id] = true
	}
	if toolTraceActionIdentity(ToolCall{Action: "mcp_call"}) != "" {
		t.Fatal("unresolved MCP target became comparison evidence")
	}
	for _, key := range []string{"tool_slug", "tool_name", "name"} {
		call := ToolCall{Action: "composio_call", Params: map[string]interface{}{key: "MAIL_READ"}}
		if got := toolTraceActionIdentity(call); got != `["composio_call","MAIL_READ"]` {
			t.Fatal(got)
		}
	}
}
