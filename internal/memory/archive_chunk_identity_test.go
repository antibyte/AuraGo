package memory

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveChunkedAutomaticDocumentBlocksReextraction(t *testing.T) {
	for _, mode := range []string{"single", "batch", "budget", "backfill"} {
		t.Run(mode, func(t *testing.T) {
			cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
			s := cv.fileIndexMemory.Load()
			const concept = "Long backup fact"
			content := strings.Repeat("The device keeps its backup on the private local NAS. ", 150)
			first, err := StoreAutomaticMemoryDocumentWithContext(t.Context(), s, cv, concept, content, MemoryMetaUpdate{})
			if err != nil || len(first.CreatedIDs) < 2 {
				t.Fatalf("chunked write=%+v err=%v", first, err)
			}
			switch mode {
			case "single":
				for _, id := range first.CreatedIDs {
					if err := s.ApplyMemoryCurationActionContext(t.Context(), MemoryCurationAction{DocID: id, Action: "archive"}, "admin", false); err != nil {
						t.Fatal(err)
					}
				}
			case "batch":
				applied, err := s.ArchiveMemoryMetaBatchContext(t.Context(), first.CreatedIDs, "archive chunked source", "admin")
				if err != nil || len(applied) != len(first.CreatedIDs) {
					t.Fatalf("archived=%v err=%v", applied, err)
				}
			case "budget":
				keep, err := StoreAutomaticMemoryDocumentWithContext(t.Context(), s, cv, "Keep", "Keep this unrelated fact", MemoryMetaUpdate{})
				if err != nil || len(keep.CreatedIDs) != 1 {
					t.Fatalf("keep=%+v err=%v", keep, err)
				}
				if err := s.SetMemoryMetaProtection(keep.CreatedIDs[0], true, false); err != nil {
					t.Fatal(err)
				}
				count, err := s.ApplyMemoryBudgetEnforcementContext(t.Context(), 1, cv)
				if err != nil || count != len(first.CreatedIDs) {
					t.Fatalf("budget archived=%d err=%v", count, err)
				}
			case "backfill":
				for _, id := range first.CreatedIDs {
					if _, err := s.db.Exec(`UPDATE memory_meta SET verification_status='archived' WHERE doc_id=?`, id); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := s.db.Exec(`DELETE FROM memory_maintenance_meta WHERE key='archive_identity_backfill_v1'`); err != nil {
					t.Fatal(err)
				}
				s.automaticMemoryMu.Lock()
				err := s.backfillArchiveIdentities(t.Context(), cv)
				s.automaticMemoryMu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := cv.DeleteDocuments(first.CreatedIDs); err != nil {
				t.Fatal(err)
			}
			second, err := StoreAutomaticMemoryDocumentWithContext(t.Context(), s, cv, concept, content, MemoryMetaUpdate{})
			if !errors.Is(err, ErrAutomaticMemorySkipped) {
				t.Fatalf("archived source was reinserted: write=%+v err=%v", second, err)
			}
		})
	}
}

func TestArchiveChunkedIdentityRestoreKeepsRemainingArchivesBlocked(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
	s := cv.fileIndexMemory.Load()
	const concept = "Long backup fact"
	content := strings.Repeat("The device keeps its backup on the private local NAS. ", 150)
	stored, err := StoreAutomaticMemoryDocumentWithContext(t.Context(), s, cv, concept, content, MemoryMetaUpdate{})
	if err != nil || len(stored.CreatedIDs) < 2 {
		t.Fatalf("chunked write=%+v err=%v", stored, err)
	}
	if _, err := s.ArchiveMemoryMetaBatchContext(t.Context(), stored.CreatedIDs, "archive chunked source", "admin"); err != nil {
		t.Fatal(err)
	}
	identity := memoryDocumentIdentity(buildContentString(concept, content), "")
	for index, id := range stored.CreatedIDs {
		if err := s.ApplyMemoryCurationActionContext(t.Context(), MemoryCurationAction{DocID: id, Action: "confirm"}, "admin", false); err != nil {
			t.Fatal(err)
		}
		blocked, err := s.archivedIdentityBlocked(t.Context(), identity)
		if wantBlocked := index < len(stored.CreatedIDs)-1; err != nil || blocked != wantBlocked {
			t.Fatalf("restored=%d blocked=%v want=%v err=%v", index+1, blocked, wantBlocked, err)
		}
	}
}
