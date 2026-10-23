package detective

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// backupBeforeMigration copies the database with VACUUM INTO before a schema step and
// syncs the copy. One backup is taken per open, named after the target version.
func backupBeforeMigration(db *sql.DB, path string, version, target int) error {
	if version >= target {
		return nil
	}
	var tables int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table'").Scan(&tables); err != nil {
		return err
	}
	if tables == 0 {
		return nil
	}
	backup := fmt.Sprintf("%s.pre-v%d-%s.bak", path, target, time.Now().UTC().Format("20060102T150405.000000000Z"))
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
			if !r.Partial && !r.Provisional {
				c.LatestCompleteRevision = max(c.LatestCompleteRevision, r.Revision)
			}
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

// migrateSummary creates the version-4 summary table and backfills every case that has
// no row yet, after moving the large body column to the end of the case row.
func migrateSummary(tx *sql.Tx, version int) error {
	if version >= SchemaVersion {
		return nil
	}
	if err := rebuildCaseTable(tx); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT c.id, c.body, c.active_ms FROM detective_cases c
		LEFT JOIN detective_summaries s ON s.case_id=c.id WHERE s.case_id IS NULL`)
	if err != nil {
		return err
	}
	type entry struct {
		key    string
		body   []byte
		active int64
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err = rows.Scan(&e.key, &e.body, &e.active); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, e := range entries {
		var c Case
		if err = json.Unmarshal(e.body, &c); err != nil {
			return fmt.Errorf("migrate Detective summary: %w", err)
		}
		if e.active > c.Run.Usage.ActiveMS {
			c.Run.Usage.ActiveMS = e.active
		}
		// The working set keeps source metadata only; the archive row keeps the excerpt.
		rewritten := false
		for i := range c.Sources {
			if c.Sources[i].Excerpt != "" {
				c.Sources[i].Excerpt = ""
				rewritten = true
			}
		}
		if rewritten {
			body, err := json.Marshal(&c)
			if err != nil {
				return err
			}
			if _, err = tx.Exec("UPDATE detective_cases SET body=? WHERE id=?", body, e.key); err != nil {
				return err
			}
		}
		summary, err := json.Marshal(caseSummary(&c))
		if err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO detective_summaries(case_id, body) VALUES(?,?)", e.key, string(summary)); err != nil {
			return err
		}
	}
	_, err = tx.Exec(fmt.Sprintf("PRAGMA user_version=%d", SchemaVersion))
	return err
}

// rebuildCaseTable moves the evidence body to the end of the case row. SQLite stores
// columns in declaration order and a value larger than the local page payload spills
// into an overflow chain; every column after it then costs a walk of that chain, which
// made `SELECT status, updated, active_ms` take milliseconds for a large case.
func rebuildCaseTable(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT name FROM pragma_table_info('detective_cases') ORDER BY cid`)
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		names = append(names, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(names) == 0 || names[len(names)-1] == "body" {
		return nil
	}
	if _, err = tx.Exec(`CREATE TABLE detective_cases_v4(id TEXT PRIMARY KEY, status TEXT NOT NULL, updated TEXT NOT NULL, active_ms INTEGER NOT NULL DEFAULT 0, body BLOB NOT NULL)`); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO detective_cases_v4(id,status,updated,active_ms,body)
		SELECT id,status,updated,active_ms,body FROM detective_cases`); err != nil {
		return err
	}
	if _, err = tx.Exec(`DROP TABLE detective_cases`); err != nil {
		return err
	}
	_, err = tx.Exec(`ALTER TABLE detective_cases_v4 RENAME TO detective_cases`)
	return err
}
