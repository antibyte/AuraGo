package dockerhost

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/dbutil"

	_ "modernc.org/sqlite"
)

const hostSchema = `
CREATE TABLE IF NOT EXISTS docker_hosts (
	id               TEXT PRIMARY KEY,
	name             TEXT NOT NULL,
	kind             TEXT NOT NULL,
	address          TEXT NOT NULL,
	port             INTEGER NOT NULL DEFAULT 0,
	stacks_dir       TEXT NOT NULL,
	host_socket_path TEXT NOT NULL,
	color            TEXT NOT NULL,
	ssh_user         TEXT NOT NULL DEFAULT '',
	sort             INTEGER NOT NULL DEFAULT 0,
	created_at       TEXT NOT NULL,
	updated_at       TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_docker_hosts_name ON docker_hosts(name COLLATE NOCASE);
`

const hostColumns = "id, name, kind, address, port, stacks_dir, host_socket_path, color, ssh_user, sort, created_at, updated_at"

// Store persists the Docker Manager's remote hosts in docker_manager.db. The
// built-in local host is never stored.
type Store struct{ db *sql.DB }

// OpenStore opens (and creates) the database at path with the shared SQLite
// settings and makes sure the docker_hosts table exists.
func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create docker manager directory: %w", err)
	}
	db, err := dbutil.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open docker manager database: %w", err)
	}
	if _, err := db.Exec(hostSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create docker manager schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the shared docker_manager.db handle so internal/dockerstacks can
// keep its tables in the same file. The Store owns the handle: callers must not
// close it.
func (s *Store) DB() *sql.DB { return s.db }

// List returns the stored hosts ordered by sort, then name (case-insensitive).
func (s *Store) List(ctx context.Context) ([]Host, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+hostColumns+" FROM docker_hosts ORDER BY sort, name COLLATE NOCASE")
	if err != nil {
		return nil, fmt.Errorf("list docker hosts: %w", err)
	}
	defer rows.Close()
	hosts := []Host{}
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

// Get returns one stored host or ErrHostNotFound.
func (s *Store) Get(ctx context.Context, id string) (Host, error) {
	h, err := scanHost(s.db.QueryRowContext(ctx, "SELECT "+hostColumns+" FROM docker_hosts WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Host{}, ErrHostNotFound
	}
	return h, err
}

// Create validates and stores a new host under a fresh "h_" id.
func (s *Store) Create(ctx context.Context, h Host) (Host, error) {
	h = h.Normalized()
	if err := ValidateHost(h); err != nil {
		return Host{}, err
	}
	id, err := newHostID()
	if err != nil {
		return Host{}, err
	}
	if err := s.checkNameFree(ctx, id, h.Name); err != nil {
		return Host{}, err
	}
	now := time.Now().UTC()
	h.ID, h.CreatedAt, h.UpdatedAt = id, now, now
	_, err = s.db.ExecContext(ctx, "INSERT INTO docker_hosts ("+hostColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?)",
		h.ID, h.Name, h.Kind, h.Address, h.Port, h.StacksDir, h.HostSocketPath, h.Color, h.SSHUser, h.Sort,
		formatTime(h.CreatedAt), formatTime(h.UpdatedAt))
	if err != nil {
		return Host{}, storeError("store docker host", err)
	}
	return h, nil
}

// Update validates and replaces the mutable fields of a stored host. The id and
// the creation time are kept; ErrHostNotFound when the id is unknown.
func (s *Store) Update(ctx context.Context, h Host) (Host, error) {
	h = h.Normalized()
	if err := ValidateHost(h); err != nil {
		return Host{}, err
	}
	current, err := s.Get(ctx, h.ID)
	if err != nil {
		return Host{}, err
	}
	if err := s.checkNameFree(ctx, h.ID, h.Name); err != nil {
		return Host{}, err
	}
	h.CreatedAt, h.UpdatedAt = current.CreatedAt, time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `UPDATE docker_hosts SET name = ?, kind = ?, address = ?, port = ?, stacks_dir = ?,
		host_socket_path = ?, color = ?, ssh_user = ?, sort = ?, updated_at = ? WHERE id = ?`,
		h.Name, h.Kind, h.Address, h.Port, h.StacksDir, h.HostSocketPath, h.Color, h.SSHUser, h.Sort,
		formatTime(h.UpdatedAt), h.ID)
	if err != nil {
		return Host{}, storeError("update docker host", err)
	}
	return h, nil
}

// Delete removes a stored host or returns ErrHostNotFound.
func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM docker_hosts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete docker host: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrHostNotFound
	}
	return nil
}

// storeError turns the unique-name index violation of a concurrent writer into
// the same validation error checkNameFree gives.
func storeError(op string, err error) error {
	if strings.Contains(err.Error(), "UNIQUE constraint failed: docker_hosts.name") {
		return invalid("name", "another host already uses this name")
	}
	return fmt.Errorf("%s: %w", op, err)
}

// checkNameFree rejects a name another host already uses (case-insensitive).
func (s *Store) checkNameFree(ctx context.Context, id, name string) error {
	var other string
	err := s.db.QueryRowContext(ctx, "SELECT id FROM docker_hosts WHERE name = ? COLLATE NOCASE AND id <> ?", name, id).Scan(&other)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("check docker host name: %w", err)
	}
	return invalid("name", "another host already uses this name")
}

type rowScanner interface{ Scan(dest ...any) error }

func scanHost(row rowScanner) (Host, error) {
	var h Host
	var created, updated string
	if err := row.Scan(&h.ID, &h.Name, &h.Kind, &h.Address, &h.Port, &h.StacksDir, &h.HostSocketPath,
		&h.Color, &h.SSHUser, &h.Sort, &created, &updated); err != nil {
		return Host{}, err
	}
	h.CreatedAt = parseTime(created)
	h.UpdatedAt = parseTime(updated)
	return h, nil
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

// newHostID returns "h_" followed by 12 lowercase hex characters.
func newHostID() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate docker host id: %w", err)
	}
	return "h_" + hex.EncodeToString(b[:]), nil
}
