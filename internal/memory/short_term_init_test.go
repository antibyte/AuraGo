package memory

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSQLiteDBPreservesNonSQLiteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.db")
	if err := os.WriteFile(path, []byte("not-a-sqlite-database"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	stm, err := NewSQLiteMemory(path, logger)
	if err == nil {
		stm.Close()
		t.Fatal("non-SQLite configuration mistake was automatically replaced")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "not-a-sqlite-database" {
		t.Fatalf("original file changed: %v", err)
	}
	if dirs, _ := filepath.Glob(path + ".corrupt-*"); len(dirs) != 0 {
		t.Fatalf("unproven corruption was quarantined: %v", dirs)
	}
}
