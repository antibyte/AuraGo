package chibisensei

import (
	"path/filepath"
	"testing"
)

func TestOpenStoreCreatesSchemaV1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "chibi_sensei.db")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = OpenStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	var version int
	if err := st.db.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil || version != 1 {
		t.Fatalf("version = %d (%v), want 1", version, err)
	}
	for _, table := range []string{"dataset_rows", "batches", "suggestions", "runs"} {
		var n int
		if err := st.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n); err != nil || n != 1 {
			t.Fatalf("table %s missing (%d, %v)", table, n, err)
		}
	}
	if st.Path() != path || st.SizeBytes() <= 0 {
		t.Fatalf("Path/SizeBytes: %q %d", st.Path(), st.SizeBytes())
	}
	if _, err := st.db.Exec(`UPDATE schema_version SET version=2`); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if st2, err := OpenStore(path); err == nil {
		st2.Close()
		t.Fatal("a database with schema version 2 opened")
	}
}
