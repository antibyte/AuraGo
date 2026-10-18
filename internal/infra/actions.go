package infra

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const (
	ActionRunning   = "running"
	ActionAccepted  = "accepted"
	ActionCompleted = "completed"
	ActionFailed    = "failed"
	ActionUnknown   = "unknown"
)

var ErrActionConflict = errors.New("infra request id conflicts with an earlier action")

// actionRunningTimeout bounds how long a reservation may look in flight before a
// read treats it as an unresolved (unknown) outcome.
const actionRunningTimeout = 2 * time.Minute

// ReserveAction persists the request before a provider mutation. Only the first
// reservation may dispatch; running rows left after restart are made unknown.
func (s *Store) ReserveAction(ctx context.Context, request ActionRequest) (ActionRecord, bool, error) {
	if err := validateText("request id", request.RequestID, 128, true); err != nil {
		return ActionRecord{}, false, err
	}
	if len(request.Fingerprint) != 64 {
		return ActionRecord{}, false, fmt.Errorf("%w: action fingerprint must be a SHA-256 hex digest", ErrValidation)
	}
	if _, err := hex.DecodeString(request.Fingerprint); err != nil {
		return ActionRecord{}, false, fmt.Errorf("%w: invalid action fingerprint", ErrValidation)
	}
	switch request.Action {
	case "start", "stop", "restart", "shutdown", "reboot", "wake":
	default:
		return ActionRecord{}, false, fmt.Errorf("%w: unsupported action", ErrValidation)
	}
	now := formatTime(time.Now().UTC())
	res, err := s.db.ExecContext(ctx, `INSERT INTO infra_action_requests(request_id,fingerprint,action,status,upid,created_at,updated_at)
		VALUES(?,?,?,'running','',?,?) ON CONFLICT(request_id) DO NOTHING`,
		request.RequestID, request.Fingerprint, request.Action, now, now)
	if err != nil {
		return ActionRecord{}, false, fmt.Errorf("reserve infra action: %w", err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		return ActionRecord{}, false, fmt.Errorf("count infra action reservation: %w", err)
	}
	if inserted == 1 {
		return ActionRecord{RequestID: request.RequestID, Action: request.Action, Status: ActionRunning}, true, nil
	}
	record, fingerprint, updatedAt, err := loadAction(ctx, s.db, request.RequestID)
	if err != nil {
		return ActionRecord{}, false, err
	}
	if fingerprint != request.Fingerprint || record.Action != request.Action {
		return ActionRecord{}, false, ErrActionConflict
	}
	return s.healInterruptedAction(ctx, record, updatedAt), false, nil
}

func (s *Store) FinishAction(ctx context.Context, requestID, fingerprint, status, upid string) (ActionRecord, error) {
	if err := validateText("request id", requestID, 128, true); err != nil {
		return ActionRecord{}, err
	}
	if len(fingerprint) != 64 {
		return ActionRecord{}, fmt.Errorf("%w: action fingerprint must be a SHA-256 hex digest", ErrValidation)
	}
	if _, err := hex.DecodeString(fingerprint); err != nil {
		return ActionRecord{}, fmt.Errorf("%w: invalid action fingerprint", ErrValidation)
	}
	if status != ActionAccepted && status != ActionCompleted && status != ActionFailed && status != ActionUnknown {
		return ActionRecord{}, fmt.Errorf("%w: invalid final action status", ErrValidation)
	}
	if err := validateText("action upid", upid, 512, false); err != nil {
		return ActionRecord{}, err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE infra_action_requests SET status=?, upid=?, updated_at=?
		WHERE request_id=? AND fingerprint=? AND status='running'`, status, upid, formatTime(time.Now().UTC()), requestID, fingerprint)
	if err != nil {
		return ActionRecord{}, fmt.Errorf("finish infra action: %w", err)
	}
	changed, err := res.RowsAffected()
	if err != nil {
		return ActionRecord{}, fmt.Errorf("count finished infra actions: %w", err)
	}
	record, storedFingerprint, _, err := loadAction(ctx, s.db, requestID)
	if err != nil {
		return ActionRecord{}, err
	}
	if storedFingerprint != fingerprint {
		return ActionRecord{}, ErrActionConflict
	}
	if changed == 0 && record.Status != status {
		return ActionRecord{}, ErrActionConflict
	}
	return record, nil
}

func (s *Store) GetAction(ctx context.Context, requestID string) (ActionRecord, error) {
	if err := validateText("request id", requestID, 128, true); err != nil {
		return ActionRecord{}, err
	}
	record, _, updatedAt, err := loadAction(ctx, s.db, requestID)
	if err != nil {
		return ActionRecord{}, err
	}
	return s.healInterruptedAction(ctx, record, updatedAt), nil
}

// healInterruptedAction resolves a reservation that can no longer be running: a
// dispatch that failed to record its outcome does not stay replayable as running
// for the rest of the process lifetime.
func (s *Store) healInterruptedAction(ctx context.Context, record ActionRecord, updatedAt time.Time) ActionRecord {
	if record.Status != ActionRunning || time.Since(updatedAt) < actionRunningTimeout {
		return record
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE infra_action_requests SET status='unknown', updated_at=?
		WHERE request_id=? AND status='running'`, formatTime(time.Now().UTC()), record.RequestID); err != nil {
		return record
	}
	record.Status = ActionUnknown
	return record
}

func loadAction(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, requestID string) (ActionRecord, string, time.Time, error) {
	var record ActionRecord
	var fingerprint, updatedAt string
	if err := q.QueryRowContext(ctx, `SELECT request_id,fingerprint,action,status,upid,updated_at FROM infra_action_requests WHERE request_id=?`, requestID).
		Scan(&record.RequestID, &fingerprint, &record.Action, &record.Status, &record.UPID, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ActionRecord{}, "", time.Time{}, ErrNotFound
		}
		return ActionRecord{}, "", time.Time{}, fmt.Errorf("read infra action result: %w", err)
	}
	parsed, err := time.Parse(timeLayout, updatedAt)
	if err != nil {
		return ActionRecord{}, "", time.Time{}, fmt.Errorf("parse infra action timestamp: %w", err)
	}
	return record, fingerprint, parsed, nil
}
