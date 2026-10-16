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
	record, fingerprint, err := loadAction(ctx, s.db, request.RequestID)
	if err != nil {
		return ActionRecord{}, false, err
	}
	if fingerprint != request.Fingerprint || record.Action != request.Action {
		return ActionRecord{}, false, ErrActionConflict
	}
	return record, false, nil
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
	record, storedFingerprint, err := loadAction(ctx, s.db, requestID)
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
	record, _, err := loadAction(ctx, s.db, requestID)
	return record, err
}

func loadAction(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, requestID string) (ActionRecord, string, error) {
	var record ActionRecord
	var fingerprint string
	if err := q.QueryRowContext(ctx, `SELECT request_id,fingerprint,action,status,upid FROM infra_action_requests WHERE request_id=?`, requestID).
		Scan(&record.RequestID, &fingerprint, &record.Action, &record.Status, &record.UPID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ActionRecord{}, "", ErrNotFound
		}
		return ActionRecord{}, "", fmt.Errorf("read infra action result: %w", err)
	}
	return record, fingerprint, nil
}
