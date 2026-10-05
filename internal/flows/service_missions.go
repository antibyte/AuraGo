package flows

import (
	"context"
	"errors"
	"time"
)

// CancelMissionRuns cancels the queued and running live runs of the flow behind a mission
// (Mission Control's cancel button). It returns how many runs were cancelled.
func (s *Service) CancelMissionRuns(ctx context.Context, missionID string) (int, error) {
	rec, err := s.store.GetFlowByMission(ctx, missionID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, status := range []RunStatus{RunRunning, RunQueued} {
		runs, err := s.store.ListRuns(ctx, rec.ID, RunFilter{Mode: ModeLive, Status: status, Limit: 200})
		if err != nil {
			return n, err
		}
		for _, run := range runs {
			if s.runner.Cancel(run.ID) {
				n++
			}
		}
	}
	return n, nil
}

// MissionEnabledChanged re-arms or clears the Date/Time timers after Mission Control switched
// the flow mission on or off. Missions without a flow are ignored.
func (s *Service) MissionEnabledChanged(ctx context.Context, missionID string) error {
	rec, err := s.store.GetFlowByMission(ctx, missionID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.armTimers(ctx, rec)
}

// NextTimer returns the earliest armed Date/Time timer of the flow behind a mission.
func (s *Service) NextTimer(ctx context.Context, missionID string) (time.Time, bool) {
	rec, err := s.store.GetFlowByMission(ctx, missionID)
	if err != nil {
		return time.Time{}, false
	}
	timers, err := s.store.ListTimers(ctx)
	if err != nil {
		return time.Time{}, false
	}
	var next time.Time
	for _, t := range timers {
		if t.FlowID == rec.ID && (next.IsZero() || t.FireAt.Before(next)) {
			next = t.FireAt
		}
	}
	return next, !next.IsZero()
}
