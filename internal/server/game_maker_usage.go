package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"aurago/internal/agent"
	"aurago/internal/gamemaker"
)

// One writer/job is active at a time. The observer stores only fingerprints and
// is replaced at the next job; event persistence owns the historical measurements.
func (r *gameMakerAgentRunner) gameUsageObserver(run gamemaker.JobRun) *agent.PromptUsageObserver {
	r.usageMu.Lock()
	defer r.usageMu.Unlock()
	if r.usage == nil || r.usageJobID != run.Job.ID {
		r.usageJobID = run.Job.ID
		r.usage = &agent.PromptUsageObserver{Observe: func(value agent.PromptUsageObservation) {
			if r.service == nil || run.Job.ID == "" {
				return
			}
			data, _ := json.Marshal(value)
			var payload map[string]any
			_ = json.Unmarshal(data, &payload)
			// Completion/cancellation measurements must survive the call deadline.
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = r.service.EmitAgentEvent(ctx, run.Project.ID, run.Job.ID, "prompt_usage", payload)
			_ = r.service.EmitAgentEvent(ctx, run.Project.ID, run.Job.ID, "diagnostic", map[string]any{"level": "info", "message": gameUsageDiagnostic(value)})
		}}
	}
	return r.usage
}

func gameUsageDiagnostic(value agent.PromptUsageObservation) string {
	count := func(n *int) string {
		if n == nil {
			return "unknown"
		}
		return strconv.Itoa(*n)
	}
	cost := "unknown"
	if value.EstimatedCostUSD != nil {
		cost = strconv.FormatFloat(*value.EstimatedCostUSD, 'f', 6, 64)
	}
	return fmt.Sprintf("prompt_usage: %s/%s; input=%s output=%s cache_read=%s cache_write=%s; local_prompt_reuse=%t; estimated_USD=%s; duration_ms=%d; context_generation=%d (%s); failed=%t",
		value.Provider, value.Model, count(value.InputTokens), count(value.OutputTokens), count(value.CacheReadTokens), count(value.CacheWriteTokens), value.LocalPromptCacheHit, cost, value.DurationMS, value.ContextGeneration, value.ContextChange, value.Failed)
}
