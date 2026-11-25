package homedash

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"aurago/internal/dbutil"

	_ "modernc.org/sqlite" // registers the "sqlite" driver used by dbutil.Open
)

const storeSchemaVersion = 1

const storeSchemaSQL = `
CREATE TABLE IF NOT EXISTS hd_meta (id INTEGER PRIMARY KEY CHECK (id = 1), version INTEGER NOT NULL);
INSERT OR IGNORE INTO hd_meta (id, version) VALUES (1, 1);
CREATE TABLE IF NOT EXISTS hd_dashboards (
	id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, schema_version INTEGER NOT NULL,
	revision INTEGER NOT NULL, approved_revision INTEGER NOT NULL DEFAULT 0, document_json TEXT NOT NULL,
	saved_by TEXT NOT NULL CHECK (saved_by IN ('user','agent')), created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS hd_dashboard_revisions (
	dashboard_id TEXT NOT NULL REFERENCES hd_dashboards(id) ON DELETE CASCADE, revision INTEGER NOT NULL,
	document_json TEXT NOT NULL, saved_by TEXT NOT NULL, saved_at TEXT NOT NULL, PRIMARY KEY (dashboard_id, revision));
CREATE TABLE IF NOT EXISTS hd_kiosk_pairings (
	code_hash TEXT PRIMARY KEY, dashboard_id TEXT NOT NULL REFERENCES hd_dashboards(id) ON DELETE CASCADE,
	label TEXT NOT NULL, view_only INTEGER NOT NULL, expires_at TEXT NOT NULL, used_at TEXT);
CREATE TABLE IF NOT EXISTS hd_kiosk_devices (
	id TEXT PRIMARY KEY, dashboard_id TEXT NOT NULL REFERENCES hd_dashboards(id) ON DELETE CASCADE,
	label TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE, view_only INTEGER NOT NULL, created_at TEXT NOT NULL,
	last_seen_at TEXT, last_ip TEXT, expires_at TEXT);
CREATE TABLE IF NOT EXISTS hd_mqtt_entities (
	id TEXT PRIMARY KEY, name TEXT NOT NULL, kind TEXT NOT NULL, state_topic TEXT NOT NULL, json_path TEXT NOT NULL DEFAULT '',
	unit TEXT NOT NULL DEFAULT '', device_class TEXT NOT NULL DEFAULT '', command_topic TEXT NOT NULL DEFAULT '',
	payload_on TEXT NOT NULL DEFAULT '', payload_off TEXT NOT NULL DEFAULT '', options_json TEXT NOT NULL DEFAULT '[]',
	min REAL, max REAL, step REAL, qos INTEGER NOT NULL DEFAULT 0, retain INTEGER NOT NULL DEFAULT 0, updated_at TEXT NOT NULL);
`

// Store is homedash.db: one connection, WAL, foreign keys on (dbutil.Open). It does not validate
// documents; callers run Normalize, Autopack and Validate first. hd_mqtt_entities CRUD comes with B12.
type Store struct {
	db  *sql.DB
	now func() time.Time
	// maxDashboards is set by SetMaxDashboards from config hot reload while requests save.
	maxDashboards atomic.Int64
}

// OpenStore opens (and on first use creates) the HomeDash database at path.
func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create HomeDash data directory: %w", err)
	}
	db, err := dbutil.Open(path, dbutil.WithMaxOpenConns(1))
	if err != nil {
		return nil, fmt.Errorf("open HomeDash database: %w", err)
	}
	if _, err := db.Exec(storeSchemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize HomeDash database: %w", err)
	}
	var v int
	if err := db.QueryRow(`SELECT version FROM hd_meta WHERE id = 1`).Scan(&v); err != nil || v != storeSchemaVersion {
		db.Close()
		return nil, errors.New("unsupported HomeDash database schema")
	}
	st := &Store{db: db, now: func() time.Time { return time.Now().UTC() }}
	st.maxDashboards.Store(50)
	return st, nil
}

func (s *Store) Close() error { return s.db.Close() }

// SetMaxDashboards sets the dashboard cap; non-positive values are ignored.
func (s *Store) SetMaxDashboards(n int) {
	if n > 0 {
		s.maxDashboards.Store(int64(n))
	}
}

// ts formats with a fixed width so that string comparison in SQL orders like time (RFC3339Nano
// drops trailing zeros); parseTS reads it back.
func ts(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000000000Z") }

func parseTS(v string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, v)
	return t
}

// hashSecret returns the SHA-256 hex digest under which pairing codes and device tokens are stored.
func hashSecret(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}
