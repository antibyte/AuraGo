package flows

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Test data kinds.
const (
	TestDataTriggerSample = "trigger_sample"
	TestDataPinned        = "pinned"
)

// RepeatYearly makes a Date/Time timer fire every year.
const RepeatYearly = "yearly"

// TimerRecord is one armed Date/Time trigger.
type TimerRecord struct {
	FlowID string    `json:"flow_id"`
	NodeID string    `json:"node_id"`
	FireAt time.Time `json:"fire_at"`
	Repeat string    `json:"repeat,omitempty"`
}

// PutTestData stores sample or pinned data for a node. It returns ErrNotFound when
// the flow does not exist. The store does not limit the size of data; the caller
// (the API) does.
func (s *Store) PutTestData(ctx context.Context, flowID, nodeID, kind string, data map[string]any, now time.Time) error {
	encoded, err := marshalMap(data)
	if err != nil {
		return err
	}
	// The WHERE clause is required by SQLite to tell the upsert's ON CONFLICT from the
	// SELECT, and it is what turns a missing flow into "no row" rather than a foreign-key error.
	res, err := s.db.ExecContext(ctx, `INSERT INTO flow_test_data (flow_id, node_id, kind, json, updated_at)
		SELECT ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM flows WHERE id = ?)
		ON CONFLICT(flow_id, node_id, kind) DO UPDATE SET json = excluded.json, updated_at = excluded.updated_at`,
		flowID, nodeID, kind, encoded, formatTime(now), flowID)
	if err != nil {
		return err
	}
	return requireRow(res, ErrNotFound)
}

// GetTestData returns stored data; ok is false when none exists.
func (s *Store) GetTestData(ctx context.Context, flowID, nodeID, kind string) (map[string]any, bool, error) {
	var encoded string
	err := s.db.QueryRowContext(ctx, `SELECT json FROM flow_test_data WHERE flow_id = ? AND node_id = ? AND kind = ?`,
		flowID, nodeID, kind).Scan(&encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(encoded), &data); err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// checkFireAt rejects a timer without a fire time: it would be stored as an empty
// string, sort before every real time and fire at once.
func checkFireAt(t TimerRecord) error {
	if t.FireAt.IsZero() {
		return fmt.Errorf("timer of node %s needs a fire_at time", quoteForError(t.NodeID))
	}
	return nil
}

// ReplaceTimers sets the timers of one flow (none when timers is empty). It stores
// them under flowID and ignores TimerRecord.FlowID. It rejects a timer without a
// FireAt before changing anything, and returns ErrNotFound (leaving the old timers)
// when the flow does not exist. Clearing the timers of a missing flow succeeds:
// there is nothing left to clear.
func (s *Store) ReplaceTimers(ctx context.Context, flowID string, timers []TimerRecord) error {
	for _, t := range timers {
		if err := checkFireAt(t); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// The DELETE comes first: the transaction takes the write lock right away, so the
	// busy handler can wait for it (a read-then-write transaction cannot).
	if _, err := tx.ExecContext(ctx, `DELETE FROM flow_timers WHERE flow_id = ?`, flowID); err != nil {
		return err
	}
	for _, t := range timers {
		res, err := tx.ExecContext(ctx, `INSERT INTO flow_timers (flow_id, node_id, fire_at, repeat)
			SELECT ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM flows WHERE id = ?)`,
			flowID, t.NodeID, formatTime(t.FireAt), t.Repeat, flowID)
		if err != nil {
			return err
		}
		if err := requireRow(res, ErrNotFound); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpsertTimer stores one timer. It rejects a zero FireAt and returns ErrNotFound
// when the flow does not exist.
func (s *Store) UpsertTimer(ctx context.Context, t TimerRecord) error {
	if err := checkFireAt(t); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO flow_timers (flow_id, node_id, fire_at, repeat)
		SELECT ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM flows WHERE id = ?)
		ON CONFLICT(flow_id, node_id) DO UPDATE SET fire_at = excluded.fire_at, repeat = excluded.repeat`,
		t.FlowID, t.NodeID, formatTime(t.FireAt), t.Repeat, t.FlowID)
	if err != nil {
		return err
	}
	return requireRow(res, ErrNotFound)
}

// DeleteTimer removes one timer.
func (s *Store) DeleteTimer(ctx context.Context, flowID, nodeID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM flow_timers WHERE flow_id = ? AND node_id = ?`, flowID, nodeID)
	return err
}

// DeleteTimerAt removes the timer only while it is still armed for fireAt. A timer
// that was re-armed for another time, or that is gone, is left alone and that is not an
// error: the caller settles one fired occurrence and must not wipe what a concurrent
// ReplaceTimers armed since. fireAt is compared at the store's microsecond precision.
func (s *Store) DeleteTimerAt(ctx context.Context, flowID, nodeID string, fireAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM flow_timers WHERE flow_id = ? AND node_id = ? AND fire_at = ?`,
		flowID, nodeID, formatTime(fireAt))
	return err
}

// MoveTimer sets the fire time of a timer from one time to another, only while the
// timer is still armed for from. Like DeleteTimerAt it does nothing, without an error,
// when the timer was re-armed, deleted or its flow is gone. It rejects a zero to.
func (s *Store) MoveTimer(ctx context.Context, flowID, nodeID string, from, to time.Time) error {
	if err := checkFireAt(TimerRecord{NodeID: nodeID, FireAt: to}); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE flow_timers SET fire_at = ? WHERE flow_id = ? AND node_id = ? AND fire_at = ?`,
		formatTime(to), flowID, nodeID, formatTime(from))
	return err
}

// nextTimerSQL reads the earliest fire time of one flow's timers. flow_id is the first
// column of the table's primary key, so it reads only that flow's rows; it is a constant
// so a test can check its query plan. The GLOB keeps only times in the stored layout
// (timeLayout): a row whose fire_at cannot be read would otherwise sort before or after
// the real times as text, and the timer service drops such rows without firing them.
const nextTimerSQL = `SELECT MIN(fire_at) FROM flow_timers WHERE flow_id = ? AND fire_at GLOB ` +
	`'[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z'`

// NextTimerAt returns the earliest fire time among the timers of one flow; ok is false
// when the flow has none (also when it does not exist). Rows whose fire_at is not in the
// stored layout are left out; an earliest value in the layout that is no valid time
// (only a damaged row can hold one) gives ok false.
func (s *Store) NextTimerAt(ctx context.Context, flowID string) (next time.Time, ok bool, err error) {
	var fireAt sql.NullString
	if err := s.db.QueryRowContext(ctx, nextTimerSQL, flowID).Scan(&fireAt); err != nil {
		return time.Time{}, false, err
	}
	if !fireAt.Valid {
		return time.Time{}, false, nil
	}
	next = parseTime(fireAt.String)
	return next, !next.IsZero(), nil
}

// ListTimers returns all timers ordered by fire time.
func (s *Store) ListTimers(ctx context.Context) ([]TimerRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT flow_id, node_id, fire_at, repeat FROM flow_timers ORDER BY fire_at, flow_id, node_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TimerRecord
	for rows.Next() {
		var t TimerRecord
		var fireAt string
		if err := rows.Scan(&t.FlowID, &t.NodeID, &fireAt, &t.Repeat); err != nil {
			return nil, err
		}
		t.FireAt = parseTime(fireAt)
		out = append(out, t)
	}
	return out, rows.Err()
}
