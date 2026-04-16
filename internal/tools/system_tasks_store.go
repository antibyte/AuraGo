package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"aurago/internal/dbutil"

	_ "modernc.org/sqlite"
)

const (
	systemTaskStoreFile          = "system_tasks.db"
	systemTaskNamespaceCron      = "cron_jobs"
	systemTaskNamespaceBackground = "background_tasks"
)

type systemTaskStore struct {
	path string
}

func newSystemTaskStore(dataDir string) (*systemTaskStore, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create system task store dir: %w", err)
	}
	store := &systemTaskStore{path: filepath.Join(dataDir, systemTaskStoreFile)}
	if err := store.init(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *systemTaskStore) init() error {
	if s == nil || s.path == "" {
		return nil
	}
	db, err := dbutil.Open(s.path)
	if err != nil {
		return fmt.Errorf("open system task store: %w", err)
	}
	defer db.Close()
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS system_tasks (
			namespace TEXT PRIMARY KEY,
			payload_json TEXT NOT NULL DEFAULT '[]',
			updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("init system task store: %w", err)
	}
	return nil
}

func (s *systemTaskStore) load(namespace string, dest interface{}) (bool, error) {
	if s == nil || s.path == "" {
		return false, nil
	}
	db, err := dbutil.Open(s.path)
	if err != nil {
		return false, fmt.Errorf("open system task store: %w", err)
	}
	defer db.Close()
	var payload string
	err = db.QueryRow(`SELECT payload_json FROM system_tasks WHERE namespace = ?`, namespace).Scan(&payload)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load %s from system task store: %w", namespace, err)
	}
	if err := json.Unmarshal([]byte(payload), dest); err != nil {
		return false, fmt.Errorf("parse %s from system task store: %w", namespace, err)
	}
	return true, nil
}

func (s *systemTaskStore) save(namespace string, value interface{}) error {
	if s == nil || s.path == "" {
		return nil
	}
	db, err := dbutil.Open(s.path)
	if err != nil {
		return fmt.Errorf("open system task store: %w", err)
	}
	defer db.Close()
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %s for system task store: %w", namespace, err)
	}
	_, err = db.Exec(
		`INSERT INTO system_tasks (namespace, payload_json, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(namespace) DO UPDATE SET payload_json = excluded.payload_json, updated_at = excluded.updated_at`,
		namespace,
		string(data),
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save %s to system task store: %w", namespace, err)
	}
	return nil
}

func (s *systemTaskStore) close() error {
	return nil
}