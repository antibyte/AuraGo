package memory

import (
	"path/filepath"
	"testing"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

func TestPrivacyCorrectionCleanupResumesAfterCommittedCorpusDeletion(t *testing.T) {
	for _, mode := range []string{"acknowledgement_failed", "orphan_index"} {
		t.Run(mode, func(t *testing.T) {
			cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
			s := cv.fileIndexMemory.Load()
			kg := newTestKG(t)
			const sessionID, correctionID, id = "private-session", "private-correction", "mc_private_unpublished"
			prepareStagedCorrectionVector(t, s, cv, sessionID, correctionID, id, "unpublished replacement")
			job, err := s.PrepareChatPrivacyDeletion(t.Context(), sessionID, cv, kg, false)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "acknowledgement_failed" {
				if _, err := s.db.Exec(`CREATE TRIGGER reject_correction_cleanup BEFORE DELETE ON memory_correction_vectors BEGIN SELECT RAISE(ABORT,'injected acknowledgement failure'); END`); err != nil {
					t.Fatal(err)
				}
				if err := s.ProcessDuePrivacyDeletionJobs(t.Context(), cv, kg, time.Now().Add(time.Minute), 10); err == nil {
					t.Fatal("expected acknowledgement failure")
				}
				if _, err := s.db.Exec(`DROP TRIGGER reject_correction_cleanup`); err != nil {
					t.Fatal(err)
				}
			} else if _, err := cv.corpus.db.Exec(`DELETE FROM documents WHERE collection='aurago_memories' AND id=?`, id); err != nil {
				t.Fatal(err)
			}
			var path string
			if err := s.db.QueryRow(`SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&path); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			s, err = NewSQLiteMemory(path, cv.logger)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { s.Close() })
			cv.BindMemoryMetadata(s)
			if err := s.ProcessDuePrivacyDeletionJobs(t.Context(), cv, kg, time.Now().Add(24*time.Hour), 10); err != nil {
				t.Fatal(err)
			}
			status, err := s.GetPrivacyDeletionJob(t.Context(), job.DeletionJobID)
			if err != nil || status.DeletionStatus != PrivacyDeletionCompleted || status.PendingCount != 0 {
				t.Fatalf("job=%+v err=%v", status, err)
			}
			var pending int
			if err := s.db.QueryRow(`SELECT COUNT(*) FROM memory_correction_vectors WHERE doc_id=?`, id).Scan(&pending); err != nil || pending != 0 {
				t.Fatalf("remaining owner receipts=%d err=%v", pending, err)
			}
			if s.hasPrivacyPublicationMarker(t.Context(), job.DeletionJobID) {
				t.Fatal("completed privacy cleanup still blocks coordinated snapshots")
			}
			if _, err := cv.collection.GetByID(t.Context(), id); err == nil {
				t.Fatal("orphan correction index entry survived")
			}
		})
	}
}

func TestPrivacyCorrectionCleanupRetainsForeignOrphanIndex(t *testing.T) {
	cv := newCorpusTestVectorDB(t, filepath.Join(t.TempDir(), "corpus"))
	s := cv.fileIndexMemory.Load()
	kg := newTestKG(t)
	const sessionID, correctionID, id = "private-session", "private-correction", "mc_foreign_orphan"
	prepareStagedCorrectionVector(t, s, cv, sessionID, correctionID, id, "unpublished replacement")
	job, err := s.PrepareChatPrivacyDeletion(t.Context(), sessionID, cv, kg, false)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cv.collection.GetByID(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	doc.Metadata["source_type"] = "foreign"
	if err := cv.collection.AddDocuments(t.Context(), []chromem.Document{doc}, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := cv.corpus.db.Exec(`DELETE FROM documents WHERE collection='aurago_memories' AND id=?`, id); err != nil {
		t.Fatal(err)
	}
	if err := s.ProcessDuePrivacyDeletionJobs(t.Context(), cv, kg, time.Now().Add(time.Minute), 10); err == nil {
		t.Fatal("expected uncertain ownership to block deletion")
	}
	status, err := s.GetPrivacyDeletionJob(t.Context(), job.DeletionJobID)
	if err != nil || status.DeletionStatus == PrivacyDeletionCompleted || status.PendingCount != 1 {
		t.Fatalf("job=%+v err=%v", status, err)
	}
	retained, err := cv.collection.GetByID(t.Context(), id)
	if err != nil || retained.Metadata["source_type"] != "foreign" {
		t.Fatalf("foreign index entry was changed: doc=%+v err=%v", retained, err)
	}
}
