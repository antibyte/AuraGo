package tools

import (
	"aurago/internal/dbutil"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const preparedMissionsSchemaVersion = 1

// InitPreparedMissionsDB initializes the prepared missions SQLite database.
func InitPreparedMissionsDB(dbPath string) (*sql.DB, error) {
	db, err := dbutil.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open prepared missions database: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS prepared_missions (
		id                  TEXT PRIMARY KEY,
		mission_id          TEXT NOT NULL UNIQUE,
		version             INTEGER NOT NULL DEFAULT 1,
		status              TEXT NOT NULL DEFAULT 'none',
		prepared_at         DATETIME,
		source_checksum     TEXT NOT NULL DEFAULT '',
		prepared_data       TEXT NOT NULL DEFAULT '{}',
		confidence          REAL NOT NULL DEFAULT 0.0,
		token_cost          INTEGER NOT NULL DEFAULT 0,
		preparation_time_ms INTEGER NOT NULL DEFAULT 0,
		error_message       TEXT NOT NULL DEFAULT '',
		created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_prepared_missions_mission_id ON prepared_missions(mission_id);
	CREATE INDEX IF NOT EXISTS idx_prepared_missions_status ON prepared_missions(status);`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create prepared_missions schema: %w", err)
	}

	db.Exec(fmt.Sprintf("PRAGMA user_version = %d", preparedMissionsSchemaVersion))
	return db, nil
}

// SavePreparedMission inserts or replaces a prepared mission record.
func SavePreparedMission(db *sql.DB, pm *PreparedMission) error {
	if db == nil || pm == nil {
		return fmt.Errorf("db and prepared mission are required")
	}

	analysisJSON, err := json.Marshal(pm.Analysis)
	if err != nil {
		return fmt.Errorf("failed to marshal analysis: %w", err)
	}

	_, err = db.Exec(`
		INSERT INTO prepared_missions (id, mission_id, version, status, prepared_at, source_checksum, prepared_data, confidence, token_cost, preparation_time_ms, error_message, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(mission_id) DO UPDATE SET
			version = excluded.version,
			status = excluded.status,
			prepared_at = excluded.prepared_at,
			source_checksum = excluded.source_checksum,
			prepared_data = excluded.prepared_data,
			confidence = excluded.confidence,
			token_cost = excluded.token_cost,
			preparation_time_ms = excluded.preparation_time_ms,
			error_message = excluded.error_message,
			updated_at = CURRENT_TIMESTAMP`,
		pm.ID, pm.MissionID, pm.Version, string(pm.Status), pm.PreparedAt,
		pm.SourceChecksum, string(analysisJSON), pm.Confidence,
		pm.TokenCost, pm.PreparationTimeMS, pm.ErrorMessage)
	if err != nil {
		return fmt.Errorf("failed to save prepared mission: %w", err)
	}
	return nil
}

// GetPreparedMission retrieves the prepared mission for a given mission ID.
func GetPreparedMission(db *sql.DB, missionID string) (*PreparedMission, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}

	row := db.QueryRow(`
		SELECT id, mission_id, version, status, prepared_at, source_checksum, prepared_data,
		       confidence, token_cost, preparation_time_ms, error_message
		FROM prepared_missions WHERE mission_id = ?`, missionID)

	var pm PreparedMission
	var status string
	var preparedAt sql.NullTime
	var analysisJSON string

	err := row.Scan(&pm.ID, &pm.MissionID, &pm.Version, &status, &preparedAt,
		&pm.SourceChecksum, &analysisJSON, &pm.Confidence,
		&pm.TokenCost, &pm.PreparationTimeMS, &pm.ErrorMessage)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get prepared mission: %w", err)
	}

	pm.Status = PreparationStatus(status)
	if preparedAt.Valid {
		pm.PreparedAt = preparedAt.Time
	}

	if analysisJSON != "" && analysisJSON != "{}" {
		var analysis PreparationAnalysis
		if err := json.Unmarshal([]byte(analysisJSON), &analysis); err == nil {
			pm.Analysis = &analysis
		}
	}

	return &pm, nil
}

// DeletePreparedMission removes a prepared mission by mission ID.
func DeletePreparedMission(db *sql.DB, missionID string) error {
	if db == nil {
		return fmt.Errorf("db is required")
	}
	_, err := db.Exec("DELETE FROM prepared_missions WHERE mission_id = ?", missionID)
	if err != nil {
		return fmt.Errorf("failed to delete prepared mission: %w", err)
	}
	return nil
}

// InvalidatePreparedMission marks a prepared mission as stale.
func InvalidatePreparedMission(db *sql.DB, missionID string) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(`
		UPDATE prepared_missions SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE mission_id = ? AND status = ?`,
		string(PrepStatusStale), missionID, string(PrepStatusPrepared))
	if err != nil {
		return fmt.Errorf("failed to invalidate prepared mission: %w", err)
	}
	return nil
}

// InvalidatePreparedMissionsByCheatsheet marks all prepared missions as stale
// that reference the given cheatsheet ID.
func InvalidatePreparedMissionsByCheatsheet(db *sql.DB, missionMgr *MissionManagerV2, cheatsheetID string) error {
	if db == nil || missionMgr == nil {
		return nil
	}

	missionMgr.mu.RLock()
	var affectedIDs []string
	for _, m := range missionMgr.missions {
		for _, csID := range m.CheatsheetIDs {
			if csID == cheatsheetID {
				affectedIDs = append(affectedIDs, m.ID)
				break
			}
		}
	}
	missionMgr.mu.RUnlock()

	for _, mid := range affectedIDs {
		if err := InvalidatePreparedMission(db, mid); err != nil {
			return err
		}
	}
	return nil
}

// ListPreparedMissions returns all prepared missions with their status.
func ListPreparedMissions(db *sql.DB) ([]*PreparedMission, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}

	rows, err := db.Query(`
		SELECT id, mission_id, version, status, prepared_at, source_checksum,
		       confidence, token_cost, preparation_time_ms, error_message
		FROM prepared_missions ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list prepared missions: %w", err)
	}
	defer rows.Close()

	var result []*PreparedMission
	for rows.Next() {
		var pm PreparedMission
		var status string
		var preparedAt sql.NullTime

		if err := rows.Scan(&pm.ID, &pm.MissionID, &pm.Version, &status, &preparedAt,
			&pm.SourceChecksum, &pm.Confidence, &pm.TokenCost,
			&pm.PreparationTimeMS, &pm.ErrorMessage); err != nil {
			return nil, fmt.Errorf("failed to scan prepared mission: %w", err)
		}

		pm.Status = PreparationStatus(status)
		if preparedAt.Valid {
			pm.PreparedAt = preparedAt.Time
		}
		result = append(result, &pm)
	}

	return result, rows.Err()
}

// CleanupExpiredPreparations removes preparations older than the given duration.
func CleanupExpiredPreparations(db *sql.DB, maxAge time.Duration) (int64, error) {
	if db == nil {
		return 0, nil
	}
	cutoff := time.Now().Add(-maxAge)
	res, err := db.Exec(`
		DELETE FROM prepared_missions
		WHERE status = ? AND prepared_at < ?`,
		string(PrepStatusPrepared), cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired preparations: %w", err)
	}
	return res.RowsAffected()
}
