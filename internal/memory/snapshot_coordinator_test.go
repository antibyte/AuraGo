package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"aurago/internal/config"
	"modernc.org/sqlite"
)

func TestPinnedReadSnapshotSurvivesConcurrentCommitAcrossBackupSteps(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.db")
	destinationPath := filepath.Join(root, "copy.db")
	source, err := sql.Open("sqlite", "file:"+filepath.ToSlash(sourcePath)+"?mode=rwc&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	source.SetMaxOpenConns(1)
	if _, err := source.Exec(`PRAGMA journal_mode=WAL; CREATE TABLE payload(id INTEGER PRIMARY KEY, body BLOB)`); err != nil {
		t.Fatal(err)
	}
	insertTx, err := source.Begin()
	if err != nil {
		t.Fatal(err)
	}
	insert, err := insertTx.Prepare(`INSERT INTO payload(body) VALUES(?)`)
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 8192)
	for i := 0; i < 1400; i++ {
		body[0] = byte(i)
		if _, err := insert.Exec(body); err != nil {
			insert.Close()
			insertTx.Rollback()
			t.Fatal(err)
		}
	}
	if err := insert.Close(); err != nil {
		t.Fatal(err)
	}
	if err := insertTx.Commit(); err != nil {
		t.Fatal(err)
	}

	reader, err := source.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err := reader.ExecContext(context.Background(), `BEGIN`); err != nil {
		t.Fatal(err)
	}
	defer reader.ExecContext(context.Background(), `ROLLBACK`)
	var pinnedCount int
	if err := reader.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM payload`).Scan(&pinnedCount); err != nil {
		t.Fatal(err)
	}
	if pinnedCount != 1400 {
		t.Fatalf("pinned source row count = %d, want 1400", pinnedCount)
	}
	file, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	writer, err := sql.Open("sqlite", "file:"+filepath.ToSlash(sourcePath)+"?mode=rw&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	writer.SetMaxOpenConns(1)
	steps := 0
	err = reader.Raw(func(raw any) (err error) {
		backuper, ok := raw.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			return fmt.Errorf("SQLite connection does not expose online backup")
		}
		backup, err := backuper.NewBackup(destinationPath)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, backup.Finish()) }()

		more, err := backup.Step(128)
		if err != nil {
			return err
		}
		if !more {
			return fmt.Errorf("large fixture completed in one page; multi-step condition was not exercised")
		}
		// Commit on a second connection after the first batch copied, while
		// the source connection still holds its explicit WAL read transaction.
		if _, err := writer.Exec(`INSERT INTO payload(body) VALUES('committed-after-pin')`); err != nil {
			return fmt.Errorf("concurrent writer commit: %w", err)
		}
		steps = 1
		for more {
			more, err = backup.Step(128)
			if err != nil {
				return err
			}
			steps++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("online backup with pinned source transaction: %v", err)
	}
	if steps < 2 {
		t.Fatalf("backup steps = %d, want multiple steps", steps)
	}
	var currentCount int
	// The reader Conn occupies source's sole pool slot until its transaction
	// is rolled back, so query through the independent writer pool.
	if err := writer.QueryRow(`SELECT COUNT(*) FROM payload`).Scan(&currentCount); err != nil {
		t.Fatal(err)
	}
	if currentCount != pinnedCount+1 {
		t.Fatalf("writer row not committed: source count=%d pinned=%d", currentCount, pinnedCount)
	}
	copyDB, err := sql.Open("sqlite", "file:"+filepath.ToSlash(destinationPath)+"?mode=ro&immutable=1")
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var copiedCount int
	if err := copyDB.QueryRow(`SELECT COUNT(*) FROM payload`).Scan(&copiedCount); err != nil {
		t.Fatal(err)
	}
	if copiedCount != pinnedCount {
		t.Fatalf("backup crossed pinned snapshot: copied=%d want pinned=%d", copiedCount, pinnedCount)
	}
}

func TestMemoryPublicationLeaseNestingAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stm.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stm := &SQLiteMemory{db: db}
	leaseCtx, release, err := stm.BeginMemoryPublication(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	nestedCtx, nestedRelease, err := stm.BeginMemoryPublication(leaseCtx)
	if err != nil {
		t.Fatal(err)
	}
	if nestedCtx != leaseCtx {
		t.Fatal("nested publication replaced the lease context")
	}
	coordinator, err := stm.snapshotCoordinator()
	if err != nil {
		t.Fatal(err)
	}
	coordinator.mu.Lock()
	writers := coordinator.writers
	coordinator.mu.Unlock()
	if writers != 1 {
		t.Fatalf("nested publication acquired %d writer leases, want 1", writers)
	}
	nestedRelease()
	coordinator.mu.Lock()
	writers = coordinator.writers
	coordinator.mu.Unlock()
	if writers != 1 {
		t.Fatalf("nested release changed outer lease count to %d", writers)
	}

	freezeCtx, cancelFreeze := context.WithCancel(context.Background())
	freezeDone := make(chan error, 1)
	go func() {
		_, err := coordinator.freeze(freezeCtx)
		freezeDone <- err
	}()
	deadline := time.Now().Add(time.Second)
	for {
		coordinator.mu.Lock()
		frozen := coordinator.frozen
		coordinator.mu.Unlock()
		if frozen {
			break
		}
		if time.Now().After(deadline) {
			cancelFreeze()
			t.Fatal("freeze did not become visible")
		}
		runtime.Gosched()
	}
	waitCtx, cancelWait := context.WithCancel(context.Background())
	cancelWait()
	if _, _, err := stm.BeginMemoryPublication(waitCtx); !errors.Is(err, context.Canceled) {
		cancelFreeze()
		t.Fatalf("canceled writer waiting behind freeze: %v", err)
	}
	cancelFreeze()
	if err := <-freezeDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled freeze returned %v", err)
	}
	coordinator.mu.Lock()
	frozen := coordinator.frozen
	coordinator.mu.Unlock()
	if frozen {
		t.Fatal("canceled freeze leaked the snapshot fence")
	}
}

func TestPendingPublicationMarkerBlocksSnapshotAfterCoordinatorRestart(t *testing.T) {
	cfg := &config.Config{}
	root := t.TempDir()
	cfg.Directories.DataDir = root
	cfg.Directories.VectorDBDir = filepath.Join(root, "vectors")
	if err := os.MkdirAll(cfg.Directories.VectorDBDir, 0700); err != nil {
		t.Fatal(err)
	}
	cfg.SQLite.ShortTermPath = filepath.Join(root, "stm.db")
	cfg.SQLite.KnowledgeGraphPath = filepath.Join(root, "kg.db")
	paths := []string{cfg.SQLite.ShortTermPath, cfg.SQLite.KnowledgeGraphPath, config.DocumentCorpusPath(cfg)}
	for _, path := range paths {
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TABLE fixture(value TEXT)`); err != nil {
			db.Close()
			t.Fatal(err)
		}
		if path == cfg.SQLite.ShortTermPath {
			if _, err := db.Exec(`CREATE TABLE memory_maintenance_meta(key TEXT PRIMARY KEY, value TEXT NOT NULL); INSERT INTO memory_maintenance_meta(key,value) VALUES('snapshot.publication.pending','1')`); err != nil {
				db.Close()
				t.Fatal(err)
			}
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
	// A fresh process has no in-memory coordinator state; the unfinished marker
	// remains the durable signal that cross-store recovery is still required.
	memorySnapshotCoordinators.Delete(snapshotPathKey(cfg.SQLite.ShortTermPath))
	if _, err := FreezeMemorySnapshots(context.Background(), cfg); err == nil {
		t.Fatal("snapshot accepted an unfinished publication marker")
	}
	db, err := sql.Open("sqlite", cfg.SQLite.ShortTermPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM memory_maintenance_meta WHERE key='snapshot.publication.pending'`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	memorySnapshotCoordinators.Delete(snapshotPathKey(cfg.SQLite.ShortTermPath))
	release, err := FreezeMemorySnapshots(context.Background(), cfg)
	if err != nil {
		t.Fatalf("snapshot remained blocked after marker recovery: %v", err)
	}
	release()
}

func TestV1SnapshotsAreNotCountedAsCoordinatedV2Retention(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	writeFixture := func(name, kind, consistency string, created time.Time) {
		t.Helper()
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		files := map[string]string{}
		for _, fileName := range []string{"short_term.sqlite", "knowledge_graph.sqlite", "documents.sqlite"} {
			path := filepath.Join(dir, fileName)
			if err := os.WriteFile(path, []byte(name+":"+fileName), 0600); err != nil {
				t.Fatal(err)
			}
			digest, err := automaticSnapshotDigest(path)
			if err != nil {
				t.Fatal(err)
			}
			files[fileName] = digest
		}
		data, err := json.Marshal(automaticSnapshotManifest{Kind: kind, Consistency: consistency, CreatedAt: created, Files: files})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "complete.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeFixture("auto-v1", "auto-memory-v1", "", now.AddDate(0, 0, -90))
	writeFixture("auto-v2-old", "auto-memory-v2", memorySnapshotConsistency, now.AddDate(0, 0, -60))
	writeFixture("auto-v2-new", "auto-memory-v2", memorySnapshotConsistency, now.AddDate(0, 0, -1))

	verified, err := verifiedAutomaticSnapshots(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(verified) != 2 {
		t.Fatalf("verified coordinated generations = %d, want 2 (v1 must not count)", len(verified))
	}
	if err := retainAutomaticSnapshots(root, verified, now); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"auto-v1", "auto-v2-old", "auto-v2-new"} {
		if _, err := os.Stat(filepath.Join(root, name, "complete.json")); err != nil {
			t.Fatalf("retention removed or rewrote %s: %v", name, err)
		}
	}
}
