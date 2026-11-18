package agent

import (
	"context"
	"slices"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestA2AIntegrationToolsAllowInspectionOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.Docker.Enabled, cfg.Ansible.Enabled = true, true
	cfg.Agent.AllowMCP, cfg.MCP.Enabled = true, true
	cfg.Agent.ActionPolicy.Enabled = true
	peer := &DispatchContext{Cfg: cfg, IsCoAgent: true, MessageSource: "a2a"}
	local := &DispatchContext{Cfg: cfg}
	for _, tc := range []struct {
		tool, op string
		read     bool
	}{
		{"docker", "list_containers", true},
		{"docker_management", "inspect", true},
		{"docker", "create_volume", false},
		{"docker_management", "compose", false},
		{"docker", "", false},
		{"ansible", "list_inventory", true},
		{"ansible", "run_playbook", false},
		{"ansible", "future_operation", false},
		{"mcp_call", "list_tools", true},
		{"mcp_call", "call_tool", false},
	} {
		t.Run(tc.tool+"/"+tc.op, func(t *testing.T) {
			call := ToolCall{Action: tc.tool, Params: map[string]interface{}{"operation": tc.op}}
			if got := roleToolRestriction(call, peer); (got == "") != tc.read {
				t.Fatalf("peer read=%v, role result=%q", tc.read, got)
			}
			if got := roleToolRestriction(call, local); got != "" {
				t.Fatalf("local role restriction: %s", got)
			}
			if !tc.read {
				action := DescribeAction(call, peer)
				cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: action.Tool, Operation: action.Operation, Target: action.Target, ArgumentsHash: action.ArgumentsHash}}
				result := checkDispatchActionPolicy(context.Background(), call, peer)
				if result == nil || !strings.Contains(result.Output, "policy_denied") {
					t.Fatalf("denial was bypassed: %+v", result)
				}
			}
		})
	}

	schemas := BuildNativeToolSchemas("", nil, ToolFeatureFlags{DockerEnabled: true, AnsibleEnabled: true, MCPEnabled: true}, nil)
	catalog := BuildToolCatalog(schemas, nil, "")
	applyCatalogEntryPolicies(catalog, peer)
	for name, want := range map[string]string{"docker": "list_containers", "ansible": "status", "mcp_call": "list_tools"} {
		entry, ok := catalog.Get(name)
		if !ok || !entry.Enabled {
			t.Fatalf("readable tool %s missing from discovery", name)
		}
		ops := toolCatalogOperationField(entry, "operation")
		if !slices.Contains(ops, want) {
			t.Fatalf("%s missing %s in discovery: %v", name, want, ops)
		}
		for _, op := range ops {
			if roleToolRestriction(ToolCall{Action: name, Operation: op}, peer) != "" {
				t.Errorf("discovery exposed %s %s", name, op)
			}
		}
	}
}
