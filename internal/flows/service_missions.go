package flows

import (
	"context"
	"errors"
	"time"
)

// CancelMissionRuns cancels the live runs of the flow behind a mission (Mission Control's
// cancel button) and returns how many runs this call cancelled. It cancels every live
// run the runner knows, whether it runs, is queued behind the flow's active run, waits
// for a global slot or is being started (see Runner.CancelFlowMode); test runs go on.
// An unknown mission gives ErrNotFound and a mission that several flows hold
// ErrMissionAmbiguous (wrapped; test both with errors.Is).
//
// It takes no flow lock: it changes neither the mission nor the timers. ctx bounds only
// the flow lookup; waiting for a Start in progress does not observe it. The runs that
// never started end before it returns, and OnRunFinished reports them to
// MissionBridge.FlowRunFinished on the caller's goroutine, so the caller must not hold a
// lock that FlowRunFinished takes.
func (s *Service) CancelMissionRuns(ctx context.Context, missionID string) (int, error) {
	rec, err := s.store.GetFlowByMission(ctx, missionID)
	if err != nil {
		return 0, err
	}
	return s.runner.CancelFlowMode(rec.ID, ModeLive), nil
}

// MissionEnabledChanged re-arms or clears the Date/Time timers after Mission Control switched
// the flow mission on or off. Missions without a flow are ignored (nil, nothing is logged);
// a mission that several flows hold gives ErrMissionAmbiguous (wrapped).
//
// It holds the flow's lock (see flowLocks) from reading the flow until the timers are
// armed, like SetEnabled, so it never arms the timers of a revision that a concurrent
// Publish replaced. The flow is read again once the lock is held, because it may have been
// published or deleted while this call waited (a deleted flow is no error). ctx bounds the
// lookup and the wait for the lock. Once the lock is held the rest ignores the
// cancellation of ctx: Mission Control's switch has happened already, and the timers must
// follow it even when the caller goes away.
//
// Taking the lock cannot deadlock: Mission Control calls this from a goroutine of its own
// (go FlowHooks.FlowEnabledChanged), never while it holds its own lock, and under the flow
// lock this calls only the store, MissionBridge.FlowMissionEnabled and
// TimerService.Replace, as SetEnabled does. Like every lock-taking method it must not be
// called synchronously from a MissionBridge method (see MissionBridge).
func (s *Service) MissionEnabledChanged(ctx context.Context, missionID string) error {
	rec, err := s.store.GetFlowByMission(ctx, missionID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	unlock, err := s.locks.lock(ctx, rec.ID)
	if err != nil {
		return err
	}
	defer unlock()
	// The mission is switched already; arm or clear the timers even when the caller goes away.
	ctx = context.WithoutCancel(ctx)
	cur, err := s.store.GetFlow(ctx, rec.ID)
	if errors.Is(err, ErrNotFound) {
		return nil // deleted while this call waited for the lock
	}
	if err != nil {
		return err
	}
	return s.armTimers(ctx, cur)
}

// NextTimer returns the earliest armed Date/Time timer of the flow behind a mission; ok is
// false when no single flow holds the mission, the flow has no timer or the store fails.
//
// It takes no flow lock and reads no flow document: one indexed lookup of the flow's id
// (at most two rows) and one indexed aggregate over that flow's timers. So it works even
// when a document cannot be parsed, and a MissionBridge may call it synchronously
// (Mission Control's broadcastMissionState asks for every flow mission's next run through
// FlowHooks.NextFlowRun). Keep it lock-free and cheap.
func (s *Service) NextTimer(ctx context.Context, missionID string) (time.Time, bool) {
	flowID, err := s.store.flowIDByMission(ctx, missionID)
	if err != nil {
		return time.Time{}, false
	}
	next, ok, err := s.store.NextTimerAt(ctx, flowID)
	if err != nil {
		return time.Time{}, false
	}
	return next, ok
}
