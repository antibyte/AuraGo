package flows

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// flowRunBindingSQL reads what NextRun needs of a flow row without decoding its documents.
const flowRunBindingSQL = `SELECT mission_id, kind, live_json <> '' FROM flows WHERE id = ?`

// flowRunBinding returns the mission of a flow and whether it has a live version. A flow
// that does not exist is ErrNotFound, a block ErrWrongFlowKind.
func (s *Store) flowRunBinding(ctx context.Context, flowID string) (missionID string, live bool, err error) {
	var kind string
	err = s.db.QueryRowContext(ctx, flowRunBindingSQL, flowID).Scan(&missionID, &kind, &live)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, ErrNotFound
	}
	if err != nil {
		return "", false, err
	}
	if kind != string(KindFlow) {
		return "", false, ErrWrongFlowKind
	}
	return missionID, live, nil
}

// nextTimerAfterSQL reads the earliest timer of one flow that fires after a given time.
// Stored times share one layout (timeLayout, UTC), so the text comparison is the time
// comparison; storedFireAtGlob keeps out rows that are not in that layout. flow_id is the
// first column of the table's primary key, so only that flow's rows are read.
const nextTimerAfterSQL = `SELECT node_id, fire_at FROM flow_timers WHERE flow_id = ? AND fire_at > ? AND fire_at GLOB ` +
	storedFireAtGlob + ` ORDER BY fire_at, node_id LIMIT 1`

// NextTimerAfter returns the earliest timer of the flow that fires after the given time; ok
// is false when there is none. A timer whose time has passed (in retry) is left out, unlike
// NextTimerAt, which returns the raw minimum.
func (s *Store) NextTimerAfter(ctx context.Context, flowID string, after time.Time) (TimerRecord, bool, error) {
	var node, fireAt string
	err := s.db.QueryRowContext(ctx, nextTimerAfterSQL, flowID, formatTime(after)).Scan(&node, &fireAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TimerRecord{}, false, nil
	}
	if err != nil {
		return TimerRecord{}, false, err
	}
	at := parseTime(fireAt)
	if at.IsZero() {
		return TimerRecord{}, false, nil
	}
	return TimerRecord{FlowID: flowID, NodeID: node, FireAt: at}, true, nil
}
