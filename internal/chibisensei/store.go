package chibisensei

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store is the Chibi Sensei SQLite store. It uses one connection; see the package rules in the plan.
type Store struct {
	db     *sql.DB
	path   string
	now    func() time.Time                      // UTC; tests replace it
	notify func(projectID string, dataRev int64) // called after a commit that bumped data_rev
}

// OpenStore opens (and creates) the database at path and checks its schema version.
func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create Chibi Sensei data directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize Chibi Sensei database: %w", err)
	}
	var version int
	if err = db.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil || version != schemaVersion {
		db.Close()
		return nil, errors.New("unsupported Chibi Sensei database schema")
	}
	return &Store{db: db, path: path, now: func() time.Time { return time.Now().UTC() }}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Path is the database file path.
func (s *Store) Path() string { return s.path }

// SetNotifier installs the data-change callback (the event hub in the server). It is never called inside a transaction.
func (s *Store) SetNotifier(fn func(projectID string, dataRev int64)) { s.notify = fn }

// SizeBytes is the size of the database file plus its WAL, for chibi_sensei.storage.max_gb.
func (s *Store) SizeBytes() int64 {
	var total int64
	for _, p := range []string{s.path, s.path + "-wal"} {
		if info, err := os.Stat(p); err == nil {
			total += info.Size()
		}
	}
	return total
}
