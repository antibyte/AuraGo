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
