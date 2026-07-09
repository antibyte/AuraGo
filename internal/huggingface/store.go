package huggingface

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/dbutil"

	_ "modernc.org/sqlite"
)

type JobRecord struct {
	ID           int64  `json:"id"`
	HFJobID      string `json:"hf_job_id"`
	Operation    string `json:"operation"`
	Hardware     string `json:"hardware,omitempty"`
	Status       string `json:"status,omitempty"`
	RequestJSON  string `json:"request_json,omitempty"`
	ResponseJSON string `json:"response_json,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	LastError    string `json:"last_error,omitempty"`
}

type JobStore struct {
	db *sql.DB
}

func OpenJobStore(dataDir string) (*JobStore, error) {
	if strings.TrimSpace(dataDir) == "" {
		dataDir = "data"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create huggingface data dir: %w", err)
	}
	db, err := dbutil.Open(filepath.Join(dataDir, "huggingface.db"))
	if err != nil {
		return nil, err
	}
	store := &JobStore{db: db}
	if err := store.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *JobStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *JobStore) RecordJob(ctx context.Context, rec JobRecord) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("huggingface job store is not initialized")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if strings.TrimSpace(rec.CreatedAt) == "" {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
INSERT INTO huggingface_jobs (hf_job_id, operation, hardware, status, request_json, response_json, created_at, updated_at, last_error)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(hf_job_id) DO UPDATE SET
  operation=excluded.operation,
  hardware=excluded.hardware,
  status=excluded.status,
  request_json=excluded.request_json,
  response_json=excluded.response_json,
  updated_at=excluded.updated_at,
  last_error=excluded.last_error
`, rec.HFJobID, rec.Operation, rec.Hardware, rec.Status, rec.RequestJSON, rec.ResponseJSON, rec.CreatedAt, rec.UpdatedAt, rec.LastError)
	return err
}

func (s *JobStore) ListJobs(ctx context.Context, limit int) ([]JobRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("huggingface job store is not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, hf_job_id, operation, hardware, status, request_json, response_json, created_at, updated_at, last_error
FROM huggingface_jobs
ORDER BY updated_at DESC, id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JobRecord
	for rows.Next() {
		var rec JobRecord
		if err := rows.Scan(&rec.ID, &rec.HFJobID, &rec.Operation, &rec.Hardware, &rec.Status, &rec.RequestJSON, &rec.ResponseJSON, &rec.CreatedAt, &rec.UpdatedAt, &rec.LastError); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func EncodeLedgerPayload(v interface{}) string {
	if v == nil {
		return ""
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}

func ExtractJobID(payload map[string]interface{}) string {
	for _, key := range []string{"id", "job_id", "name"} {
		if v, ok := payload[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (s *JobStore) init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS huggingface_jobs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  hf_job_id TEXT NOT NULL UNIQUE,
  operation TEXT NOT NULL,
  hardware TEXT,
  status TEXT,
  request_json TEXT,
  response_json TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  last_error TEXT
);
CREATE INDEX IF NOT EXISTS idx_huggingface_jobs_updated_at ON huggingface_jobs(updated_at DESC);
`)
	return err
}
