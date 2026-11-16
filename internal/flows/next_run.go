package flows

import (
	"context"
	"time"
)

// NextRun is the next time a flow starts by itself: the earliest Date/Time timer or schedule
// fire of its live version, and the trigger node that fires.
type NextRun struct {
	At     time.Time
	NodeID string
}

// ScheduleNexter tells when the schedule triggers of a flow fire next. The server implements it
// over the cron engine and injects it with Service.SetScheduleNexter, so this package does not
// import the cron code. NextScheduleRun returns the earliest fire after now among the flow's
// schedule jobs and the id of the schedule node that owns it; ok is false when there is none
// (no schedule trigger, or the engine has not started). It must not call into a Service method
// that takes a flow lock.
type ScheduleNexter interface {
	NextScheduleRun(flowID string, now time.Time) (time.Time, string, bool)
}

// nextRunBudget bounds the store reads of one NextRun call.
const nextRunBudget = 2 * time.Second

// SetScheduleNexter injects the schedule source of NextRun; nil removes it. Call it once while
// the server wires the service; it is safe to call at any time.
func (s *Service) SetScheduleNexter(n ScheduleNexter) {
	s.mu.Lock()
	s.nexter = n
	s.mu.Unlock()
}

func (s *Service) scheduleNexter() ScheduleNexter {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nexter
}

// NextRun returns the next time the flow starts by itself: the earliest fire after now of the
// live version's Date/Time timers and schedule triggers. ok is false for a flow that is
// unknown, is a block, has no live version, is switched off, or has no time-based trigger
// that fires later. A stored fire time that has passed (a timer in retry, or not yet
// processed) is not "next".
//
// It is read-only and takes no flow lock: one lookup of the flow row, the mission's switch
// (MissionBridge.FlowMissionEnabled), one indexed timer query and the ScheduleNexter. A
// MissionBridge may therefore call it synchronously. Mission Control's next run and the
// flow API both answer through it.
func (s *Service) NextRun(flowID string) (NextRun, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), nextRunBudget)
	defer cancel()
	return s.nextRun(ctx, flowID)
}

// NextRunForMission is NextRun for the flow behind a Mission Control mission; ok is false
// when no single flow holds the mission.
func (s *Service) NextRunForMission(missionID string) (NextRun, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), nextRunBudget)
	defer cancel()
	flowID, err := s.store.flowIDByMission(ctx, missionID)
	if err != nil {
		return NextRun{}, false
	}
	return s.nextRun(ctx, flowID)
}

func (s *Service) nextRun(ctx context.Context, flowID string) (NextRun, bool) {
	missionID, live, err := s.store.flowRunBinding(ctx, flowID)
	if err != nil || !live || missionID == "" || !s.bridge.FlowMissionEnabled(missionID) {
		return NextRun{}, false
	}
	now := s.now()
	var best NextRun
	found := false
	consider := func(at time.Time, node string) {
		if at.IsZero() || !at.After(now) {
			return
		}
		if !found || at.Before(best.At) || (at.Equal(best.At) && node < best.NodeID) {
			best, found = NextRun{At: at, NodeID: node}, true
		}
	}
	if tm, ok, err := s.store.NextTimerAfter(ctx, flowID, now); err == nil && ok {
		consider(tm.FireAt, tm.NodeID)
	}
	if n := s.scheduleNexter(); n != nil {
		if at, node, ok := n.NextScheduleRun(flowID, now); ok {
			consider(at, node)
		}
	}
	return best, found
}
