package kaggle

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Job struct {
	ID        string          `json:"id"`
	Account   string          `json:"account"`
	Domain    string          `json:"domain"`
	Operation string          `json:"operation"`
	Status    string          `json:"status"`
	Progress  int64           `json:"progress"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	Created   string          `json:"created"`
	Updated   string          `json:"updated"`
	Request   json.RawMessage `json:"-"`
	Hash      string          `json:"-"`
}

type Event struct {
	ID   int64           `json:"id"`
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}
type Store struct{ db *sql.DB }

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func validID(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
CREATE TABLE IF NOT EXISTS schema_version(version INTEGER NOT NULL);
INSERT INTO schema_version SELECT 1 WHERE NOT EXISTS(SELECT 1 FROM schema_version);
CREATE TABLE IF NOT EXISTS jobs(id TEXT PRIMARY KEY,account TEXT NOT NULL,domain TEXT NOT NULL,operation TEXT NOT NULL,status TEXT NOT NULL,progress INTEGER NOT NULL DEFAULT 0,result BLOB,error TEXT NOT NULL DEFAULT '',created TEXT NOT NULL,updated TEXT NOT NULL,request BLOB NOT NULL,hash TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS events(id INTEGER PRIMARY KEY AUTOINCREMENT,kind TEXT NOT NULL,data BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS projects(id TEXT PRIMARY KEY,title TEXT NOT NULL,created TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS uploads(id TEXT PRIMARY KEY,account TEXT NOT NULL,token TEXT NOT NULL);
`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize Kaggle database: %w", err)
	}
	var version int
	if err = db.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil || version != 1 {
		db.Close()
		return nil, errors.New("unsupported Kaggle database schema")
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }

const jobColumns = `id,account,domain,operation,status,progress,result,error,created,updated,request,hash`

type scanner interface{ Scan(...any) error }

func scanJob(row scanner) (Job, error) {
	var j Job
	var result, request []byte
	err := row.Scan(&j.ID, &j.Account, &j.Domain, &j.Operation, &j.Status, &j.Progress, &result, &j.Error, &j.Created, &j.Updated, &request, &j.Hash)
	j.Result, j.Request = result, request
	return j, err
}
func (s *Store) Get(ctx context.Context, id string) (Job, error) {
	return scanJob(s.db.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM jobs WHERE id=?`, id))
}
func (s *Store) List(ctx context.Context) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+jobColumns+` FROM jobs WHERE status IN ('queued','running','unknown','remote_running','cancel_requested') OR id IN (SELECT id FROM jobs ORDER BY created DESC LIMIT 200) ORDER BY created DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		j, e := scanJob(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
func (s *Store) admit(ctx context.Context, j Job) (Job, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO jobs(id,account,domain,operation,status,created,updated,request,hash) VALUES(?,?,?,?,?,?,?,?,?)`, j.ID, j.Account, j.Domain, j.Operation, "queued", now, now, j.Request, j.Hash)
	if err != nil {
		return Job{}, false, err
	}
	n, _ := res.RowsAffected()
	got, err := s.Get(ctx, j.ID)
	if err != nil {
		return got, false, err
	}
	if got.Account != j.Account || got.Hash != j.Hash {
		return Job{}, false, errors.New("request ID already belongs to a different account or request")
	}
	return got, n == 1, nil
}
func (s *Store) update(ctx context.Context, id, status string, progress int64, result json.RawMessage, message string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE jobs SET status=?,progress=?,result=?,error=?,updated=? WHERE id=?`, status, progress, result, message, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err == nil {
		if j, e := s.Get(ctx, id); e == nil {
			_, err = s.Emit(ctx, "job", j)
		}
	}
	return err
}
func (s *Store) Emit(ctx context.Context, kind string, data any) (int64, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}
	if len(b) > 2<<20 {
		return 0, errors.New("event is too large")
	}
	r, err := s.db.ExecContext(ctx, `INSERT INTO events(kind,data) VALUES(?,?)`, kind, b)
	if err != nil {
		return 0, err
	}
	id, err := r.LastInsertId()
	// Bound both count and bytes: notebook output must not grow the ledger indefinitely.
	_, _ = s.db.ExecContext(ctx, `DELETE FROM events WHERE id < ? OR id IN (SELECT id FROM (SELECT id,SUM(length(data)) OVER (ORDER BY id DESC) AS bytes FROM events) WHERE bytes > 67108864)`, id-10000)
	return id, err
}
func (s *Store) Events(ctx context.Context, after int64) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,kind,data FROM events WHERE id>? ORDER BY id LIMIT 100`, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		if err = rows.Scan(&e.ID, &e.Kind, &e.Data); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type Project struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Created string `json:"created"`
}

func (s *Store) Projects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,title,created FROM projects ORDER BY created DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		var p Project
		if err = rows.Scan(&p.ID, &p.Title, &p.Created); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) addProject(ctx context.Context, title string) (Project, error) {
	p := Project{newID(), title, time.Now().UTC().Format(time.RFC3339Nano)}
	_, err := s.db.ExecContext(ctx, `INSERT INTO projects VALUES(?,?,?)`, p.ID, p.Title, p.Created)
	return p, err
}
