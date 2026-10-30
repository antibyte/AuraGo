package server

import (
	"context"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/decisions/consumers"
)

type taskDecisionService struct{ s *Server }
type decisionUsageKey struct{}

func withDecisionUsageCategory(ctx context.Context, category string) context.Context {
	return context.WithValue(ctx, decisionUsageKey{}, category)
}

func decisionUsageCategory(ctx context.Context) string {
	if category, _ := ctx.Value(decisionUsageKey{}).(string); category != "" {
		return category
	}
	return "decisions"
}

// RoutingSnapshot is passive. Its closures never start/install a runtime and
// cannot switch to another provider after the input was admitted.
func (service taskDecisionService) RoutingSnapshot(cfg *config.Config) agent.TaskDecisionSnapshot {
	if service.s == nil {
		return agent.TaskDecisionSnapshot{Code: "service_not_ready"}
	}
	snapshot := service.s.decisionHub().Snapshot(cfg, consumers.Router)
	return agent.TaskDecisionSnapshot{
		Available: snapshot.Available, Code: snapshot.Code, Backend: snapshot.Backend,
		Provider: snapshot.Provider, Model: snapshot.Model, Identity: snapshot.Identity,
		Evaluate: snapshot.Evaluate, Current: snapshot.Current,
	}
}

// Snapshot lets every subsystem reach the hub through the same handle.
func (service taskDecisionService) Snapshot(cfg *config.Config, id consumers.ID) consumers.Snapshot {
	if service.s == nil {
		return consumers.Snapshot{Consumer: id, Code: "service_not_ready"}
	}
	return service.s.decisionHub().Snapshot(cfg, id)
}
