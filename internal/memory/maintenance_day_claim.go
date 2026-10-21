package memory

import (
	"database/sql"
	"fmt"
	"time"
)

// MaintenanceAutomaticDayKey is the memory-maintenance state key that records
// the local calendar day claimed by the automatic scheduler. It is deliberately
// kept in memory_maintenance_meta so the claim survives process restarts.
const MaintenanceAutomaticDayKey = "maintenance.automatic_day"
const maintenanceAutomaticDayOwnerKey = "maintenance.automatic_day_owner"

func maintenanceDayClaimOwner(startedAt time.Time) string {
	return startedAt.UTC().Format(time.RFC3339Nano)
}

func persistMaintenanceDayClaim(tx *sql.Tx, day, owner string) error {
	if _, err := tx.Exec(`
		INSERT INTO memory_maintenance_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, MaintenanceAutomaticDayKey, day); err != nil {
		return fmt.Errorf("persist maintenance day: %w", err)
	}
	if owner == "" {
		if _, err := tx.Exec(`DELETE FROM memory_maintenance_meta WHERE key=?`, maintenanceAutomaticDayOwnerKey); err != nil {
			return fmt.Errorf("clear maintenance day owner: %w", err)
		}
		return nil
	}
	if _, err := tx.Exec(`
		INSERT INTO memory_maintenance_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, maintenanceAutomaticDayOwnerKey, owner); err != nil {
		return fmt.Errorf("persist maintenance day owner: %w", err)
	}
	return nil
}

// IsMaintenanceDayClaimed reports whether the local calendar day already has
// a durable claim or a completed run. It does not create or change state and
// is used to keep the scheduler's advertised next_run aligned with the claim.
func (s *SQLiteMemory) IsMaintenanceDayClaimed(startedAt time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("memory maintenance day claim store is unavailable")
	}
	if startedAt.IsZero() {
		return false, fmt.Errorf("maintenance day claim start time is required")
	}
	if err := s.InitMaintenanceRunsTable(); err != nil {
		return false, fmt.Errorf("initialize maintenance day claim ledger: %w", err)
	}
	day := startedAt.Format("2006-01-02")
	var stored string
	err := s.db.QueryRow(`SELECT value FROM memory_maintenance_meta WHERE key = ?`, MaintenanceAutomaticDayKey).Scan(&stored)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("read maintenance day claim: %w", err)
	}
	if stored == day {
		return true, nil
	}
	var latestStartedAt string
	err = s.db.QueryRow(`
		SELECT started_at
		FROM maintenance_runs
		ORDER BY finished_at DESC, id DESC
		LIMIT 1`).Scan(&latestStartedAt)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read latest maintenance day: %w", err)
	}
	latest, err := time.Parse(time.RFC3339, latestStartedAt)
	if err != nil {
		return false, fmt.Errorf("parse latest maintenance start %q: %w", latestStartedAt, err)
	}
	return latest.In(startedAt.Location()).Format("2006-01-02") == day, nil
}

// ClaimMaintenanceDay atomically claims the local calendar day for an
// automatic maintenance run. It returns false when that day has already been
// claimed or when a completed run for the same day predates the state key.
// The latter check seeds installations upgraded before the state key existed.
//
// The caller must use the same local time zone as the scheduler. A failed
// transaction is returned as an error so callers can fail closed rather than
// starting a run whose day claim was not durable.
func (s *SQLiteMemory) ClaimMaintenanceDay(startedAt time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("memory maintenance day claim store is unavailable")
	}
	if startedAt.IsZero() {
		return false, fmt.Errorf("maintenance day claim start time is required")
	}
	if err := s.InitMaintenanceRunsTable(); err != nil {
		return false, fmt.Errorf("initialize maintenance day claim ledger: %w", err)
	}
	day := startedAt.Format("2006-01-02")
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin maintenance day claim: %w", err)
	}
	rollback := func() {
		_ = tx.Rollback()
	}
	defer rollback()

	var stored string
	err = tx.QueryRow(`SELECT value FROM memory_maintenance_meta WHERE key = ?`, MaintenanceAutomaticDayKey).Scan(&stored)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("read maintenance day claim: %w", err)
	}

	// A completed run is authoritative when upgrading from a version that did
	// not persist the state key. Check it even when an older state value exists,
	// so an interrupted upgrade cannot permit a same-day duplicate.
	var latestStartedAt string
	err = tx.QueryRow(`
		SELECT started_at
		FROM maintenance_runs
		ORDER BY finished_at DESC, id DESC
		LIMIT 1`).Scan(&latestStartedAt)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("read latest maintenance day: %w", err)
	}
	if err == nil {
		latest, parseErr := time.Parse(time.RFC3339, latestStartedAt)
		if parseErr != nil {
			return false, fmt.Errorf("parse latest maintenance start %q: %w", latestStartedAt, parseErr)
		}
		if latest.In(startedAt.Location()).Format("2006-01-02") == day {
			if err := persistMaintenanceDayClaim(tx, day, ""); err != nil {
				return false, fmt.Errorf("seed maintenance day claim: %w", err)
			}
			if err := tx.Commit(); err != nil {
				return false, fmt.Errorf("commit maintenance day seed: %w", err)
			}
			return false, nil
		}
	}

	if stored == day {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit existing maintenance day claim: %w", err)
		}
		return false, nil
	}
	if err := persistMaintenanceDayClaim(tx, day, maintenanceDayClaimOwner(startedAt)); err != nil {
		return false, fmt.Errorf("persist maintenance day claim: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit maintenance day claim: %w", err)
	}
	return true, nil
}

// ReleaseMaintenanceDayClaim conditionally releases a scheduler claim when a
// run was canceled before its durable maintenance_runs row was written. It
// never releases a day already represented by a persisted run.
func (s *SQLiteMemory) ReleaseMaintenanceDayClaim(startedAt time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("memory maintenance day claim store is unavailable")
	}
	if startedAt.IsZero() {
		return false, fmt.Errorf("maintenance day claim start time is required")
	}
	if err := s.InitMaintenanceRunsTable(); err != nil {
		return false, fmt.Errorf("initialize maintenance day claim ledger: %w", err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin maintenance day claim release: %w", err)
	}
	defer tx.Rollback()
	day := startedAt.Format("2006-01-02")
	var stored string
	err = tx.QueryRow(`SELECT value FROM memory_maintenance_meta WHERE key=?`, MaintenanceAutomaticDayKey).Scan(&stored)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("read maintenance day claim before release: %w", err)
	}
	if stored != day {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit unchanged maintenance day claim: %w", err)
		}
		return false, nil
	}
	var owner string
	err = tx.QueryRow(`SELECT value FROM memory_maintenance_meta WHERE key=?`, maintenanceAutomaticDayOwnerKey).Scan(&owner)
	if err == sql.ErrNoRows {
		// A day-only claim predates owner tokens. Keep it fail-closed because
		// this caller cannot prove that it owns the legacy claim.
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit retained legacy maintenance day claim: %w", err)
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read maintenance day owner before release: %w", err)
	}
	if owner != maintenanceDayClaimOwner(startedAt) {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit unchanged maintenance day owner: %w", err)
		}
		return false, nil
	}
	var completed bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM maintenance_runs WHERE started_at=?)`, startedAt.UTC().Format(time.RFC3339)).Scan(&completed); err != nil {
		return false, fmt.Errorf("check persisted maintenance run before release: %w", err)
	}
	if completed {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit retained maintenance day claim: %w", err)
		}
		return false, nil
	}
	res, err := tx.Exec(`DELETE FROM memory_maintenance_meta WHERE key=? AND value=? AND EXISTS (
		SELECT 1 FROM memory_maintenance_meta WHERE key=? AND value=?
	)`, MaintenanceAutomaticDayKey, day, maintenanceAutomaticDayOwnerKey, owner)
	if err != nil {
		return false, fmt.Errorf("release maintenance day claim: %w", err)
	}
	changed, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("confirm maintenance day claim release: %w", err)
	}
	if changed > 0 {
		if _, err := tx.Exec(`DELETE FROM memory_maintenance_meta WHERE key=? AND value=?`, maintenanceAutomaticDayOwnerKey, owner); err != nil {
			return false, fmt.Errorf("release maintenance day owner: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit maintenance day claim release: %w", err)
	}
	return changed > 0, nil
}
