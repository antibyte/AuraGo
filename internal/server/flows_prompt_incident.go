package server

import (
	"crypto/sha256"
	"fmt"
	"time"

	"aurago/internal/flows"
	"aurago/internal/planner"
)

// Persist fixed runtime diagnostics only. Prompt bodies, snippets and model-written
// reasons must never become trusted operational-notice text.
func (s *Server) recordFlowPromptIncident(req flows.LLMRequest, outcome string) {
	if s == nil || s.PlannerDB == nil {
		return
	}
	details := map[string]string{
		"advisory":           "A local check flagged an internal workflow prompt. The finding remained advisory because no LLM reviewer is enabled.",
		"review_allowed":     "A local check flagged an internal workflow prompt. The LLM Guardian reviewed the complete task and allowed it.",
		"review_advisory":    "The LLM Guardian reviewed an internal workflow prompt and found uncertainty without confirmed danger. Execution continued with an advisory.",
		"review_blocked":     "The LLM Guardian confirmed a dangerous instruction in an internal workflow prompt. The task prompt was withheld.",
		"critical_block":     "A critical local security finding blocked an internal workflow prompt before the task model was called.",
		"incomplete_scan":    "The local security scan could not read the complete internal workflow prompt. The task prompt was withheld.",
		"review_unavailable": "An internal workflow prompt required review, but the enabled LLM Guardian could not complete a decision. The task prompt was withheld.",
	}
	detail, ok := details[outcome]
	if !ok {
		return
	}
	severity := "warning"
	if outcome == "critical_block" || outcome == "review_blocked" || outcome == "review_unavailable" || outcome == "incomplete_scan" {
		severity = "error"
	}
	identity := sha256.Sum256([]byte(req.FlowID + "\x00" + req.NodeID))
	_, err := planner.RecordOperationalIssue(s.PlannerDB, planner.OperationalIssue{
		Source: planner.OperationalIssueSourcePromptSecurity, Context: "flow_ai_step",
		Title: "Internal workflow prompt security check", Detail: detail, Severity: severity,
		Kind: planner.OperationalIssueKindRuntimeFailure, Reference: req.FlowID,
		Fingerprint: fmt.Sprintf("prompt_security:%x", identity[:16]), OccurredAt: time.Now(),
	})
	if err != nil && s.Logger != nil {
		s.Logger.Warn("[Flows] Could not persist prompt security incident", "outcome", outcome, "error", err)
	}
}
