package server

import (
	"context"
	"errors"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/security"
)

func (f *flowLLM) reviewComposedPrompt(ctx context.Context, cfg *config.Config, req flows.LLMRequest) (string, error) {
	if !cfg.LLMGuardian.Enabled && !cfg.Guardian.PromptSec.LLMJudge.Enabled {
		return "advisory", nil
	}
	review := f.reviewPrompt
	if review == nil && f.s.LLMGuardian != nil {
		review = f.s.LLMGuardian.EvaluateComposedPrompt
	}
	if review == nil {
		return "review_unavailable", errors.New("the security reviewer is enabled but unavailable; the prompt was not sent")
	}
	result, err := review(ctx, req.System, req.Prompt)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "review_unavailable", errors.New("the security reviewer could not complete its decision; the prompt was not sent")
	}
	if f.s.Logger != nil {
		f.s.Logger.Info("[Flows] Composed prompt reviewed", "flow", req.FlowID, "run", req.RunID,
			"node", req.NodeID, "decision", result.Decision)
	}
	switch result.Decision {
	case security.DecisionAllow:
		return "review_allowed", nil
	case security.DecisionQuarantine:
		// A completed suspicious verdict is still uncertainty, not a confirmed
		// danger. Internal compositions keep it advisory; external gates retain
		// their stricter quarantine policy.
		if result.QuarantineReason == security.QuarantineSuspicious {
			return "review_advisory", nil
		}
	case security.DecisionBlock:
		return "review_blocked", flows.NewNodeError("FLOW_PARAM_INVALID", "the prompt was not sent: the security reviewer confirmed a dangerous instruction")
	}
	return "review_unavailable", errors.New("the security reviewer did not return a complete decision; the prompt was not sent")
}
