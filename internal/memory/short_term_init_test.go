package memory

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSQLiteDBRecoversCorruptedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.db")
	if err := os.WriteFile(path, []byte("not-a-sqlite-database"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	stm, err := NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatalf("expected backup of corrupt database, stat error: %v", err)
	}
	if _, err := stm.InsertMessage("default", "user", "hello", false, false); err != nil {
		t.Fatalf("InsertMessage on recovered DB: %v", err)
	}
}
