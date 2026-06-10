package dbutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenRejectsNonSQLiteFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "bad.db")
	if err := os.WriteFile(path, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Open(path)
	if err == nil {
		t.Fatal("expected Open to fail on non-SQLite file")
	}
	if !strings.Contains(err.Error(), "integrity check") && !strings.Contains(err.Error(), "PRAGMA") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenSucceedsOnFreshDatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fresh.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := HealthCheck(db); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
}