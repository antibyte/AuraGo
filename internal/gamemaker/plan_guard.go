package gamemaker

import (
	"errors"
	"fmt"
	"strings"
)

// Planning uses a charged-attempt budget instead of a flat call counter: an
// attempt that reduces the number of independent design issues is free, so a
// weak model can iterate toward a valid design. The hard call ceiling keeps the
// token cost bounded even when every failure looks "smaller".
const (
	planChargedLimit = 3
	planCallLimit    = 5
	planPreviewLimit = 3
)

// planBudget tracks one planning job. charged counts the corrections that count
// against the budget, calls counts every submission (charged or not) and issues
// remembers the smallest independent issue count seen so far.
type planBudget struct {
	charged  int
	calls    int
	issues   int
	previews int
}

func (s *Service) planBudgetLocked(jobID string) *planBudget {
	if s.planBudgets == nil {
		s.planBudgets = map[string]*planBudget{}
	}
	budget := s.planBudgets[jobID]
	if budget == nil {
		budget = &planBudget{}
		s.planBudgets[jobID] = budget
	}
	return budget
}

func (s *Service) remainingPlanAttemptsLocked(jobID string) int {
	budget := s.planBudgets[jobID]
	if budget == nil {
		return planChargedLimit
	}
	return max(0, planChargedLimit-budget.charged)
}

// chargePlanAttemptLocked counts a failure only when it does not reduce the
// independent issue count. Identical or larger issue sets stay charged.
func (s *Service) chargePlanAttemptLocked(jobID string, issues int) {
	budget := s.planBudgetLocked(jobID)
	if issues > 0 && budget.issues > 0 && issues < budget.issues {
		budget.issues = issues
		return
	}
	budget.charged++
	if issues > 0 {
		budget.issues = issues
	}
}

// planningFailure names the honest cause of a failed planning phase. A job whose
// model never reached set_design (calls==0) did not spend a correction budget:
// its tool calls never arrived or never validated, which is a protocol problem
// rather than rejected design content. The planning loop can also end after its
// attempts with corrections left (for example prose-only replies after one
// rejection), so the wording follows the real budget state.
func (s *Service) planningFailure(jobID string, diagnostics []Diagnostic) *JobError {
	s.mu.RLock()
	budget := s.planBudgets[jobID]
	submitted, charged := budget != nil && budget.calls > 0, 0
	if budget != nil {
		charged = budget.charged
	}
	s.mu.RUnlock()
	switch {
	case !submitted:
		return &JobError{Code: "planning_protocol", Err: fmt.Errorf("game planning failed: the model never submitted a design through the Game Maker tools (no valid set_design call was received)")}
	case charged >= planChargedLimit:
		return &JobError{Code: "planning", Err: fmt.Errorf("game planning failed after the charged correction budget was spent: %s", diagnosticsText(diagnostics))}
	}
	return &JobError{Code: "planning", Err: fmt.Errorf("game planning ended after its attempts without an accepted design (%d of %d corrections used): %s", charged, planChargedLimit, diagnosticsText(diagnostics))}
}

func (s *Service) planBudgetExhausted(jobID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	budget := s.planBudgets[jobID]
	return budget != nil && budget.charged >= planChargedLimit
}

func (s *Service) takePlanSubmission(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	budget := s.planBudgetLocked(jobID)
	if budget.calls >= planCallLimit {
		return fmt.Errorf("plan attempt limit reached (%d submissions per planning job)", planCallLimit)
	}
	budget.calls++
	return nil
}

func (s *Service) takeDesignPreview(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	budget := s.planBudgetLocked(jobID)
	if budget.previews >= planPreviewLimit {
		return fmt.Errorf("design preview limit reached (%d dry runs per planning job)", planPreviewLimit)
	}
	budget.previews++
	return nil
}

// noteDesignWarning records one deterministic repair for the model response. The
// list stays bounded so a pathological draft cannot flood the answer.
func (s *Service) noteDesignWarning(jobID, message string) {
	if jobID == "" || message == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.designWarnings == nil {
		s.designWarnings = map[string][]string{}
	}
	if len(s.designWarnings[jobID]) >= 8 {
		return
	}
	s.designWarnings[jobID] = append(s.designWarnings[jobID], message)
}

func designIssueCount(err error) int {
	var detail *DesignValidationError
	if errors.As(err, &detail) {
		return len(detail.Issues)
	}
	return 1
}

// The example plan doubles as schema documentation. Its instruction prose must
// never reach an accepted plan, where the building prompt would treat it as an
// order. Only fields the model supplied in this round are checked; inherited
// assets of a published revision keep their authored content.
var planPlaceholderMarkers = []string{
	"Replace with",
	"Describe score",
	"Describe defeat",
	"Describe victory",
	"Choose an exact",
}

func rejectDesignPlaceholders(d GameDesign) error {
	check := func(field, value string) error {
		for _, marker := range planPlaceholderMarkers {
			if strings.Contains(value, marker) {
				return fmt.Errorf("design.%s: replace the example text (%q) with concrete content for this game", field, marker)
			}
		}
		return nil
	}
	if err := check("objective", d.Objective); err != nil {
		return err
	}
	for i, feature := range d.Features {
		if err := check(fmt.Sprintf("features[%d]", i), feature); err != nil {
			return err
		}
	}
	for i, stage := range d.Stages {
		if err := check(fmt.Sprintf("stages[%d]", i), stage); err != nil {
			return err
		}
	}
	for i, asset := range d.Assets {
		if err := check(fmt.Sprintf("assets[%d].fallback", i), asset.Fallback); err != nil {
			return err
		}
	}
	return nil
}
