// Package tresor stores opaque, browser-encrypted vault records.
package tresor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	_ "modernc.org/sqlite"
)

var ErrConflict = errors.New("tresor revision conflict")

// kdfLabelPattern bounds the Argon2id parameter label the browser writes. The
// client owns the values; the server only rejects a shape no client could have
// produced. An empty label means "format default" and stays valid for headers
// written before the field existed.
var kdfLabelPattern = regexp.MustCompile(`^argon2id\$v=[0-9]{1,2}\$m=[0-9]{1,7},t=[0-9]{1,2},p=[0-9]{1,2}$`)

// ValidKDFLabel reports whether label is an acceptable key derivation label.
func ValidKDFLabel(label string) bool {
	return label == "" || (len(label) <= 96 && kdfLabelPattern.MatchString(label))
}

type Header struct {
	Salt             []byte `json:"salt"`
	PasswordEnvelope []byte `json:"password_envelope"`
	RecoveryEnvelope []byte `json:"recovery_envelope"`
	// KDF names the Argon2id parameters of PasswordEnvelope so a later client
	// version can still open this vault. Empty selects the format default.
	KDF      string `json:"kdf,omitempty"`
	Revision int64  `json:"revision"`
}

// kdfMetaKey stores the key derivation label beside the single header row. A
// separate key/value table keeps an existing tresor.db readable without an
// ALTER TABLE in the one database that must never be migrated in place.
const kdfMetaKey = "kdf"

type Record struct {
	ID       string `json:"id"`
	Meta     []byte `json:"meta"`
	Body     []byte `json:"body,omitempty"`
	Revision int64  `json:"revision"`
}

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create tresor directory: %w", err)
	}
	// Create or tighten the file before SQLite touches it: the driver would
	// create it under the process umask, and SQLite gives the -wal/-shm
	// sidecars the database's mode.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open or create tresor database: %w", err)
	}
	if err = f.Chmod(0o600); err != nil {
		f.Close()
		return nil, fmt.Errorf("open or create tresor database: %w", err)
	}
	if err = f.Close(); err != nil {
		return nil, fmt.Errorf("open or create tresor database: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open tresor database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL;
		CREATE TABLE IF NOT EXISTS header (id INTEGER PRIMARY KEY CHECK (id=1), salt BLOB NOT NULL,
			password_envelope BLOB NOT NULL, recovery_envelope BLOB NOT NULL, revision INTEGER NOT NULL);
		CREATE TABLE IF NOT EXISTS records (id TEXT PRIMARY KEY, meta BLOB NOT NULL,
			body BLOB NOT NULL, revision INTEGER NOT NULL);
		CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);`); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize tresor database: %w", err)
	}
	if err = os.Chmod(path, 0600); err != nil {
		db.Close()
		return nil, fmt.Errorf("protect tresor database: %w", err)
	}
	// Belt and braces: sidecars left by an older, wider database file keep
	// their mode, so tighten whichever exist now.
	for _, sidecar := range []string{path + "-wal", path + "-shm"} {
		if err = os.Chmod(sidecar, 0600); err != nil && !errors.Is(err, os.ErrNotExist) {
			db.Close()
			return nil, fmt.Errorf("protect tresor database sidecar: %w", err)
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close tresor database: %w", err)
	}
	return nil
}

func (s *Store) Header(ctx context.Context) (Header, error) {
	var h Header
	err := s.db.QueryRowContext(ctx, `SELECT h.salt,h.password_envelope,h.recovery_envelope,h.revision,COALESCE(m.value,'')
		FROM header h LEFT JOIN meta m ON m.key=? WHERE h.id=1`, kdfMetaKey).
		Scan(&h.Salt, &h.PasswordEnvelope, &h.RecoveryEnvelope, &h.Revision, &h.KDF)
	if err != nil {
		return Header{}, fmt.Errorf("read tresor header: %w", err)
	}
	return h, nil
}

// Setup writes the single header row and its key derivation label atomically.
func (s *Store) Setup(ctx context.Context, h Header) error {
	return s.mutate(ctx, "setup", func(tx *sql.Tx) error {
		r, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO header(id,salt,password_envelope,recovery_envelope,revision) VALUES(1,?,?,?,1)`, h.Salt, h.PasswordEnvelope, h.RecoveryEnvelope)
		if err = changed(r, err); err != nil {
			return err
		}
		return writeKDF(ctx, tx, h.KDF)
	})
}

// Rewrap replaces both envelopes and the label in one revision step.
func (s *Store) Rewrap(ctx context.Context, h Header, expected int64) error {
	return s.mutate(ctx, "rewrap", func(tx *sql.Tx) error {
		r, err := tx.ExecContext(ctx, `UPDATE header SET salt=?,password_envelope=?,recovery_envelope=?,revision=revision+1 WHERE id=1 AND revision=?`, h.Salt, h.PasswordEnvelope, h.RecoveryEnvelope, expected)
		if err = changed(r, err); err != nil {
			return err
		}
		return writeKDF(ctx, tx, h.KDF)
	})
}

// mutate runs one header change together with its key derivation label so a
// failed label write can never leave an envelope without its parameters.
func (s *Store) mutate(ctx context.Context, action string, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tresor %s: %w", action, err)
	}
	defer func() { _ = tx.Rollback() }()
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tresor %s: %w", action, err)
	}
	return nil
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func writeKDF(ctx context.Context, db sqlExecer, label string) error {
	if _, err := db.ExecContext(ctx, `INSERT INTO meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, kdfMetaKey, label); err != nil {
		return fmt.Errorf("store tresor key derivation parameters: %w", err)
	}
	return nil
}

func (s *Store) List(ctx context.Context) ([]Record, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,meta,revision FROM records ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list tresor records: %w", err)
	}
	defer rows.Close()
	result := []Record{}
	for rows.Next() {
		var item Record
		if err := rows.Scan(&item.ID, &item.Meta, &item.Revision); err != nil {
			return nil, fmt.Errorf("read tresor metadata: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tresor records: %w", err)
	}
	return result, nil
}

func (s *Store) Get(ctx context.Context, id string) (Record, error) {
	var item Record
	err := s.db.QueryRowContext(ctx, `SELECT id,meta,body,revision FROM records WHERE id=?`, id).Scan(&item.ID, &item.Meta, &item.Body, &item.Revision)
	if err != nil {
		return Record{}, fmt.Errorf("read tresor record: %w", err)
	}
	return item, nil
}

func (s *Store) Create(ctx context.Context, item Record) error {
	r, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO records(id,meta,body,revision) VALUES(?,?,?,1)`, item.ID, item.Meta, item.Body)
	return changed(r, err)
}

func (s *Store) Update(ctx context.Context, item Record, expected int64) error {
	r, err := s.db.ExecContext(ctx, `UPDATE records SET meta=?,body=?,revision=revision+1 WHERE id=? AND revision=?`, item.Meta, item.Body, item.ID, expected)
	return changed(r, err)
}

func (s *Store) Delete(ctx context.Context, id string, expected int64) error {
	r, err := s.db.ExecContext(ctx, `DELETE FROM records WHERE id=? AND revision=?`, id, expected)
	return changed(r, err)
}

func changed(r sql.Result, err error) error {
	if err != nil {
		return fmt.Errorf("mutate tresor database: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return fmt.Errorf("check tresor mutation: %w", err)
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
