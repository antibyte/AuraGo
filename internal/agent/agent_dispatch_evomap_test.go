package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchEvomapDisabled(t *testing.T) {
	out := dispatchEvomapCall(context.Background(), evomapArgs{Operation: "status"}, &config.Config{})
	if !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, "EvoMap is disabled") {
		t.Fatalf("unexpected output for disabled evomap: %s", out)
	}
}

func TestDispatchEvomapKGQueryRequiresGateAndAPIKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.Evomap.Enabled = true
	cfg.Evomap.BaseURL = "https://evomap.ai"

	out := dispatchEvomapCall(context.Background(), evomapArgs{Operation: "kg_query", Question: "what changed?"}, cfg)
	if !strings.Contains(out, `"status":"policy_denied"`) || !strings.Contains(out, "kg_enabled") {
		t.Fatalf("expected kg_enabled policy denial, got: %s", out)
	}

	cfg.Evomap.KGEnabled = true
	out = dispatchEvomapCall(context.Background(), evomapArgs{Operation: "kg_query", Question: "what changed?"}, cfg)
	if !strings.Contains(out, `"status":"error"`) || !strings.Contains(out, "API key") {
		t.Fatalf("expected missing API key error, got: %s", out)
	}
}

func TestDispatchEvomapMutatingOperationsArePolicyDeniedInMVP(t *testing.T) {
	cfg := &config.Config{}
	cfg.Evomap.Enabled = true
	cfg.Evomap.BaseURL = "https://evomap.ai"
	cfg.Evomap.AllowPublish = true
	cfg.Evomap.AllowReport = true

	for _, op := range []string{"publish_bundle", "submit_report", "kg_ingest", "claim_bounty", "heartbeat"} {
		out := dispatchEvomapCall(context.Background(), evomapArgs{Operation: op}, cfg)
		if !strings.Contains(out, `"status":"policy_denied"`) {
			t.Fatalf("operation %s should be policy denied, got: %s", op, out)
		}
	}
}

func TestEvomapRegistrationRequiresWritableServerContext(t *testing.T) {
	cfg := &config.Config{}
	cfg.Evomap.Enabled = true
	cfg.Evomap.ReadOnly = true
	calls := 0
	cfg.RegisterEvomapNode = func(context.Context) (string, string, bool, error) {
		calls++
		return "fixture-node", "https://example.invalid/claim", true, nil
	}
	out := dispatchEvomapCall(context.Background(), evomapArgs{Operation: "register_node"}, cfg)
	if !strings.Contains(out, "policy_denied") || calls != 0 {
		t.Fatalf("read-only registration: %s, calls=%d", out, calls)
	}
	cfg.Evomap.ReadOnly = false
	out = dispatchEvomapCall(context.Background(), evomapArgs{Operation: "register_node"}, cfg)
	if !strings.Contains(out, `"status":"success"`) || calls != 1 || cfg.Evomap.NodeID != "" {
		t.Fatalf("server registration not used: %s", out)
	}
	cfg.RegisterEvomapNode = nil
	if out = dispatchEvomapCall(context.Background(), evomapArgs{Operation: "register_node"}, cfg); !strings.Contains(out, "policy_denied") {
		t.Fatalf("unserialized registration: %s", out)
	}
}
