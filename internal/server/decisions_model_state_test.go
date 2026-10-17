package server

import (
	"context"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/decisions"
)

func TestDecisionModelAvailabilityAliasEpochAndExpiry(t *testing.T) {
	s, d, _, remote := routedDecisionFixture(t)
	s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
	s.Cfg.LLMRouter.Enabled = true
	model := decisions.DecisionModel{ID: s.Cfg.Decisions.Model, AliasTarget: "fixture/version-1", ContextLength: 8192, SupportsDecisions: true}
	remote.models = func(context.Context) ([]decisions.DecisionModel, error) { return []decisions.DecisionModel{model}, nil }
	first := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
	if d.Status().ModelAvailability.State != "unknown" || remote.catalogCalls.Load() != 0 {
		t.Fatal("passive status performed discovery")
	}
	if _, err := first.Evaluate(context.Background(), decisions.Request{}); err != nil || !first.Current() {
		t.Fatalf("first resolution revoked itself: %v", err)
	}
	known := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
	model.AliasTarget = "fixture/version-2"
	if _, err := d.Models(context.Background(), "cloud"); err != nil {
		t.Fatal(err)
	}
	fresh := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
	if known.Current() || known.Identity == fresh.Identity || !d.Status().AliasChanged {
		t.Fatal("alias drift retained cached routing")
	}
	if _, err := known.Evaluate(context.Background(), decisions.Request{}); decisions.ErrorCode(err) != "configuration_changed" || remote.calls.Load() != 1 {
		t.Fatal("old alias was sent")
	}
	model.ContextLength = 0
	if _, err := d.Models(context.Background(), "cloud"); err != nil {
		t.Fatal(err)
	}
	if (taskDecisionService{s}).RoutingSnapshot(s.Cfg).Available || d.Status().ModelAvailability.Code != decisions.CodeModelContextUnverified {
		t.Fatal("unverified context was usable")
	}
	if availability := (&flowCatalogEnv{decisionsServer: s}).DecisionAvailability(); availability.Code != "DECISIONS_MODEL_CONTEXT_UNVERIFIED" {
		t.Fatalf("flow lost context restriction: %+v", availability)
	}
	d.mu.Lock()
	d.generation.status.ModelAvailability.CheckedAt = time.Now().Add(-6 * time.Minute)
	d.mu.Unlock()
	before := remote.catalogCalls.Load()
	if d.Status().ModelAvailability.State != "unknown" || !(taskDecisionService{s}).RoutingSnapshot(s.Cfg).Available || remote.catalogCalls.Load() != before {
		t.Fatal("expired metadata did not become passive unknown")
	}
	remote.models = func(context.Context) ([]decisions.DecisionModel, error) { return nil, nil }
	if _, err := d.Evaluate(context.Background(), decisions.Request{}); decisions.ErrorCode(err) != decisions.CodeModelNotAvailable || remote.calls.Load() != 1 {
		t.Fatal("missing saved model reached inference")
	}
}
