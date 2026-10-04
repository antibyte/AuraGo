package flows

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

// PutTestData stores sample or pinned data for a node.
func (s *Store) PutTestData(ctx context.Context, flowID, nodeID, kind string, data map[string]any, now time.Time) error {
	encoded, err := marshalMap(data)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO flow_test_data (flow_id, node_id, kind, json, updated_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(flow_id, node_id, kind) DO UPDATE SET json = excluded.json, updated_at = excluded.updated_at`,
		flowID, nodeID, kind, encoded, formatTime(now))
	return err
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

// ReplaceTimers sets the timers of one flow (none when timers is empty).
func (s *Store) ReplaceTimers(ctx context.Context, flowID string, timers []TimerRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM flow_timers WHERE flow_id = ?`, flowID); err != nil {
		return err
	}
	for _, t := range timers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO flow_timers (flow_id, node_id, fire_at, repeat) VALUES (?, ?, ?, ?)`,
			flowID, t.NodeID, formatTime(t.FireAt), t.Repeat); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpsertTimer stores one timer.
func (s *Store) UpsertTimer(ctx context.Context, t TimerRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO flow_timers (flow_id, node_id, fire_at, repeat) VALUES (?, ?, ?, ?)
		ON CONFLICT(flow_id, node_id) DO UPDATE SET fire_at = excluded.fire_at, repeat = excluded.repeat`,
		t.FlowID, t.NodeID, formatTime(t.FireAt), t.Repeat)
	return err
}

// DeleteTimer removes one timer.
func (s *Store) DeleteTimer(ctx context.Context, flowID, nodeID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM flow_timers WHERE flow_id = ? AND node_id = ?`, flowID, nodeID)
	return err
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
