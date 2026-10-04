package sqlconnections

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sqliteFixture(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("CREATE TABLE accounts(id INTEGER); INSERT INTO accounts VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func importFixture(t *testing.T, pool *ConnectionPool, id string) {
	t.Helper()
	if err := pool.ImportSQLite(context.Background(), id, bytes.NewReader(sqliteFixture(t))); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteImportBoundaryAndReadOnlyConnection(t *testing.T) {
	p, meta, cleanup := setupTestPool(t)
	defer cleanup()
	defer p.CloseAll()
	id, err := Create(meta, "local", "sqlite", "", 0, filepath.Join(t.TempDir(), "must-not-create.db"), "", true, true, true, true, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.GetConnection(id); err == nil || !strings.Contains(err.Error(), SQLiteImportRequired) {
		t.Fatalf("legacy path opened: %v", err)
	}
	importFixture(t, p, id)
	rec, _ := GetByID(meta, id)
	reader, err := p.openWithMode(rec, true)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err = reader.Exec("DELETE FROM accounts"); err == nil {
		t.Fatal("read connection accepted mutation")
	}
	for _, q := range []string{"SELECT '--'; DELETE FROM accounts", "ATTACH DATABASE '/tmp/escape.db' AS escape", "SELECT writefile('/tmp/escape', 'x')"} {
		if _, err = ExecuteQuery(context.Background(), p, meta, "local", q, 20, time.Second, true); err == nil {
			t.Fatalf("query accepted: %s", q)
		}
	}
	result, err := ExecuteQuery(context.Background(), p, meta, "local", "SELECT count(*) AS n FROM accounts", 20, time.Second, true)
	if err != nil || len(result.Rows) != 1 || result.Rows[0]["n"] != int64(1) {
		t.Fatalf("rows changed or unavailable: %+v %v", result, err)
	}
	if err = p.ImportSQLite(context.Background(), id, strings.NewReader("not SQLite")); err == nil {
		t.Fatal("invalid import accepted")
	}
	after, _ := GetByID(meta, id)
	if after.DatabaseName != rec.DatabaseName {
		t.Fatal("failed import replaced generation")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = p.ImportSQLite(ctx, id, bytes.NewReader(sqliteFixture(t))); err == nil {
		t.Fatal("cancelled import accepted")
	}
}
