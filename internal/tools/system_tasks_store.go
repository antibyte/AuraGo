package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"aurago/internal/dbutil"

	_ "modernc.org/sqlite"
)

const (
	systemTaskStoreFile           = "system_tasks.db"
	systemTaskNamespaceCron       = "cron_jobs"
	systemTaskNamespaceBackground = "background_tasks"
)

type systemTaskStore struct {
	path string
	db   *sql.DB
	mu   sync.Mutex
}

var (
	systemTaskStorePoolMu sync.Mutex
	systemTaskStorePool   = make(map[string]*systemTaskStoreRef)
)

type systemTaskStoreRef struct {
	store *systemTaskStore
	refs  int
}

func newSystemTaskStore(dataDir string) (*systemTaskStore, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create system task store dir: %w", err)
	}
	path := filepath.Clean(filepath.Join(dataDir, systemTaskStoreFile))

	systemTaskStorePoolMu.Lock()
	defer systemTaskStorePoolMu.Unlock()

	if ref, ok := systemTaskStorePool[path]; ok {
		ref.refs++
		return ref.store, nil
	}

	store := &systemTaskStore{path: path}
	if err := store.init(); err != nil {
		return nil, err
	}
	systemTaskStorePool[path] = &systemTaskStoreRef{store: store, refs: 1}
	return store, nil
}

func (s *systemTaskStore) init() error {
	if s == nil || s.path == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return nil
	}
	db, err := dbutil.Open(s.path)
	if err != nil {
		return fmt.Errorf("open system task store: %w", err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS system_tasks (
			namespace TEXT PRIMARY KEY,
			payload_json TEXT NOT NULL DEFAULT '[]',
			updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("init system task store: %w", err)
	}
	s.db = db
	return nil
}

func (s *systemTaskStore) load(namespace string, dest interface{}) (bool, error) {
	if s == nil || s.path == "" {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return false, fmt.Errorf("system task store is not open")
	}
	var payload string
	err := s.db.QueryRow(`SELECT payload_json FROM system_tasks WHERE namespace = ?`, namespace).Scan(&payload)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return fmt.Errorf("system task store is not open")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %s for system task store: %w", namespace, err)
	}
	_, err = s.db.Exec(
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

func (s *systemTaskStore) release() error {
	if s == nil {
		return nil
	}

	systemTaskStorePoolMu.Lock()
	defer systemTaskStorePoolMu.Unlock()

	ref, ok := systemTaskStorePool[s.path]
	if !ok {
		return s.closeLocked()
	}
	ref.refs--
	if ref.refs > 0 {
		return nil
	}
	delete(systemTaskStorePool, s.path)
	return ref.store.closeLocked()
}

func (s *systemTaskStore) close() error {
	return s.release()
}

func (s *systemTaskStore) closeLocked() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// closeAllSystemTaskStores closes any pooled stores still held open.
// Used by tests to release Windows file locks when a test forgot Close().
func closeAllSystemTaskStores() {
	systemTaskStorePoolMu.Lock()
	defer systemTaskStorePoolMu.Unlock()
	for path, ref := range systemTaskStorePool {
		_ = ref.store.closeLocked()
		delete(systemTaskStorePool, path)
		ref.refs = 0
	}
}