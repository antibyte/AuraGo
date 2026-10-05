package flows

import (
	"context"
	"errors"
	"fmt"
)

// ErrMissionAmbiguous is returned (wrapped, with the mission id) by
// GetFlowByMission when more than one flow is linked to the mission. The index
// on flows.mission_id is not unique and SetMissionID does not check for another
// owner, so the store can hold such a state; the lookup refuses to pick one of
// the flows, because Mission Control would then run the wrong one.
var ErrMissionAmbiguous = errors.New("more than one flow is linked to this mission")

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
	// LIMIT 2 is enough to tell one owner from several without reading them all.
	rows, err := s.db.QueryContext(ctx, `SELECT `+flowColumns+` FROM flows WHERE mission_id = ? LIMIT 2`, missionID)
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
