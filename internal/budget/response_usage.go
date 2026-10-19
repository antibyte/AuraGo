package budget

import (
	"context"
	"strings"

	"aurago/internal/llm"
)

type trackingKey struct{}
type tracking struct {
	tracker  *Tracker
	category string
}

// WithTracking attaches request-scoped budget ownership, never singleton state.
func WithTracking(ctx context.Context, tracker *Tracker, category string) context.Context {
	return context.WithValue(ctx, trackingKey{}, tracking{tracker, category})
}
func Tracking(ctx context.Context) (*Tracker, string) {
	value, _ := ctx.Value(trackingKey{}).(tracking)
	return value.tracker, value.category
}

// RecordResponseUsage books each captured physical response once. If transport
// measurements are unavailable, callers supply their explicitly estimated usage.
// Call this instead of RecordForCategory, never in addition to it.
func (t *Tracker) RecordResponseUsage(category, provider, model string, capture *llm.UsageCapture, input, output int, fallback bool) bool {
	if t == nil {
		return false
	}
	recorded := false
	warning := false
	if capture != nil {
		items := capture.Snapshot()
		for i, usage := range items {
			supplemented := false
			// A capture belongs to one logical call; retry/failover requests finish
			// sequentially. Only its final successful response owns the caller's
			// output evidence. Never spread that estimate over earlier attempts.
			if fallback && i == len(items)-1 && usage.Status >= 200 && usage.Status < 300 &&
				(usage.Model == "" || strings.EqualFold(usage.Model, model)) {
				if usage.InputTokens == nil {
					n := max(0, input)
					usage.InputTokens = &n
					supplemented = true
				}
				if usage.OutputTokens == nil || *usage.OutputTokens == 0 && output > 0 {
					n := max(0, output)
					usage.OutputTokens = &n
					supplemented = true
				}
			}
			if usage.InputTokens == nil && usage.OutputTokens == nil {
				continue
			}
			actualModel := usage.Model
			if actualModel == "" {
				actualModel = model
			}
			in, out := 0, 0
			if usage.InputTokens != nil {
				in = *usage.InputTokens
			}
			if usage.OutputTokens != nil {
				out = *usage.OutputTokens
			}
			cost := usage.EstimatedCostUSD
			estimated := cost == nil || supplemented
			if supplemented {
				// A previous estimate may cover only initial stream usage.
				cost = nil
			}
			if cost == nil {
				cost = llm.EstimateMeasuredCost(usage)
			}
			if cost == nil { // Exact provider identity avoids another configured provider's family price.
				if price, ok := llm.StaticPricingForModel(usage.Provider, actualModel); ok {
					value := (float64(in)*price.InputPerMillion + float64(out)*price.OutputPerMillion) / 1e6
					cost = &value
				}
			}
			if custom, ok := t.responseCostOverride(usage.Provider, actualModel, in, out); ok {
				cost = &custom
				estimated = true
			}
			warning = t.recordUsage(category, actualModel, in, out, cost, estimated) || warning
			recorded = true
		}
	}
	if !recorded && fallback {
		var cost *float64
		if price, ok := llm.StaticPricingForModel(provider, model); ok {
			value := (float64(input)*price.InputPerMillion + float64(output)*price.OutputPerMillion) / 1e6
			cost = &value
		}
		if custom, ok := t.responseCostOverride(provider, model, input, output); ok {
			cost = &custom
		}
		warning = t.recordUsage(category, model, input, output, cost, true)
	}
	return warning
}

func (t *Tracker) responseCostOverride(provider, model string, input, output int) (float64, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, p := range t.cfg.Providers {
		if !strings.EqualFold(p.Type, provider) && !strings.EqualFold(p.ID, provider) {
			continue
		}
		for _, m := range p.Models {
			if strings.EqualFold(m.Name, model) {
				return (float64(input)*m.InputPerMillion + float64(output)*m.OutputPerMillion) / 1e6, true
			}
		}
	}
	for _, m := range t.cfg.Budget.Models {
		if strings.EqualFold(m.Name, model) {
			return (float64(input)*m.InputPerMillion + float64(output)*m.OutputPerMillion) / 1e6, true
		}
	}
	return 0, false
}
