package memory

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCanonicalRepairCommitFailurePreservesCursorAndRecoversAfterRestart(t *testing.T) {
	cfg, stm, vector := publicationSnapshotFixture(t)
	const concept = "[fact] AuroraGo uses Go."
	const body = "source:memory_analysis session:example"
	write, err := StoreAutomaticMemoryDocumentWithContext(t.Context(), stm, vector, concept, body, MemoryMetaUpdate{
		SourceType: "memory_analysis", SourceSessionID: "example",
	})
	if err != nil || len(write.CreatedIDs) != 1 {
		t.Fatalf("store test source: IDs=%v err=%v", write.CreatedIDs, err)
	}
	oldID := write.CreatedIDs[0]
	fingerprint := vector.EmbeddingFingerprint()
	key := memoryReplacementKey(oldID)
	if _, err := stm.db.Exec(`CREATE TABLE test_parent(id TEXT PRIMARY KEY);
		CREATE TABLE test_commit_guard(id TEXT REFERENCES test_parent(id) DEFERRABLE INITIALLY DEFERRED);
		CREATE TRIGGER fail_replacement_commit AFTER UPDATE ON memory_maintenance_meta
		WHEN NEW.key LIKE 'memory.publication.replacement.%' AND json_extract(NEW.value,'$.status')='ready'
		BEGIN INSERT INTO test_commit_guard VALUES('absent'); END`); err != nil {
		t.Fatal(err)
	}
	if err := stm.SetMemoryMaintenanceState(canonicalRepairCursorKey, "before-first"); err != nil {
		t.Fatal(err)
	}
	report, err := stm.RepairCanonicalMemoryNamesContext(t.Context(), vector, CanonicalRepairOptions{Limit: 1})
	var commitErr *memoryReplacementMetadataCommitError
	if !errors.As(err, &commitErr) || len(report.Items) != 1 || len(report.Items[0].NewDocIDs) != 1 {
		t.Fatalf("expected ambiguous commit with a receipt: report=%+v err=%v", report, err)
	}
	abortedID := report.Items[0].NewDocIDs[0]
	if cursor, err := stm.GetMemoryMaintenanceState(canonicalRepairCursorKey); cursor != "before-first" || err != nil {
		t.Fatalf("cursor=%q err=%v", cursor, err)
	}
	if err := CreateAutomaticSnapshot(t.Context(), cfg, time.Now()); !errors.Is(err, ErrMemorySnapshotBlocked) {
		t.Fatalf("snapshot accepted unresolved replacement: %v", err)
	}
	if err := vector.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	memorySnapshotCoordinators.Delete(snapshotPathKey(cfg.SQLite.ShortTermPath))
	stm, err = NewSQLiteMemory(cfg.SQLite.ShortTermPath, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	vector = newAutomaticPublicationVectorWithFingerprint(t, cfg.Directories.VectorDBDir, stm, func(context.Context, string) ([]float32, error) {
		return []float32{1, 0, 0}, nil
	}, fingerprint)
	if err := CreateAutomaticSnapshot(t.Context(), cfg, time.Now()); !errors.Is(err, ErrMemorySnapshotBlocked) {
		t.Fatalf("reopening lost the replacement fence: %v", err)
	}
	if _, err := stm.db.Exec(`DROP TRIGGER fail_replacement_commit`); err != nil {
		t.Fatal(err)
	}
	report, err = stm.RepairCanonicalMemoryNamesContext(t.Context(), vector, CanonicalRepairOptions{Limit: 1})
	if err != nil || report.RepairedCount != 1 {
		t.Fatalf("retry after recovery: report=%+v err=%v", report, err)
	}
	if _, err := vector.GetByID(abortedID); err == nil {
		t.Fatal("the previous aborted replacement survived the retry")
	}
	if value, err := stm.GetMemoryMaintenanceState(key); value != "" || err != nil {
		t.Fatalf("replacement journal remains=%v err=%v", value != "", err)
	}
	if err := CreateAutomaticSnapshot(t.Context(), cfg, time.Now()); err != nil {
		t.Fatalf("snapshot after reconciliation: %v", err)
	}
}

func TestCommittedChunkedMemoryReplacementResumesWithoutCreatingCopies(t *testing.T) {
	stm, vector, dir := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	write, err := publishTestMemory(t, stm, vector, "original source", nil)
	if err != nil || len(write.CreatedIDs) != 1 {
		t.Fatalf("store source: IDs=%v err=%v", write.CreatedIDs, err)
	}
	oldID := write.CreatedIDs[0]
	oldBody, err := vector.GetByID(oldID)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := stm.GetMemoryMeta(oldID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vector.corpus.db.Exec(`CREATE TRIGGER fail_replacement_activation BEFORE UPDATE OF pending ON documents
		WHEN NEW.pending=0 BEGIN SELECT RAISE(ABORT,'test activation failure'); END`); err != nil {
		t.Fatal(err)
	}
	replacement := strings.Repeat("ordinary archive notes ", 250)
	ids, err := stm.ReplaceMemoryDocumentContext(t.Context(), vector, oldID, "replacement", oldBody, replacement, meta, "test", "system")
	if !errors.Is(err, ErrAutomaticMemoryPublicationPending) || len(ids) < 2 {
		t.Fatalf("expected hidden chunked replacement: IDs=%v err=%v", ids, err)
	}
	if _, err := vector.GetByID(oldID); err != nil {
		t.Fatalf("source was retired before activation: %v", err)
	}
	if err := vector.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(dir), "memory.db")
	memorySnapshotCoordinators.Delete(snapshotPathKey(path))
	reopened, err := NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	resumed := newAutomaticPublicationVector(t, dir, reopened, func(context.Context, string) ([]float32, error) {
		t.Error("recovery must use saved embeddings")
		return []float32{1, 0, 0}, nil
	})
	if _, err := resumed.corpus.db.Exec(`DROP TRIGGER fail_replacement_activation`); err != nil {
		t.Fatal(err)
	}
	if err := reopened.RecoverAutomaticMemoryPublicationsContext(t.Context(), resumed, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := resumed.GetByID(oldID); err == nil {
		t.Fatal("the old source was not retired")
	}
	for _, id := range ids {
		if _, err := resumed.GetByID(id); err != nil {
			t.Fatalf("replacement %s was lost: %v", id, err)
		}
	}
	var count int
	if err := resumed.corpus.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE collection='aurago_memories'`).Scan(&count); count != len(ids) || err != nil {
		t.Fatalf("unexpected replacement copies: count=%d err=%v", count, err)
	}
}
