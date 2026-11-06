package flows

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"regexp"
)

// checkpointBlocked authenticates the actual ciphertext before claiming a run
// is recoverable. It does not mutate or expose the saved execution state.
func (s *Service) checkpointBlocked(ctx context.Context, runID string, generation int64, key []byte) (string, error) {
	var sealed []byte
	var flowID, triggerNode, triggerType, deadline, wake, missionID, frozenMission, mode, checkpointState, runStatus, lease string
	var revision int
	err := s.store.db.QueryRowContext(ctx, `SELECT c.ciphertext,r.flow_id,r.revision,r.trigger_node,r.trigger_type,c.deadline,c.wake_at,f.mission_id,r.durable_mission_id,r.mode,c.state,r.status,c.lease_until FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id JOIN flows f ON f.id=r.flow_id WHERE c.run_id=? AND c.generation=? AND length(c.ciphertext)<=?`, runID, generation, maxCheckpointBytes).Scan(&sealed, &flowID, &revision, &triggerNode, &triggerType, &deadline, &wake, &missionID, &frozenMission, &mode, &checkpointState, &runStatus, &lease)
	if errors.Is(err, sql.ErrNoRows) {
		return "checkpoint_unreadable", nil
	}
	if err != nil {
		return "", err
	}
	plain, err := openCheckpoint(key, runID, uint64(generation), sealed)
	clear(sealed)
	if err != nil {
		return "checkpoint_unreadable", nil
	}
	state, err := decodeDurableState(plain)
	clear(plain)
	if err != nil {
		return "checkpoint_unreadable", nil
	}
	if state.Version < 2 || state.Version > 3 {
		return "incompatible", nil
	}
	if (checkpointState == "waiting" && runStatus != string(RunWaiting)) || (checkpointState == "claimed" && ((runStatus != string(RunQueued) && runStatus != string(RunRunning)) || parseTime(lease).IsZero())) || (checkpointState != "waiting" && checkpointState != "claimed") {
		return "incompatible", nil
	}
	if state.RunID != runID || state.Flow.ID != flowID || state.Revision != revision || state.TriggerNode != triggerNode || state.Trigger["type"] != "manual" || triggerType != "manual" || mode != string(ModeLive) || !durableWakeMatches(state, wake) || formatTime(state.Deadline) != deadline {
		return "checkpoint_unreadable", nil
	}
	if _, err := restoreDurableLocation(state.Flow, state.Location); err != nil {
		return "incompatible", nil
	}
	rec, err := s.requireFlow(ctx, flowID)
	if err != nil {
		return "authority", nil
	}
	if rec.Live == nil || rec.MissionID != missionID || missionID == "" || frozenMission != missionID {
		return "authority", nil
	}
	version, err := s.store.GetVersion(ctx, flowID, revision)
	if err != nil {
		return "authority", nil
	}
	live, e1 := rec.Live.Marshal()
	saved, e2 := state.Flow.Marshal()
	versionDoc, e3 := version.Marshal()
	if e1 != nil || e2 != nil || e3 != nil || (rec.LiveRevision == revision && !bytes.Equal(live, saved)) || !bytes.Equal(versionDoc, saved) {
		return "incompatible", nil
	}
	authority, ok := s.bridge.(DurableAuthority)
	if !ok || authority.AuthorizeDurable(ctx, flowID, missionID, revision) != nil {
		return "authority", nil
	}
	return "", nil
}

var durableProjectionCode = regexp.MustCompile(`^[A-Z0-9_]{1,64}$`)

func (s *Service) durableReadiness(ctx context.Context, needsKey bool) (DurableStatus, []byte) {
	var out DurableStatus
	authority, ok := s.bridge.(DurableAuthority)
	if ok {
		enabled, err := authority.DurableEnabled(ctx)
		if err != nil {
			out.Reason = "authority"
		} else {
			out.Configured = enabled
		}
	} else {
		out.Reason = "authority"
	}
	out.Effective = s.cfg.DurableEnabled && out.Configured && ok
	if out.Effective && authority.DurableAllowed(ctx) != nil {
		out.Effective = false
		out.Reason = "authority"
	}
	if !out.Effective && out.Reason == "" {
		out.Reason = "disabled"
	}
	s.runner.mu.Lock()
	running, failure, faulted := s.runner.durableStarted, s.runner.recoveryErr, s.runner.durableFaulted
	s.runner.mu.Unlock()
	if faulted {
		out.Reason = DurableRuntimeUnavailableCode
		return out, nil
	}
	out.RecoveryAvailable = out.Effective && running && failure == nil && s.services.CheckpointKeys != nil
	var key []byte
	if out.RecoveryAvailable && needsKey {
		var err error
		key, err = s.services.CheckpointKeys.CheckpointKey(ctx, false)
		if err != nil || len(key) != 32 {
			clear(key)
			key = nil
			out.RecoveryAvailable = false
			out.Reason = "key_unavailable"
		}
	}
	if out.Effective && !out.RecoveryAvailable && out.Reason == "" {
		out.Reason = "storage_unavailable"
	}
	return out, key
}

// The diagnostic slot prevents concurrent callers from multiplying the
// bounded ciphertext decoding work; waiting respects the request context.
func (s *Service) acquireDurableDiagnostic(ctx context.Context) error {
	select {
	case s.runner.durableDiagnostics <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// DurableStatus reports bounded metadata; ciphertext and resolved inputs never leave the store.
func (s *Service) DurableStatus(ctx context.Context) (DurableStatus, error) {
	if err := s.acquireDurableDiagnostic(ctx); err != nil {
		return DurableStatus{}, err
	}
	defer func() { <-s.runner.durableDiagnostics }()
	type item struct {
		id, state  string
		generation int64
		marker     int
	}
	rows, err := s.store.db.QueryContext(ctx, `SELECT c.run_id,c.state,c.generation,r.durable FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id ORDER BY c.run_id LIMIT ?`, maxDurableCheckpoints+1)
	if err != nil {
		return DurableStatus{}, err
	}
	var items []item
	for rows.Next() {
		var v item
		if err := rows.Scan(&v.id, &v.state, &v.generation, &v.marker); err != nil {
			rows.Close()
			return DurableStatus{}, err
		}
		items = append(items, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return DurableStatus{}, err
	}
	if len(items) > maxDurableCheckpoints {
		return DurableStatus{}, errCheckpointCapacity
	}
	out, key := s.durableReadiness(ctx, len(items) > 0)
	defer clear(key)
	for _, v := range items {
		if err := ctx.Err(); err != nil {
			return DurableStatus{}, err
		}
		if v.marker != 1 || !out.RecoveryAvailable {
			out.BlockedCount++
			continue
		}
		blocked, err := s.checkpointBlocked(ctx, v.id, v.generation, key)
		if err != nil {
			return DurableStatus{}, err
		}
		if blocked != "" {
			out.BlockedCount++
			continue
		}
		switch v.state {
		case "waiting":
			out.WaitingCount++
		case "claimed":
			out.ClaimedCount++
		default:
			out.BlockedCount++
		}
	}
	// A failed admission/start may leave a marker without a checkpoint or an
	// in-memory owner. Count it as blocked instead of reporting an empty store.
	unowned, err := s.store.db.QueryContext(ctx, `SELECT r.id FROM flow_runs r LEFT JOIN flow_checkpoints c ON c.run_id=r.id WHERE r.durable=1 AND r.status IN ('queued','running') AND c.run_id IS NULL ORDER BY r.id LIMIT ?`, maxDurableCheckpoints+1)
	if err != nil {
		return DurableStatus{}, err
	}
	var ids []string
	for unowned.Next() {
		var id string
		if err := unowned.Scan(&id); err != nil {
			unowned.Close()
			return DurableStatus{}, err
		}
		ids = append(ids, id)
	}
	err = unowned.Err()
	unowned.Close()
	if err != nil {
		return DurableStatus{}, err
	}
	if len(ids) > maxDurableCheckpoints {
		return DurableStatus{}, errCheckpointCapacity
	}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return DurableStatus{}, err
		}
		active, queued := s.runner.durableRunOwnership(id)
		if !active && !queued {
			out.BlockedCount++
		}
	}
	return out, nil
}

// DurableRunStatus reads checkpoint metadata with its owning run, then performs
// bounded current-policy and ciphertext diagnostics. The observation may race
// later changes; cancellation still checks the generation transactionally.
// Legacy rows without the marker remain blocked and explicitly cancellable.
func (s *Service) DurableRunStatus(ctx context.Context, runID string) (*DurableRunStatus, error) {
	var marker int
	var runStatus string
	var gen sql.NullInt64
	var state, wake, deadline, gateID, gateState sql.NullString
	err := s.store.db.QueryRowContext(ctx, `SELECT r.durable,r.status,c.generation,c.state,c.wake_at,c.deadline,
		(SELECT g.gate_id FROM flow_durable_gates g WHERE g.run_id=r.id ORDER BY g.created_at DESC,g.gate_id DESC LIMIT 1),
		(SELECT g.state FROM flow_durable_gates g WHERE g.run_id=r.id ORDER BY g.created_at DESC,g.gate_id DESC LIMIT 1)
		FROM flow_runs r LEFT JOIN flow_checkpoints c ON c.run_id=r.id WHERE r.id=?`, runID).Scan(&marker, &runStatus, &gen, &state, &wake, &deadline, &gateID, &gateState)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}
	if !gen.Valid {
		out := &DurableRunStatus{RunID: runID, CancelAvailable: marker == 1 && !RunStatus(runStatus).Terminal()}
		if gateID.Valid {
			out.ApprovalGateID = gateID.String
		}
		if marker == 1 && !RunStatus(runStatus).Terminal() && s.runner.durableRuntimeFaulted() {
			out.State = "blocked"
			out.BlockedCode = DurableRuntimeUnavailableCode
			return out, nil
		}
		if gateState.Valid && gateState.String == DurableApprovalPending {
			out.State = "approval_pending"
			return out, nil
		}
		if marker == 1 && (runStatus == string(RunQueued) || runStatus == string(RunRunning)) {
			active, queued := s.runner.durableRunOwnership(runID)
			switch {
			case active:
				out.State = "running"
			case queued:
				out.State = "waiting"
			default:
				out.State = "blocked"
				out.BlockedCode = "storage_unavailable"
			}
			return out, nil
		}
		if gateID.Valid {
			out.State = runStatus
			return out, nil
		}
		return nil, nil
	}
	out := &DurableRunStatus{RunID: runID, Generation: gen.Int64, CancelAvailable: true}
	if gateID.Valid {
		out.ApprovalGateID = gateID.String
	}
	if t := parseTime(wake.String); !t.IsZero() && !(gateState.Valid && gateState.String == DurableApprovalPending) {
		u := t.UTC()
		out.WakeAt = &u
	}
	if t := parseTime(deadline.String); !t.IsZero() {
		u := t.UTC()
		out.Deadline = &u
	}
	if marker == 0 {
		out.State = "blocked"
		out.BlockedCode = "incompatible"
		return out, nil
	}
	if s.runner.durableRuntimeFaulted() {
		out.State = "blocked"
		out.BlockedCode = DurableRuntimeUnavailableCode
		return out, nil
	}
	if err := s.acquireDurableDiagnostic(ctx); err != nil {
		return nil, err
	}
	defer func() { <-s.runner.durableDiagnostics }()
	status, key := s.durableReadiness(ctx, true)
	defer clear(key)
	if !status.Effective {
		out.State = "blocked"
		out.BlockedCode = status.Reason
		return out, nil
	}
	if !status.RecoveryAvailable {
		out.State = "blocked"
		out.BlockedCode = status.Reason
		return out, nil
	}
	blocked, err := s.checkpointBlocked(ctx, runID, gen.Int64, key)
	if err != nil {
		return nil, err
	}
	if blocked != "" {
		out.State, out.BlockedCode = "blocked", blocked
		return out, nil
	}
	if gateState.Valid && gateState.String == DurableApprovalPending {
		out.State = "approval_pending"
	} else if state.String == "waiting" {
		out.State = "waiting"
	} else if state.String == "claimed" {
		active, _ := s.runner.durableRunOwnership(runID)
		if active {
			out.State = "running"
		} else {
			out.State = "waiting"
		}
	} else {
		out.State = "blocked"
		out.BlockedCode = "checkpoint_unreadable"
	}
	return out, nil
}

func (r *Runner) durableRunOwnership(runID string) (active, queued bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, active = r.cancels[runID]
	for _, p := range r.waiting {
		if p.rec.ID == runID {
			queued = true
			return
		}
	}
	for _, q := range r.flowQueue {
		for _, p := range q {
			if p.rec.ID == runID {
				queued = true
				return
			}
		}
	}
	return
}

func scanDurableProjection(row scanner) (*DurableProjection, error) {
	var p DurableProjection
	var status, started, finished string
	if err := row.Scan(&p.RunID, &p.FlowID, &p.MissionID, &p.Revision, &status, &started, &finished, &p.ErrorCode); err != nil {
		return nil, err
	}
	p.Status = RunStatus(status)
	p.StartedAt = parseTime(started)
	if finished != "" {
		at := parseTime(finished)
		p.FinishedAt = &at
	}
	if p.ErrorCode != "" && !durableProjectionCode.MatchString(p.ErrorCode) {
		p.ErrorCode = "FLOW_NODE_UNAVAILABLE"
	}
	return &p, nil
}

const durableProjectionSQL = `SELECT r.id,r.flow_id,r.durable_mission_id,r.revision,r.status,r.started_at,r.finished_at,r.error_code FROM flow_runs r JOIN flows f ON f.id=r.flow_id WHERE r.durable=1 AND r.durable_mission_id<>'' AND f.kind='flow'`

func (s *Service) DurableProjection(ctx context.Context, runID string) (*DurableProjection, error) {
	p, err := scanDurableProjection(s.store.db.QueryRowContext(ctx, durableProjectionSQL+` AND r.id=?`, runID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRunNotFound
	}
	return p, err
}

func (s *Service) ListDurableProjections(ctx context.Context, cursor string, limit int) (*DurableProjectionPage, error) {
	if limit < 1 || limit > 100 || len(cursor) > 128 {
		return nil, ErrDurableUnavailable
	}
	rows, err := s.store.db.QueryContext(ctx, durableProjectionSQL+` AND r.id>? ORDER BY r.id LIMIT ?`, cursor, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &DurableProjectionPage{Items: []DurableProjection{}}
	for rows.Next() {
		p, err := scanDurableProjection(rows)
		if err != nil {
			return nil, err
		}
		if len(out.Items) == limit {
			out.NextCursor = out.Items[len(out.Items)-1].RunID
			break
		}
		out.Items = append(out.Items, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
