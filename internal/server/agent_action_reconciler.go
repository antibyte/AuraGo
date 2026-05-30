package server

import (
	"context"
	"time"

	"aurago/internal/agent"
)

const agentActionReconcileInterval = time.Minute

func startAgentActionReconciler(ctx context.Context, s *Server, broker agent.FeedbackBroker) {
	if s == nil || s.ShortTermMem == nil {
		return
	}
	run := func() {
		if err := agent.MarkAllStalledAgentActions(s.ShortTermMem, s.Logger, broker, 5*time.Minute); err != nil && s.Logger != nil {
			s.Logger.Warn("Failed to reconcile stalled agent actions", "error", err)
		}
	}
	run()
	go func() {
		ticker := time.NewTicker(agentActionReconcileInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				run()
			case <-ctx.Done():
				return
			}
		}
	}()
}
