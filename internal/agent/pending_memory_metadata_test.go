package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/dbutil"
	"aurago/internal/memory"
)

func TestAuditQueuedAnalysisPreservesExtractionConfidence(t *testing.T) {
	for _, kind := range []string{"fact", "preference", "correction"} {
		t.Run(kind, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			path := filepath.Join(t.TempDir(), "memory.db")
			stm, err := memory.NewSQLiteMemory(path, logger)
			if err != nil {
				t.Fatal(err)
			}
			result := memoryAnalysisResult{human: "FYI the NAS uses an XFS filesystem."}
			fact := extractedFact{Content: "The NAS uses an XFS filesystem.", Category: "fact", Confidence: .99, Quote: "the NAS uses an XFS filesystem"}
			switch kind {
			case "fact":
				result.Facts = []extractedFact{fact}
			case "preference":
				result.Preferences = []extractedFact{fact}
			case "correction":
				result.Corrections = []extractedFact{fact}
			}
			vdb := &fakeVectorDB{storeErr: errors.New("provider unavailable")}
			applyMemoryAnalysisResult(enabledMemoryTestConfig(), logger, stm, vdb, "synthetic-session", result)
			if count, err := stm.CountPendingMemoryWrites(); err != nil || count != 1 {
				t.Fatalf("queue count=%d err=%v", count, err)
			}
			if err := stm.Close(); err != nil {
				t.Fatal(err)
			}
			stm, err = memory.NewSQLiteMemory(path, logger)
			if err != nil {
				t.Fatal(err)
			}
			defer stm.Close()
			queued, err := stm.GetDuePendingMemoryWrites(time.Now().UTC().Add(time.Minute), 10)
			if err != nil || len(queued) != 1 || queued[0].Metadata == nil || queued[0].Metadata.SourceRole != "user_and_assistant" || queued[0].Metadata.SourceSessionID != "synthetic-session" {
				t.Fatalf("source provenance lost after restart: %+v %v", queued, err)
			}
			vector := newPublicationAgentVector(t, stm, nil)
			if succeeded, failed := retryPendingMemoryWrites(context.Background(), logger, stm, vector); succeeded != 1 || failed != 0 {
				t.Fatalf("retry %d successful, %d failed", succeeded, failed)
			}
			metas, err := stm.GetAllMemoryMeta(10, 0)
			wantReliability := .6 // Assistant context must never imply human confirmation.
			if err != nil || len(metas) != 1 || metas[0].ExtractionConfidence != .99 || metas[0].SourceReliability != wantReliability || metas[0].SourceType != "memory_analysis" || metas[0].VerificationStatus != "unverified" {
				t.Fatalf("retry metadata=%+v err=%v", metas, err)
			}
			texts, _, err := vector.SearchMemoriesOnly("NAS filesystem", 10)
			if err != nil || len(texts) != 1 || !strings.Contains(texts[0], "role:user_and_assistant") {
				t.Fatalf("retry source role=%v err=%v", texts, err)
			}
		})
	}
}

func TestPendingMemoryMetadataRetryRejectsCorruptionAndPreservesCuration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	path := filepath.Join(t.TempDir(), "memory.db")
	stm, err := memory.NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	details := memory.MemoryMetaUpdate{ExtractionConfidence: .99, VerificationStatus: "unverified", SourceType: "memory_analysis", SourceReliability: .85}
	if err := stm.EnqueuePendingMemoryWrite(memory.PendingMemoryWrite{Concept: "fact", Content: "content", Metadata: &details}, nil); err != nil {
		t.Fatal(err)
	}
	db, err := dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE pending_memory_writes SET metadata_json='broken'`); err != nil {
		t.Fatal(err)
	}
	corruptVector := &fakeVectorDB{}
	if succeeded, failed := retryPendingMemoryWrites(context.Background(), logger, stm, corruptVector); succeeded != 0 || failed != 1 || len(corruptVector.storedConcepts) != 0 {
		t.Fatalf("corrupt retry wrote vectors: %d/%d %v", succeeded, failed, corruptVector.storedConcepts)
	}
	if _, err := db.Exec(`DELETE FROM pending_memory_writes`); err != nil {
		t.Fatal(err)
	}
	if err := stm.EnqueuePendingMemoryWrite(memory.PendingMemoryWrite{Concept: "fact", Content: "content", Metadata: &details}, nil); err != nil {
		t.Fatal(err)
	}
	vdb := newPublicationAgentVector(t, stm, nil)
	existing, err := memory.StoreAutomaticMemoryDocumentWithContext(t.Context(), stm, vdb, "fact", "content", memory.MemoryMetaUpdate{
		ExtractionConfidence: .88, VerificationStatus: "confirmed", SourceType: "user", SourceReliability: 1,
	})
	if err != nil || len(existing.CreatedIDs) != 1 {
		t.Fatalf("store curated source fixture=%+v err=%v", existing, err)
	}
	storedID := existing.CreatedIDs[0]
	if err := stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: storedID, Action: memory.MemoryCurationActionProtect}, "admin", false); err != nil {
		t.Fatal(err)
	}
	before, err := stm.GetMemoryMeta(storedID)
	if err != nil {
		t.Fatal(err)
	}
	if succeeded, failed := retryPendingMemoryWrites(context.Background(), logger, stm, vdb); succeeded != 1 || failed != 0 {
		t.Fatalf("curated retry = %d/%d", succeeded, failed)
	}
	after, err := stm.GetMemoryMeta(storedID)
	if err != nil || before.ExtractionConfidence != after.ExtractionConfidence || !after.Protected || after.VerificationStatus != "confirmed" || after.SourceType != "user" {
		t.Fatalf("retry overwrote curation: before=%+v after=%+v err=%v", before, after, err)
	}
	if due, err := stm.GetDuePendingMemoryWrites(time.Now().Add(time.Hour), 20); err != nil || len(due) != 0 {
		t.Fatalf("completed queue=%v err=%v", due, err)
	}
}

func TestPendingAnalysisRetryClearsTemporaryExpiry(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", testLogger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	vector := newPublicationAgentVector(t, stm, nil)
	const sessionID = "retry-expiry-session"
	concept := "[personal] User is on vacation until tomorrow"
	content := "source:memory_analysis session:" + sessionID
	details := memory.MemoryMetaUpdate{
		ExtractionConfidence: .95,
		VerificationStatus:   "unverified",
		SourceType:           "memory_analysis",
		SourceReliability:    .6,
		SourceSessionID:      sessionID,
		SourceRole:           "user_and_assistant",
	}
	seedExpiresAt := time.Now().UTC().Add(24 * time.Hour)
	seed, err := memory.StoreAutomaticMemoryDocumentWithOptionsContext(t.Context(), stm, vector, concept, content, details, memory.AutomaticMemoryWriteOptions{ExpiresAt: &seedExpiresAt})
	if err != nil || len(seed.CreatedIDs) != 1 {
		t.Fatalf("store expired memory fixture=%+v err=%v", seed, err)
	}
	docID := seed.CreatedIDs[0]
	if got, ok, err := stm.MemoryExpiry(t.Context(), docID); err != nil || !ok || !got.Equal(seedExpiresAt.Truncate(time.Second)) {
		t.Fatalf("seed expiry=%v present=%v err=%v", got, ok, err)
	}
	if err := stm.EnqueuePendingMemoryWrite(memory.PendingMemoryWrite{Concept: concept, Content: content, Domain: "memory_analysis", Metadata: &details}, nil); err != nil {
		t.Fatal(err)
	}

	if succeeded, failed := retryPendingMemoryWrites(t.Context(), testLogger, stm, vector, enabledMemoryTestConfig()); succeeded != 1 || failed != 0 {
		t.Fatalf("retry=%d/%d", succeeded, failed)
	}
	remainingExpiry, ok, err := stm.MemoryExpiry(t.Context(), docID)
	if err != nil || ok {
		metas, _ := stm.GetAllMemoryMeta(10, 0)
		texts, vectorIDs, _ := vector.SearchMemoriesOnly("vacation", 10)
		t.Fatalf("permanent retry left expiry=%v present=%v err=%v; original=%s metadata=%+v vector_count=%d ids=%v texts=%v", remainingExpiry, ok, err, docID, metas, vector.Count(), vectorIDs, texts)
	}
	if vector.Count() != 1 {
		t.Fatalf("retry created a duplicate vector document: %d", vector.Count())
	}
	states, err := stm.CheckMemoryEligibility(t.Context(), []string{docID})
	if err != nil || !states[docID].Visible() {
		t.Fatalf("retried permanent fact is not visible: %+v err=%v", states[docID], err)
	}
}
