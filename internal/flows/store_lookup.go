package flows

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrMissionAmbiguous is returned (wrapped, with the mission id) by
// GetFlowByMission when more than one flow is linked to the mission. The index
// on flows.mission_id is not unique and SetMissionID does not check for another
// owner, so the store can hold such a state; the lookup refuses to pick one of
// the flows, because Mission Control would then run the wrong one.
var ErrMissionAmbiguous = errors.New("more than one flow is linked to this mission")

// flowByMissionSQL reads at most two owners of a mission: enough to tell one from
// several without loading them all. It is a constant so a test can check its
// query plan (it must use idx_flows_mission).
const flowByMissionSQL = `SELECT ` + flowColumns + ` FROM flows WHERE mission_id = ? LIMIT 2`

// GetFlowByMission returns the flow that owns a Mission Control mission.
//
// It returns ErrNotFound when no flow holds the mission (an empty id is "not
// linked", never a mission) and ErrMissionAmbiguous, without returning either
// flow, when several do. A row whose document cannot be read fails with the
// error GetFlow gives for it. A cancelled or expired ctx fails with the
// context's error, not with a verdict about the mission.
func (s *Store) GetFlowByMission(ctx context.Context, missionID string) (*FlowRecord, error) {
	if missionID == "" {
		return nil, ErrNotFound
	}
	rows, err := s.db.QueryContext(ctx, flowByMissionSQL, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	r, scanErr := scanFlow(rows)
	// Look for a second owner before judging the first row, so that ambiguity
	// depends on the rows and not on what their documents contain.
	if rows.Next() {
		return nil, fmt.Errorf("%w: %s", ErrMissionAmbiguous, quoteForError(missionID))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if scanErr != nil {
		return nil, scanErr
	}
	return r, nil
}

// flowIDByMissionSQL reads at most two ids of a mission's flows, like flowByMissionSQL but
// without the documents. It is a constant so a test can check its query plan (it must use
// idx_flows_mission).
const flowIDByMissionSQL = `SELECT id FROM flows WHERE mission_id = ? LIMIT 2`

// flowIDByMission returns the id of the flow that owns a mission without reading its
// documents, so it works even when a document cannot be parsed. Like GetFlowByMission it
// returns ErrNotFound when no flow holds the mission (an empty id included) and
// ErrMissionAmbiguous (wrapped, with the bounded mission id) when several do.
func (s *Store) flowIDByMission(ctx context.Context, missionID string) (string, error) {
	if missionID == "" {
		return "", ErrNotFound
	}
	rows, err := s.db.QueryContext(ctx, flowIDByMissionSQL, missionID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	switch len(ids) {
	case 0:
		return "", ErrNotFound
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("%w: %s", ErrMissionAmbiguous, quoteForError(missionID))
	}
}

// flowMissionRef is a flow id with the mission that represents it.
type flowMissionRef struct {
	id, missionID string
}

// flowMissionRefs lists every flow with its mission id, ordered by flow id, without
// reading the documents (ReconcileMissions).
func (s *Store) flowMissionRefs(ctx context.Context) ([]flowMissionRef, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, mission_id FROM flows ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []flowMissionRef
	for rows.Next() {
		var ref flowMissionRef
		if err := rows.Scan(&ref.id, &ref.missionID); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// flowTimerSlots returns the stored timers of a flow as node id → repeat, without their
// fire times (ReconcileMissions).
func (s *Store) flowTimerSlots(ctx context.Context, flowID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT node_id, repeat FROM flow_timers WHERE flow_id = ?`, flowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var node, repeat string
		if err := rows.Scan(&node, &repeat); err != nil {
			return nil, err
		}
		out[node] = repeat
	}
	return out, rows.Err()
}

// flowMissionAndName reads only the mission id and the name of a flow, without
// parsing its documents: the run hooks need nothing more, and they run inside
// DeleteFlow and Shutdown once per run. It returns ErrNotFound when the flow does not
// exist.
func (s *Store) flowMissionAndName(ctx context.Context, id string) (missionID, name string, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT mission_id, name FROM flows WHERE id = ?`, id).Scan(&missionID, &name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return missionID, name, err
}
