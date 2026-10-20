package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestChatPrivacyDeletionRetainsMemoryArchivedAfterPreparation(t *testing.T) {
	store := openPrivacyJobsStore(t, filepath.Join(t.TempDir(), "memory.db"))
	kg := openPrivacyJobsGraph(t)
	const sessionID = "chat-conflict-archive"
	addPrivacyJobTranscript(t, store, sessionID)
	addPrivacyJobOwnedMemory(t, store, "losing-fact", sessionID)
	if err := store.UpsertMemoryMetaWithDetails("winning-fact", MemoryMetaUpdate{SourceType: "memory_analysis"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RegisterMemoryConflict("losing-fact", "winning-fact", "same-fact", "old", "new", "test"); err != nil {
		t.Fatal(err)
	}
	conflicts, err := store.GetOpenMemoryConflicts(10)
	if err != nil || len(conflicts) != 1 {
		t.Fatalf("open conflicts=%d err=%v", len(conflicts), err)
	}
	vectors := &privacyJobsVectors{documents: map[string]string{"losing-fact": "private fact"}}
	job, err := store.PrepareChatPrivacyDeletion(context.Background(), sessionID, vectors, kg, true)
	if err != nil || job.DeletionStatus != PrivacyDeletionPending {
		t.Fatalf("prepare job=%+v err=%v", job, err)
	}
	if err := store.ResolveMemoryConflict(conflicts[0].ID, "winning-fact", "reviewed winner"); err != nil {
		t.Fatal(err)
	}
	if err := store.ProcessDuePrivacyDeletionJobs(context.Background(), vectors, kg, time.Now().UTC().Add(time.Minute), 10); err != nil {
		t.Fatal(err)
	}
	if _, exists := vectors.documents["losing-fact"]; !exists {
		t.Fatal("privacy retry deleted a memory archived after preparation")
	}
	job, err = store.GetPrivacyDeletionJob(context.Background(), job.DeletionJobID)
	if err != nil || job.DeletionStatus != PrivacyDeletionRetainedReview || job.RetainedCount != 1 {
		t.Fatalf("job after changed semantic metadata=%+v err=%v", job, err)
	}
}

func TestChatPrivacyDeletionIgnoresUsageOnlyMetadataChanges(t *testing.T) {
	store := openPrivacyJobsStore(t, filepath.Join(t.TempDir(), "memory.db"))
	kg := openPrivacyJobsGraph(t)
	const sessionID = "chat-usage-update"
	addPrivacyJobTranscript(t, store, sessionID)
	addPrivacyJobOwnedMemory(t, store, "used-fact", sessionID)
	vectors := &privacyJobsVectors{documents: map[string]string{"used-fact": "private fact"}}
	job, err := store.PrepareChatPrivacyDeletion(context.Background(), sessionID, vectors, kg, true)
	if err != nil || job.DeletionStatus != PrivacyDeletionPending {
		t.Fatalf("prepare job=%+v err=%v", job, err)
	}
	if _, err := store.db.Exec(`UPDATE memory_meta SET access_count=access_count+1,last_accessed=CURRENT_TIMESTAMP,useful_count=useful_count+1,useless_count=useless_count+1,last_effectiveness_at=CURRENT_TIMESTAMP WHERE doc_id='used-fact'`); err != nil {
		t.Fatal(err)
	}
	if err := store.ProcessDuePrivacyDeletionJobs(context.Background(), vectors, kg, time.Now().UTC().Add(time.Minute), 10); err != nil {
		t.Fatal(err)
	}
	if _, exists := vectors.documents["used-fact"]; exists {
		t.Fatal("usage-only metadata change invalidated the authorized deletion")
	}
	job, err = store.GetPrivacyDeletionJob(context.Background(), job.DeletionJobID)
	if err != nil || job.DeletionStatus != PrivacyDeletionCompleted || job.DeletedCount != 1 {
		t.Fatalf("job after usage-only metadata changes=%+v err=%v", job, err)
	}
}

func TestChatPrivacyDeletionReconcilesMissingMetadataAndVectorConservatively(t *testing.T) {
	for _, vectorPresent := range []bool{false, true} {
		name := "both_missing"
		if vectorPresent {
			name = "metadata_missing_vector_present"
		}
		t.Run(name, func(t *testing.T) {
			store := openPrivacyJobsStore(t, filepath.Join(t.TempDir(), "memory.db"))
			kg := openPrivacyJobsGraph(t)
			const sessionID = "chat-missing-target"
			addPrivacyJobTranscript(t, store, sessionID)
			addPrivacyJobOwnedMemory(t, store, "missing-fact", sessionID)
			vectors := &privacyJobsVectors{documents: map[string]string{"missing-fact": "private fact"}}
			job, err := store.PrepareChatPrivacyDeletion(context.Background(), sessionID, vectors, kg, true)
			if err != nil || job.DeletionStatus != PrivacyDeletionPending {
				t.Fatalf("prepare job=%+v err=%v", job, err)
			}
			if _, err := store.db.Exec(`DELETE FROM memory_meta WHERE doc_id='missing-fact'; DELETE FROM memory_extraction_sources WHERE doc_id='missing-fact'`); err != nil {
				t.Fatal(err)
			}
			if !vectorPresent {
				delete(vectors.documents, "missing-fact")
			}
			if err := store.ProcessDuePrivacyDeletionJobs(context.Background(), vectors, kg, time.Now().UTC().Add(time.Minute), 10); err != nil {
				t.Fatal(err)
			}
			job, err = store.GetPrivacyDeletionJob(context.Background(), job.DeletionJobID)
			if err != nil {
				t.Fatal(err)
			}
			if vectorPresent {
				if job.DeletionStatus != PrivacyDeletionRetainedReview || job.RetainedCount != 1 {
					t.Fatalf("metadata-missing job=%+v, want retained_review", job)
				}
				if _, exists := vectors.documents["missing-fact"]; !exists {
					t.Fatal("vector was deleted while its authorization metadata was missing")
				}
			} else if job.DeletionStatus != PrivacyDeletionCompleted || job.DeletedCount != 1 {
				t.Fatalf("both-missing job=%+v, want completed", job)
			}
		})
	}
}

func TestPrivacyPublicationMarkerReadPropagatesDatabaseErrors(t *testing.T) {
	store := openPrivacyJobsStore(t, filepath.Join(t.TempDir(), "memory.db"))
	if _, err := store.db.Exec(`DROP TABLE memory_maintenance_meta`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.privacyPublicationMarkerState(context.Background(), "missing"); err == nil {
		t.Fatal("marker read failure was swallowed")
	}
}
