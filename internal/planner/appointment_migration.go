package planner

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"

	"aurago/internal/fileutil"
)

func normalizeAppointmentInstant(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	instant, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "", fmt.Errorf("invalid appointment instant: %w", err)
	}
	return instant.UTC().Format(time.RFC3339Nano), nil
}

func appointmentEpoch(value string) any {
	if value == "" {
		return nil
	}
	instant, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	return instant.UnixMilli()
}

func backupPlannerBeforeV11(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM appointments`).Scan(&count); err != nil {
		return fmt.Errorf("count planner migration sources: %w", err)
	}
	if count == 0 {
		return nil
	}
	rows, err := db.Query(`PRAGMA database_list`)
	if err != nil {
		return err
	}
	var path string
	for rows.Next() {
		var seq int
		var name, file string
		if err = rows.Scan(&seq, &name, &file); err != nil {
			rows.Close()
			return err
		}
		if name == "main" {
			path = file
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	backup := path + ".pre-v11-" + rand.Text() + ".bak"
	if _, err = db.Exec(`VACUUM main INTO ?`, backup); err != nil {
		return fmt.Errorf("back up planner before v11 migration: %w", err)
	}
	if err = fileutil.RestrictToOwner(backup); err != nil {
		return fmt.Errorf("protect planner backup: %w", err)
	}
	return nil
}

func migratePlannerToV11(db *sql.DB) error {
	columns := []struct{ name, ddl string }{
		{"date_time_epoch", `ALTER TABLE appointments ADD COLUMN date_time_epoch INTEGER`},
		{"notification_epoch", `ALTER TABLE appointments ADD COLUMN notification_epoch INTEGER`},
		{"notification_revision", `ALTER TABLE appointments ADD COLUMN notification_revision INTEGER NOT NULL DEFAULT 1`},
	}
	var missing []string
	for _, column := range columns {
		exists, err := plannerColumnExists(db, "appointments", column.name)
		if err != nil {
			return err
		}
		if !exists {
			missing = append(missing, column.ddl)
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, ddl := range missing {
		if _, err = tx.Exec(ddl); err != nil {
			return fmt.Errorf("add appointment epoch column: %w", err)
		}
	}

	rows, err := tx.Query(`SELECT id,date_time,notification_at FROM appointments`)
	if err != nil {
		return err
	}
	type row struct{ id, date, due string }
	var sources []row
	for rows.Next() {
		var source row
		if err = rows.Scan(&source.id, &source.date, &source.due); err != nil {
			rows.Close()
			return err
		}
		sources = append(sources, source)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, source := range sources {
		date, err := normalizeAppointmentInstant(source.date)
		if err != nil {
			return fmt.Errorf("migrate appointment date: %w", err)
		}
		due, err := normalizeAppointmentInstant(source.due)
		if err != nil {
			return fmt.Errorf("migrate appointment notification: %w", err)
		}
		if _, err = tx.Exec(`UPDATE appointments SET date_time=?,notification_at=?,date_time_epoch=?,notification_epoch=? WHERE id=?`, date, due, appointmentEpoch(date), appointmentEpoch(due), source.id); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_appointment_notification_epoch ON appointments(notification_epoch,notified,status);
 CREATE INDEX IF NOT EXISTS idx_appointment_date_epoch ON appointments(date_time_epoch);
 CREATE TABLE IF NOT EXISTS appointment_notification_outbox (
 event_id TEXT NOT NULL,destination TEXT NOT NULL,appointment_id TEXT NOT NULL,revision INTEGER NOT NULL,
 state TEXT NOT NULL DEFAULT 'pending',updated_at TEXT NOT NULL,last_error TEXT NOT NULL DEFAULT '',
 PRIMARY KEY(event_id,destination));`); err != nil {
		return fmt.Errorf("create appointment outbox: %w", err)
	}
	if _, err = tx.Exec(`PRAGMA user_version=11`); err != nil {
		return err
	}
	return tx.Commit()
}
