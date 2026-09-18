package detective

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
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

type Runner interface {
	Run(context.Context, *Session) error
}
type Options struct {
	Path     string
	Profiles map[string]Profile
	Now      func() time.Time
	PDF      func(context.Context, Report) ([]byte, error)
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
	if err = db.QueryRow("PRAGMA user_version").Scan(&schemaVersion); err != nil || schemaVersion > 1 {
		db.Close()
		return nil, fmt.Errorf("unsupported Detective database version %d: %v", schemaVersion, err)
	}
	_, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL;
CREATE TABLE IF NOT EXISTS detective_cases(id TEXT PRIMARY KEY, body BLOB NOT NULL, status TEXT NOT NULL, updated TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS detective_continuations(case_id TEXT PRIMARY KEY REFERENCES detective_cases(id), body BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS detective_keys(case_id TEXT NOT NULL, key TEXT NOT NULL, run_id TEXT NOT NULL, PRIMARY KEY(case_id,key));
CREATE TABLE IF NOT EXISTS detective_events(id INTEGER PRIMARY KEY AUTOINCREMENT, case_id TEXT NOT NULL, kind TEXT NOT NULL, body TEXT NOT NULL, at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS detective_event_case ON detective_events(case_id,id);
CREATE TABLE IF NOT EXISTS detective_artifacts(case_id TEXT NOT NULL, revision INTEGER NOT NULL, format TEXT NOT NULL, body BLOB NOT NULL, mime TEXT NOT NULL, filename TEXT NOT NULL, sha256 TEXT NOT NULL, PRIMARY KEY(case_id,revision,format));
PRAGMA user_version=1;`)
	if err != nil {
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
	s := &Service{db: db, profiles: p, now: now, ctx: ctx, cancel: cancel, kick: make(chan struct{}, 1), exports: make(chan struct{}, 1), pdf: opts.PDF}
	cases, err := s.List()
	if err != nil {
		db.Close()
		cancel()
		return nil, err
	}
	for _, c := range cases {
		if c.Run.Status == "running" || c.Run.Status == "queued" {
			c.Run.Status = "interrupted"
			c.Run.Reason = "server_restart"
			if err := s.saveLocked(&c); err != nil {
				db.Close()
				cancel()
				return nil, err
			}
		}
	}
	s.wg.Add(1)
	go s.worker()
	return s, nil
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
	err := s.db.QueryRow("SELECT body FROM detective_cases WHERE id=?", key).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return Case{}, ErrNotFound
	}
	var c Case
	if err == nil {
		err = json.Unmarshal(b, &c)
	}
	return c, err
}
func (s *Service) saveLocked(c *Case) error {
	c.UpdatedAt = s.now().UTC()
	c.Run.UpdatedAt = c.UpdatedAt
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO detective_cases(id,body,status,updated) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body,status=excluded.status,updated=excluded.updated", c.ID, b, c.Run.Status, c.UpdatedAt.Format(time.RFC3339Nano))
	return err
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

func (s *Service) list(summary bool) ([]Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := "SELECT body FROM detective_cases ORDER BY updated DESC LIMIT 100"
	if summary {
		query = "SELECT json_remove(CAST(body AS TEXT),'$.sources','$.findings','$.reports','$.answers','$.plan') FROM detective_cases ORDER BY updated DESC LIMIT 100"
	}
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Case{}
	for rows.Next() {
		var b []byte
		var c Case
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Service) Create(req Request) (Case, error) {
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
	for _, q := range []string{"DELETE FROM detective_artifacts WHERE case_id=?", "DELETE FROM detective_events WHERE case_id=?", "DELETE FROM detective_continuations WHERE case_id=?", "DELETE FROM detective_keys WHERE case_id=?", "DELETE FROM detective_cases WHERE id=?"} {
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
