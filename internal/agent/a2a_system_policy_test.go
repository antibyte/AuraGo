package agent

import (
	"context"
	"slices"
	"strings"
	"testing"

	"aurago/internal/config"
	"github.com/sashabaranov/go-openai"
)

func TestA2ASystemToolsReadOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowShell, cfg.Agent.AllowPackageManager, cfg.PackageManager.Enabled = true, true, true
	cfg.Agent.ActionPolicy.Enabled = true
	cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: "*", Operation: "*", Target: "*"}}
	peer := &DispatchContext{Cfg: cfg, SessionID: "a2a-system-test", IsCoAgent: true, MessageSource: "a2a"}
	for tool, ops := range map[string][]string{
		"service_manager": {"status", "start", "stop", "restart", "enable", "disable", "", "unknown"},
		"package_manager": {"detect", "search", "list_installed", "info", "install", "remove", "update", "upgrade", "", "unknown"},
		"install_package": {"", "info"},
	} {
		for _, op := range ops {
			t.Run(tool+"/"+op, func(t *testing.T) {
				read := tool == "service_manager" && op == "status" || tool == "package_manager" && oneOf(op, "detect", "search", "list_installed", "info")
				call := ToolCall{Action: tool, Operation: op, Params: map[string]interface{}{"operation": op, "service": "fixture", "package": "fixture"}}
				action := DescribeAction(call, peer)
				cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: action.Tool, Operation: action.Operation, Target: action.Target, ArgumentsHash: action.ArgumentsHash}}
				if !actionAllowRulesCover(cfg.Agent.ActionPolicy.Allow, action) {
					t.Fatal("fixture must have a matching allow rule")
				}
				if got := roleToolRestriction(call, peer); (got == "") != read {
					t.Fatalf("read=%v, role result=%s", read, got)
				}
				result := checkDispatchActionPolicy(context.Background(), call, peer)
				if read && tool == "package_manager" && result != nil || !read && (result == nil || !strings.Contains(result.Output, "policy_denied")) {
					t.Fatalf("read=%v, policy result=%+v", read, result)
				}
				for _, local := range []*DispatchContext{{SessionID: "default"}, {SessionID: "coagent-local", IsCoAgent: true}} {
					if got := roleToolRestriction(call, local); got != "" {
						t.Fatalf("local role changed: %s", got)
					}
				}
				if tool == "package_manager" {
					a := DescribeAction(call, peer)
					if a.Mutating == read || a.HighRisk == read {
						t.Fatalf("incorrect package classification: %+v", a)
					}
				}
			})
		}
	}
	schemas := BuildNativeToolSchemas("", nil, ToolFeatureFlags{AllowShell: true, PackageManagerEnabled: true}, nil)
	// The legacy service dispatcher has no native schema in the default catalog.
	// Any catalog exposing it must still apply the same operation restriction.
	schemas = append(schemas, openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
		Name: "service_manager", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
			"operation": map[string]interface{}{"type": "string", "enum": []string{"status", "start", "stop"}},
		}},
	}})
	catalog := BuildToolCatalog(schemas, nil, "")
	applyCatalogEntryPolicies(catalog, peer)
	for name, want := range map[string][]string{"service_manager": {"status"}, "package_manager": {"detect", "search", "list_installed", "info"}} {
		entry, ok := catalog.Get(name)
		if !ok || !entry.Enabled || !slices.Equal(toolCatalogOperationField(entry, "operation"), want) {
			t.Fatalf("%s discovery mismatch: %+v", name, entry)
		}
	}
	if entry, ok := catalog.Get("install_package"); ok && entry.Enabled {
		t.Fatal("package installer exposed to peer")
	}
}
