package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/decisions"
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
	result := agent.TaskDecisionSnapshot{Code: "service_not_ready"}
	s := service.s
	if s == nil || cfg == nil || s.Decisions == nil {
		return result
	}
	settings := cfg.Decisions.Normalize()
	result.Backend, result.Provider, result.Model = settings.Backend, settings.Provider, settings.Model
	if !settings.Enabled {
		result.Code = "service_disabled"
		return result
	}
	if !cfg.LLMRouter.Enabled || !cfg.LLMRouter.DecisionsFallback {
		result.Code = "router_disabled"
		return result
	}
	status := s.Decisions.Status()
	if status.Model != "" {
		result.Model = status.Model
	}
	var generation *decisionProviderGeneration
	var generationID uint64
	var modelEpoch uint64
	var modelIdentity [32]byte
	local := s.Decisions
	if router, ok := s.Decisions.(*routedDecisions); ok {
		router.mu.Lock()
		generation = router.generation
		generationID = generation.id
		modelEpoch, modelIdentity = generation.modelEpoch, generation.modelIdentity
		closed := router.closed || generation.ctx.Err() != nil
		router.mu.Unlock()
		if closed {
			return result
		}
		local = router.local
	}
	localGeneration := func() uint64 {
		if settings.Backend != "provider" {
			if manager, ok := local.(interface{ CacheGeneration() uint64 }); ok {
				return manager.CacheGeneration()
			}
		}
		return 0
	}
	version := localGeneration()
	epoch := s.routingDecisionEpoch.Load()
	identity, _ := json.Marshal([]any{settings, generationID, modelIdentity, status.ModelAvailability.State, version, epoch, status.Enabled, status.Installed, status.Ready && settings.Backend != "provider"})
	result.Identity = sha256.Sum256(identity)
	result.Current = func() bool {
		current := s.ConfigSnapshot()
		if current == nil || s.routingDecisionEpoch.Load() != epoch || current.Decisions.Normalize() != settings || !current.LLMRouter.Enabled || !current.LLMRouter.DecisionsFallback {
			return false
		}
		if router, ok := s.Decisions.(*routedDecisions); ok {
			router.mu.Lock()
			valid := !router.closed && router.generation == generation && generation.settings == settings && generation.ctx.Err() == nil && generation.modelEpoch == modelEpoch
			router.mu.Unlock()
			if !valid {
				return false
			}
		}
		return localGeneration() == version
	}
	if !result.Current() {
		result.Code = "configuration_changed"
		return result
	}
	switch {
	case !status.Enabled:
		result.Code = "service_disabled"
	case settings.Backend == "provider" && !status.Configured:
		result.Code = "provider_not_configured"
	case settings.Backend == "provider" && status.ModelAvailability.State == "blocked":
		result.Code = status.ModelAvailability.Code
	case settings.Backend != "provider" && !status.Ready:
		result.Code = "service_not_ready"
	default:
		result.Available, result.Code = true, "available"
	}
	if !result.Available {
		return result
	}
	result.Evaluate = func(ctx context.Context, request decisions.Request) (decisions.Response, error) {
		if !result.Current() {
			return decisions.Response{}, decisions.NewError("configuration_changed", context.Canceled)
		}
		ctx = withDecisionUsageCategory(ctx, "routing")
		if router, ok := s.Decisions.(*routedDecisions); ok {
			ctx = context.WithValue(ctx, router, generation)
			ctx = context.WithValue(ctx, decisionModelEpochKey{}, modelEpoch)
		}
		response, err := s.Decisions.Evaluate(ctx, request)
		if !result.Current() {
			return response, decisions.NewError("configuration_changed", context.Canceled)
		}
		return response, err
	}
	return result
}
