package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
)

func publicationSnapshotFixture(t *testing.T) (*config.Config, *SQLiteMemory, *ChromemVectorDB) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.Directories.VectorDBDir = filepath.Join(cfg.Directories.DataDir, "vectors#percent%")
	v := newCorpusTestVectorDB(t, cfg.Directories.VectorDBDir)
	s := v.fileIndexMemory.Load()
	if err := s.db.QueryRow(`SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&cfg.SQLite.ShortTermPath); err != nil {
		t.Fatal(err)
	}
	cfg.SQLite.KnowledgeGraphPath = filepath.Join(cfg.Directories.DataDir, "kg#%.sqlite")
	kg, err := sql.Open("sqlite", snapshotSQLiteURI(cfg.SQLite.KnowledgeGraphPath, "mode=rwc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kg.Exec(`PRAGMA journal_mode=WAL; CREATE TABLE fixture(id TEXT)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kg.Close() })
	return cfg, s, v
}

func TestSnapshotWaitsForCorpusMetadataAndProvenancePublication(t *testing.T) {
	cfg, stm, vector := publicationSnapshotFixture(t)
	corpusCommitted, allowMetadata := make(chan struct{}), make(chan struct{})
	writeDone := make(chan error, 1)
	go func() {
		_, err := storeAutomaticMemoryDocument(t.Context(), stm, vector, "snapshot fact", "complete content", MemoryMetaUpdate{SourceSessionID: "session-1"}, func(ctx context.Context, v VectorDB, concept, body string) (VectorStoreResult, error) {
			result, err := StoreDocumentWithOwnershipContext(ctx, v, concept, body)
			close(corpusCommitted)
			select {
			case <-allowMetadata:
			case <-ctx.Done():
				return result, ctx.Err()
			}
			return result, err
		})
		writeDone <- err
	}()
	<-corpusCommitted
	snapshotDone := make(chan error, 1)
	go func() { snapshotDone <- CreateAutomaticSnapshot(t.Context(), cfg, time.Now()) }()
	c, err := stm.snapshotCoordinator()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		c.mu.Lock()
		frozen := c.frozen
		c.mu.Unlock()
		if frozen {
			break
		}
		select {
		case err := <-snapshotDone:
			t.Fatalf("snapshot finished during publication: %v", err)
		case <-deadline:
			t.Fatal("snapshot never waited for publication")
		case <-time.After(time.Millisecond):
		}
	}
	select {
	case err := <-snapshotDone:
		t.Fatalf("snapshot crossed corpus/metadata commit: %v", err)
	default:
	}
	close(allowMetadata)
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	if err := <-snapshotDone; err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(cfg.Directories.DataDir, "memory_backups")
	snapshots, err := verifiedAutomaticSnapshots(root)
	if err != nil || len(snapshots) != 1 {
		t.Fatalf("snapshots=%d err=%v", len(snapshots), err)
	}
	for _, check := range []struct{ file, query string }{{"documents.sqlite", `SELECT COUNT(*) FROM documents`}, {"short_term.sqlite", `SELECT COUNT(*) FROM memory_meta`}, {"short_term.sqlite", `SELECT COUNT(*) FROM memory_extraction_sources`}} {
		db, err := sql.Open("sqlite", snapshotSQLiteURI(filepath.Join(root, snapshots[0].name, check.file), "mode=ro&immutable=1"))
		if err != nil {
			t.Fatal(err)
		}
		var count int
		err = db.QueryRow(check.query).Scan(&count)
		db.Close()
		if err != nil || count != 1 {
			t.Fatalf("%s: count=%d err=%v", check.query, count, err)
		}
	}
}

func TestBlockedEmbeddingDoesNotHoldSnapshotLease(t *testing.T) {
	cfg, stm, vector := publicationSnapshotFixture(t)
	started, unblock := make(chan struct{}), make(chan struct{})
	vector.embeddingFunc = func(ctx context.Context, _ string) ([]float32, error) {
		close(started)
		select {
		case <-unblock:
			return []float32{1, 0, 0}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	done := make(chan error, 1)
	go func() {
		_, err := StoreAutomaticMemoryDocumentWithContext(t.Context(), stm, vector, "pending fact", "body", MemoryMetaUpdate{})
		done <- err
	}()
	<-started
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := CreateAutomaticSnapshot(ctx, cfg, time.Now())
	close(unblock)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPartialAutomaticPublicationPreventsSnapshotCertification(t *testing.T) {
	cfg, stm, vector := publicationSnapshotFixture(t)
	if _, err := stm.db.Exec(`CREATE TRIGGER fail_metadata BEFORE INSERT ON memory_meta BEGIN SELECT RAISE(ABORT,'injected metadata failure'); END`); err != nil {
		t.Fatal(err)
	}
	result, err := StoreAutomaticMemoryDocumentWithContext(t.Context(), stm, vector, "partial fact", "body", MemoryMetaUpdate{})
	if err == nil || len(result.CreatedIDs) != 1 {
		t.Fatalf("receipt=%+v err=%v", result, err)
	}
	if err := CreateAutomaticSnapshot(t.Context(), cfg, time.Now()); err == nil {
		t.Fatal("partial publication certified")
	}
	root := filepath.Join(cfg.Directories.DataDir, "memory_backups")
	snapshots, err := verifiedAutomaticSnapshots(root)
	if err != nil || len(snapshots) != 0 {
		t.Fatalf("snapshots=%d err=%v", len(snapshots), err)
	}
}

func TestV1SnapshotOnSameDayDoesNotSuppressCoordinatedV2(t *testing.T) {
	cfg, _, _ := publicationSnapshotFixture(t)
	cfg.Directories.DataDir = filepath.Join(t.TempDir(), "snapshots#percent%")
	now := time.Now()
	if err := CreateAutomaticSnapshot(t.Context(), cfg, now); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(cfg.Directories.DataDir, "memory_backups")
	first, err := verifiedAutomaticSnapshots(root)
	if err != nil || len(first) != 1 {
		t.Fatalf("first snapshots=%d err=%v", len(first), err)
	}
	old := first[0].manifest
	old.Kind = "auto-memory-v1"
	old.Consistency = ""
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(root, first[0].name, "complete.json")
	if err := os.WriteFile(oldPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := CreateAutomaticSnapshot(t.Context(), cfg, now); err != nil {
		t.Fatal(err)
	}
	current, err := verifiedAutomaticSnapshots(root)
	if err != nil || len(current) != 1 || current[0].name == first[0].name {
		t.Fatalf("v1 suppressed v2: %+v %v", current, err)
	}
	retained, err := os.ReadFile(oldPath)
	if err != nil || string(retained) != string(data) {
		t.Fatalf("legacy snapshot changed: %v", err)
	}
}

func TestPanickingAutomaticPublicationKeepsMarkerAndReleasesLease(t *testing.T) {
	cfg, stm, vector := publicationSnapshotFixture(t)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("expected injected publication panic")
			}
		}()
		_, _ = storeAutomaticMemoryDocument(t.Context(), stm, vector, "panic fact", "body", MemoryMetaUpdate{}, func(ctx context.Context, v VectorDB, concept, body string) (VectorStoreResult, error) {
			if _, err := StoreDocumentWithOwnershipContext(ctx, v, concept, body); err != nil {
				t.Fatal(err)
			}
			panic("injected after corpus commit")
		})
	}()
	c, err := stm.snapshotCoordinator()
	if err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	writers := c.writers
	c.mu.Unlock()
	if writers != 0 {
		t.Fatalf("panic leaked %d publication leases", writers)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := CreateAutomaticSnapshot(ctx, cfg, time.Now()); err == nil {
		t.Fatal("panic was certified as a complete publication")
	}
	var pending int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM memory_maintenance_meta WHERE key LIKE 'snapshot.publication.%'`).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("panic marker count=%d err=%v", pending, err)
	}
}
