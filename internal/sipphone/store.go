package sipphone

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create SIP data directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open SIP call store: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS sip_calls (
  id TEXT PRIMARY KEY,
  direction TEXT NOT NULL,
  remote_party TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  answered_at INTEGER,
  ended_at INTEGER,
  state TEXT NOT NULL,
  end_reason TEXT NOT NULL DEFAULT '',
  backend TEXT NOT NULL,
  session_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_sip_calls_started_at ON sip_calls(started_at DESC);
PRAGMA user_version=1;`)
	if err != nil {
		return fmt.Errorf("migrate SIP call store: %w", err)
	}
	return nil
}

func (s *Store) Upsert(ctx context.Context, call CallRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sip_calls
(id,direction,remote_party,started_at,answered_at,ended_at,state,end_reason,backend,session_id)
VALUES(?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
 answered_at=excluded.answered_at, ended_at=excluded.ended_at, state=excluded.state,
 end_reason=excluded.end_reason, backend=excluded.backend, session_id=excluded.session_id`,
		call.ID, call.Direction, call.RemoteParty, call.StartedAt.UnixMilli(), nullableMillis(call.AnsweredAt),
		nullableMillis(call.EndedAt), call.State, call.EndReason, call.Backend, call.SessionID)
	if err != nil {
		return fmt.Errorf("persist SIP call: %w", err)
	}
	return nil
}

func (s *Store) List(ctx context.Context, limit int) ([]CallRecord, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,direction,remote_party,started_at,answered_at,ended_at,state,end_reason,backend,session_id
FROM sip_calls ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list SIP calls: %w", err)
	}
	defer rows.Close()
	result := make([]CallRecord, 0)
	for rows.Next() {
		var call CallRecord
		var started int64
		var answered, ended sql.NullInt64
		if err := rows.Scan(&call.ID, &call.Direction, &call.RemoteParty, &started, &answered, &ended, &call.State, &call.EndReason, &call.Backend, &call.SessionID); err != nil {
			return nil, fmt.Errorf("scan SIP call: %w", err)
		}
		call.StartedAt = time.UnixMilli(started).UTC()
		call.AnsweredAt = timeFromNullMillis(answered)
		call.EndedAt = timeFromNullMillis(ended)
		result = append(result, call)
	}
	return result, rows.Err()
}

func (s *Store) DeleteOlderThan(ctx context.Context, cutoff time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sip_calls WHERE started_at < ?`, cutoff.UnixMilli())
	if err != nil {
		return fmt.Errorf("prune SIP calls: %w", err)
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

func nullableMillis(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UnixMilli()
}

func timeFromNullMillis(value sql.NullInt64) *time.Time {
	if !value.Valid {
		return nil
	}
	t := time.UnixMilli(value.Int64).UTC()
	return &t
}
