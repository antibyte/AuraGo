package planner

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/dbutil"
)

func TestAppointmentV11MigrationBacksUpAndRehearsesOnCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "planner.db")
	legacy, err := dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := createPlannerTables(legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`PRAGMA user_version=10;
 INSERT INTO appointments(id,title,date_time,notification_at,created_at,updated_at) VALUES('legacy','legacy','2026-10-19T02:00:00+02:00','2026-10-18T23:59:59.9Z','old','old')`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	apt, err := GetAppointment(db, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if apt.DateTime != "2026-10-19T00:00:00Z" || apt.NotificationAt != "2026-10-18T23:59:59.9Z" {
		t.Fatalf("migration changed instants: %+v", apt)
	}
	backups, err := filepath.Glob(path + ".pre-v11-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v err=%v", backups, err)
	}
	backup, err := dbutil.Open(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	var value string
	if err := backup.QueryRow(`SELECT date_time FROM appointments WHERE id='legacy'`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "2026-10-19T02:00:00+02:00" {
		t.Fatal("backup was taken after migration")
	}
	backup.Close()
	data, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(t.TempDir(), "stage.db")
	if err := os.WriteFile(stage, data, 0600); err != nil {
		t.Fatal(err)
	}
	staged, err := InitDB(stage)
	if err != nil {
		t.Fatal(err)
	}
	defer staged.Close()
	var integrity string
	if err := staged.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("staging integrity=%s error=%v", integrity, err)
	}
	stagedApt, err := GetAppointment(staged, "legacy")
	if err != nil || stagedApt.DateTime != apt.DateTime {
		t.Fatalf("staging result differs: %+v %v", stagedApt, err)
	}
}
