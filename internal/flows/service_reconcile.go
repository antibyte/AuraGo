package flows

import (
	"context"
	"errors"
	"sort"
	"time"
)

// reconcileLockWait bounds how long ReconcileMissions waits for the lock of one flow. A
// flow that stays busy longer is skipped and logged: the operation holding the lock
// updates Mission Control itself. A variable so tests can shorten it.
var reconcileLockWait = 10 * time.Second

// MissionReconciler is an optional extension of MissionBridge for ReconcileMissions. Its
// methods follow the bridge rule (no flow-lock-taking Service method, see MissionBridge).
// Without it ReconcileMissions re-syncs every published flow and learns of a missing
// mission only through SyncFlowMission's error.
type MissionReconciler interface {
	// FlowMissions returns the flow missions Mission Control holds, mission id → flow id,
	// or nil when it cannot tell (then nothing is reported as missing).
	FlowMissions() map[string]string
	// FlowMissionInSync reports whether the mission is marked published and holds name
	// and bindings already, as SyncFlowMission would store them.
	FlowMissionInSync(missionID, name string, bindings []TriggerBinding) bool
}

// reconcileTally counts what ReconcileMissions did, for its Debug summary.
type reconcileTally struct {
	synced, armed, cleared, busy, problems int
}

// ReconcileMissions brings Mission Control back in line with the store once, after Start
// (the server runs it on a goroutine of its own, so a large flows.db does not hold up the
// start). A crash between a store write and the Mission Control update (Publish,
// SetEnabled, MissionEnabledChanged, the deletes) can leave the two apart, and Start
// repairs nothing.
//
// It visits the flows one at a time, in id order, each under its flow lock (the lock
// order holds: never two flow locks, and under the lock only the store, the bridge and
// TimerService.Replace), waiting at most reconcileLockWait per flow; a busy flow is
// skipped. For a published flow it
//   - re-syncs the mission's name and trigger bindings from the live revision, bound at
//     its publish time so that a one-off date that has passed since still binds as it did
//     then, unless the bridge (MissionReconciler) reports them in sync already;
//   - lets the Date/Time timers follow the mission's enabled switch, which Mission Control
//     owns (the store keeps no enabled flag), and the live revision: a disabled flow loses
//     its timers, and an enabled flow whose stored timers are stale (staleTimers: a node
//     or repeat that the live revision does not have, or a trigger without its timer) is
//     armed again. Timers that match stay as they are, so an occurrence that is due right
//     now is never dropped by a re-bind.
//
// It deletes nothing. It logs at Warn a flow whose mission is empty or gone (also an
// unpublished one) and, with a MissionReconciler, a flow mission whose flow is gone. A
// flow created while it runs can be reported once as having no flow.
//
// It returns ctx's error when ctx ends, ErrRunnerClosed after Shutdown (Shutdown waits
// for it; it stops after the flow it is working on), and the store's error when the
// flows cannot be listed. Problems with single flows are logged and do not stop it.
// The total duration is logged at Debug.
func (s *Service) ReconcileMissions(ctx context.Context) error {
	begin := time.Now()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrRunnerClosed
	}
	s.reconciling.Add(1)
	s.mu.Unlock()
	defer s.reconciling.Done()
	// Shutdown ends the waits for flow locks.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-s.stop:
			cancel()
		case <-ctx.Done():
		}
	}()

	recon, _ := s.bridge.(MissionReconciler)
	var missions map[string]string
	if recon != nil {
		// Taken before the flows are listed: CreateFlow makes the mission first, so every
		// listed flow's mission is in it unless it is really gone (or was made later,
		// which missionGone checks).
		missions = recon.FlowMissions()
	}
	refs, err := s.store.flowMissionRefs(ctx)
	if err != nil {
		return s.reconcileStopped(err)
	}
	var tally reconcileTally
	for _, ref := range refs {
		if err := s.reconcileFlow(ctx, ref.id, recon, &missions, &tally); err != nil {
			return s.reconcileStopped(err)
		}
	}
	if missions != nil {
		s.reportOrphanMissions(ctx, recon, missions, refs, &tally)
	}
	s.logger.Debug("flow missions reconciled with Mission Control", "flows", len(refs), "synced", tally.synced,
		"timers_armed", tally.armed, "timers_cleared", tally.cleared, "busy", tally.busy, "problems", tally.problems,
		"duration", time.Since(begin).String())
	return nil
}

// reconcileStopped returns ErrRunnerClosed for an error that ended ReconcileMissions after
// Shutdown (a cancelled lock wait, a closed store), else err.
func (s *Service) reconcileStopped(err error) error {
	if s.isClosed() {
		return ErrRunnerClosed
	}
	return err
}

// reconcileFlow reconciles one flow under its lock. It returns only ctx's error and
// ErrRunnerClosed after Shutdown; every other problem is logged and counted.
func (s *Service) reconcileFlow(ctx context.Context, id string, recon MissionReconciler, missions *map[string]string, tally *reconcileTally) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.isClosed() {
		return ErrRunnerClosed
	}
	lockCtx, cancelLock := context.WithTimeout(ctx, reconcileLockWait)
	unlock, err := s.locks.lock(lockCtx, id)
	cancelLock()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		tally.busy++
		s.logger.Warn("a flow stayed busy; it was not checked against Mission Control", "flow", id,
			"waited", reconcileLockWait.String())
		return nil
	}
	defer unlock()
	rec, err := s.store.GetFlow(ctx, id)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !errors.Is(err, ErrNotFound) { // a flow deleted meanwhile is fine
			tally.problems++
			s.logger.Warn("a flow could not be read; it was not checked against Mission Control", "flow", id, "error", err)
		}
		return nil
	}
	if rec.MissionID == "" {
		tally.problems++
		s.logger.Warn("a flow has no Mission Control mission and cannot run from it", "flow", id)
		return nil
	}
	if recon != nil && *missions != nil && s.missionGone(recon, missions, rec.MissionID) {
		tally.problems++
		s.logger.Warn("the Mission Control mission of a flow is gone; the flow cannot run from Mission Control "+
			"(delete it in EasyDrag, or export and import it again)", "flow", id, "mission_id", rec.MissionID)
		return nil
	}
	if rec.Live == nil {
		return nil
	}
	// From here on Mission Control and the timers change: finish this flow whatever
	// happens to ctx (see flowLocks).
	ctx = context.WithoutCancel(ctx)
	s.reconcileBindings(rec, recon, tally)
	s.reconcileTimers(ctx, rec, tally)
	return nil
}

// missionGone reports whether Mission Control holds no mission missionID. The snapshot is
// refreshed once before a mission counts as gone, since a mission made after it is no
// problem; the refreshed snapshot is kept for the next flows.
func (s *Service) missionGone(recon MissionReconciler, missions *map[string]string, missionID string) bool {
	if _, ok := (*missions)[missionID]; ok {
		return false
	}
	fresh := recon.FlowMissions()
	if fresh == nil {
		return false
	}
	*missions = fresh
	_, ok := fresh[missionID]
	return !ok
}

// reconcileBindings re-syncs the mission of a published flow unless it is in sync. The
// caller holds the flow's lock.
func (s *Service) reconcileBindings(rec *FlowRecord, recon MissionReconciler, tally *reconcileTally) {
	at := rec.PublishedAt
	if at.IsZero() {
		at = s.now()
	}
	bindings, err := BindTriggers(rec.Live, s.reg, s.services.Loc(), at)
	if err != nil {
		tally.problems++
		s.logger.Warn("the triggers of a published flow could not be bound; its Mission Control triggers were not checked",
			"flow", rec.ID, "error", truncateRunes(err.Error(), maxIssueMessageRunes))
		return
	}
	if recon != nil && recon.FlowMissionInSync(rec.MissionID, rec.Live.Name, bindings) {
		return
	}
	if err := s.bridge.SyncFlowMission(rec.MissionID, rec.Live.Name, bindings); err != nil {
		tally.problems++
		s.logger.Warn("the Mission Control mission of a published flow could not be updated", "flow", rec.ID,
			"mission_id", rec.MissionID, "error", truncateRunes(err.Error(), maxIssueMessageRunes))
		return
	}
	tally.synced++
	s.logger.Info("a flow's Mission Control triggers were out of date and were updated", "flow", rec.ID,
		"mission_id", rec.MissionID)
}

// reconcileTimers lets the Date/Time timers of a published flow follow the mission's
// enabled switch and the live revision. It writes only when they disagree: the timers of
// a disabled flow are cleared, and an enabled flow is armed again (armTimers) when its
// stored timers are stale (staleTimers). Otherwise the stored fire times stay as they
// are, so an occurrence that is due now is never dropped. The caller holds the flow's
// lock.
func (s *Service) reconcileTimers(ctx context.Context, rec *FlowRecord, tally *reconcileTally) {
	enabled := s.bridge.FlowMissionEnabled(rec.MissionID)
	stored, err := s.store.flowTimerSlots(ctx, rec.ID)
	if err != nil {
		tally.problems++
		s.logger.Warn("the timers of a flow could not be read", "flow", rec.ID, "error", err)
		return
	}
	switch {
	case !enabled && len(stored) > 0:
		if err := s.timers.Replace(ctx, rec.ID, nil); err != nil {
			tally.problems++
			s.logger.Warn("the timers of a disabled flow could not be cleared", "flow", rec.ID, "error", err)
			return
		}
		tally.cleared++
	case enabled && s.staleTimers(rec, stored):
		if err := s.armTimers(ctx, rec); err != nil {
			tally.problems++
			s.logger.Warn("the timers of an enabled flow could not be armed", "flow", rec.ID, "error", err)
			return
		}
		tally.armed++
	}
}

// staleTimers reports whether the stored timers (node id → repeat) of an enabled flow
// disagree with its live revision, as after a crash between the store publish and
// armTimers: a stored timer whose node is no enabled Date/Time trigger of the live
// revision or repeats differently (bound at the publish time, like the bindings), or an
// armTimers candidate (a trigger that binds now) without a stored timer of the same
// repeat. A stored one-off timer whose time has passed is no candidate any more but
// still belongs to the revision: it is due, and it stays.
func (s *Service) staleTimers(rec *FlowRecord, stored map[string]string) bool {
	at := rec.PublishedAt
	if at.IsZero() {
		at = s.now()
	}
	loc, now := s.services.Loc(), s.now()
	live := map[string]string{}
	for i := range rec.Live.Nodes {
		n := &rec.Live.Nodes[i]
		if n.Settings.Disabled || n.Type != TypeTriggerDateTime {
			continue
		}
		if b, err := bindDateTime(n, loc, at); err == nil {
			live[n.ID] = b.Repeat
		}
		if b, err := bindDateTime(n, loc, now); err == nil {
			if repeat, ok := stored[n.ID]; !ok || repeat != b.Repeat {
				return true // an armTimers candidate without its timer
			}
		}
	}
	for node, repeat := range stored {
		if want, ok := live[node]; !ok || want != repeat {
			return true
		}
	}
	return false
}

// reportOrphanMissions logs every flow mission whose flow is gone: no listed flow holds
// it, Mission Control still holds it now (a fresh snapshot, so a mission deleted
// meanwhile is not reported) and the store, asked again, knows no flow for it. Nothing
// is deleted; Mission Control's delete removes such a mission.
func (s *Service) reportOrphanMissions(ctx context.Context, recon MissionReconciler, missions map[string]string,
	refs []flowMissionRef, tally *reconcileTally) {
	held := make(map[string]bool, len(refs))
	for _, ref := range refs {
		held[ref.missionID] = true
	}
	var ids []string
	for id := range missions {
		if !held[id] {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return
	}
	current := recon.FlowMissions()
	sort.Strings(ids)
	for _, missionID := range ids {
		if _, ok := current[missionID]; !ok {
			continue // deleted meanwhile, or Mission Control cannot tell now
		}
		if _, err := s.store.flowIDByMission(ctx, missionID); !errors.Is(err, ErrNotFound) {
			continue // made meanwhile, or the store failed: no report
		}
		tally.problems++
		s.logger.Warn("Mission Control holds a flow mission whose flow is gone; delete the mission in Mission Control",
			"mission_id", truncateRunes(missionID, 80), "flow", truncateRunes(missions[missionID], 80))
	}
}
