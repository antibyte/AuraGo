package dbutil

import (
	"context"
	"database/sql"
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

func TestOpenConfiguresEveryPooledConnection(t *testing.T) {
	t.Parallel()

	path := filepath.ToSlash(filepath.Join(t.TempDir(), "pooled.db"))
	dsn := "file:" + path + "?mode=rwc" +
		"&_pragma=foreign_keys(0)" +
		"&_pragma=synchronous(OFF)" +
		"&_pragma=busy_timeout(1)" +
		"&_pragma=cache_size(1234)"
	db, err := Open(dsn, WithMaxOpenConns(4), WithSynchronous("FULL"), WithBusyTimeout(2468))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE parents (id INTEGER PRIMARY KEY);
		CREATE TABLE children (
			id INTEGER PRIMARY KEY,
			parent_id INTEGER NOT NULL REFERENCES parents(id)
		);
	`); err != nil {
		t.Fatalf("create foreign key schema: %v", err)
	}

	ctx := context.Background()
	connections := make([]*sql.Conn, 0, 4)
	for i := 0; i < 4; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("db.Conn(%d): %v", i, err)
		}
		connections = append(connections, conn)
	}
	t.Cleanup(func() {
		for _, conn := range connections {
			_ = conn.Close()
		}
	})

	for i, conn := range connections {
		assertPragmaInt(t, conn, "foreign_keys", 1, i)
		assertPragmaInt(t, conn, "synchronous", 2, i)
		assertPragmaInt(t, conn, "busy_timeout", 2468, i)
		assertPragmaInt(t, conn, "cache_size", 1234, i)

		if _, err := conn.ExecContext(ctx,
			"INSERT INTO children(id, parent_id) VALUES (?, ?)", i+1, 9999); err == nil {
			t.Fatalf("connection %d accepted an invalid foreign key row", i)
		}
	}
}

func TestOpenKeepsWALWhenCallerDSNConflictsOnPooledConnections(t *testing.T) {
	t.Parallel()

	path := filepath.ToSlash(filepath.Join(t.TempDir(), "wal-conflict.db"))
	dsn := "file:" + path + "?mode=rwc&_pragma=journal_mode(DELETE)"
	db, err := Open(dsn, WithMaxOpenConns(4))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	connections := make([]*sql.Conn, 0, 4)
	t.Cleanup(func() {
		for _, conn := range connections {
			_ = conn.Close()
		}
	})
	for i := 0; i < 4; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("db.Conn(%d): %v", i, err)
		}
		connections = append(connections, conn)
	}

	for i, conn := range connections {
		var journalMode string
		if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
			t.Fatalf("connection %d PRAGMA journal_mode: %v", i, err)
		}
		if !strings.EqualFold(journalMode, "wal") {
			t.Fatalf("connection %d journal_mode = %q, want WAL", i, journalMode)
		}
	}
}

func assertPragmaInt(t *testing.T, conn *sql.Conn, pragma string, want, connection int) {
	t.Helper()
	var got int
	if err := conn.QueryRowContext(context.Background(), "PRAGMA "+pragma).Scan(&got); err != nil {
		t.Fatalf("connection %d PRAGMA %s: %v", connection, pragma, err)
	}
	if got != want {
		t.Fatalf("connection %d PRAGMA %s = %d, want %d", connection, pragma, got, want)
	}
}
