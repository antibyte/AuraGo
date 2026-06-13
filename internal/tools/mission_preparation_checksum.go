package tools

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
)

const missionPreparationChecksumVersion = "mission-preparation-v2"

// MissionPreparationSourceChecksum returns a stable digest for the preparation
// contract and canonical mission inputs used to generate prepared context.
func MissionPreparationSourceChecksum(mission *MissionV2, cheatsheetDB *sql.DB) string {
	h := sha256.New()
	h.Write([]byte(missionPreparationChecksumVersion))
	if mission == nil {
		return fmt.Sprintf("%x", h.Sum(nil))
	}

	h.Write([]byte(StripMissionExecutionPlanAdvisory(mission.Prompt)))
	for _, id := range mission.CheatsheetIDs {
		if cheatsheetDB != nil {
			if cs, err := CheatsheetGet(cheatsheetDB, id); err == nil && cs != nil {
				h.Write([]byte(cs.ID))
				h.Write([]byte(cs.Name))
				h.Write([]byte(cs.Content))
				for _, a := range cs.Attachments {
					h.Write([]byte(a.Filename))
					h.Write([]byte(a.Content))
				}
				continue
			}
		}
		h.Write([]byte(id))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
