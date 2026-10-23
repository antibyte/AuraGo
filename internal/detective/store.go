package detective

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

// SchemaVersion is the newest Detective database layout. Version 3 moved evidence into
// archive tables; version 4 adds the bounded case summary the live view reads.
const SchemaVersion = 4

type Runner interface {
	Run(context.Context, *Session) error
}
type Options struct {
	Path     string
	Profiles map[string]Profile
	Now      func() time.Time
	PDF      func(context.Context, Report) ([]byte, error)
	Allowed  func() bool
	Logger   *slog.Logger
}

type Service struct {
	mu       sync.Mutex
	db       *sql.DB
	profiles map[string]Profile
	now      func() time.Time
	runner   Runner
	ctx      context.Context
	cancel   context.CancelFunc
	kick     chan struct{}
	exports  chan struct{}
	wg       sync.WaitGroup
	active   *Session
	closed   bool
	pdf      func(context.Context, Report) ([]byte, error)
	allowed  func() bool
	logger   *slog.Logger
	failures map[string]int
}

// log returns a usable logger; the service must not require one to run.
func (s *Service) log() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

func New(opts Options) (*Service, error) {
	if err := os.MkdirAll(filepath.Dir(opts.Path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", opts.Path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	var schemaVersion int
	if err = db.QueryRow("PRAGMA user_version").Scan(&schemaVersion); err != nil || schemaVersion > SchemaVersion {
		db.Close()
		return nil, fmt.Errorf("unsupported Detective database version %d: %v", schemaVersion, err)
	}
	if err = backupBeforeMigration(db, opts.Path, schemaVersion, SchemaVersion); err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL;`); err != nil {
		db.Close()
		return nil, err
	}
	if err = migrateDatabase(db, schemaVersion); err != nil {
		db.Close()
		return nil, err
	}
	p := Profiles()
	for k, v := range opts.Profiles {
		if _, ok := p[k]; !ok {
			db.Close()
			return nil, fmt.Errorf("unknown profile %q", k)
		}
		if err := v.Validate(); err != nil {
			db.Close()
			return nil, err
		}
		p[k] = v
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{db: db, profiles: p, now: now, ctx: ctx, cancel: cancel, kick: make(chan struct{}, 1), exports: make(chan struct{}, 1), pdf: opts.PDF, allowed: opts.Allowed, logger: opts.Logger, failures: map[string]int{}}
	// Only the restart pass needs the bodies; the status column answers which cases
	// were active, so a corrupt or oversized unrelated case cannot block starting.
	active, err := s.activeIDs()
	if err != nil {
		db.Close()
		cancel()
		return nil, err
	}
	for _, key := range active {
		c, err := s.getLocked(key)
		if err != nil {
			db.Close()
			cancel()
			return nil, err
		}
		c.Run.Status = "interrupted"
		c.Run.Reason = "server_restart"
		if err = s.saveLocked(&c); err != nil {
			db.Close()
			cancel()
			return nil, err
		}
	}
	s.wg.Add(1)
	go s.worker()
	return s, nil
}

// activeIDs lists the cases that were running or queued when the process stopped.
func (s *Service) activeIDs() ([]string, error) {
	rows, err := s.db.Query("SELECT id FROM detective_cases WHERE status IN ('running','queued') ORDER BY updated")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			return nil, err
		}
		ids = append(ids, key)
	}
	return ids, rows.Err()
}

func migrateDatabase(db *sql.DB, schemaVersion int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS detective_cases(id TEXT PRIMARY KEY, status TEXT NOT NULL, updated TEXT NOT NULL, active_ms INTEGER NOT NULL DEFAULT 0, body BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS detective_continuations(case_id TEXT PRIMARY KEY REFERENCES detective_cases(id), body BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS detective_keys(case_id TEXT NOT NULL, key TEXT NOT NULL, run_id TEXT NOT NULL, PRIMARY KEY(case_id,key));
CREATE TABLE IF NOT EXISTS detective_events(id INTEGER PRIMARY KEY AUTOINCREMENT, case_id TEXT NOT NULL, kind TEXT NOT NULL, body TEXT NOT NULL, at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS detective_event_case ON detective_events(case_id,id);
CREATE TABLE IF NOT EXISTS detective_artifacts(case_id TEXT NOT NULL, revision INTEGER NOT NULL, format TEXT NOT NULL, body BLOB NOT NULL, mime TEXT NOT NULL, filename TEXT NOT NULL, sha256 TEXT NOT NULL, PRIMARY KEY(case_id,revision,format));
CREATE TABLE IF NOT EXISTS detective_summaries(case_id TEXT PRIMARY KEY REFERENCES detective_cases(id), body TEXT NOT NULL);`)
	if err != nil {
		return err
	}
	// Columns used by saveCaseTx must exist before any migration writes through it, so
	// the whole chain runs on one shape of the table.
	if err = ensureCaseColumns(tx, schemaVersion); err != nil {
		return err
	}
	if err = migrateArchive(tx, schemaVersion); err != nil {
		return err
	}
	if err = migrateSummary(tx, schemaVersion); err != nil {
		return err
	}
	return tx.Commit()
}

// ensureCaseColumns adds the case columns the version-2 backfill needs. It is
// idempotent and runs inside the migration transaction, so a failed open leaves the
// old schema.
func ensureCaseColumns(tx *sql.Tx, schemaVersion int) error {
	var present int
	if err := tx.QueryRow(`SELECT count(*) FROM pragma_table_info('detective_cases') WHERE name='active_ms'`).Scan(&present); err != nil {
		return err
	}
	if present == 0 {
		if _, err := tx.Exec(`ALTER TABLE detective_cases ADD COLUMN active_ms INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	if schemaVersion < 2 {
		// The version-2 backfill of the active time; the column exists by now.
		if _, err := tx.Exec(`UPDATE detective_cases SET active_ms = COALESCE(CAST(json_extract(body,'$.run.usage.active_ms') AS INTEGER), 0)`); err != nil {
			return err
		}
	}
	return nil
}

func id(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}
func bounded(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
func activeStatus(s string) bool { return s == "running" || s == "queued" }

func (s *Service) SetRunner(r Runner) { s.mu.Lock(); s.runner = r; s.mu.Unlock(); s.wake() }
func (s *Service) wake() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}
func (s *Service) Close() error {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
	return s.db.Close()
}
func (s *Service) Profiles() map[string]Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]Profile{}
	for k, v := range s.profiles {
		out[k] = v
	}
	return out
}

func (s *Service) getLocked(key string) (Case, error) {
	var b []byte
	var active int64
	err := s.db.QueryRow("SELECT body, active_ms FROM detective_cases WHERE id=?", key).Scan(&b, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return Case{}, ErrNotFound
	}
	var c Case
	if err == nil {
		err = json.Unmarshal(b, &c)
	}
	if active > c.Run.Usage.ActiveMS {
		c.Run.Usage.ActiveMS = active
	}
	return c, err
}
func (s *Service) saveLocked(c *Case) error {
	c.UpdatedAt = s.now().UTC()
	c.Run.UpdatedAt = c.UpdatedAt
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = saveCaseTx(tx, c); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	c.pendingSources, c.pendingFindings, c.pendingReports = nil, nil, nil
	return nil
}
func (s *Service) Get(key string) (Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getLocked(key)
}
func (s *Service) List() ([]Case, error) {
	return s.list(false)
}

// ListSummaries avoids loading report revisions and source bodies on every poll.
func (s *Service) ListSummaries() ([]Case, error) { return s.list(true) }

type LiveView struct {
	ID             string     `json:"id"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Status         string     `json:"status"`
	Phase          string     `json:"phase"`
	Reason         string     `json:"reason"`
	Effort         string     `json:"effort"`
	Topic          string     `json:"topic"`
	Usage          Usage      `json:"usage"`
	Profile        Profile    `json:"profile"`
	Sources        int        `json:"sources"`
	Findings       int        `json:"findings"`
	LatestRevision int        `json:"latest_revision"`
	LatestSourceID string     `json:"latest_source_id"`
	AutoUpdate     AutoUpdate `json:"auto_update"`
}

func (s *Service) Exists(key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var one int
	err := s.db.QueryRow("SELECT 1 FROM detective_cases WHERE id=?", key).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// Live reports the bounded status of one case from its summary row. It never parses
// the evidence body, so the cost of a poll does not grow with the case.
func (s *Service) Live(key string) (LiveView, error) {
	allowed := s.allowed == nil || s.allowed()
	s.mu.Lock()
	defer s.mu.Unlock()
	var id, updated, stored string
	var active int64
	err := s.db.QueryRow(`SELECT c.id, c.updated, c.active_ms, COALESCE(s.body,'')
		FROM detective_cases c LEFT JOIN detective_summaries s ON s.case_id=c.id
		WHERE c.id=?`, key).Scan(&id, &updated, &active, &stored)
	if errors.Is(err, sql.ErrNoRows) {
		return LiveView{}, ErrNotFound
	}
	if err != nil {
		return LiveView{}, err
	}
	sum, err := s.decodeSummary(key, stored)
	if err != nil {
		return LiveView{}, err
	}
	v := liveView(id, active, sum)
	v.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return LiveView{}, err
	}
	if v.AutoUpdate.Enabled && !allowed {
		v.AutoUpdate.Outcome = "permission_blocked"
	}
	return v, nil
}

// decodeSummary reads a stored summary. An empty value means a row written before the
// column existed (or a half-migrated file); it is projected from the body once.
func (s *Service) decodeSummary(key, stored string) (CaseSummary, error) {
	if stored != "" {
		var sum CaseSummary
		if err := json.Unmarshal([]byte(stored), &sum); err != nil {
			return CaseSummary{}, err
		}
		return sum, nil
	}
	var body []byte
	if err := s.db.QueryRow("SELECT body FROM detective_cases WHERE id=?", key).Scan(&body); err != nil {
		return CaseSummary{}, err
	}
	var c Case
	if err := json.Unmarshal(body, &c); err != nil {
		return CaseSummary{}, err
	}
	return caseSummary(&c), nil
}

func (s *Service) list(summary bool) ([]Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if summary {
		return s.listSummariesLocked()
	}
	rows, err := s.db.Query("SELECT body, active_ms FROM detective_cases ORDER BY updated DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Case{}
	for rows.Next() {
		var b []byte
		var active int64
		var c Case
		if err = rows.Scan(&b, &active); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
		if active > c.Run.Usage.ActiveMS {
			c.Run.Usage.ActiveMS = active
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// listSummariesLocked serves the case list from the summary column. Rows without a
// summary (a file that has not been migrated yet) fall back to their body once; the
// scan finishes before any fallback query runs, because the pool has one connection.
func (s *Service) listSummariesLocked() ([]Case, error) {
	type entry struct {
		key    string
		stored string
		active int64
	}
	rows, err := s.db.Query(`SELECT c.id, c.active_ms, COALESCE(s.body,'')
		FROM detective_cases c LEFT JOIN detective_summaries s ON s.case_id=c.id
		ORDER BY c.updated DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err = rows.Scan(&e.key, &e.active, &e.stored); err != nil {
			rows.Close()
			return nil, err
		}
		entries = append(entries, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []Case{}
	for _, e := range entries {
		if e.stored == "" {
			c, err := s.getLocked(e.key)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
			continue
		}
		sum, err := s.decodeSummary(e.key, e.stored)
		if err != nil {
			return nil, err
		}
		out = append(out, summaryCase(e.key, e.active, sum))
	}
	return out, nil
}

func (s *Service) Create(req Request) (Case, error) {
	req.AutoUpdate = defaultAutoUpdate(req.AutoUpdate)
	if err := s.validateAutoUpdate(req.AutoUpdate); err != nil {
		return Case{}, err
	}
	if strings.TrimSpace(req.Topic) == "" || len(req.Topic) > 8000 || len(req.Scope) > 8000 || len(req.SourceURLs) > 20 || len(req.PrivateSources) > 20 {
		return Case{}, errors.New("invalid research request")
	}
	if req.Effort == "" {
		req.Effort = "normal"
	}
	if _, ok := s.profiles[req.Effort]; !ok {
		return Case{}, errors.New("unknown effort")
	}
	for _, u := range req.SourceURLs {
		if _, err := canonicalURL(u); err != nil {
			return Case{}, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Case{}, ErrClosed
	}
	var count int
	_ = s.db.QueryRow("SELECT count(*) FROM detective_cases").Scan(&count)
	if count >= 100 {
		return Case{}, errors.New("research case limit reached")
	}
	c := Case{ID: id("case_"), Request: req, Run: Run{Status: "draft"}, CreatedAt: s.now().UTC(), Sources: []Source{}, Findings: []Finding{}, Reports: []Report{}}
	c.AutoUpdate.AutoUpdateSettings = req.AutoUpdate
	err := s.saveLocked(&c)
	return c, err
}

func (s *Service) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.getLocked(key)
	if err != nil {
		return err
	}
	if activeStatus(c.Run.Status) || (s.active != nil && s.active.CaseID == key) {
		return ErrConflict
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{"DELETE FROM detective_artifacts WHERE case_id=?", "DELETE FROM detective_sources WHERE case_id=?", "DELETE FROM detective_findings WHERE case_id=?", "DELETE FROM detective_reports WHERE case_id=?", "DELETE FROM detective_events WHERE case_id=?", "DELETE FROM detective_continuations WHERE case_id=?", "DELETE FROM detective_keys WHERE case_id=?", "DELETE FROM detective_summaries WHERE case_id=?", "DELETE FROM detective_cases WHERE id=?"} {
		if _, err = tx.Exec(q, key); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) eventLocked(c *Case, kind, text string) {
	_, _ = s.db.Exec("INSERT INTO detective_events(case_id,kind,body,at) VALUES(?,?,?,?)", c.ID, kind, bounded(text, 600), s.now().UTC().Format(time.RFC3339Nano))
	_, _ = s.db.Exec("DELETE FROM detective_events WHERE case_id=? AND id NOT IN (SELECT id FROM detective_events WHERE case_id=? ORDER BY id DESC LIMIT 500)", c.ID, c.ID)
}
func (s *Service) Events(key string, after int64) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query("SELECT id,kind,body,at FROM detective_events WHERE case_id=? AND id>? ORDER BY id LIMIT 100", key, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		e := Event{CaseID: key}
		var at string
		if err = rows.Scan(&e.ID, &e.Kind, &e.Text, &at); err != nil {
			return nil, err
		}
		e.At, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Service) SaveContinuation(key string, v Continuation) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(b) > 4<<20 {
		return errors.New("research continuation exceeds 4 MiB")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err = s.getLocked(key); err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO detective_continuations(case_id,body) VALUES(?,?) ON CONFLICT(case_id) DO UPDATE SET body=excluded.body", key, b)
	return err
}
func (s *Service) Continuation(key string) (Continuation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b []byte
	var v Continuation
	err := s.db.QueryRow("SELECT body FROM detective_continuations WHERE case_id=?", key).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return v, nil
	}
	if err == nil {
		err = json.Unmarshal(b, &v)
	}
	return v, err
}
