package memory

import (
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
)

func seedLegacyCoreMemory(t *testing.T, dbPath string, facts ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE core_memory (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		fact TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create core_memory: %v", err)
	}
	for _, fact := range facts {
		if _, err := db.Exec(`INSERT INTO core_memory (fact) VALUES (?)`, fact); err != nil {
			t.Fatalf("insert %q: %v", fact, err)
		}
	}
}

func TestCoreMemoryDedupeBacksUpOnceAndWritesMarker(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stm.db")
	seedLegacyCoreMemory(t, dbPath, "WLAN SSID is HomeNet", "wlan ssid is   homenet", "Other fact")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	stm, err := NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	backups, err := filepath.Glob(dbPath + ".core-memory-normalized-v1-*.bak")
	if err != nil {
		_ = stm.Close()
		t.Fatal(err)
	}
	if len(backups) != 1 {
		_ = stm.Close()
		t.Fatalf("core memory backups = %v, want exactly one before duplicates are deleted", backups)
	}
	var marker string
	if err := stm.db.QueryRow(`SELECT value FROM memory_schema_meta WHERE key = 'core_memory.normalized_fact'`).Scan(&marker); err != nil || marker != "1" {
		_ = stm.Close()
		t.Fatalf("normalization marker = %q, err = %v, want 1", marker, err)
	}
	facts, err := stm.GetCoreMemoryFacts()
	if err != nil || len(facts) != 2 {
		_ = stm.Close()
		t.Fatalf("facts = %+v, err = %v, want 2 after deduplication", facts, err)
	}
	if err := stm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	backupDB, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	var backedUp int
	err = backupDB.QueryRow(`SELECT COUNT(*) FROM core_memory`).Scan(&backedUp)
	_ = backupDB.Close()
	if err != nil || backedUp != 3 {
		t.Fatalf("backup rows = %d, err = %v, want all 3 original facts", backedUp, err)
	}

	again, err := NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer again.Close()
	backups, err = filepath.Glob(dbPath + ".core-memory-normalized-v1-*.bak")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("backups after restart = %v, want no second backup", backups)
	}
}

func TestCoreMemoryNormalizationWithoutDuplicatesCreatesNoBackup(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stm.db")
	seedLegacyCoreMemory(t, dbPath, "First fact", "Second fact")
	stm, err := NewSQLiteMemory(dbPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	backups, err := filepath.Glob(dbPath + ".core-memory-normalized-v1-*.bak")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Fatalf("backups = %v, want none without duplicates", backups)
	}
	var marker string
	if err := stm.db.QueryRow(`SELECT value FROM memory_schema_meta WHERE key = 'core_memory.normalized_fact'`).Scan(&marker); err != nil || marker != "1" {
		t.Fatalf("normalization marker = %q, err = %v, want 1", marker, err)
	}
}
