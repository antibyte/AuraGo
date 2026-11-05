package flows

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"time"
)

var errDurableDeadline = errors.New("durable deadline elapsed")

// durableResume holds only a claimed immutable coordinator snapshot.
type durableResume struct {
	state      durableCheckpointState
	owner      string
	generation int64
	load       func(context.Context) (durableCheckpointState, string, int64, error)
}

func (s *Service) startDurableScheduler() {
	r := s.runner
	if !s.cfg.DurableEnabled || r.cfg.DurableAuthority == nil || r.cfg.DurableHistory == nil || s.services.CheckpointKeys == nil {
		return
	}
	r.mu.Lock()
	if r.closed || r.durableStarted {
		r.mu.Unlock()
		return
	}
	r.durableStarted = true
	for id := range r.durable {
		r.bus.Open(id)
	}
	r.wg.Add(1)
	r.mu.Unlock()
	go func() {
		defer r.wg.Done()
		for {
			s.recoverDurableDue(r.baseCtx)
			select {
			case <-r.baseCtx.Done():
				return
			case <-s.services.clock().After(time.Second):
			}
		}
	}()
}

// recoverDurableDue admits only authenticated pure snapshots and keeps the
// original run ID and per-flow reservation. Failed claims are never retried.
func (s *Service) recoverDurableDue(ctx context.Context) {
	r := s.runner
	reservations, err := s.store.durableReservations(ctx)
	if err != nil {
		return
	}
	for _, item := range reservations {
		if ctx.Err() != nil {
			return
		}
		now := r.now()
		if item.Incompatible || now.Before(item.WakeAt) || (item.State == "claimed" && now.Before(item.LeaseUntil)) {
			continue
		}
		rec, err := s.requireFlow(ctx, item.FlowID)
		if err != nil || rec.Live == nil || rec.MissionID == "" || rec.MissionID != item.MissionID {
			continue
		}
		select {
		case r.durableSlots <- struct{}{}:
		default:
			return
		}
		admitCtx, cancelBase := context.WithCancel(r.baseCtx)
		stopCaller := context.AfterFunc(ctx, cancelBase)
		cancel := func() { stopCaller(); cancelBase() }
		token := r.durableActive.add(item.FlowID, rec.MissionID, ModeLive, cancel, true)
		release := func() { r.durableActive.remove(token); cancel(); <-r.durableSlots }
		header, err := s.store.GetRunHeader(admitCtx, item.RunID)
		if err != nil {
			release()
			continue
		}
		resume := &durableResume{load: func(workerCtx context.Context) (durableCheckpointState, string, int64, error) {
			if s.now().Before(item.Deadline) {
				bounded, cancel := context.WithDeadline(workerCtx, item.Deadline)
				defer cancel()
				return s.claimDurableForWorker(bounded, item, rec.MissionID)
			}
			return s.claimDurableForWorker(workerCtx, item, rec.MissionID)
		}}
		p := &pendingRun{rec: *header, counted: true, waited: true, durableCtx: admitCtx, durableCancel: cancel, durableToken: token, durableResume: resume}
		p.req = StartRequest{Flow: rec.Live, Revision: item.Revision, Mode: ModeLive, TriggerNode: header.TriggerNode, TriggerType: "manual", missionID: rec.MissionID, durable: true}
		r.startMu.Lock()
		// Cancellation or deletion may have committed after the first read.
		var current int
		err = s.store.db.QueryRowContext(admitCtx, `SELECT 1 FROM flow_runs r JOIN flow_checkpoints c ON c.run_id=r.id WHERE r.id=? AND r.durable=1 AND r.status IN ('queued','running','waiting') AND c.generation=?`, item.RunID, item.Generation).Scan(&current)
		if err != nil {
			r.startMu.Unlock()
			release()
			continue
		}
		r.mu.Lock()
		_, active := r.cancels[item.RunID]
		queued := false
		for _, pending := range r.waiting {
			if pending.rec.ID == item.RunID {
				queued = true
				break
			}
		}
		if r.closed || active || queued || admitCtx.Err() != nil {
			r.mu.Unlock()
			r.startMu.Unlock()
			release()
			continue
		}
		r.nextTicket++
		p.ticket = r.nextTicket
		r.waiting = append(r.waiting, p)
		r.dispatchLocked()
		r.mu.Unlock()
		r.startMu.Unlock()
	}
}

func (r *Runner) forgetDeletedDurableFlow(flowID string) {
	r.startMu.Lock()
	r.mu.Lock()
	var ended []string
	for id, rec := range r.durable {
		if rec.FlowID != flowID {
			continue
		}
		delete(r.durable, id)
		if _, active := r.cancels[id]; !active {
			if (rec.Incompatible || !rec.RunStatus.Terminal()) && r.live[flowID] > 0 {
				r.releaseFlowLocked(flowID)
			}
			ended = append(ended, id)
		}
	}
	r.mu.Unlock()
	r.startMu.Unlock()
	for _, id := range ended {
		r.bus.Finish(id)
	}
}

func (s *Service) claimDurableForWorker(ctx context.Context, item durableReservation, missionID string) (durableCheckpointState, string, int64, error) {
	var empty durableCheckpointState
	authority := s.runner.cfg.DurableAuthority
	enabled, err := authority.DurableEnabled(ctx)
	if err != nil || !enabled || authority.DurableAllowed(ctx) != nil {
		return empty, "", 0, ErrDurableUnavailable
	}
	rec, err := s.requireFlow(ctx, item.FlowID)
	if err != nil || rec.Live == nil || rec.MissionID != missionID || authority.AuthorizeDurable(ctx, item.FlowID, missionID, item.Revision) != nil {
		return empty, "", 0, ErrDurableUnavailable
	}
	key, err := s.services.CheckpointKeys.CheckpointKey(ctx, false)
	if err != nil || len(key) != 32 {
		clear(key)
		return empty, "", 0, ErrDurableUnavailable
	}
	blocked, err := s.checkpointBlocked(ctx, item.RunID, item.Generation, key)
	clear(key)
	if err != nil || blocked != "" || ctx.Err() != nil {
		return empty, "", 0, ErrDurableUnavailable
	}
	version, err := s.store.GetVersion(ctx, item.FlowID, item.Revision)
	if err != nil {
		return empty, "", 0, ErrDurableUnavailable
	}
	document, err := version.Marshal()
	if err != nil {
		return empty, "", 0, ErrDurableUnavailable
	}
	guard := durableClaimGuard{flowID: item.FlowID, missionID: missionID, revision: item.Revision, document: document}
	now := s.now()
	if !now.Before(item.Deadline) {
		if err := s.store.expireDurableCheckpoint(ctx, item.RunID, item.Generation, now, guard, s.services.CheckpointKeys); err != nil {
			return empty, "", 0, err
		}
		return empty, "", 0, errDurableDeadline
	}
	owner := NewRunID()
	claimed, err := claimDurableCheckpoint(ctx, s.store.db, item.RunID, item.Generation, owner, now, now.Add(durableLease), guard)
	if err != nil || !claimed {
		return empty, "", 0, ErrDurableUnavailable
	}
	state, err := s.store.claimedDurableState(ctx, item.RunID, item.Generation, owner, s.now(), s.services.CheckpointKeys)
	if err != nil || ctx.Err() != nil {
		return empty, "", 0, ErrDurableUnavailable
	}
	saved, e2 := state.Flow.Marshal()
	if e2 != nil || !bytes.Equal(document, saved) {
		return empty, "", 0, ErrDurableUnavailable
	}
	return state, owner, item.Generation, nil
}

// expireDurableCheckpoint runs only after authentication and authorization.
// The original deadline and generation fence the terminal transition.
func (s *Store) expireDurableCheckpoint(ctx context.Context, runID string, generation int64, now time.Time, guard durableClaimGuard, keys CheckpointKeyProvider) error {
	return withCheckpointKeyMode(ctx, s.db, keys, false, func(tx *sql.Tx, key []byte) error {
		if err := verifyDurableClaimGuard(ctx, tx, runID, guard); err != nil {
			return err
		}
		if err := authenticateCheckpointTx(ctx, tx, runID, generation, "", now, key); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE flow_runs SET status='error',error_code='FLOW_RUN_TIMEOUT',error_message='the run took longer than its time limit',finished_at=? WHERE id=? AND durable=1 AND status IN ('queued','running','waiting') AND EXISTS (SELECT 1 FROM flow_checkpoints WHERE run_id=? AND generation=? AND deadline<=? AND (state='waiting' OR (state='claimed' AND lease_until<=?)))`, formatTime(now), runID, runID, generation, formatTime(now), formatTime(now))
		if err != nil {
			return err
		}
		if err := requireRow(res, errCheckpointConflict); err != nil {
			return err
		}
		res, err = tx.ExecContext(ctx, `DELETE FROM flow_checkpoints WHERE run_id=? AND generation=?`, runID, generation)
		if err != nil {
			return err
		}
		return requireRow(res, errCheckpointConflict)
	})
}

// Heartbeats never call an authority while holding a runner/store lock. A
// failed renewal or revocation stops the worker and preserves its checkpoint.
func (r *Runner) startDurableHeartbeat(ctx context.Context, cancel context.CancelFunc, p *pendingRun, control *durableRunControl) func() {
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-r.engine.services.clock().After(durableLease / 3):
				allowed := r.cfg.DurableAuthority.DurableAllowed(ctx)
				if allowed == nil {
					allowed = r.cfg.DurableAuthority.AuthorizeDurable(ctx, p.rec.FlowID, p.req.missionID, p.rec.Revision)
				}
				if allowed != nil || control.renew(ctx) != nil {
					cancel()
					return
				}
			}
		}
	}()
	return func() { close(stop); <-done }
}
