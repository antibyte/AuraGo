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
	if _, err := s.db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		return fmt.Errorf("configure SIP call store: %w", err)
	}
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read SIP call store version: %w", err)
	}
	if version > 3 {
		return fmt.Errorf("SIP call store version %d is newer than supported version 3", version)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin SIP call store migration: %w", err)
	}
	defer tx.Rollback()
	if version == 0 {
		if _, err = tx.Exec(`CREATE TABLE IF NOT EXISTS sip_calls (
  id TEXT PRIMARY KEY,
  direction TEXT NOT NULL,
  remote_party TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  answered_at INTEGER,
  ended_at INTEGER,
  state TEXT NOT NULL,
  end_reason TEXT NOT NULL DEFAULT '',
  backend TEXT NOT NULL,
  media_mode TEXT NOT NULL DEFAULT 'agent',
  session_id TEXT NOT NULL DEFAULT '',
  persist_transcripts INTEGER NOT NULL DEFAULT 1
);`); err != nil {
			return fmt.Errorf("create SIP call store schema: %w", err)
		}
	} else if version == 1 {
		if _, err = tx.Exec(`ALTER TABLE sip_calls ADD COLUMN persist_transcripts INTEGER NOT NULL DEFAULT 1`); err != nil {
			return fmt.Errorf("migrate SIP call store to version 2: %w", err)
		}
	}
	if version > 0 && version < 3 {
		if _, err = tx.Exec(`ALTER TABLE sip_calls ADD COLUMN media_mode TEXT NOT NULL DEFAULT 'agent'`); err != nil {
			return fmt.Errorf("migrate SIP call store to version 3: %w", err)
		}
		if _, err = tx.Exec(`UPDATE sip_calls SET media_mode='browser' WHERE backend='browser'`); err != nil {
			return fmt.Errorf("backfill SIP browser media mode: %w", err)
		}
	}
	if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_sip_calls_started_at ON sip_calls(started_at DESC)`); err != nil {
		return fmt.Errorf("create SIP call store index: %w", err)
	}
	if _, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_sip_calls_daily_agent ON sip_calls(direction, media_mode, started_at)`); err != nil {
		return fmt.Errorf("create SIP daily agent call index: %w", err)
	}
	if _, err = tx.Exec(`PRAGMA user_version=3`); err != nil {
		return fmt.Errorf("set SIP call store version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SIP call store migration: %w", err)
	}
	return nil
}

func (s *Store) Upsert(ctx context.Context, call CallRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sip_calls
(id,direction,remote_party,started_at,answered_at,ended_at,state,end_reason,backend,media_mode,session_id,persist_transcripts)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
 answered_at=excluded.answered_at, ended_at=excluded.ended_at, state=excluded.state,
 end_reason=excluded.end_reason, backend=excluded.backend, media_mode=excluded.media_mode, session_id=excluded.session_id,
 persist_transcripts=excluded.persist_transcripts`,
		call.ID, call.Direction, call.RemoteParty, call.StartedAt.UnixMilli(), nullableMillis(call.AnsweredAt),
		nullableMillis(call.EndedAt), call.State, call.EndReason, call.Backend, storedMediaMode(call), call.SessionID, call.persistTranscripts)
	if err != nil {
		return fmt.Errorf("persist SIP call: %w", err)
	}
	return nil
}

func (s *Store) List(ctx context.Context, limit int) ([]CallRecord, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,direction,remote_party,started_at,answered_at,ended_at,state,end_reason,backend,media_mode,session_id,persist_transcripts
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
		if err := rows.Scan(&call.ID, &call.Direction, &call.RemoteParty, &started, &answered, &ended, &call.State, &call.EndReason, &call.Backend, &call.MediaMode, &call.SessionID, &call.persistTranscripts); err != nil {
			return nil, fmt.Errorf("scan SIP call: %w", err)
		}
		call.StartedAt = time.UnixMilli(started).UTC()
		call.AnsweredAt = timeFromNullMillis(answered)
		call.EndedAt = timeFromNullMillis(ended)
		result = append(result, call)
	}
	return result, rows.Err()
}

func (s *Store) CountAgentOutbound(ctx context.Context, start, end time.Time) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sip_calls
WHERE direction='outbound' AND media_mode='agent' AND started_at>=? AND started_at<?`, start.UnixMilli(), end.UnixMilli()).Scan(&count); err != nil {
		return 0, fmt.Errorf("count daily telephone agent calls: %w", err)
	}
	return count, nil
}

// AdmitAgentOutbound atomically checks the daily quota and writes the initial
// call record. The single transaction prevents concurrent admissions from
// exceeding the configured limit.
func (s *Store) AdmitAgentOutbound(ctx context.Context, call CallRecord, start, end time.Time, limit int) (int, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, fmt.Errorf("begin telephone agent call admission: %w", err)
	}
	defer tx.Rollback()
	var used int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sip_calls
WHERE direction='outbound' AND media_mode='agent' AND started_at>=? AND started_at<?`, start.UnixMilli(), end.UnixMilli()).Scan(&used); err != nil {
		return 0, false, fmt.Errorf("count daily telephone agent calls: %w", err)
	}
	if used >= limit {
		return used, false, nil
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sip_calls
(id,direction,remote_party,started_at,answered_at,ended_at,state,end_reason,backend,media_mode,session_id,persist_transcripts)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, call.ID, call.Direction, call.RemoteParty, call.StartedAt.UnixMilli(), nullableMillis(call.AnsweredAt),
		nullableMillis(call.EndedAt), call.State, call.EndReason, call.Backend, MediaModeAgent, call.SessionID, call.persistTranscripts)
	if err != nil {
		return 0, false, fmt.Errorf("persist admitted telephone agent call: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, false, fmt.Errorf("commit telephone agent call admission: %w", err)
	}
	return used + 1, true, nil
}

func (s *Store) NonPersistentSessionIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT session_id FROM sip_calls WHERE persist_transcripts=0 AND session_id LIKE 'sip-%' AND session_id <> ''`)
	if err != nil {
		return nil, fmt.Errorf("list transient SIP sessions: %w", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return nil, fmt.Errorf("scan transient SIP session: %w", err)
		}
		result = append(result, sessionID)
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

func (s *Store) DeleteAll(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sip_calls`); err != nil {
		return fmt.Errorf("delete SIP call history: %w", err)
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

func storedMediaMode(call CallRecord) string {
	if call.MediaMode == MediaModeBrowser || call.Backend == MediaModeBrowser {
		return MediaModeBrowser
	}
	return MediaModeAgent
}
