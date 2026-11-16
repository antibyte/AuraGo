package memory

import (
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	chromem "github.com/philippgille/chromem-go"
	"modernc.org/sqlite"
)

func writeLegacyMigrationFixture(t *testing.T, dir, id, content string) string {
	t.Helper()
	folder := filepath.Join(dir, "collection")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(file string, value any) {
		f, err := os.Create(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := gob.NewEncoder(f).Encode(value); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(folder, "00000000.gob"), struct {
		Name     string
		Metadata map[string]string
	}{"legacy", map[string]string{"kind": "fixture"}})
	hash := sha256.Sum256([]byte(id))
	path := filepath.Join(folder, fmt.Sprintf("%x.gob", hash[:4]))
	write(path, chromem.Document{ID: id, Content: content, Metadata: map[string]string{"source": "fixture"}, Embedding: []float32{1, 0}})
	return path
}

func unverifiedLegacyCorpus(t *testing.T) (*documentCorpus, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "vectors")
	c, err := openDocumentCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.db.Close() })
	if _, err := c.db.Exec("DELETE FROM corpus_meta WHERE key='verified'"); err != nil {
		t.Fatal(err)
	}
	return c, dir
}

func failLegacyDocumentWrites(t *testing.T, c *documentCorpus) {
	t.Helper()
	if _, err := c.db.Exec(`CREATE TRIGGER fail_legacy_write BEFORE INSERT ON documents BEGIN SELECT RAISE(ABORT,'fixture failure after backup'); END`); err != nil {
		t.Fatal(err)
	}
}

func legacyBackups(t *testing.T, dir string, want int) []string {
	t.Helper()
	found, err := filepath.Glob(dir + ".legacy-backup-*")
	if err != nil || len(found) != want {
		t.Fatalf("backup count got %v, want %d: %v", found, want, err)
	}
	return found
}

func TestLegacyVectorBackupReusedAcrossFailuresAndRestart(t *testing.T) {
	c, dir := unverifiedLegacyCorpus(t)
	writeLegacyMigrationFixture(t, dir, "one", "original")
	before, err := inventoryLegacyFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	failLegacyDocumentWrites(t, c)
	for range 2 {
		if err := c.migrateLegacy(dir); err == nil {
			t.Fatal("injected failure ignored")
		}
		legacyBackups(t, dir, 1)
	}
	c.db.Close()
	if reopened, err := openDocumentCorpus(dir); err == nil {
		reopened.db.Close()
		t.Fatal("restart ignored injected failure")
	}
	backup := legacyBackups(t, dir, 1)[0]
	if err := verifyLegacyInventory(backup, before); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := filepath.Walk(backup, func(_ string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.Mode().Perm()&0077 != 0 {
				return fmt.Errorf("backup is not private")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "documents.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TRIGGER fail_legacy_write"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	recovered, err := openDocumentCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.db.Close()
	var count int
	if err := recovered.db.QueryRow("SELECT count(*) FROM documents").Scan(&count); err != nil || count != 1 {
		t.Fatalf("recovery count=%d: %v", count, err)
	}
	legacyBackups(t, dir, 1)
	if err := verifyLegacyInventory(dir, before); err != nil {
		t.Fatalf("sources changed: %v", err)
	}
}

func TestLegacyVectorBackupCorruptionAndSourceChangePreserveOldCopies(t *testing.T) {
	c, dir := unverifiedLegacyCorpus(t)
	writeLegacyMigrationFixture(t, dir, "one", "first")
	failLegacyDocumentWrites(t, c)
	if err := c.migrateLegacy(dir); err == nil {
		t.Fatal("injected failure ignored")
	}
	old := legacyBackups(t, dir, 1)[0]
	inventory, err := inventoryLegacyFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	damaged := filepath.Join(old, filepath.FromSlash(inventory[0].Path))
	if err := os.WriteFile(damaged, []byte("damaged backup retained"), 0600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := c.migrateLegacy(dir); err == nil {
			t.Fatal("injected failure ignored")
		}
		legacyBackups(t, dir, 2)
	}
	writeLegacyMigrationFixture(t, dir, "one", "changed source")
	for range 2 {
		if err := c.migrateLegacy(dir); err == nil {
			t.Fatal("injected failure ignored")
		}
		legacyBackups(t, dir, 3)
	}
	if got, err := os.ReadFile(damaged); err != nil || string(got) != "damaged backup retained" {
		t.Fatal("old damaged backup was modified")
	}
	unknown := dir + ".legacy-backup-unknown"
	if err := os.Mkdir(unknown, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unknown, "keep"), []byte("operator copy"), 0600); err != nil {
		t.Fatal(err)
	}
	_ = c.migrateLegacy(dir)
	legacyBackups(t, dir, 4)
	if _, err := os.Stat(filepath.Join(unknown, "keep")); err != nil {
		t.Fatal("unknown backup removed")
	}
}

func TestLegacyVectorBackupDiskBudgetIncludesSnapshotButNotReusedCopy(t *testing.T) {
	c, dir := unverifiedLegacyCorpus(t)
	writeLegacyMigrationFixture(t, dir, "one", strings.Repeat("content", 2000))
	failLegacyDocumentWrites(t, c)
	inventory, err := inventoryLegacyFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, required, err := inspectLegacyCollections(dir, inventory)
	if err != nil {
		t.Fatal(err)
	}
	available := CorpusDiskReserveBytes + required*3 + 65536
	actualFree := c.freeDisk
	c.freeDisk = func(string) (int64, error) { return available, nil }
	if err := c.migrateLegacy(dir); !errors.Is(err, ErrCorpusCapacity) || !strings.Contains(err.Error(), "legacy_backup_disk_reserve") {
		t.Fatalf("missing backup budget: %v", err)
	}
	legacyBackups(t, dir, 0)
	c.freeDisk = actualFree
	if err := c.migrateLegacy(dir); err == nil {
		t.Fatal("injected failure ignored")
	}
	legacyBackups(t, dir, 1)
	c.freeDisk = func(string) (int64, error) { return available, nil }
	if err := c.migrateLegacy(dir); err == nil || errors.Is(err, ErrCorpusCapacity) {
		t.Fatalf("verified copy was budgeted twice: %v", err)
	}
	legacyBackups(t, dir, 1)
}

func TestLegacyVectorSourceChangeAfterBackupRollsBackSnapshotImport(t *testing.T) {
	name := "legacy_source_change_" + rand.Text()
	var changedPath string
	if err := sqlite.RegisterScalarFunction(name, 0, func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
		if changedPath != "" {
			path := changedPath
			changedPath = ""
			if err := os.WriteFile(path, []byte("concurrent source edit"), 0600); err != nil {
				return nil, err
			}
		}
		return int64(1), nil
	}); err != nil {
		t.Fatal(err)
	}
	c, dir := unverifiedLegacyCorpus(t)
	writeLegacyMigrationFixture(t, dir, "one", "first")
	writeLegacyMigrationFixture(t, dir, "two", "second")
	inventory, err := inventoryLegacyFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	changedPath = filepath.Join(dir, filepath.FromSlash(inventory[len(inventory)-1].Path))
	if _, err := c.db.Exec(`CREATE TRIGGER mutate_legacy_source AFTER INSERT ON documents BEGIN SELECT ` + name + `(); END`); err != nil {
		t.Fatal(err)
	}
	if err := c.migrateLegacy(dir); err == nil || !strings.Contains(err.Error(), "legacy source changed before migration commit") {
		t.Fatalf("import did not use verified snapshot then check source: %v", err)
	}
	var count int
	if err := c.db.QueryRow("SELECT count(*) FROM documents").Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial import committed: %d, %v", count, err)
	}
	var verified string
	if err := c.db.QueryRow("SELECT value FROM corpus_meta WHERE key='verified'").Scan(&verified); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("failed import marked verified: %q, %v", verified, err)
	}
	if err := verifyLegacyInventory(legacyBackups(t, dir, 1)[0], inventory); err != nil {
		t.Fatal(err)
	}
}

type legacyZeroReader struct{}

func (legacyZeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestLegacyVectorGobReadLimit(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		t.Run(fmt.Sprint(compressed), func(t *testing.T) {
			c, dir := unverifiedLegacyCorpus(t)
			file := writeLegacyMigrationFixture(t, dir, "one", "source")
			if compressed {
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
				file += ".gz"
			}
			f, err := os.Create(file)
			if err != nil {
				t.Fatal(err)
			}
			if compressed {
				z := gzip.NewWriter(f)
				if _, err := io.CopyN(z, legacyZeroReader{}, legacyImportReadLimit+1); err != nil {
					t.Fatal(err)
				}
				if err := z.Close(); err != nil {
					t.Fatal(err)
				}
			} else if err := f.Truncate(legacyImportReadLimit + 1); err != nil {
				t.Fatal(err)
			}
			f.Close()
			before, err := os.Stat(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := c.migrateLegacy(dir); err == nil || !strings.Contains(err.Error(), "64 MiB") {
				t.Fatalf("missing explicit read limit: %v", err)
			}
			after, err := os.Stat(file)
			if err != nil || after.Size() != before.Size() {
				t.Fatal("source changed")
			}
			legacyBackups(t, dir, 0)
		})
	}
}

func TestLegacyVectorBackupFailureCleansOnlyOwnStaging(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vectors")
	writeLegacyMigrationFixture(t, dir, "one", "original")
	files, err := inventoryLegacyFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := makeLegacyManifest(files)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Files[0].SHA256 = strings.Repeat("0", 64)
	unknown := filepath.Join(filepath.Dir(dir), ".vectors.legacy-stage-operator")
	if err := os.Mkdir(unknown, 0700); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if attempt == 1 {
			manifest.Files[0].Size--
		}
		if _, err := publishLegacyBackup(dir, manifest); err == nil {
			t.Fatal("unverified backup published")
		}
		stages, err := filepath.Glob(filepath.Join(filepath.Dir(dir), ".vectors.legacy-stage-*"))
		if err != nil || len(stages) != 1 || stages[0] != unknown {
			t.Fatalf("unexpected staging cleanup: %v, %v", stages, err)
		}
		legacyBackups(t, dir, 0)
	}
}
