package memory

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestArchiveIdentitySurvivesVectorRemovalAndAnalysisProvenance(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
	s := cv.fileIndexMemory.Load()
	concept := "[preference:editor] User prefers Vim"
	first, err := StoreAutomaticMemoryDocumentWithContext(t.Context(), s, cv, concept, "source:memory_analysis session:one", MemoryMetaUpdate{SourceType: "memory_analysis", SourceSessionID: "one"})
	if err != nil || len(first.CreatedIDs) != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	id := first.CreatedIDs[0]
	if err := s.ApplyMemoryCurationActionContext(t.Context(), MemoryCurationAction{DocID: id, Action: "archive"}, "admin", false); err != nil {
		t.Fatal(err)
	}
	if err := cv.DeleteDocument(id); err != nil {
		t.Fatal(err)
	}
	_, err = StoreAutomaticMemoryDocumentWithContext(t.Context(), s, cv, concept, "source:memory_analysis session:two", MemoryMetaUpdate{SourceType: "memory_analysis", SourceSessionID: "two"})
	if !errors.Is(err, ErrAutomaticMemorySkipped) {
		t.Fatalf("re-extraction error=%v", err)
	}
	if err := s.ApplyMemoryCurationActionContext(t.Context(), MemoryCurationAction{DocID: id, Action: "confirm"}, "admin", false); err == nil {
		t.Fatal("reactivated a deleted source")
	}
}

func TestArchiveIdentityExplicitRestoreClearsOnlyAfterSuccess(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
	s := cv.fileIndexMemory.Load()
	stored, err := StoreAutomaticMemoryDocumentWithContext(t.Context(), s, cv, "Editor", "Vim is preferred", MemoryMetaUpdate{})
	if err != nil {
		t.Fatal(err)
	}
	id := stored.CreatedIDs[0]
	for _, action := range []string{"archive", "confirm"} {
		if err := s.ApplyMemoryCurationActionContext(t.Context(), MemoryCurationAction{DocID: id, Action: action}, "admin", false); err != nil {
			t.Fatal(err)
		}
	}
	var blocked bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM memory_archive_identities WHERE doc_id=?)`, id).Scan(&blocked); err != nil || blocked {
		t.Fatalf("blocked=%v err=%v", blocked, err)
	}
}

func TestArchiveBudgetCommitsDecisionBeforeDeletionAndRetriesCleanup(t *testing.T) {
	s := newInMemorySQLiteMemory(t)
	failure := errors.New("injected vector failure")
	vector := &fakeRepairVectorDB{docs: map[string]string{"old": "old fact", "keep": "keep fact"}, deleteErr: map[string]error{"old": failure}}
	for _, id := range []string{"old", "keep"} {
		if err := s.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetMemoryMetaProtection("keep", true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyMemoryBudgetEnforcementContext(t.Context(), 1, vector); !errors.Is(err, failure) {
		t.Fatalf("err=%v", err)
	}
	meta, err := s.GetMemoryMeta("old")
	if err != nil || !IsMemoryArchived(meta) {
		t.Fatalf("meta=%+v err=%v", meta, err)
	}
	var pending int
	if err := s.db.QueryRow(`SELECT count(*) FROM memory_archive_cleanup WHERE doc_id='old'`).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("pending=%d err=%v", pending, err)
	}
	delete(vector.deleteErr, "old")
	if _, err := s.ApplyMemoryBudgetEnforcementContext(t.Context(), 0, vector); err != nil {
		t.Fatal(err)
	}
	if _, ok := vector.docs["old"]; ok {
		t.Fatal("cleanup did not resume")
	}
}

func TestArchiveBudgetSQLFailureNeverDeletesVector(t *testing.T) {
	s := newInMemorySQLiteMemory(t)
	vector := &fakeRepairVectorDB{docs: map[string]string{"a": "a", "b": "b"}}
	for _, id := range []string{"a", "b"} {
		if err := s.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(`CREATE TRIGGER reject_archive BEFORE UPDATE ON memory_meta WHEN NEW.verification_status='archived' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyMemoryBudgetEnforcementContext(t.Context(), 1, vector); err == nil {
		t.Fatal("expected archive failure")
	}
	if len(vector.deleted) != 0 {
		t.Fatalf("deleted before metadata commit: %v", vector.deleted)
	}
}

type archiveLegacyVector struct{ VectorDB }

func TestArchiveCleanupRemovesDerivedOrphanAfterCanonicalDeletion(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
	s := cv.fileIndexMemory.Load()
	stored, err := StoreAutomaticMemoryDocumentWithContext(t.Context(), s, cv, "Old", "archived fact", MemoryMetaUpdate{})
	if err != nil {
		t.Fatal(err)
	}
	id := stored.CreatedIDs[0]
	content, err := cv.GetByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyMemoryCurationActionContext(t.Context(), MemoryCurationAction{DocID: id, Action: "archive"}, "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO memory_archive_cleanup(doc_id,expected_hash) VALUES(?,?)`, id, contentSHA256(content)); err != nil {
		t.Fatal(err)
	}
	if _, err := cv.corpus.db.Exec(`DELETE FROM documents WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	if cv.collection.Count() != 1 {
		t.Fatal("fixture must retain the stale derived index entry")
	}
	if _, err := s.ApplyMemoryBudgetEnforcementContext(t.Context(), 0, cv); err != nil {
		t.Fatal(err)
	}
	if cv.collection.Count() != 0 {
		t.Fatal("cleanup acknowledged a stale derived entry")
	}
}

func TestArchiveBudgetRetainsVectorWithoutConditionalOwnership(t *testing.T) {
	s := newInMemorySQLiteMemory(t)
	underlying := &fakeRepairVectorDB{docs: map[string]string{"a": "a", "b": "b"}}
	vector := archiveLegacyVector{VectorDB: underlying}
	for _, id := range []string{"a", "b"} {
		if err := s.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ApplyMemoryBudgetEnforcementContext(t.Context(), 1, vector); err == nil {
		t.Fatal("cleanup must retain ownership-unknown vectors")
	}
	var pending int
	if err := s.db.QueryRow(`SELECT count(*) FROM memory_archive_cleanup`).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("pending=%d err=%v", pending, err)
	}
	if len(underlying.deleted) != 0 {
		t.Fatalf("unconditionally deleted vectors: %v", underlying.deleted)
	}
}

func TestArchiveAndRollbackRespectLockCancellation(t *testing.T) {
	s := newInMemorySQLiteMemory(t)
	s.automaticMemoryMu.Lock()
	defer s.automaticMemoryMu.Unlock()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	vector := &fakeRepairVectorDB{}
	if _, err := s.ApplyMemoryBudgetEnforcementContext(ctx, 1, vector); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("archive err=%v", err)
	}
	if err := s.RollbackAutomaticMemoryWritesContext(ctx, vector, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("rollback err=%v", err)
	}
}

type archiveReadHookVector struct {
	*fakeRepairVectorDB
	readHook func()
}

func (v *archiveReadHookVector) GetByID(id string) (string, error) {
	if v.readHook != nil {
		hook := v.readHook
		v.readHook = nil
		hook()
	}
	return v.fakeRepairVectorDB.GetByID(id)
}

func TestArchiveBudgetRechecksProtectionAfterCandidateSelection(t *testing.T) {
	s := newInMemorySQLiteMemory(t)
	vector := &archiveReadHookVector{fakeRepairVectorDB: &fakeRepairVectorDB{docs: map[string]string{"a": "a", "b": "b"}}}
	for _, id := range []string{"a", "b"} {
		if err := s.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	vector.readHook = func() {
		// Simulate an independent DB writer between preview and archive.
		if _, err := s.db.Exec(`UPDATE memory_meta SET keep_forever=1`); err != nil {
			t.Fatal(err)
		}
	}
	count, err := s.ApplyMemoryBudgetEnforcementContext(t.Context(), 1, vector)
	if err != nil || count != 0 || len(vector.deleted) != 0 {
		t.Fatalf("count=%d deleted=%v err=%v", count, vector.deleted, err)
	}
}

func TestArchiveCleanupResumesAfterVectorDeleteAndRestart(t *testing.T) {
	dir := t.TempDir()
	cv := newCorpusTestVectorDB(t, filepath.Join(dir, "corpus"))
	path := filepath.Join(dir, "metadata.db")
	s, err := NewSQLiteMemory(path, cv.logger)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	cv.BindMemoryMetadata(s)
	for _, concept := range []string{"Alpha", "Beta"} {
		if _, err := StoreAutomaticMemoryDocumentWithContext(t.Context(), s, cv, concept, concept+" fact", MemoryMetaUpdate{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(`CREATE TRIGGER reject_cleanup BEFORE DELETE ON memory_archive_cleanup BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyMemoryBudgetEnforcementContext(t.Context(), 1, cv); err == nil {
		t.Fatal("expected acknowledgement failure")
	}
	if _, err := s.db.Exec(`DROP TRIGGER reject_cleanup`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewSQLiteMemory(path, cv.logger)
	if err != nil {
		t.Fatal(err)
	}
	cv.BindMemoryMetadata(s)
	if _, err := s.ApplyMemoryBudgetEnforcementContext(t.Context(), 0, cv); err != nil {
		t.Fatal(err)
	}
	var pending, identities int
	if err := s.db.QueryRow(`SELECT count(*) FROM memory_archive_cleanup`).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM memory_archive_identities`).Scan(&identities); err != nil {
		t.Fatal(err)
	}
	if pending != 0 || identities != 1 || cv.Count() != 1 {
		t.Fatalf("pending=%d identities=%d corpus=%d", pending, identities, cv.Count())
	}
}

func TestArchiveIdentityIncludesFullNormalizedContentAndDomain(t *testing.T) {
	body := "Title\n" + string(make([]byte, 5000))
	if memoryDocumentIdentity(body+"a", "home") == memoryDocumentIdentity(body+"b", "home") {
		t.Fatal("identity ignored the full content")
	}
	if memoryDocumentIdentity(" Title\r\nbody ", " Home ") != memoryDocumentIdentity("Title\nbody", "Home") {
		t.Fatal("identity did not normalize content and domain")
	}
	if memoryDocumentIdentity("same", "home") == memoryDocumentIdentity("same", "work") {
		t.Fatal("identity ignored the domain")
	}
}

func TestArchiveIdentityBackfillUsesOnlyExistingCorpusContent(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
	s := cv.fileIndexMemory.Load()
	stored, err := StoreAutomaticMemoryDocumentWithContext(t.Context(), s, cv, "Editor", "Vim", MemoryMetaUpdate{})
	if err != nil {
		t.Fatal(err)
	}
	id := stored.CreatedIDs[0]
	if _, err := s.db.Exec(`UPDATE memory_meta SET verification_status='archived' WHERE doc_id=?;
		DELETE FROM memory_maintenance_meta WHERE key='archive_identity_backfill_v1';
		INSERT INTO memory_meta(doc_id,verification_status) VALUES('missing-history','archived')`, id); err != nil {
		t.Fatal(err)
	}
	_, err = StoreAutomaticMemoryDocumentWithContext(t.Context(), s, cv, "Editor", "Vim", MemoryMetaUpdate{})
	if !errors.Is(err, ErrAutomaticMemorySkipped) {
		t.Fatalf("backfilled archive allowed re-extraction: %v", err)
	}
	var actual, invented int
	if err := s.db.QueryRow(`SELECT count(*) FROM memory_archive_identities WHERE doc_id=?`, id).Scan(&actual); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM memory_archive_identities WHERE doc_id='missing-history'`).Scan(&invented); err != nil {
		t.Fatal(err)
	}
	if actual != 1 || invented != 0 {
		t.Fatalf("actual=%d invented=%d", actual, invented)
	}
}
