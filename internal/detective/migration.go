package detective

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func backupBeforeArchive(db *sql.DB, path string, version int) error {
	if version >= 3 {
		return nil
	}
	var tables int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table'").Scan(&tables); err != nil {
		return err
	}
	if tables == 0 {
		return nil
	}
	backup := path + ".pre-v3-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".bak"
	if _, err := db.Exec("VACUUM INTO ?", backup); err != nil {
		return fmt.Errorf("back up Detective before migration: %w", err)
	}
	if err := os.Chmod(backup, 0600); err != nil {
		return fmt.Errorf("protect Detective backup: %w", err)
	}
	f, err := os.OpenFile(backup, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	err = f.Sync()
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func migrateArchive(tx *sql.Tx, version int) error {
	if version >= 3 {
		return nil
	}
	_, err := tx.Exec(`CREATE TABLE detective_sources(seq INTEGER PRIMARY KEY AUTOINCREMENT,case_id TEXT NOT NULL,id TEXT NOT NULL,fingerprint TEXT NOT NULL,body BLOB NOT NULL,UNIQUE(case_id,id));
CREATE INDEX detective_source_lookup ON detective_sources(case_id,fingerprint);
CREATE INDEX detective_source_page ON detective_sources(case_id,seq);
CREATE TABLE detective_findings(seq INTEGER PRIMARY KEY AUTOINCREMENT,case_id TEXT NOT NULL,id TEXT NOT NULL,fingerprint TEXT NOT NULL,body BLOB NOT NULL,published_revision INTEGER NOT NULL DEFAULT 0,UNIQUE(case_id,id));
CREATE INDEX detective_finding_lookup ON detective_findings(case_id,fingerprint);
CREATE INDEX detective_finding_page ON detective_findings(case_id,seq);
CREATE TABLE detective_reports(case_id TEXT NOT NULL,revision INTEGER NOT NULL,body BLOB NOT NULL,PRIMARY KEY(case_id,revision));`)
	if err != nil {
		return fmt.Errorf("create Detective archive: %w", err)
	}
	rows, err := tx.Query("SELECT id FROM detective_cases")
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		var b []byte
		var active int64
		if err = tx.QueryRow("SELECT body,active_ms FROM detective_cases WHERE id=?", id).Scan(&b, &active); err != nil {
			return err
		}
		var c Case
		if err = json.Unmarshal(b, &c); err != nil {
			return fmt.Errorf("migrate Detective case: %w", err)
		}
		c.pendingSources, c.pendingFindings, c.pendingReports = c.Sources, c.Findings, c.Reports
		c.SourceCount, c.FindingCount = len(c.Sources), len(c.Findings)
		if len(c.Sources) > 0 {
			c.LatestSourceID = c.Sources[len(c.Sources)-1].ID
		}
		for _, r := range c.Reports {
			c.LatestRevision = max(c.LatestRevision, r.Revision)
		}
		if len(c.Reports) > 0 {
			c.Reports = c.Reports[len(c.Reports)-1:]
		}
		c.Run.Usage.ActiveMS = max(active, c.Run.Usage.ActiveMS)
		c.AutoUpdate = AutoUpdate{AutoUpdateSettings: defaultAutoUpdate(AutoUpdateSettings{})}
		c.Request.AutoUpdate = c.AutoUpdate.AutoUpdateSettings
		if err = saveCaseTx(tx, &c); err != nil {
			return fmt.Errorf("archive Detective case: %w", err)
		}
	}
	if _, err = tx.Exec("PRAGMA user_version=3"); err != nil {
		return err
	}
	return nil
}
