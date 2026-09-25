package newspaper

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create newspaper directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open newspaper database: %w", err)
	}
	db.SetMaxOpenConns(1)
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version > 1 {
		db.Close()
		return nil, fmt.Errorf("unsupported newspaper database version %d: %w", version, err)
	}
	_, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS newspaper_profile(id INTEGER PRIMARY KEY CHECK(id=1), version INTEGER NOT NULL, body BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS newspaper_runs(id TEXT PRIMARY KEY, local_date TEXT NOT NULL, revision INTEGER NOT NULL, status TEXT NOT NULL, body BLOB NOT NULL, UNIQUE(local_date,revision));
CREATE UNIQUE INDEX IF NOT EXISTS newspaper_one_active ON newspaper_runs(status) WHERE status='running';
CREATE INDEX IF NOT EXISTS newspaper_runs_date ON newspaper_runs(local_date DESC,revision DESC);
CREATE TABLE IF NOT EXISTS newspaper_editions(id TEXT PRIMARY KEY, local_date TEXT NOT NULL, revision INTEGER NOT NULL, hash TEXT NOT NULL, body BLOB NOT NULL, created_at TEXT NOT NULL, UNIQUE(local_date,revision));
CREATE INDEX IF NOT EXISTS newspaper_editions_date ON newspaper_editions(local_date DESC,revision DESC);
CREATE TABLE IF NOT EXISTS newspaper_sections(edition_id TEXT NOT NULL REFERENCES newspaper_editions(id) ON DELETE CASCADE, name TEXT NOT NULL, position INTEGER NOT NULL, PRIMARY KEY(edition_id,name));
CREATE TABLE IF NOT EXISTS newspaper_stories(edition_id TEXT NOT NULL REFERENCES newspaper_editions(id) ON DELETE CASCADE, id TEXT NOT NULL, section TEXT NOT NULL, position INTEGER NOT NULL, body BLOB NOT NULL, PRIMARY KEY(edition_id,id));
CREATE TABLE IF NOT EXISTS newspaper_sources(edition_id TEXT NOT NULL REFERENCES newspaper_editions(id) ON DELETE CASCADE, id TEXT NOT NULL, url TEXT NOT NULL, body BLOB NOT NULL, PRIMARY KEY(edition_id,id));
CREATE TABLE IF NOT EXISTS newspaper_evidence(edition_id TEXT NOT NULL, source_id TEXT NOT NULL, excerpt TEXT NOT NULL, PRIMARY KEY(edition_id,source_id), FOREIGN KEY(edition_id,source_id) REFERENCES newspaper_sources(edition_id,id) ON DELETE CASCADE);
CREATE TABLE IF NOT EXISTS newspaper_corrections(edition_id TEXT NOT NULL REFERENCES newspaper_editions(id) ON DELETE CASCADE, position INTEGER NOT NULL, note TEXT NOT NULL, PRIMARY KEY(edition_id,position));
CREATE TABLE IF NOT EXISTS newspaper_deliveries(id INTEGER PRIMARY KEY AUTOINCREMENT, edition_id TEXT NOT NULL REFERENCES newspaper_editions(id) ON DELETE CASCADE, hash TEXT NOT NULL, channel TEXT NOT NULL, kind TEXT NOT NULL, target_hash TEXT NOT NULL, request_key TEXT NOT NULL, status TEXT NOT NULL, reason TEXT NOT NULL, provider_id TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(edition_id,channel,target_hash,request_key));
CREATE TABLE IF NOT EXISTS newspaper_events(id INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT NOT NULL REFERENCES newspaper_runs(id) ON DELETE CASCADE, at TEXT NOT NULL, phase TEXT NOT NULL, text TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS newspaper_events_run ON newspaper_events(run_id,id);
CREATE TABLE IF NOT EXISTS newspaper_run_sources(run_id TEXT NOT NULL REFERENCES newspaper_runs(id) ON DELETE CASCADE, id TEXT NOT NULL, body BLOB NOT NULL, PRIMARY KEY(run_id,id));
CREATE TABLE IF NOT EXISTS newspaper_email_challenges(address TEXT PRIMARY KEY, account_id TEXT NOT NULL, digest BLOB NOT NULL, expires_at TEXT NOT NULL, attempts INTEGER NOT NULL);
PRAGMA user_version=1;`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate newspaper database: %w", err)
	}
	s := &Store{db: db}
	profile := DefaultProfile()
	profile.Version = 1
	b, _ := json.Marshal(profile)
	if _, err = db.Exec("INSERT OR IGNORE INTO newspaper_profile(id,version,body) VALUES(1,1,?)", b); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize newspaper profile: %w", err)
	}
	// An interrupted process cannot continue its in-memory research safely.
	if _, err = db.Exec("UPDATE newspaper_runs SET status='interrupted', body=json_set(body,'$.status','interrupted','$.reason','server_restart') WHERE status='running'"); err != nil {
		db.Close()
		return nil, fmt.Errorf("reconcile newspaper runs: %w", err)
	}
	if _, err = db.Exec("UPDATE newspaper_deliveries SET status='uncertain',reason='Delivery outcome unknown after server restart' WHERE status='sending'"); err != nil {
		db.Close()
		return nil, fmt.Errorf("reconcile newspaper deliveries: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Profile(ctx context.Context) (Profile, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT body FROM newspaper_profile WHERE id=1").Scan(&b)
	var p Profile
	if err == nil {
		err = json.Unmarshal(b, &p)
	}
	return p, err
}

func (s *Store) SaveProfile(ctx context.Context, p Profile) (Profile, error) {
	if err := p.Validate(); err != nil {
		return Profile{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Profile{}, err
	}
	defer tx.Rollback()
	var version int64
	var oldBody []byte
	if err = tx.QueryRowContext(ctx, "SELECT version,body FROM newspaper_profile WHERE id=1").Scan(&version, &oldBody); err != nil {
		return Profile{}, err
	}
	if p.Version != version {
		return Profile{}, ErrConflict
	}
	var old Profile
	if err = json.Unmarshal(oldBody, &old); err != nil {
		return Profile{}, err
	}
	p.EmailVerified = old.EmailVerified && p.EmailTo == old.EmailTo && p.EmailAccountID == old.EmailAccountID
	if p.EmailTo != old.EmailTo || p.EmailAccountID != old.EmailAccountID {
		p.EmailDaily = false
	}
	if p.EmailDaily && !p.EmailVerified {
		return Profile{}, errors.New("verify the email destination before enabling daily email")
	}
	p.Version++
	b, err := json.Marshal(p)
	if err != nil {
		return Profile{}, err
	}
	res, err := tx.ExecContext(ctx, "UPDATE newspaper_profile SET version=?,body=? WHERE id=1 AND version=?", p.Version, b, version)
	if err != nil {
		return Profile{}, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return Profile{}, ErrConflict
	}
	return p, tx.Commit()
}

func (s *Store) MarkEmailVerified(ctx context.Context, address, accountID string) (Profile, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Profile{}, err
	}
	defer tx.Rollback()
	var b []byte
	if err = tx.QueryRowContext(ctx, "SELECT body FROM newspaper_profile WHERE id=1").Scan(&b); err != nil {
		return Profile{}, err
	}
	var p Profile
	if err = json.Unmarshal(b, &p); err != nil {
		return Profile{}, err
	}
	if p.EmailTo != address || p.EmailAccountID != accountID {
		return Profile{}, ErrConflict
	}
	p.EmailVerified = true
	p.Version++
	b, _ = json.Marshal(p)
	if _, err = tx.ExecContext(ctx, "UPDATE newspaper_profile SET version=?,body=? WHERE id=1", p.Version, b); err != nil {
		return Profile{}, err
	}
	return p, tx.Commit()
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func (s *Store) Start(ctx context.Context, date string, newRevision bool, now time.Time) (Run, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return Run{}, errors.New("invalid local edition date")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, err
	}
	defer tx.Rollback()
	var active []byte
	err = tx.QueryRowContext(ctx, "SELECT body FROM newspaper_runs WHERE status='running' LIMIT 1").Scan(&active)
	if err == nil {
		return Run{}, ErrBusy
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Run{}, err
	}
	var revision int
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(revision),0) FROM newspaper_runs WHERE local_date=?", date).Scan(&revision); err != nil {
		return Run{}, err
	}
	if revision > 0 && !newRevision {
		return Run{}, ErrConflict
	}
	id, err := newID()
	if err != nil {
		return Run{}, err
	}
	r := Run{ID: id, LocalDate: date, Revision: revision + 1, Status: "running", Phase: "finding", StartedAt: now.UTC(), UpdatedAt: now.UTC()}
	b, _ := json.Marshal(r)
	if _, err = tx.ExecContext(ctx, "INSERT INTO newspaper_runs(id,local_date,revision,status,body) VALUES(?,?,?,?,?)", id, date, r.Revision, r.Status, b); err != nil {
		return Run{}, ErrBusy
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO newspaper_events(run_id,at,phase,text) VALUES(?,?,?,?)", id, now.UTC().Format(time.RFC3339Nano), "finding", "Research started"); err != nil {
		return Run{}, err
	}
	return r, tx.Commit()
}

func (s *Store) UpdateRun(ctx context.Context, r Run, message string) error {
	r.UpdatedAt = time.Now().UTC()
	b, _ := json.Marshal(r)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE newspaper_runs SET status=?,body=? WHERE id=?", r.Status, b, r.ID); err != nil {
		return err
	}
	if message != "" {
		if len(message) > 400 {
			message = message[:400]
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO newspaper_events(run_id,at,phase,text) VALUES(?,?,?,?)", r.ID, r.UpdatedAt.Format(time.RFC3339Nano), r.Phase, message); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM newspaper_events WHERE run_id=? AND id NOT IN (SELECT id FROM newspaper_events WHERE run_id=? ORDER BY id DESC LIMIT 300)", r.ID, r.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Run(ctx context.Context, id string) (Run, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT body FROM newspaper_runs WHERE id=?", id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	var r Run
	if err == nil {
		err = json.Unmarshal(b, &r)
	}
	return r, err
}

func (s *Store) RecordRunSource(ctx context.Context, runID string, source Source) error {
	b, err := json.Marshal(source)
	if err != nil {
		return err
	}
	if len(b) > 10_000 {
		return errors.New("research source exceeds retention bound")
	}
	_, err = s.db.ExecContext(ctx, "INSERT OR IGNORE INTO newspaper_run_sources(run_id,id,body) SELECT ?,?,? WHERE (SELECT COUNT(*) FROM newspaper_run_sources WHERE run_id=?)<60", runID, source.ID, b, runID)
	return err
}

func (s *Store) RunSources(ctx context.Context, runID string) ([]Source, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT body FROM newspaper_run_sources WHERE run_id=? ORDER BY id LIMIT 60", runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Source{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var source Source
		if err := json.Unmarshal(b, &source); err != nil {
			return nil, err
		}
		out = append(out, source)
	}
	return out, rows.Err()
}

func (s *Store) LatestRun(ctx context.Context) (Run, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT body FROM newspaper_runs ORDER BY local_date DESC,revision DESC LIMIT 1").Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	var r Run
	if err == nil {
		err = json.Unmarshal(b, &r)
	}
	return r, err
}

func (s *Store) Publish(ctx context.Context, run Run, e Edition) error {
	if e.ID != run.ID || e.LocalDate != run.LocalDate || e.Revision != run.Revision || e.Hash == "" {
		return errors.New("edition does not match run")
	}
	if err := verifyEdition(e); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO newspaper_editions(id,local_date,revision,hash,body,created_at) VALUES(?,?,?,?,?,?)", e.ID, e.LocalDate, e.Revision, e.Hash, b, e.CreatedAt.Format(time.RFC3339Nano)); err != nil {
		return err
	}
	sections := map[string]bool{}
	for i, story := range e.Stories {
		if !sections[story.Section] {
			sections[story.Section] = true
			if _, err = tx.ExecContext(ctx, "INSERT INTO newspaper_sections(edition_id,name,position) VALUES(?,?,?)", e.ID, story.Section, len(sections)); err != nil {
				return err
			}
		}
		body, _ := json.Marshal(story)
		if _, err = tx.ExecContext(ctx, "INSERT INTO newspaper_stories(edition_id,id,section,position,body) VALUES(?,?,?,?,?)", e.ID, story.ID, story.Section, i, body); err != nil {
			return err
		}
	}
	for _, source := range e.Sources {
		body, _ := json.Marshal(source)
		if _, err = tx.ExecContext(ctx, "INSERT INTO newspaper_sources(edition_id,id,url,body) VALUES(?,?,?,?)", e.ID, source.ID, source.URL, body); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO newspaper_evidence(edition_id,source_id,excerpt) VALUES(?,?,?)", e.ID, source.ID, source.Excerpt); err != nil {
			return err
		}
	}
	for i, note := range e.Corrections {
		if _, err = tx.ExecContext(ctx, "INSERT INTO newspaper_corrections(edition_id,position,note) VALUES(?,?,?)", e.ID, i, note); err != nil {
			return err
		}
	}
	run.Status = "published"
	run.Phase = "published"
	run.Stories = len(e.Stories)
	run.Sources = len(e.Sources)
	run.UpdatedAt = e.CreatedAt
	rb, _ := json.Marshal(run)
	if _, err = tx.ExecContext(ctx, "UPDATE newspaper_runs SET status='published',body=? WHERE id=?", rb, run.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO newspaper_events(run_id,at,phase,text) VALUES(?,?,?,?)", run.ID, e.CreatedAt.Format(time.RFC3339Nano), "published", "Edition published"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Get(ctx context.Context, id string) (Edition, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT body FROM newspaper_editions WHERE id=?", id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return Edition{}, ErrNotFound
	}
	var e Edition
	if err == nil {
		err = json.Unmarshal(b, &e)
	}
	if err == nil {
		err = verifyEdition(e)
	}
	return e, err
}

func verifyEdition(e Edition) error {
	stored := e.Hash
	if err := e.Seal(); err != nil {
		return err
	}
	if stored == "" || e.Hash != stored {
		return errors.New("newspaper edition hash mismatch")
	}
	return nil
}

func (s *Store) List(ctx context.Context, limit int) ([]Edition, error) {
	if limit < 1 || limit > 100 {
		limit = 40
	}
	rows, err := s.db.QueryContext(ctx, "SELECT body FROM newspaper_editions ORDER BY local_date DESC,revision DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Edition{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var e Edition
		if err = json.Unmarshal(b, &e); err != nil {
			return nil, err
		}
		if err = verifyEdition(e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) Events(ctx context.Context, runID string, after int64) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,at,phase,text FROM newspaper_events WHERE run_id=? AND id>? ORDER BY id LIMIT 300", runID, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		var at string
		e.RunID = runID
		if err = rows.Scan(&e.ID, &at, &e.Phase, &e.Text); err != nil {
			return nil, err
		}
		e.At, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) ClaimDelivery(ctx context.Context, e Edition, channel, kind, targetHash, key string) (Delivery, bool, error) {
	if channel != "email" && channel != "telegram" || kind != "daily" && kind != "manual" && kind != "test" {
		return Delivery{}, false, errors.New("invalid delivery channel or kind")
	}
	if len(targetHash) != 64 || key == "" || len(key) > 128 {
		return Delivery{}, false, errors.New("invalid delivery target or request key")
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO newspaper_deliveries(edition_id,hash,channel,kind,target_hash,request_key,status,reason,provider_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", e.ID, e.Hash, channel, kind, targetHash, key, "sending", "", "", now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return Delivery{}, false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Delivery{}, false, err
	}
	if n == 0 {
		d := Delivery{EditionID: e.ID}
		var created, updated string
		err = s.db.QueryRowContext(ctx, "SELECT id,hash,channel,kind,target_hash,request_key,status,reason,provider_id,created_at,updated_at FROM newspaper_deliveries WHERE edition_id=? AND channel=? AND target_hash=? AND request_key=?", e.ID, channel, targetHash, key).Scan(&d.ID, &d.Hash, &d.Channel, &d.Kind, &d.TargetHash, &d.Key, &d.Status, &d.Reason, &d.ProviderID, &created, &updated)
		if err != nil {
			return Delivery{}, false, err
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		d.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		return d, false, nil
	}
	id, _ := res.LastInsertId()
	return Delivery{ID: id, EditionID: e.ID, Hash: e.Hash, Channel: channel, Kind: kind, TargetHash: targetHash, Key: key, Status: "sending", CreatedAt: now, UpdatedAt: now}, true, nil
}

func (s *Store) FinishDelivery(ctx context.Context, d Delivery, status, reason, providerID string) error {
	if status != "sent" && status != "failed" && status != "uncertain" {
		return errors.New("invalid delivery status")
	}
	if len(reason) > 300 {
		reason = reason[:300]
	}
	if len(providerID) > 200 {
		providerID = providerID[:200]
	}
	res, err := s.db.ExecContext(ctx, "UPDATE newspaper_deliveries SET status=?,reason=?,provider_id=?,updated_at=? WHERE id=? AND status='sending'", status, reason, providerID, time.Now().UTC().Format(time.RFC3339Nano), d.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}

func (s *Store) Deliveries(ctx context.Context, id string) ([]Delivery, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,hash,channel,kind,target_hash,request_key,status,reason,provider_id,created_at,updated_at FROM newspaper_deliveries WHERE edition_id=? ORDER BY id DESC", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Delivery{}
	for rows.Next() {
		var d Delivery
		var created, updated string
		d.EditionID = id
		if err = rows.Scan(&d.ID, &d.Hash, &d.Channel, &d.Kind, &d.TargetHash, &d.Key, &d.Status, &d.Reason, &d.ProviderID, &created, &updated); err != nil {
			return nil, err
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		d.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) Prune(ctx context.Context, maxEditions int) error {
	if maxEditions < 30 {
		maxEditions = 30
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM newspaper_editions WHERE id NOT IN (SELECT id FROM newspaper_editions ORDER BY local_date DESC,revision DESC LIMIT ?)", maxEditions); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM newspaper_runs WHERE status!='running' AND id NOT IN (SELECT id FROM newspaper_runs ORDER BY local_date DESC,revision DESC LIMIT ?)", maxEditions*2); err != nil {
		return err
	}
	return tx.Commit()
}
