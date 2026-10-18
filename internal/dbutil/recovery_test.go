package dbutil

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

type recoveryCodeError int

func (e recoveryCodeError) Error() string { return "SQLite test error" }
func (e recoveryCodeError) Code() int     { return int(e) }

func TestRecoveryClassifiesSQLiteCodes(t *testing.T) {
	for _, code := range []int{3, 5, 6, 9, 10, 13, 14, 26, 10 | (1 << 8)} {
		if isCorruption(recoveryCodeError(code)) {
			t.Fatalf("noncorruption code %d permits recovery", code)
		}
	}
	for _, code := range []int{11, 11 | (1 << 8)} {
		if !isCorruption(recoveryCodeError(code)) {
			t.Fatalf("corruption code %d rejected", code)
		}
	}
}

func TestRecoveryOnlyQuarantinesProvenCorruption(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, kind := range []string{"invalid_dsn", "missing_parent", "directory"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "memory.db")
			switch kind {
			case "invalid_dsn":
				path = "file:" + filepath.ToSlash(path) + "?mode=invalid"
			case "missing_parent":
				path = filepath.Join(root, "absent", "memory.db")
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if db, err := Open(path, WithCorruptionRecovery(logger)); err == nil {
				db.Close()
				t.Fatal("expected failure")
			}
			entries, err := filepath.Glob(filepath.Join(root, "*.corrupt-*"))
			if err != nil || len(entries) != 0 {
				t.Fatalf("unexpected quarantine %v: %v", entries, err)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "memory.db")
	original := []byte("not SQLite")
	for i := 0; i < 2; i++ {
		if err := os.WriteFile(path, original, 0600); err != nil {
			t.Fatal(err)
		}
		if db, err := Open(path, WithCorruptionRecovery(logger)); err == nil {
			db.Close()
			t.Fatal("non-SQLite file was replaced")
		}
		if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
			t.Fatal("non-SQLite file changed")
		}
		if err := recoverCorruptDB(path, logger); err != nil {
			t.Fatal(err)
		}
		db, err := Open(path, WithCorruptionRecovery(logger))
		if err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
	dirs, _ := filepath.Glob(path + ".corrupt-*")
	if len(dirs) != 2 {
		t.Fatalf("quarantines = %v", dirs)
	}
	for _, dir := range dirs {
		got, err := os.ReadFile(filepath.Join(dir, "memory.db"))
		if err != nil || !bytes.Equal(got, original) {
			t.Fatalf("original changed: %v", err)
		}
	}
	if isCorruption(errors.New("permission denied")) {
		t.Fatal("ordinary error treated as corruption")
	}
}

func TestQuarantineRejectsSidecarDirectoryWithoutMovingOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+"-wal", 0700); err != nil {
		t.Fatal(err)
	}
	if err := recoverCorruptDB(path, nil); err == nil {
		t.Fatal("expected sidecar failure")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "original" {
		t.Fatalf("original moved: %v", err)
	}
}

func TestQuarantinePreservesAllSQLiteSidecars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		if err := os.WriteFile(path+suffix, []byte("original"+suffix), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := recoverCorruptDB(path, nil); err != nil {
		t.Fatal(err)
	}
	dirs, _ := filepath.Glob(path + ".corrupt-*")
	if len(dirs) != 1 {
		t.Fatalf("quarantine=%v", dirs)
	}
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		got, err := os.ReadFile(filepath.Join(dirs[0], "memory.db"+suffix))
		if err != nil || string(got) != "original"+suffix {
			t.Fatalf("sidecar %s lost: %v", suffix, err)
		}
		if suffix != "" {
			if _, err := os.Stat(path + suffix); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("sidecar left at live path: %s %v", suffix, err)
			}
		}
	}
}

func TestImmediateTransactionsReserveWriterAtBegin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	db, err := Open(path, WithImmediateTransactions(), WithBusyTimeout(0))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	other, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := other.BeginTx(context.Background(), nil); err == nil {
		t.Fatal("second writer was not reserved")
	}
}
