package flows

import (
	"context"
	"time"
)

func (s *Service) onRunStarted(RunRecord) {}

func (s *Service) onRunFinished(RunRecord, RunResult) {}

func (s *Service) onTimerFired(string, string, time.Time) {}

func (s *Service) onTimerMissed(string, string, time.Time) {}

// RunDetail is a run with its steps (and optionally the document it executed).
type RunDetail struct {
	Run   *RunRecord   `json:"run"`
	Steps []StepRecord `json:"steps"`
	Doc   *Flow        `json:"doc,omitempty"`
}

// Run returns one run.
func (s *Service) Run(ctx context.Context, runID string, includeDoc bool) (*RunDetail, error) {
	rec, steps, err := s.store.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	return &RunDetail{Run: rec, Steps: steps}, nil
}
