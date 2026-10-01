package memory

import (
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestAnalysisSourcesMigrationBacksUpLegacyStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertMemoryMetaWithDetails("legacy", MemoryMetaUpdate{VerificationStatus: "confirmed", SourceType: "user", ExtractionConfidence: .99}); err != nil {
		t.Fatal(err)
	}
	before, _ := s.GetMemoryMeta("legacy")
	if _, err := s.db.Exec(`DROP TABLE memory_extraction_sources`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := s.GetMemoryMeta("legacy")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("migration changed curation")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(path + ".memory-sources-v1-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v %v", backups, err)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var status, integrity string
	if err := backup.QueryRow(`SELECT verification_status FROM memory_meta WHERE doc_id='legacy'`).Scan(&status); err != nil || status != "confirmed" {
		t.Fatalf("backup status=%s %v", status, err)
	}
	if err := backup.QueryRow(`PRAGMA quick_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("backup integrity=%s %v", integrity, err)
	}
	var hasSources bool
	if err := backup.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name='memory_extraction_sources')`).Scan(&hasSources); err != nil || hasSources {
		t.Fatal("backup was made after migration")
	}
	s, err = NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	backups, _ = filepath.Glob(path + ".memory-sources-v1-*.bak")
	if len(backups) != 1 {
		t.Fatal("migration repeated")
	}
}

func automaticTestStore(t *testing.T) *SQLiteMemory {
	t.Helper()
	s, err := NewSQLiteMemory(filepath.Join(t.TempDir(), "memory.db"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestAnalysisSessionIdentityPreservesArchivedFact(t *testing.T) {
	s := automaticTestStore(t)
	v := newTestChromemVectorDB(t, nearIdenticalMemoryEmbeddings)
	concept := "[preference:workflow] User prefers Vim"
	first, err := v.StoreDocumentOwned(concept, "source:memory_analysis session:chat-A", VectorStoreDeduplicate)
	if err != nil {
		t.Fatal(err)
	}
	id := first.CreatedIDs[0]
	if err := s.UpsertMemoryMeta(id); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyMemoryCurationAction(MemoryCurationAction{DocID: id, Action: "archive", Reason: "human archive"}, "user", false); err != nil {
		t.Fatal(err)
	}
	before, _ := s.GetMemoryMeta(id)
	second, err := StoreAutomaticMemoryDocument(s, v, concept, "source:memory_analysis session:chat-B", MemoryMetaUpdate{SourceType: "memory_analysis"})
	if err != nil || !reflect.DeepEqual(second.ReusedIDs, []string{id}) || v.Count() != 1 {
		t.Fatalf("revived: %+v %v", second, err)
	}
	after, _ := s.GetMemoryMeta(id)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("metadata changed on reuse")
	}
	sources, err := s.GetMemoryExtractionSources(id)
	if err != nil || len(sources) != 2 || sources[0].SessionID != "chat-A" || sources[1].SessionID != "chat-B" {
		t.Fatalf("sources: %+v %v", sources, err)
	}
}

func TestAutomaticMemoryRollbackPreservesInterveningChanges(t *testing.T) {
	for _, change := range []string{"none", "confirm", "reuse", "content", "closed"} {
		t.Run(change, func(t *testing.T) {
			s := automaticTestStore(t)
			v := newTestChromemVectorDB(t, nearIdenticalMemoryEmbeddings)
			write, err := StoreAutomaticMemoryDocument(s, v, "[preference] User prefers Vim", "source:memory_analysis session:A", MemoryMetaUpdate{SourceType: "memory_analysis"})
			if err != nil {
				t.Fatal(err)
			}
			id := write.CreatedIDs[0]
			switch change {
			case "confirm":
				err = s.ApplyMemoryCurationAction(MemoryCurationAction{DocID: id, Action: "confirm"}, "user", false)
			case "reuse":
				_, err = StoreAutomaticMemoryDocument(s, v, "[preference] User prefers Vim", "source:memory_analysis session:B", MemoryMetaUpdate{SourceType: "memory_analysis"})
			case "content":
				doc, _ := v.collection.GetByID(t.Context(), id)
				doc.Content = "different fact"
				err = v.collection.AddDocument(t.Context(), doc)
			case "closed":
				err = s.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			err = s.RollbackAutomaticMemoryWrites(v, write.Writes)
			if change == "none" {
				if err != nil || v.Count() != 0 {
					t.Fatalf("rollback: %v count=%d", err, v.Count())
				}
			} else if err == nil || v.Count() != 1 {
				t.Fatalf("unsafe rollback: %v count=%d", err, v.Count())
			}
		})
	}
}

func TestAnalysisIdentityRetainsFactualBoundaries(t *testing.T) {
	base, ok := AnalysisMemoryIdentity("[preference:editor] User prefers Vim", "source:memory_analysis session:A", "work")
	if !ok {
		t.Fatal("missing identity")
	}
	for _, item := range []struct {
		concept, content, domain string
		equal                    bool
	}{
		{" [preference:editor] User prefers Vim\r\n", "source:memory_analysis session:B", " work ", true},
		{"[preference:editor] User prefers Emacs", MemoryAnalysisSourceMarker, "work", false},
		{"[correction:editor] User prefers Vim", MemoryAnalysisSourceMarker, "work", false},
		{"[preference:other] User prefers Vim", MemoryAnalysisSourceMarker, "work", false},
		{"[preference:editor] User prefers Vim", MemoryAnalysisSourceMarker, "home", false},
		{"[preference:editor] User prefers Vim", "source:memory_analysis session:A\nAdditional fact", "work", false},
	} {
		key, ok := AnalysisMemoryIdentity(item.concept, item.content, item.domain)
		if (ok && key == base) != item.equal {
			t.Fatalf("identity: %+v", item)
		}
	}
}

func TestAnalysisParallelSessionsReuseOneDocument(t *testing.T) {
	v := newTestChromemVectorDB(t, nearIdenticalMemoryEmbeddings)
	s := automaticTestStore(t)
	var group sync.WaitGroup
	for _, session := range []string{"A", "B", "C", "D"} {
		group.Go(func() {
			_, err := StoreAutomaticMemoryDocument(s, v, "[preference] User prefers Vim", MemoryAnalysisSourceMarker+" session:"+session, MemoryMetaUpdate{SourceType: "memory_analysis"})
			if err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	if v.Count() != 1 {
		t.Fatalf("count=%d", v.Count())
	}
	metas, _ := s.GetAllMemoryMeta(10, 0)
	for _, meta := range metas {
		sources, err := s.GetMemoryExtractionSources(meta.DocID)
		if err != nil || len(sources) != 4 {
			t.Fatalf("sources=%+v %v", sources, err)
		}
	}
}

type pausedAutomaticReuse struct {
	*ChromemVectorDB
	reused, resume chan struct{}
}

func (v *pausedAutomaticReuse) StoreDocumentOwned(concept, content string, mode VectorStoreMode) (VectorStoreResult, error) {
	result, err := v.ChromemVectorDB.StoreDocumentOwned(concept, content, mode)
	if len(result.ReusedIDs) > 0 {
		close(v.reused)
		<-v.resume
	}
	return result, err
}

func TestAutomaticMemoryRollbackWaitsForConcurrentReuse(t *testing.T) {
	s := automaticTestStore(t)
	v := &pausedAutomaticReuse{ChromemVectorDB: newTestChromemVectorDB(t, nearIdenticalMemoryEmbeddings), reused: make(chan struct{}), resume: make(chan struct{})}
	concept := "[preference] User prefers Vim"
	first, err := StoreAutomaticMemoryDocument(s, v, concept, "source:memory_analysis session:A", MemoryMetaUpdate{SourceType: "memory_analysis"})
	if err != nil {
		t.Fatal(err)
	}
	storeDone := make(chan error, 1)
	go func() {
		_, err := StoreAutomaticMemoryDocument(s, v, concept, "source:memory_analysis session:B", MemoryMetaUpdate{SourceType: "memory_analysis"})
		storeDone <- err
	}()
	<-v.reused
	rollbackDone := make(chan error, 1)
	go func() { rollbackDone <- s.RollbackAutomaticMemoryWrites(v, first.Writes) }()
	select {
	case err := <-rollbackDone:
		close(v.resume)
		<-storeDone
		t.Fatalf("rollback passed in-flight reuse: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(v.resume)
	if err := <-storeDone; err != nil {
		t.Fatal(err)
	}
	if err := <-rollbackDone; err == nil || v.Count() != 1 {
		t.Fatalf("reused memory was removed: %v count=%d", err, v.Count())
	}
}
