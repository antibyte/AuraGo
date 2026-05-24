package desktop

import (
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"aurago/internal/dbutil"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	db, err := dbutil.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	schema := `
	CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		protocol TEXT NOT NULL DEFAULT 'ssh',
		ip_address TEXT,
		port INTEGER NOT NULL,
		username TEXT,
		vault_secret_id TEXT,
		credential_id TEXT,
		description TEXT,
		tags TEXT,
		mac_address TEXT
	);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		t.Fatalf("failed to create schema: %v", err)
	}
	return db, func() { db.Close() }
}
