package flows

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

const durableApprovalPageSize = 50

type durableApprovalCursor struct {
	ExpiresAt string
	GateID    string
}

type durableApprovalSnapshot struct {
	Item       DurableApprovalItem
	Checkpoint durableCheckpointState
	Ciphertext []byte
	Generation int64
	RunStatus  string
	CPState    string
	WakeAt     time.Time
	Deadline   time.Time
}

func encodeDurableApprovalCursor(item DurableApprovalItem) string {
	return base64.RawURLEncoding.EncodeToString([]byte(formatTime(item.ExpiresAt) + "\n" + item.GateID))
}

func decodeDurableApprovalCursor(raw string) (durableApprovalCursor, error) {
	if raw == "" {
		return durableApprovalCursor{}, nil
	}
	if len(raw) > 256 {
		return durableApprovalCursor{}, ErrDurableApprovalConflict
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) > 192 {
		return durableApprovalCursor{}, ErrDurableApprovalConflict
	}
	parts := strings.SplitN(string(decoded), "\n", 2)
	if len(parts) != 2 || parseTime(parts[0]).IsZero() || parts[1] == "" || len(parts[1]) > 128 {
		return durableApprovalCursor{}, ErrDurableApprovalConflict
	}
	return durableApprovalCursor{ExpiresAt: parts[0], GateID: parts[1]}, nil
}

func (s *Service) ListDurableApprovals(ctx context.Context, cursor string) (DurableApprovalPage, error) {
	page := DurableApprovalPage{Items: make([]DurableApprovalItem, 0)}
	decoded, err := decodeDurableApprovalCursor(cursor)
	if err != nil {
		return page, err
	}
	if !s.runner.durableRuntimeFaulted() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		stop := context.AfterFunc(s.runner.durableRuntimeCtx, cancel)
		defer func() { stop(); cancel() }()
	}
	if err := s.acquireDurableDiagnostic(ctx); err != nil {
		return page, err
	}
	defer func() { <-s.runner.durableDiagnostics }()
	query := `SELECT g.gate_id,g.run_id,g.flow_id,f.name,g.revision,g.node_id,g.node_key,g.admission_owner,g.gate_generation,g.content_digest,g.expires_at,g.state,g.created_at,g.decided_at,g.decision,g.actor,
		EXISTS(SELECT 1 FROM flow_checkpoints c WHERE c.run_id=g.run_id)
		FROM flow_durable_gates g JOIN flows f ON f.id=g.flow_id WHERE g.state='pending'`
	args := []any{}
	if decoded.GateID != "" {
		query += ` AND (g.expires_at>? OR (g.expires_at=? AND g.gate_id>?))`
		args = append(args, decoded.ExpiresAt, decoded.ExpiresAt, decoded.GateID)
	}
	query += ` ORDER BY g.expires_at,g.gate_id LIMIT ?`
	args = append(args, durableApprovalPageSize+1)
	rows, err := s.store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var checkpointsPresent []bool
	for rows.Next() {
		item, hasCheckpoint, err := scanDurableApprovalItem(rows)
		if err != nil {
			return page, err
		}
		page.Items = append(page.Items, item)
		checkpointsPresent = append(checkpointsPresent, hasCheckpoint)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	if err := rows.Close(); err != nil {
		return page, err
	}
	if len(page.Items) > durableApprovalPageSize {
		last := page.Items[durableApprovalPageSize-1]
		page.Items = page.Items[:durableApprovalPageSize]
		checkpointsPresent = checkpointsPresent[:durableApprovalPageSize]
		page.NextCursor = encodeDurableApprovalCursor(last)
	}
	checks := &durableApprovalPageChecks{flows: make(map[string]durableApprovalFlowCheck)}
	for i := range page.Items {
		if err := ctx.Err(); err != nil {
			return DurableApprovalPage{Items: make([]DurableApprovalItem, 0)}, err
		}
		item := &page.Items[i]
		item.BlockedReason = s.durableApprovalPageBlockedReason(ctx, *item, checks)
		item.CanDecide = item.BlockedReason == ""
		item.ReviewAvailable = checkpointsPresent[i] && item.BlockedReason != DurableApprovalBlockKeyUnavailable && item.BlockedReason != DurableApprovalBlockCheckpoint
	}
	return page, nil
}

type durableApprovalFlowCheck struct {
	missionID string
	hasLive   bool
}

// This cache is scoped to one bounded page and stores only fields needed to
// validate approvals; document contents never live past a single read.
type durableApprovalPageChecks struct {
	flows        map[string]durableApprovalFlowCheck
	keyChecked   bool
	keyAvailable bool
}

func (s *Service) durableApprovalPageBlockedReason(ctx context.Context, item DurableApprovalItem, checks *durableApprovalPageChecks) string {
	if item.State != DurableApprovalPending {
		return DurableApprovalBlockDecided
	}
	if !s.now().Before(item.ExpiresAt) {
		return DurableApprovalBlockDeadline
	}
	if s.runner.durableRuntimeFaulted() {
		return DurableRuntimeUnavailableCode
	}
	if !s.cfg.DurableEnabled {
		return DurableApprovalBlockDurableDisabled
	}
	manager, ok := s.bridge.(DurableAuthority)
	if !ok {
		return DurableApprovalBlockDurableDisabled
	}
	enabled, err := manager.DurableEnabled(ctx)
	if err != nil || !enabled || manager.DurableAllowed(ctx) != nil {
		return DurableApprovalBlockDurableDisabled
	}
	if auth, ok := s.bridge.(DurableApprovalAuthority); !ok || auth.DurableApprovalAllowed(ctx) != nil {
		return DurableApprovalBlockAuthUnavailable
	}
	flow, ok := checks.flows[item.FlowID]
	if !ok {
		rec, err := s.requireFlow(ctx, item.FlowID)
		flow = durableApprovalFlowCheck{hasLive: err == nil && rec.Live != nil}
		if err == nil {
			flow.missionID = rec.MissionID
		}
		checks.flows[item.FlowID] = flow
	}
	if !flow.hasLive || flow.missionID != item.AdmissionOwner || manager.AuthorizeDurable(ctx, item.FlowID, item.AdmissionOwner, item.Revision) != nil {
		return DurableApprovalBlockStale
	}
	if s.services.CheckpointKeys == nil {
		return DurableApprovalBlockKeyUnavailable
	}
	if !checks.keyChecked {
		checks.keyChecked = true
		key, err := s.services.CheckpointKeys.CheckpointKey(ctx, false)
		checks.keyAvailable = err == nil && len(key) == 32
		clear(key)
	}
	if !checks.keyAvailable {
		return DurableApprovalBlockKeyUnavailable
	}
	return ""
}

func scanDurableApprovalItem(rows *sql.Rows) (DurableApprovalItem, bool, error) {
	var item DurableApprovalItem
	var expires, created, decided string
	var hasCheckpoint bool
	err := rows.Scan(&item.GateID, &item.RunID, &item.FlowID, &item.FlowName, &item.Revision, &item.NodeID, &item.NodeKey,
		&item.AdmissionOwner, &item.GateGeneration, &item.ContentDigest, &expires, &item.State, &created, &decided,
		&item.Decision, &item.Actor, &hasCheckpoint)
	if err != nil {
		return DurableApprovalItem{}, false, err
	}
	item.ExpiresAt, item.CreatedAt = parseTime(expires), parseTime(created)
	if decided != "" {
		value := parseTime(decided)
		if value.IsZero() {
			return DurableApprovalItem{}, false, errInvalidCheckpoint
		}
		item.DecidedAt = &value
	}
	if item.GateID == "" || item.RunID == "" || item.FlowID == "" || item.FlowName == "" || item.Revision <= 0 || item.NodeID == "" || item.NodeKey == "" ||
		item.AdmissionOwner == "" || item.GateGeneration <= 0 || len(item.ContentDigest) != 64 || item.ExpiresAt.IsZero() || item.CreatedAt.IsZero() {
		return DurableApprovalItem{}, false, errInvalidCheckpoint
	}
	item.ReviewAvailable = item.State == DurableApprovalPending && hasCheckpoint
	return item, hasCheckpoint, nil
}

func (s *Service) GetDurableApproval(ctx context.Context, gateID string) (DurableApprovalDetail, error) {
	item, err := s.getDurableApprovalItem(ctx, gateID)
	if err != nil {
		return DurableApprovalDetail{}, err
	}
	item.BlockedReason = s.durableApprovalBlockedReason(ctx, item)
	item.CanDecide = item.BlockedReason == ""
	detail := DurableApprovalDetail{DurableApprovalItem: item}
	if item.State != DurableApprovalPending {
		item.ReviewAvailable = false
		detail.DurableApprovalItem = item
		return detail, nil
	}
	snapshot, err := s.loadDurableApprovalSnapshot(ctx, item)
	if err != nil {
		item.CanDecide = false
		if errors.Is(err, errCheckpointConflict) || errors.Is(err, errInvalidCheckpoint) {
			item.BlockedReason = DurableApprovalBlockCheckpoint
		} else {
			item.BlockedReason = DurableApprovalBlockKeyUnavailable
		}
		item.ReviewAvailable = false
		detail.DurableApprovalItem = item
		return detail, nil
	}
	if snapshot.Checkpoint.PendingApproval == nil {
		item.CanDecide, item.BlockedReason, item.ReviewAvailable = false, DurableApprovalBlockStale, false
		detail.DurableApprovalItem = item
		return detail, nil
	}
	approval := snapshot.Checkpoint.PendingApproval
	detail.DurableApprovalItem = item
	detail.DurableApprovalItem.NodeLabel = approval.NodeLabel
	detail.DurableApprovalItem.ReviewAvailable = true
	detail.Review = &DurableApprovalReview{Summary: approval.Summary, ReviewData: append([]byte(nil), approval.ReviewData...)}
	return detail, nil
}

func (s *Service) getDurableApprovalItem(ctx context.Context, gateID string) (DurableApprovalItem, error) {
	if gateID == "" || len(gateID) > 128 {
		return DurableApprovalItem{}, ErrDurableApprovalNotFound
	}
	rows, err := s.store.db.QueryContext(ctx, `SELECT g.gate_id,g.run_id,g.flow_id,f.name,g.revision,g.node_id,g.node_key,g.admission_owner,g.gate_generation,g.content_digest,g.expires_at,g.state,g.created_at,g.decided_at,g.decision,g.actor,
		EXISTS(SELECT 1 FROM flow_checkpoints c WHERE c.run_id=g.run_id)
		FROM flow_durable_gates g JOIN flows f ON f.id=g.flow_id WHERE g.gate_id=?`, gateID)
	if err != nil {
		return DurableApprovalItem{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return DurableApprovalItem{}, err
		}
		return DurableApprovalItem{}, ErrDurableApprovalNotFound
	}
	item, _, err := scanDurableApprovalItem(rows)
	if err != nil {
		return DurableApprovalItem{}, err
	}
	return item, nil
}

func (s *Service) getDurableApprovalByRun(ctx context.Context, runID string) (DurableApprovalItem, error) {
	var gateID string
	err := s.store.db.QueryRowContext(ctx, `SELECT gate_id FROM flow_durable_gates WHERE run_id=? AND state='pending' ORDER BY created_at DESC,gate_id DESC LIMIT 1`, runID).Scan(&gateID)
	if errors.Is(err, sql.ErrNoRows) {
		return DurableApprovalItem{}, ErrDurableApprovalNotFound
	}
	if err != nil {
		return DurableApprovalItem{}, err
	}
	return s.getDurableApprovalItem(ctx, gateID)
}

func (s *Service) durableApprovalBlockedReason(ctx context.Context, item DurableApprovalItem) string {
	if item.State != DurableApprovalPending {
		return DurableApprovalBlockDecided
	}
	if s.runner.durableRuntimeFaulted() {
		return DurableRuntimeUnavailableCode
	}
	if !s.now().Before(item.ExpiresAt) {
		return DurableApprovalBlockDeadline
	}
	if !s.cfg.DurableEnabled {
		return DurableApprovalBlockDurableDisabled
	}
	manager, ok := s.bridge.(DurableAuthority)
	if !ok {
		return DurableApprovalBlockDurableDisabled
	}
	enabled, err := manager.DurableEnabled(ctx)
	if err != nil || !enabled || manager.DurableAllowed(ctx) != nil {
		return DurableApprovalBlockDurableDisabled
	}
	auth, ok := s.bridge.(DurableApprovalAuthority)
	if !ok || auth.DurableApprovalAllowed(ctx) != nil {
		return DurableApprovalBlockAuthUnavailable
	}
	rec, err := s.requireFlow(ctx, item.FlowID)
	if err != nil || rec.MissionID != item.AdmissionOwner || rec.Live == nil || manager.AuthorizeDurable(ctx, item.FlowID, item.AdmissionOwner, item.Revision) != nil {
		return DurableApprovalBlockStale
	}
	if s.services.CheckpointKeys == nil {
		return DurableApprovalBlockKeyUnavailable
	}
	key, err := s.services.CheckpointKeys.CheckpointKey(ctx, false)
	clear(key)
	if err != nil || len(key) != 32 {
		return DurableApprovalBlockKeyUnavailable
	}
	return ""
}

func (s *Service) loadDurableApprovalSnapshot(ctx context.Context, item DurableApprovalItem) (durableApprovalSnapshot, error) {
	var snapshot durableApprovalSnapshot
	snapshot.Item = item
	var generation int64
	var ciphertext []byte
	var cpState, wake, deadline, runStatus, flowID, missionID string
	var revision int
	err := s.store.db.QueryRowContext(ctx, `SELECT c.generation,c.state,c.wake_at,c.deadline,c.ciphertext,r.status,r.flow_id,r.revision,r.durable_mission_id
		FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id WHERE c.run_id=? AND length(c.ciphertext)<=?`, item.RunID, maxCheckpointBytes).
		Scan(&generation, &cpState, &wake, &deadline, &ciphertext, &runStatus, &flowID, &revision, &missionID)
	if errors.Is(err, sql.ErrNoRows) {
		return snapshot, errCheckpointConflict
	}
	if err != nil {
		return snapshot, err
	}
	defer clear(ciphertext)
	if generation <= 0 || flowID != item.FlowID || revision != item.Revision || missionID != item.AdmissionOwner || cpState != "waiting" || runStatus != string(RunWaiting) || generation != item.GateGeneration {
		return snapshot, errCheckpointConflict
	}
	if s.services.CheckpointKeys == nil {
		return snapshot, errInvalidCheckpoint
	}
	key, err := s.services.CheckpointKeys.CheckpointKey(ctx, false)
	if err != nil || len(key) != 32 {
		clear(key)
		return snapshot, errInvalidCheckpoint
	}
	defer clear(key)
	plain, err := openCheckpoint(key, item.RunID, uint64(generation), ciphertext)
	if err != nil {
		return snapshot, errInvalidCheckpoint
	}
	defer clear(plain)
	state, err := decodeDurableState(plain)
	if err != nil || state.PendingApproval == nil || state.WaitingNode != item.NodeID || state.Flow.ID != item.FlowID || state.Revision != item.Revision ||
		state.PendingApproval.GateID != item.GateID || state.PendingApproval.NodeID != item.NodeID || state.PendingApproval.NodeKey != item.NodeKey || state.PendingApproval.AdmissionOwner != item.AdmissionOwner ||
		state.PendingApproval.GateGeneration != item.GateGeneration || state.PendingApproval.ContentDigest != item.ContentDigest || state.PendingApproval.Decision != "" || formatTime(state.Deadline) != deadline || formatTime(item.ExpiresAt) != deadline || !durableWakeMatches(state, wake) {
		return snapshot, errInvalidCheckpoint
	}
	digest, err := durableApprovalDigest(state, state.PendingApproval)
	if err != nil || digest != item.ContentDigest {
		return snapshot, errInvalidCheckpoint
	}
	snapshot.Checkpoint, snapshot.Ciphertext, snapshot.Generation = state, append([]byte(nil), ciphertext...), generation
	snapshot.RunStatus, snapshot.CPState, snapshot.WakeAt, snapshot.Deadline = runStatus, cpState, parseTime(wake), parseTime(deadline)
	return snapshot, nil
}

func (s *Service) DecideDurableApproval(ctx context.Context, gateID string, gateGeneration int64, contentDigest, decision, actor string) (DurableApprovalResult, error) {
	if actor != "local_admin" {
		return DurableApprovalResult{}, ErrDurableApprovalAuthUnavailable
	}
	if s.runner.durableRuntimeFaulted() {
		return DurableApprovalResult{}, ErrDurableApprovalUnavailable
	}
	decisionCtx, cancelDecision := context.WithCancel(ctx)
	stopRuntime := context.AfterFunc(s.runner.durableRuntimeCtx, cancelDecision)
	defer func() { stopRuntime(); cancelDecision() }()
	ctx = decisionCtx
	if (decision != DurableApprovalDecisionApprove && decision != DurableApprovalDecisionReject) || gateGeneration <= 0 || len(contentDigest) != 64 {
		return DurableApprovalResult{}, ErrDurableApprovalConflict
	}
	item, err := s.getDurableApprovalItem(ctx, gateID)
	if err != nil {
		return DurableApprovalResult{}, err
	}
	if item.GateGeneration != gateGeneration || item.ContentDigest != contentDigest {
		return DurableApprovalResult{}, ErrDurableApprovalConflict
	}
	if item.State != DurableApprovalPending {
		if item.Decision == decision && item.Actor == actor && (item.State == DurableApprovalApproved || item.State == DurableApprovalRejected) {
			return s.durableApprovalResult(ctx, item, true, nil)
		}
		return DurableApprovalResult{}, ErrDurableApprovalConflict
	}
	if !s.now().Before(item.ExpiresAt) {
		if s.runner.durableRuntimeFaulted() {
			return DurableApprovalResult{}, ErrDurableApprovalUnavailable
		}
		_ = s.expireDurableApproval(ctx, item)
		return DurableApprovalResult{}, ErrDurableApprovalConflict
	}
	if reason := s.durableApprovalBlockedReason(ctx, item); reason != "" {
		if reason == DurableApprovalBlockAuthUnavailable {
			return DurableApprovalResult{}, ErrDurableApprovalAuthUnavailable
		}
		return DurableApprovalResult{}, ErrDurableApprovalUnavailable
	}
	snapshot, err := s.loadDurableApprovalSnapshot(ctx, item)
	if err != nil {
		return DurableApprovalResult{}, ErrDurableApprovalUnavailable
	}
	if !s.now().Before(item.ExpiresAt) {
		if s.runner.durableRuntimeFaulted() {
			return DurableApprovalResult{}, ErrDurableApprovalUnavailable
		}
		_ = s.expireDurableApproval(ctx, item)
		return DurableApprovalResult{}, ErrDurableApprovalConflict
	}
	resumeGeneration := int64(0)
	decisionAt := time.Time{}
	err = withCheckpointKeyMode(ctx, s.store.db, s.services.CheckpointKeys, false, func(tx *sql.Tx, key []byte) error {
		if s.runner.durableRuntimeFaulted() || ctx.Err() != nil {
			return ErrDurableApprovalUnavailable
		}
		if err := lockDurableGuardTx(ctx, tx); err != nil {
			return err
		}
		decisionAt = s.now()
		if !decisionAt.Before(item.ExpiresAt) {
			return ErrDurableApprovalConflict
		}
		var current []byte
		var currentGen int64
		var currentState, status string
		if err := tx.QueryRowContext(ctx, `SELECT c.generation,c.state,c.ciphertext,r.status FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id WHERE c.run_id=?`, item.RunID).Scan(&currentGen, &currentState, &current, &status); err != nil {
			return errCheckpointConflict
		}
		defer clear(current)
		if currentGen != snapshot.Generation || currentState != "waiting" || status != string(RunWaiting) || !bytes.Equal(current, snapshot.Ciphertext) {
			return errCheckpointConflict
		}
		if err := authenticateCheckpointTx(ctx, tx, item.RunID, snapshot.Generation, "", decisionAt, key); err != nil {
			return errCheckpointConflict
		}
		document, err := snapshot.Checkpoint.Flow.Marshal()
		if err != nil {
			return errInvalidCheckpoint
		}
		if err := verifyDurableClaimGuard(ctx, tx, item.RunID, durableClaimGuard{flowID: item.FlowID, missionID: item.AdmissionOwner, revision: item.Revision, document: document}); err != nil {
			return errCheckpointConflict
		}
		if s.runner.durableRuntimeFaulted() || ctx.Err() != nil {
			return ErrDurableApprovalUnavailable
		}
		if decision == DurableApprovalDecisionReject {
			result, err := tx.ExecContext(ctx, `UPDATE flow_durable_gates SET state='rejected',decision='reject',actor=?,decided_at=? WHERE gate_id=? AND run_id=? AND gate_generation=? AND content_digest=? AND state='pending' AND expires_at>?`, actor, formatTime(decisionAt), gateID, item.RunID, gateGeneration, contentDigest, formatTime(decisionAt))
			if err != nil {
				return err
			}
			if n, err := result.RowsAffected(); err != nil || n != 1 {
				return errCheckpointConflict
			}
			result, err = tx.ExecContext(ctx, `UPDATE flow_runs SET status='error',error_code='FLOW_APPROVAL_REJECTED',error_message='the durable approval was rejected',finished_at=? WHERE id=? AND status='waiting' AND durable=1`, formatTime(decisionAt), item.RunID)
			if err != nil {
				return err
			}
			if n, err := result.RowsAffected(); err != nil || n != 1 {
				return errCheckpointConflict
			}
			result, err = tx.ExecContext(ctx, `DELETE FROM flow_checkpoints WHERE run_id=? AND generation=? AND state='waiting'`, item.RunID, snapshot.Generation)
			if err != nil {
				return err
			}
			if n, err := result.RowsAffected(); err != nil || n != 1 {
				return errCheckpointConflict
			}
			return nil
		}
		next := snapshot.Generation + 1
		snapshot.Checkpoint.PendingApproval.Decision = DurableApprovalDecisionApprove
		wakeAt := decisionAt.UTC().Truncate(time.Microsecond)
		if wakeAt.Before(decisionAt) {
			wakeAt = wakeAt.Add(time.Microsecond)
		}
		snapshot.Checkpoint.WakeAt = wakeAt
		plain, err := encodeDurableState(snapshot.Checkpoint)
		if err != nil {
			return err
		}
		defer clear(plain)
		sealed, err := sealCheckpoint(key, item.RunID, uint64(next), plain)
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `UPDATE flow_durable_gates SET state='approved',decision='approve',actor=?,decided_at=? WHERE gate_id=? AND run_id=? AND gate_generation=? AND content_digest=? AND state='pending' AND expires_at>?`, actor, formatTime(decisionAt), gateID, item.RunID, gateGeneration, contentDigest, formatTime(decisionAt))
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			return errCheckpointConflict
		}
		var used int64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(length(ciphertext)),0) FROM flow_checkpoints WHERE run_id<>?`, item.RunID).Scan(&used); err != nil {
			return err
		}
		if !checkpointBytesFit(used, len(sealed)) {
			return errCheckpointCapacity
		}
		result, err = tx.ExecContext(ctx, `UPDATE flow_checkpoints SET generation=?,wake_at=?,ciphertext=?,updated_at=? WHERE run_id=? AND generation=? AND state='waiting' AND EXISTS (SELECT 1 FROM flow_runs WHERE id=? AND status='waiting')`, next, formatTime(wakeAt), sealed, formatTime(decisionAt), item.RunID, snapshot.Generation, item.RunID)
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			return errCheckpointConflict
		}
		resumeGeneration = next
		return nil
	})
	if err != nil {
		if s.runner.durableRuntimeFaulted() {
			return DurableApprovalResult{}, ErrDurableApprovalUnavailable
		}
		if errors.Is(err, errCheckpointConflict) || errors.Is(err, ErrDurableApprovalConflict) {
			current, readErr := s.getDurableApprovalItem(ctx, gateID)
			if readErr == nil && current.GateGeneration == gateGeneration && current.ContentDigest == contentDigest && current.Decision == decision && current.Actor == actor && current.State == map[string]string{DurableApprovalDecisionApprove: DurableApprovalApproved, DurableApprovalDecisionReject: DurableApprovalRejected}[decision] {
				return s.durableApprovalResult(ctx, current, true, nil)
			}
			if !s.now().Before(item.ExpiresAt) {
				if s.runner.durableRuntimeFaulted() {
					return DurableApprovalResult{}, ErrDurableApprovalUnavailable
				}
				_ = s.expireDurableApproval(ctx, item)
			}
			return DurableApprovalResult{}, ErrDurableApprovalConflict
		}
		if !s.now().Before(item.ExpiresAt) {
			if s.runner.durableRuntimeFaulted() {
				return DurableApprovalResult{}, ErrDurableApprovalUnavailable
			}
			_ = s.expireDurableApproval(ctx, item)
			return DurableApprovalResult{}, ErrDurableApprovalConflict
		}
		return DurableApprovalResult{}, err
	}
	item.State, item.Decision, item.Actor = map[string]string{DurableApprovalDecisionApprove: DurableApprovalApproved, DurableApprovalDecisionReject: DurableApprovalRejected}[decision], decision, actor
	item.DecidedAt, item.ReviewAvailable, item.CanDecide, item.BlockedReason = &decisionAt, false, false, DurableApprovalBlockDecided
	if decision == DurableApprovalDecisionReject {
		s.runner.releaseDurableTerminal(item.RunID, item.FlowID)
	}
	return s.durableApprovalResult(ctx, item, false, int64Pointer(resumeGeneration))
}

func int64Pointer(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func (s *Service) durableApprovalResult(ctx context.Context, item DurableApprovalItem, idempotent bool, resume *int64) (DurableApprovalResult, error) {
	var status string
	if err := s.store.db.QueryRowContext(ctx, `SELECT status FROM flow_runs WHERE id=?`, item.RunID).Scan(&status); err != nil {
		return DurableApprovalResult{}, ErrDurableApprovalUnavailable
	}
	return DurableApprovalResult{DurableApprovalItem: item, RunState: RunStatus(status), ResumeGeneration: resume, Idempotent: idempotent}, nil
}

func (s *Service) expireDurableApproval(ctx context.Context, item DurableApprovalItem) error {
	if s.services.CheckpointKeys == nil {
		return ErrDurableApprovalUnavailable
	}
	err := withCheckpointKeyMode(ctx, s.store.db, s.services.CheckpointKeys, false, func(tx *sql.Tx, key []byte) error {
		if err := lockDurableGuardTx(ctx, tx); err != nil {
			return err
		}
		var generation int64
		var ciphertext []byte
		var cpState, status string
		if err := tx.QueryRowContext(ctx, `SELECT c.generation,c.state,c.ciphertext,r.status FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id WHERE c.run_id=?`, item.RunID).Scan(&generation, &cpState, &ciphertext, &status); err != nil {
			return errCheckpointConflict
		}
		defer clear(ciphertext)
		now := s.now()
		if generation != item.GateGeneration || cpState != "waiting" || status != string(RunWaiting) || now.Before(item.ExpiresAt) {
			return errCheckpointConflict
		}
		if err := authenticateCheckpointTx(ctx, tx, item.RunID, generation, "", now, key); err != nil {
			return err
		}
		plain, err := openCheckpoint(key, item.RunID, uint64(generation), ciphertext)
		if err != nil {
			return errInvalidCheckpoint
		}
		state, err := decodeDurableState(plain)
		clear(plain)
		if err != nil || state.RunID != item.RunID || state.Flow.ID != item.FlowID || state.Revision != item.Revision || state.WaitingNode != item.NodeID ||
			state.PendingApproval == nil || state.PendingApproval.GateID != item.GateID || state.PendingApproval.NodeID != item.NodeID || state.PendingApproval.NodeKey != item.NodeKey ||
			state.PendingApproval.AdmissionOwner != item.AdmissionOwner || state.PendingApproval.ContentDigest != item.ContentDigest || state.PendingApproval.GateGeneration != item.GateGeneration ||
			state.PendingApproval.Decision != "" || formatTime(state.Deadline) != formatTime(item.ExpiresAt) {
			return errInvalidCheckpoint
		}
		digest, err := durableApprovalDigest(state, state.PendingApproval)
		if err != nil || digest != item.ContentDigest {
			return errInvalidCheckpoint
		}
		result, err := tx.ExecContext(ctx, `UPDATE flow_durable_gates SET state='expired',decision='',decided_at=? WHERE gate_id=? AND state='pending' AND gate_generation=? AND content_digest=? AND expires_at<=?`, formatTime(now), item.GateID, item.GateGeneration, item.ContentDigest, formatTime(now))
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			return errCheckpointConflict
		}
		result, err = tx.ExecContext(ctx, `UPDATE flow_runs SET status='error',error_code='FLOW_APPROVAL_EXPIRED',error_message='the durable approval expired',finished_at=? WHERE id=? AND status='waiting' AND durable=1`, formatTime(now), item.RunID)
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			return errCheckpointConflict
		}
		result, err = tx.ExecContext(ctx, `DELETE FROM flow_checkpoints WHERE run_id=? AND generation=? AND state='waiting'`, item.RunID, generation)
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			return errCheckpointConflict
		}
		return nil
	})
	if err == nil {
		s.runner.releaseDurableTerminal(item.RunID, item.FlowID)
	}
	return err
}

func (r *Runner) releaseDurableTerminal(runID, flowID string) {
	r.startMu.Lock()
	r.mu.Lock()
	_, active := r.cancels[runID]
	_, reserved := r.durable[runID]
	delete(r.durable, runID)
	if reserved && !active && r.live[flowID] > 0 {
		r.releaseFlowLocked(flowID)
	}
	r.mu.Unlock()
	r.startMu.Unlock()
	r.bus.Finish(runID)
	r.projectDurable(runID)
}
