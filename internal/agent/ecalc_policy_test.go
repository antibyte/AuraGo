package agent

import (
	"slices"
	"strings"
	"testing"

	"aurago/internal/config"
)

// The ecalc tool is pure computation except for open: only open mutates, and an A2A peer (a remote agent whose
// request is not the owner's) may search, describe and calculate but not put a window on the owner's desktop.

func TestECalcActionPolicyOnlyOpenMutates(t *testing.T) {
	for op, want := range map[string]bool{"search": false, "describe": false, "calculate": false, "open": true} {
		a := DescribeAction(ToolCall{Action: "ecalc", Operation: op}, nil)
		if a.Mutating != want || a.HighRisk || a.Tool != "ecalc" || a.Operation != op {
			t.Errorf("%s: %+v, want mutating=%v and no high risk", op, a, want)
		}
	}
	// The operation may also come in the params of a native call.
	if a := DescribeAction(ecalcNativeCall(t, `{"operation":"open","id":"ohms_law"}`), nil); !a.Mutating {
		t.Fatalf("native open = %+v", a)
	}
	if a := DescribeAction(ecalcNativeCall(t, `{"operation":"describe","id":"ohms_law"}`), nil); a.Mutating {
		t.Fatalf("native describe = %+v", a)
	}
}

func TestECalcRoleRestrictionKeepsComputationForPeersButNotOpen(t *testing.T) {
	main := &DispatchContext{SessionID: "default"}
	local := &DispatchContext{SessionID: "coagent-1", IsCoAgent: true, MessageSource: "co_agent", CoAgentSpecialist: "coder"}
	a2a := &DispatchContext{SessionID: "a2a-task-1", IsCoAgent: true, MessageSource: "a2a"}
	prefixOnly := &DispatchContext{SessionID: "a2a-task-2", IsCoAgent: true}

	for _, op := range []string{"search", "describe", "calculate"} {
		for name, dc := range map[string]*DispatchContext{"main": main, "local": local, "a2a": a2a, "a2a prefix": prefixOnly} {
			if got := roleToolRestriction(ToolCall{Action: "ecalc", Operation: op}, dc); got != "" {
				t.Errorf("%s run is denied ecalc %s: %q", name, op, got)
			}
		}
	}
	for name, dc := range map[string]*DispatchContext{"main": main, "local": local} {
		if got := roleToolRestriction(ToolCall{Action: "ecalc", Operation: "open"}, dc); got != "" {
			t.Errorf("%s run is denied ecalc open: %q", name, got)
		}
	}
	for _, op := range []string{"open", "", "table", "OPEN_APP"} {
		for name, dc := range map[string]*DispatchContext{"a2a": a2a, "a2a prefix": prefixOnly} {
			if got := roleToolRestriction(ToolCall{Action: "ecalc", Operation: op}, dc); !strings.Contains(got, "policy_denied") || !strings.Contains(got, "cannot change the virtual desktop") {
				t.Errorf("%s run may call ecalc %q: %q", name, op, got)
			}
		}
	}
}

func TestECalcDiscoveryOffersAPeerOnlyTheReadOperations(t *testing.T) {
	catalog := BuildToolCatalog(BuildNativeToolSchemas("", nil, ToolFeatureFlags{}, nil), nil, "")
	a2a := &DispatchContext{Cfg: &config.Config{}, SessionID: "a2a-task-1", IsCoAgent: true, MessageSource: "a2a"}
	applyCatalogEntryPolicies(catalog, a2a)
	entry, ok := catalog.Get("ecalc")
	if !ok || !entry.Enabled {
		t.Fatalf("ecalc must stay discoverable for a peer: %+v", entry)
	}
	if ops := toolCatalogOperationField(entry, "operation"); !slices.Equal(ops, []string{"search", "describe", "calculate"}) {
		t.Fatalf("a peer is offered %v", ops)
	}
	local := BuildToolCatalog(BuildNativeToolSchemas("", nil, ToolFeatureFlags{}, nil), nil, "")
	applyCatalogEntryPolicies(local, &DispatchContext{Cfg: &config.Config{}, SessionID: "default"})
	entry, _ = local.Get("ecalc")
	if ops := toolCatalogOperationField(entry, "operation"); !slices.Equal(ops, []string{"search", "describe", "calculate", "open"}) {
		t.Fatalf("the main agent is offered %v", ops)
	}
}
