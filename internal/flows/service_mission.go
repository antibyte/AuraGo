package flows

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// flowLocks serializes the Service operations that change a flow's mission and timers:
// Publish, SetEnabled, DeleteFlow, DeleteFlowForMission and MissionEnabledChanged. Each
// reads the flow record and then updates Mission Control and the timers from what it
// read; interleaved, an operation working from an older record could undo a newer one
// (SetEnabled arming the timers of a revision that a concurrent Publish just replaced).
// SaveDraft needs no lock: the store's revision check covers it.
//
// There is one lock per flow id, so flows never wait for each other. An entry lives
// while somebody holds or waits for it and is removed with the last one, so the map
// only holds flows with an operation in progress.
//
// The caller's context bounds waiting for the lock and the steps up to the first one
// that cannot be undone (the store publish, the mission switch, the mission delete).
// From there on the operation runs with context.WithoutCancel, so a caller that goes
// away (an HTTP client disconnecting) cannot leave the mission and the timers apart.
// MissionEnabledChanged needs no switch: its only such step is its last, the timer write.
//
// Lock order and why it cannot deadlock: a flow lock is the outermost lock. While it is
// held the Service calls the store, the bridge, TimerService.Replace (no timer lock,
// a non-blocking wake-up) and Runner.CancelFlow (the runner's locks, released before
// it calls OnRunFinished for the runs it ended, on this goroutine). None of these may
// take a flow lock: the run paths (starting runs, runner hooks, timer callbacks) never
// do, and a bridge must not synchronously call a Service method that does (see
// MissionBridge).
type flowLocks struct {
	mu    sync.Mutex
	locks map[string]*flowLock
}

// flowLock is the lock of one flow: sem holds a token while it is taken.
type flowLock struct {
	sem  chan struct{}
	refs int // holders and waiters; guarded by flowLocks.mu
}

// lock takes the lock of flow id, or gives up with ctx's error when ctx ends first; a
// ctx that has already ended never takes the lock, even a free one. The returned function
// releases the lock; calling it again does nothing.
func (l *flowLocks) lock(ctx context.Context, id string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	if l.locks == nil {
		l.locks = map[string]*flowLock{}
	}
	fl := l.locks[id]
	if fl == nil {
		fl = &flowLock{sem: make(chan struct{}, 1)}
		l.locks[id] = fl
	}
	fl.refs++
	l.mu.Unlock()
	select {
	case fl.sem <- struct{}{}:
		var once sync.Once
		return func() {
			once.Do(func() {
				<-fl.sem
				l.drop(id, fl)
			})
		}, nil
	case <-ctx.Done():
		l.drop(id, fl)
		return nil, ctx.Err()
	}
}

// drop gives up one reference to fl and removes the entry with the last one.
func (l *flowLocks) drop(id string, fl *flowLock) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fl.refs--
	if fl.refs == 0 {
		delete(l.locks, id)
	}
}

// Publish validates the draft (publish rules), publishes it and updates Mission Control and timers.
//
// It holds the flow's lock (see flowLocks) from reading the flow until the timers are
// armed, and validates and binds the triggers with the same time. When it returns a
// record together with an error, the new revision is live but Mission Control or the
// timers were not updated. Publishing the same draft revision again repeats the update:
// the store treats a revision that is already live as a no-op, so no version is added
// and the bindings and timers are synced from the live revision. Any later successful
// publish heals it too. Once the store published, the rest runs to the end even if ctx
// is cancelled (see flowLocks).
//
// Known limit, no repair at start-up: when AuraGo stops between the store publish and the
// mission sync, Start does not detect it; the next successful Publish of the flow
// repairs it (see Service.Start).
//
// A trigger.mission_completed that waits for the flow's own mission is refused: every
// run would start the next one. Loops across several flows are not detected.
func (s *Service) Publish(ctx context.Context, id string, baseRevision int) (*FlowRecord, []Issue, error) {
	unlock, err := s.locks.lock(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	defer unlock()
	rec, err := s.store.GetFlow(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if rec.DraftRevision != baseRevision {
		return nil, nil, ErrRevisionConflict
	}
	now, loc := s.now(), s.services.Loc()
	issues := Validate(rec.Draft, s.reg, ValidateContext{Mode: ModePublish, Now: now, Location: loc})
	issues = append(issues, selfTriggerIssues(rec.Draft, rec.MissionID)...)
	if HasErrors(issues) {
		return nil, issues, &ValidationError{Issues: issues}
	}
	bindings, err := BindTriggers(rec.Draft, s.reg, loc, now)
	if err != nil {
		issues = append(issues, bindingIssue(err))
		return nil, issues, &ValidationError{Issues: issues}
	}
	pub, err := s.store.Publish(ctx, id, baseRevision, now)
	if err != nil {
		return nil, issues, err
	}
	// The revision is live: Mission Control and the timers must follow it even when the
	// caller goes away (an HTTP client that disconnects cancels ctx).
	ctx = context.WithoutCancel(ctx)
	if err := s.bridge.SyncFlowMission(pub.MissionID, pub.Live.Name, bindings); err != nil {
		return pub, issues, fmt.Errorf("update the flow mission: %w", err)
	}
	if err := s.armTimers(ctx, pub); err != nil {
		return pub, issues, err
	}
	return pub, issues, nil
}

// bindingIssue reports a trigger that could not be bound. Validate checks the triggers
// with the same rules, so this is a safety net; the message is bounded like every issue
// message (maxIssueMessageRunes).
func bindingIssue(err error) Issue {
	return Issue{Code: IssueParamInvalid, Severity: SeverityError, Message: truncateRunes(err.Error(), maxIssueMessageRunes)}
}

// selfTriggerIssues reports the enabled trigger.mission_completed nodes of f that wait
// for missionID, the flow's own mission. Mission Control has no guard against that
// loop, so publishing it would run the flow again after every run.
func selfTriggerIssues(f *Flow, missionID string) []Issue {
	if f == nil || missionID == "" {
		return nil
	}
	var issues []Issue
	for i := range f.Nodes {
		n := &f.Nodes[i]
		if n.Settings.Disabled || n.Type != TypeTriggerMission || textParam(n.Params, "source") != missionID {
			continue
		}
		issues = append(issues, Issue{Code: IssueParamInvalid, Severity: SeverityError, NodeID: n.ID, Param: "source",
			Message: "a flow cannot be started by its own mission; every run would start the next one"})
	}
	return issues
}

// SetEnabled activates or deactivates a published flow. It holds the flow's lock, like
// Publish, so it never arms the timers of a revision that a concurrent Publish replaced.
// From the mission switch on it ignores the cancellation of ctx (see flowLocks).
func (s *Service) SetEnabled(ctx context.Context, id string, enabled bool) error {
	unlock, err := s.locks.lock(ctx, id)
	if err != nil {
		return err
	}
	defer unlock()
	rec, err := s.store.GetFlow(ctx, id)
	if err != nil {
		return err
	}
	if enabled && rec.Live == nil {
		return ErrNotPublished
	}
	// Once the switch is flipped the timers must follow, whatever happens to the caller.
	ctx = context.WithoutCancel(ctx)
	if err := s.bridge.SetFlowMissionEnabled(rec.MissionID, enabled); err != nil {
		return err
	}
	return s.armTimers(ctx, rec)
}

// armTimers arms the Date/Time triggers of the live revision when the flow is enabled,
// and clears them otherwise. One-off times in the past are skipped. The caller holds
// the flow's lock.
func (s *Service) armTimers(ctx context.Context, rec *FlowRecord) error {
	if rec.Live == nil || !s.bridge.FlowMissionEnabled(rec.MissionID) {
		return s.timers.Replace(ctx, rec.ID, nil)
	}
	var timers []TimerRecord
	for i := range rec.Live.Nodes {
		n := &rec.Live.Nodes[i]
		if n.Settings.Disabled || n.Type != TypeTriggerDateTime {
			continue
		}
		b, err := bindDateTime(n, s.services.Loc(), s.now())
		if err != nil {
			continue
		}
		timers = append(timers, TimerRecord{FlowID: rec.ID, NodeID: n.ID, FireAt: b.FireAt, Repeat: b.Repeat})
	}
	return s.timers.Replace(ctx, rec.ID, timers)
}

// DeleteFlow deletes the flow and its mission, and cancels the flow's runs.
//
// The order is: the mission, the timers, the runs, the flow. When a step fails the
// steps before it stay done and DeleteFlow can simply be called again: deleting a
// mission that is already gone is not an error (see MissionBridge.DeleteFlowMission).
// Once the mission is deleted the rest ignores the cancellation of ctx.
func (s *Service) DeleteFlow(ctx context.Context, id string) error {
	unlock, err := s.locks.lock(ctx, id)
	if err != nil {
		return err
	}
	defer unlock()
	rec, err := s.store.GetFlow(ctx, id)
	if err != nil {
		return err
	}
	if rec.MissionID != "" {
		if err := s.bridge.DeleteFlowMission(rec.MissionID); err != nil {
			return err
		}
	}
	// The mission is gone; finish the delete even when the caller goes away.
	return s.deleteLocked(context.WithoutCancel(ctx), id)
}

// DeleteFlowForMission is called when Mission Control deletes a flow mission. A mission
// that no flow holds is ignored. When several flows hold it, it returns
// ErrMissionAmbiguous and deletes nothing. Once it holds the flow's lock it ignores the
// cancellation of ctx: the mission is gone already.
func (s *Service) DeleteFlowForMission(ctx context.Context, missionID string) error {
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
	// Mission Control has deleted the mission already; finish even when the caller goes away.
	ctx = context.WithoutCancel(ctx)
	// The flow may have been deleted while this call waited for the lock.
	cur, err := s.store.GetFlow(ctx, rec.ID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if cur.MissionID != missionID {
		return nil
	}
	return s.deleteLocked(ctx, cur.ID)
}

// deleteLocked switches the flow's timers off, cancels its runs and deletes the flow.
// The caller holds the flow's lock and has removed the mission.
//
// Runs are cancelled before the delete, whose cascade removes their rows; a cancelled
// run that is still running writes its last steps afterwards, finds no row and the
// runner logs that at Debug. A run whose Start recorded it between the first cancel and
// the delete is caught by the second cancel: Runner.CancelFlow waits for a Start in
// progress, and after the delete no Start can record a run of the flow.
func (s *Service) deleteLocked(ctx context.Context, id string) error {
	if err := s.timers.Replace(ctx, id, nil); err != nil {
		return err
	}
	s.runner.CancelFlow(id)
	if err := s.store.DeleteFlow(ctx, id); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	s.runner.CancelFlow(id)
	return nil
}
