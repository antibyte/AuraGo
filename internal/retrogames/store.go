package retrogames

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/dbutil"

	"modernc.org/sqlite" // also registers the "sqlite" driver that dbutil.Open uses
)

const (
	schemaVersion = 1
	dbFileName    = "retrogames.db"
	gameColumns   = "g.identifier, g.title, g.year, g.creator, g.platform, g.system, g.emulator, g.downloads"

	// busyTimeoutMS is generous because replacing a large source holds the
	// write lock for several seconds.
	busyTimeoutMS = 30000
)

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS games (
		id INTEGER PRIMARY KEY,
		identifier TEXT NOT NULL UNIQUE,
		title TEXT NOT NULL,
		year INTEGER NOT NULL DEFAULT 0,
		creator TEXT NOT NULL DEFAULT '',
		platform TEXT NOT NULL,
		system TEXT NOT NULL,
		emulator TEXT NOT NULL DEFAULT '',
		downloads INTEGER NOT NULL DEFAULT 0,
		pop_rank INTEGER NOT NULL DEFAULT 0,
		tags TEXT NOT NULL DEFAULT '',
		source TEXT NOT NULL,
		synced_at INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS games_source ON games(source)`,
	`CREATE INDEX IF NOT EXISTS games_downloads ON games(downloads DESC)`,
	`CREATE INDEX IF NOT EXISTS games_platform_downloads ON games(platform, downloads DESC)`,
	`CREATE INDEX IF NOT EXISTS games_title ON games(title COLLATE NOCASE)`,
	`CREATE INDEX IF NOT EXISTS games_platform_title ON games(platform, title COLLATE NOCASE)`,
	`CREATE INDEX IF NOT EXISTS games_platform_year ON games(platform, year)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS games_fts USING fts5(title, creator, tags, content='games', content_rowid='id', tokenize='unicode61 remove_diacritics 2', prefix='2 3')`,
	`CREATE TRIGGER IF NOT EXISTS games_ai AFTER INSERT ON games BEGIN
		INSERT INTO games_fts(rowid, title, creator, tags) VALUES (new.id, new.title, new.creator, new.tags);
	END`,
	`CREATE TRIGGER IF NOT EXISTS games_ad AFTER DELETE ON games BEGIN
		INSERT INTO games_fts(games_fts, rowid, title, creator, tags) VALUES ('delete', old.id, old.title, old.creator, old.tags);
	END`,
	`CREATE TRIGGER IF NOT EXISTS games_au AFTER UPDATE OF title, creator, tags ON games BEGIN
		INSERT INTO games_fts(games_fts, rowid, title, creator, tags) VALUES ('delete', old.id, old.title, old.creator, old.tags);
		INSERT INTO games_fts(rowid, title, creator, tags) VALUES (new.id, new.title, new.creator, new.tags);
	END`,
	`CREATE TABLE IF NOT EXISTS favorites (identifier TEXT PRIMARY KEY, added_at INTEGER NOT NULL, title TEXT NOT NULL, platform TEXT NOT NULL, system TEXT NOT NULL, year INTEGER NOT NULL DEFAULT 0)`,
	`CREATE TABLE IF NOT EXISTS plays (identifier TEXT PRIMARY KEY, last_played_at INTEGER NOT NULL, play_count INTEGER NOT NULL DEFAULT 0, title TEXT NOT NULL, platform TEXT NOT NULL, system TEXT NOT NULL, year INTEGER NOT NULL DEFAULT 0)`,
	`CREATE INDEX IF NOT EXISTS plays_last ON plays(last_played_at DESC)`,
	`CREATE TABLE IF NOT EXISTS broken (identifier TEXT PRIMARY KEY, marked_at INTEGER NOT NULL, title TEXT NOT NULL, platform TEXT NOT NULL, system TEXT NOT NULL, year INTEGER NOT NULL DEFAULT 0)`,
	`CREATE TABLE IF NOT EXISTS sync_state (source TEXT PRIMARY KEY, last_success_at INTEGER NOT NULL DEFAULT 0, last_attempt_at INTEGER NOT NULL DEFAULT 0, item_count INTEGER NOT NULL DEFAULT 0, last_error TEXT NOT NULL DEFAULT '')`,
}

// insertGameSQL keeps the first source that owns an identifier: another source
// listing the same item does not steal it.
const insertGameSQL = `INSERT INTO games(identifier, title, year, creator, platform, system, emulator, downloads, pop_rank, tags, source, synced_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(identifier) DO NOTHING`

// Store is the SQLite index plus the user's library.
type Store struct {
	db *sql.DB
	// writeSem is the store's in-process write lock, a one-slot semaphore.
	// Every write takes it before it touches SQLite, so writes of this
	// process never wait for each other inside SQLite's busy timeout; a
	// caller with a critical section of its own (the desktop publication
	// gate) takes the lock first, outside that section (see lockWrites).
	writeSem chan struct{}
	// beforeReplaceCommit, set only by tests, runs inside ReplaceSource's
	// transaction (write lock held, rows written) just before the commit.
	beforeReplaceCommit func()
}

// SyncState is the persisted sync bookkeeping of one source.
type SyncState struct {
	Source        string
	LastSuccessAt time.Time
	LastAttemptAt time.Time
	ItemCount     int
	LastError     string
}

// OpenStore opens or creates the index database in dir. Only a database that
// is really corrupt is moved aside (to .corrupt-<timestamp>) and replaced by a
// fresh one; any other open error is returned unchanged.
func OpenStore(dir string, logger *slog.Logger) (*Store, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, errors.New("retrogames: empty data directory")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("retrogames: create data directory: %w", err)
	}
	path := filepath.Join(dir, dbFileName)
	db, err := dbutil.Open(path, dbutil.WithBusyTimeout(busyTimeoutMS))
	if err != nil && isCorruptDB(err) {
		logger.Error("retrogames: index database is corrupt, starting a fresh one", "path", path, "error", err)
		if moveErr := moveCorruptAside(path, time.Now().UTC()); moveErr != nil {
			return nil, fmt.Errorf("retrogames: move corrupt index aside: %w", moveErr)
		}
		db, err = dbutil.Open(path, dbutil.WithBusyTimeout(busyTimeoutMS))
	}
	if err != nil {
		return nil, fmt.Errorf("retrogames: open index: %w", err)
	}
	version, err := dbutil.GetUserVersion(db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("retrogames: read schema version: %w", err)
	}
	if version > schemaVersion {
		_ = db.Close()
		return nil, fmt.Errorf("retrogames: index was created by a newer AuraGo (schema %d)", version)
	}
	for _, stmt := range schemaStatements {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("retrogames: create schema: %w", err)
		}
	}
	if version < schemaVersion {
		if err := dbutil.SetUserVersion(db, schemaVersion); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("retrogames: set schema version: %w", err)
		}
	}
	// The schema statements above ran before anyone else could reach the
	// store, so they need no write lock.
	return &Store{db: db, writeSem: make(chan struct{}, 1)}, nil
}

// lockWrites takes the store's write lock, waiting at most until ctx ends (a
// canceled ctx is refused even when the lock is free). The returned unlock is
// idempotent: a second call never frees a lock taken by someone else.
func (st *Store) lockWrites(ctx context.Context) (unlock func(), err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case st.writeSem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	var once sync.Once
	return func() { once.Do(func() { <-st.writeSem }) }, nil
}

// isCorruptDB reports whether err means the database file itself is damaged.
// The library tables (favorites, plays, broken) are user data, so only real
// corruption may move the file; locks, I/O and permission errors must not.
func isCorruptDB(err error) bool {
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() & 0xff {
		case 11, 26: // SQLITE_CORRUPT, SQLITE_NOTADB
			return true
		}
	}
	msg := err.Error()
	return strings.Contains(msg, "database integrity check failed") || strings.Contains(msg, "file is not a database")
}

// moveCorruptAside renames the database and its WAL/SHM companions that exist
// to <name>.corrupt-<timestamp>.
func moveCorruptAside(path string, now time.Time) error {
	stamp := now.Format("20060102-150405")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := path + suffix
		if _, err := os.Stat(src); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		if err := os.Rename(src, src+".corrupt-"+stamp); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the database.
func (st *Store) Close() error {
	return st.db.Close()
}

// ReplaceSource swaps all rows of one source for games in a single
// transaction and records the successful sync. It holds the write lock for
// the whole transaction.
func (st *Store) ReplaceSource(ctx context.Context, source string, games []Game, now time.Time) error {
	if !ValidPlatform(source) {
		return fmt.Errorf("retrogames: unknown source %q", source)
	}
	if len(games) == 0 {
		return fmt.Errorf("retrogames: refusing to replace source %s with no games", source)
	}
	unlock, err := st.lockWrites(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("retrogames: begin replace: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM games WHERE source = ?`, source); err != nil {
		return fmt.Errorf("retrogames: clear source %s: %w", source, err)
	}
	stmt, err := tx.PrepareContext(ctx, insertGameSQL)
	if err != nil {
		return fmt.Errorf("retrogames: prepare insert: %w", err)
	}
	defer stmt.Close()
	ts := now.Unix()
	for _, g := range games {
		if _, err := stmt.ExecContext(ctx, g.Identifier, g.Title, g.Year, g.Creator, g.Platform, g.System, g.Emulator, g.Downloads, popRank(g.Downloads), g.Tags, source, ts); err != nil {
			return fmt.Errorf("retrogames: insert %s: %w", g.Identifier, err)
		}
	}
	// Identifiers already owned by another source were skipped above, so the
	// stored count is what this source really holds.
	var stored int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM games WHERE source = ?`, source).Scan(&stored); err != nil {
		return fmt.Errorf("retrogames: count %s: %w", source, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sync_state(source, last_success_at, last_attempt_at, item_count, last_error) VALUES (?, ?, ?, ?, '')
		ON CONFLICT(source) DO UPDATE SET last_success_at = excluded.last_success_at, last_attempt_at = excluded.last_attempt_at, item_count = excluded.item_count, last_error = ''`,
		source, ts, ts, stored); err != nil {
		return fmt.Errorf("retrogames: record sync of %s: %w", source, err)
	}
	if st.beforeReplaceCommit != nil {
		st.beforeReplaceCommit()
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("retrogames: commit replace: %w", err)
	}
	return nil
}

// RecordSyncFailure stores a failed attempt without touching the last success.
func (st *Store) RecordSyncFailure(ctx context.Context, source string, now time.Time, message string) error {
	unlock, err := st.lockWrites(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	_, err = st.db.ExecContext(ctx, `INSERT INTO sync_state(source, last_attempt_at, last_error) VALUES (?, ?, ?)
		ON CONFLICT(source) DO UPDATE SET last_attempt_at = excluded.last_attempt_at, last_error = excluded.last_error`,
		source, now.Unix(), truncateRunes(message, 300))
	if err != nil {
		return fmt.Errorf("retrogames: record failure of %s: %w", source, err)
	}
	return nil
}

// SyncStates returns the sync bookkeeping keyed by source.
func (st *Store) SyncStates(ctx context.Context) (map[string]SyncState, error) {
	rows, err := st.db.QueryContext(ctx, `SELECT source, last_success_at, last_attempt_at, item_count, last_error FROM sync_state`)
	if err != nil {
		return nil, fmt.Errorf("retrogames: read sync state: %w", err)
	}
	defer rows.Close()
	out := map[string]SyncState{}
	for rows.Next() {
		var s SyncState
		var success, attempt int64
		if err := rows.Scan(&s.Source, &success, &attempt, &s.ItemCount, &s.LastError); err != nil {
			return nil, fmt.Errorf("retrogames: scan sync state: %w", err)
		}
		s.LastSuccessAt = fromUnix(success)
		s.LastAttemptAt = fromUnix(attempt)
		out[s.Source] = s
	}
	return out, rows.Err()
}

// NeedsSync reports whether any of keys never synced or is older than maxAge.
func (st *Store) NeedsSync(ctx context.Context, now time.Time, maxAge time.Duration, keys []string) (bool, error) {
	states, err := st.SyncStates(ctx)
	if err != nil {
		return false, err
	}
	for _, key := range keys {
		s, ok := states[key]
		if !ok || s.LastSuccessAt.IsZero() || now.Sub(s.LastSuccessAt) > maxAge {
			return true, nil
		}
	}
	return false, nil
}

// Game looks up one indexed game.
func (st *Store) Game(ctx context.Context, id string) (Game, bool, error) {
	g, err := scanGame(st.db.QueryRowContext(ctx, `SELECT `+gameColumns+` FROM games g WHERE g.identifier = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Game{}, false, nil
	}
	if err != nil {
		return Game{}, false, fmt.Errorf("retrogames: read game: %w", err)
	}
	return g, true, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

// scanGame reads the gameColumns of one row; Game.Tags is not loaded.
func scanGame(r rowScanner) (Game, error) {
	var g Game
	err := r.Scan(&g.Identifier, &g.Title, &g.Year, &g.Creator, &g.Platform, &g.System, &g.Emulator, &g.Downloads)
	return g, err
}

func fromUnix(v int64) time.Time {
	if v <= 0 {
		return time.Time{}
	}
	return time.Unix(v, 0).UTC()
}
