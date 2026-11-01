package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
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

func TestPendingAnalysisRetryDefersIfPermissionRevokedDuringPublication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	stm, err := memory.NewSQLiteMemory(path, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	sessionID := "retry-policy-session"
	details := memory.MemoryMetaUpdate{
		VerificationStatus: "unverified",
		SourceType:         "memory_analysis",
		SourceReliability:  .6,
		SourceSessionID:    sessionID,
		SourceRole:         "user_and_assistant",
	}
	if err := stm.EnqueuePendingMemoryWrite(memory.PendingMemoryWrite{
		Concept:  "NAS uses XFS",
		Content:  "source:memory_analysis session:" + sessionID,
		Domain:   "memory_analysis",
		Metadata: &details,
	}, errors.New("embedding unavailable")); err != nil {
		t.Fatal(err)
	}

	baseline := enabledMemoryTestConfig()
	blocked := *baseline
	blocked.Tools.Memory.ReadOnly = true
	var current atomic.Pointer[config.Config]
	current.Store(baseline)
	cfg := *baseline
	cfg.AuthorizationSnapshots = func() (*config.Config, *config.Config) {
		return baseline, current.Load()
	}
	var revoke atomic.Bool
	vector := newPublicationAgentVector(t, stm, func() {
		if revoke.Load() {
			current.Store(&blocked)
		}
	})
	revoke.Store(true)

	if succeeded, failed := retryPendingMemoryWrites(t.Context(), testLogger, stm, vector, &cfg); succeeded != 0 || failed != 0 {
		t.Fatalf("retry=%d/%d, want a deferred write without counting a failure", succeeded, failed)
	}
	if count, err := stm.CountPendingMemoryWrites(); err != nil || count != 1 {
		t.Fatalf("pending writes=%d err=%v, want the queued fact retained", count, err)
	}
	deferred, err := stm.GetDuePendingMemoryWrites(time.Now().UTC().Add(time.Hour), 10)
	if err != nil || len(deferred) != 1 || deferred[0].Attempts != 0 {
		t.Fatalf("revoked write retry budget=%+v err=%v, want one retained write with zero attempts", deferred, err)
	}
	if vector.Count() != 0 {
		t.Fatalf("revoked write published %d vector documents", vector.Count())
	}

	revoke.Store(false)
	current.Store(baseline)
	db, err := dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE pending_memory_writes SET next_attempt_at=? WHERE id=?`, time.Now().UTC().Add(-time.Minute), deferred[0].ID); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	ready, err := stm.GetDuePendingMemoryWrites(time.Now().UTC(), 10)
	if err != nil || len(ready) != 1 || ready[0].Attempts != 0 {
		t.Fatalf("authorized retry was not due without consuming budget: %+v err=%v", ready, err)
	}
	if !runtimeMemoryWriteAllowed(&cfg, "memory", sessionID) {
		t.Fatal("authorized retry config still denies memory writes")
	}
	if succeeded, failed := retryPendingMemoryWrites(t.Context(), testLogger, stm, vector, &cfg); succeeded != 1 || failed != 0 {
		remaining, _ := stm.GetDuePendingMemoryWrites(time.Now().UTC().Add(time.Hour), 10)
		t.Fatalf("authorized retry=%d/%d, pending=%+v, want retained fact to publish", succeeded, failed, remaining)
	}
	if count, err := stm.CountPendingMemoryWrites(); err != nil || count != 0 {
		t.Fatalf("completed queued writes=%d err=%v", count, err)
	}
	metas, err := stm.GetAllMemoryMeta(10, 0)
	if err != nil || len(metas) != 1 || metas[0].SourceType != "memory_analysis" {
		t.Fatalf("authorized retry metadata=%+v err=%v", metas, err)
	}
	sources, err := stm.GetMemoryExtractionSources(metas[0].DocID)
	if err != nil || len(sources) != 1 || sources[0].SessionID != sessionID {
		t.Fatalf("authorized retry provenance=%+v err=%v", sources, err)
	}
	states, err := stm.CheckMemoryEligibility(t.Context(), []string{metas[0].DocID})
	if err != nil || !states[metas[0].DocID].Visible() || vector.Count() != 1 {
		t.Fatalf("authorized fact not visible exactly once: state=%+v vectors=%d err=%v", states[metas[0].DocID], vector.Count(), err)
	}
}

type retryRecoveryVector struct {
	*memory.ChromemVectorDB
	failNextActivation atomic.Bool
	blockActivation    atomic.Bool
	activationStarted  chan struct{}
}

func (v *retryRecoveryVector) ActivateAutomaticMemoryWriteContext(ctx context.Context, publicationID string, receipts []memory.MemoryPublicationDocument) error {
	if v.blockActivation.Load() {
		if v.activationStarted != nil {
			select {
			case v.activationStarted <- struct{}{}:
			default:
			}
		}
		<-ctx.Done()
		return ctx.Err()
	}
	if v.failNextActivation.CompareAndSwap(true, false) {
		return errors.New("connection reset by activation fixture")
	}
	return v.ChromemVectorDB.ActivateAutomaticMemoryWriteContext(ctx, publicationID, receipts)
}

func TestPendingMemoryRetryRecoversReadyPublicationWithCurrentPermission(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	stm, err := memory.NewSQLiteMemory(path, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	details := memory.MemoryMetaUpdate{
		VerificationStatus: "unverified",
		SourceType:         "memory_analysis",
		SourceReliability:  .6,
		SourceSessionID:    "retry-recovery-session",
		SourceRole:         "user_and_assistant",
	}
	if err := stm.EnqueuePendingMemoryWrite(memory.PendingMemoryWrite{
		Concept:  "NAS uses XFS",
		Content:  "source:memory_analysis session:" + details.SourceSessionID,
		Domain:   "memory_analysis",
		Metadata: &details,
	}, errors.New("embedding unavailable")); err != nil {
		t.Fatal(err)
	}
	var embeds atomic.Int32
	vector := &retryRecoveryVector{ChromemVectorDB: newPublicationAgentVector(t, stm, func() { embeds.Add(1) })}
	vector.failNextActivation.Store(true)
	readyCfg := enabledMemoryTestConfig()
	if succeeded, failed := retryPendingMemoryWrites(t.Context(), testLogger, stm, vector, readyCfg); succeeded != 0 || failed != 1 {
		t.Fatalf("initial retry=%d/%d, want activation failure", succeeded, failed)
	}
	metas, err := stm.GetAllMemoryMeta(10, 0)
	if err != nil || len(metas) != 1 {
		t.Fatalf("failed publication metadata=%+v err=%v", metas, err)
	}
	docID := metas[0].DocID
	states, err := stm.CheckMemoryEligibility(t.Context(), []string{docID})
	if err != nil || states[docID].Visible() || !states[docID].PublicationPending {
		t.Fatalf("ready publication was not hidden: state=%+v err=%v", states[docID], err)
	}
	embedsAfterPrepare := embeds.Load()
	if embedsAfterPrepare == 0 {
		t.Fatal("initial publication did not create an embedding")
	}

	readOnlyCfg := *readyCfg
	readOnlyCfg.Tools.Memory.ReadOnly = true
	if succeeded, failed := retryPendingMemoryWrites(t.Context(), testLogger, stm, vector, &readOnlyCfg); succeeded != 0 || failed != 0 {
		t.Fatalf("read-only recovery=%d/%d, want a deferred recovery", succeeded, failed)
	}
	states, err = stm.CheckMemoryEligibility(t.Context(), []string{docID})
	if err != nil || states[docID].Visible() || embeds.Load() != embedsAfterPrepare || vector.Count() != 1 {
		t.Fatalf("read-only recovery exposed or re-embedded fact: state=%+v embeds=%d vectors=%d err=%v", states[docID], embeds.Load(), vector.Count(), err)
	}

	if succeeded, failed := retryPendingMemoryWrites(t.Context(), testLogger, stm, vector, readyCfg); succeeded != 0 || failed != 0 {
		t.Fatalf("authorized recovery=%d/%d, want recovery without a due queue write", succeeded, failed)
	}
	states, err = stm.CheckMemoryEligibility(t.Context(), []string{docID})
	if err != nil || !states[docID].Visible() || embeds.Load() != embedsAfterPrepare || vector.Count() != 1 {
		t.Fatalf("authorized recovery failed or duplicated fact: state=%+v embeds=%d vectors=%d err=%v", states[docID], embeds.Load(), vector.Count(), err)
	}
	queued, err := stm.GetDuePendingMemoryWrites(time.Now().UTC().Add(time.Hour), 10)
	if err != nil || len(queued) != 1 || queued[0].Attempts != 0 {
		t.Fatalf("recovery changed queued retry budget: %+v err=%v", queued, err)
	}
}

func TestPendingMemoryRetryRecoversReadyPublicationWithEmptyQueue(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", testLogger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	var embeds atomic.Int32
	vector := &retryRecoveryVector{ChromemVectorDB: newPublicationAgentVector(t, stm, func() { embeds.Add(1) })}
	vector.failNextActivation.Store(true)
	details := memory.MemoryMetaUpdate{SourceType: "chat", SourceReliability: .6, SourceSessionID: "empty-queue-session", SourceRole: "user_and_assistant", VerificationStatus: "unverified"}
	if _, err := publishModelMemory(t.Context(), enabledMemoryTestConfig(), stm, vector, "User prefers XFS on the NAS", "source transcript", details, nil); err == nil {
		t.Fatal("fixture activation failure was not returned")
	}
	metas, err := stm.GetAllMemoryMeta(10, 0)
	if err != nil || len(metas) != 1 {
		t.Fatalf("failed publication metadata=%+v err=%v", metas, err)
	}
	docID := metas[0].DocID
	before := embeds.Load()
	if count, err := stm.CountPendingMemoryWrites(); err != nil || count != 0 {
		t.Fatalf("fixture unexpectedly queued work: count=%d err=%v", count, err)
	}
	if succeeded, failed := retryPendingMemoryWrites(t.Context(), testLogger, stm, vector, enabledMemoryTestConfig()); succeeded != 0 || failed != 0 {
		t.Fatalf("empty-queue recovery=%d/%d", succeeded, failed)
	}
	states, err := stm.CheckMemoryEligibility(t.Context(), []string{docID})
	if err != nil || !states[docID].Visible() || vector.Count() != 1 || embeds.Load() != before {
		t.Fatalf("empty-queue journal was not recovered exactly once: state=%+v vectors=%d embeds=%d/%d err=%v", states[docID], vector.Count(), before, embeds.Load(), err)
	}
}

func TestPendingMemoryRetryRecoveryDrainsOnCancellation(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", testLogger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	vector := &retryRecoveryVector{ChromemVectorDB: newPublicationAgentVector(t, stm, nil), activationStarted: make(chan struct{}, 1)}
	vector.failNextActivation.Store(true)
	details := memory.MemoryMetaUpdate{SourceType: "chat", SourceReliability: .6, SourceSessionID: "cancel-recovery-session", SourceRole: "user_and_assistant", VerificationStatus: "unverified"}
	if _, err := publishModelMemory(t.Context(), enabledMemoryTestConfig(), stm, vector, "User prefers XFS on the NAS", "source transcript", details, nil); err == nil {
		t.Fatal("fixture activation failure was not returned")
	}
	metas, err := stm.GetAllMemoryMeta(10, 0)
	if err != nil || len(metas) != 1 {
		t.Fatalf("failed publication metadata=%+v err=%v", metas, err)
	}
	docID := metas[0].DocID
	vector.blockActivation.Store(true)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct {
		succeeded int
		failed    int
	}, 1)
	go func() {
		succeeded, failed := retryPendingMemoryWrites(ctx, testLogger, stm, vector, enabledMemoryTestConfig())
		done <- struct {
			succeeded int
			failed    int
		}{succeeded, failed}
	}()
	select {
	case <-vector.activationStarted:
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("recovery did not start activation")
	}
	cancel()
	select {
	case result := <-done:
		if result.succeeded != 0 || result.failed != 0 {
			t.Fatalf("cancelled recovery processed queue: %+v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("recovery did not drain after cancellation")
	}
	states, err := stm.CheckMemoryEligibility(t.Context(), []string{docID})
	if err != nil || states[docID].Visible() || !states[docID].PublicationPending {
		t.Fatalf("cancelled recovery changed journal visibility: state=%+v err=%v", states[docID], err)
	}
}

func TestPendingMemoryRetryDefersMatchingJournalAndPublishesUnrelatedFact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	stm, err := memory.NewSQLiteMemory(path, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	var embeds atomic.Int32
	vector := &retryRecoveryVector{ChromemVectorDB: newPublicationAgentVector(t, stm, func() { embeds.Add(1) })}
	readyCfg := enabledMemoryTestConfig()
	journalDetails := memory.MemoryMetaUpdate{SourceType: "chat", SourceReliability: .6, SourceSessionID: "journal-session", SourceRole: "user_and_assistant", VerificationStatus: "unverified"}
	matchingConcept := "The user prefers XFS on the NAS."
	matchingContent := "source transcript: The user prefers XFS on the NAS."
	vector.failNextActivation.Store(true)
	if _, err := publishModelMemory(t.Context(), readyCfg, stm, vector, matchingConcept, matchingContent, journalDetails, nil); err == nil {
		t.Fatal("fixture activation failure was not returned")
	}
	metas, err := stm.GetAllMemoryMeta(10, 0)
	if err != nil || len(metas) != 1 {
		t.Fatalf("failed publication metadata=%+v err=%v", metas, err)
	}
	journalDocID := metas[0].DocID

	matchingDetails := memory.MemoryMetaUpdate{SourceType: "chat", SourceReliability: .6, SourceSessionID: "queued-session", SourceRole: "user_and_assistant", VerificationStatus: "unverified"}
	unrelatedDetails := memory.MemoryMetaUpdate{SourceType: "chat", SourceReliability: .6, SourceSessionID: "unrelated-session", SourceRole: "user_and_assistant", VerificationStatus: "unverified"}
	if err := stm.EnqueuePendingMemoryWrite(memory.PendingMemoryWrite{Concept: matchingConcept, Content: matchingContent, Domain: "chat", Metadata: &matchingDetails}, errors.New("initial embedding unavailable")); err != nil {
		t.Fatal(err)
	}
	if err := stm.EnqueuePendingMemoryWrite(memory.PendingMemoryWrite{Concept: "The user stores bicycle equipment in the garden shed.", Content: "source transcript: the user stores bicycle equipment in the garden shed", Domain: "chat", Metadata: &unrelatedDetails}, errors.New("initial embedding unavailable")); err != nil {
		t.Fatal(err)
	}
	db, err := dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE pending_memory_writes SET next_attempt_at=?`, time.Now().UTC().Add(-time.Minute)); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	vector.failNextActivation.Store(true)
	if succeeded, failed := retryPendingMemoryWrites(t.Context(), testLogger, stm, vector, readyCfg); succeeded != 1 || failed != 0 {
		t.Fatalf("retry with unresolved journal=%d/%d, want unrelated fact to complete", succeeded, failed)
	}
	queued, err := stm.GetDuePendingMemoryWrites(time.Now().UTC().Add(time.Hour), 10)
	if err != nil || len(queued) != 1 || queued[0].Concept != matchingConcept || queued[0].Attempts != 0 {
		t.Fatalf("matching queued fact was not deferred without retry loss: %+v err=%v", queued, err)
	}
	metas, err = stm.GetAllMemoryMeta(10, 0)
	if err != nil || len(metas) != 2 || vector.Count() != 2 {
		t.Fatalf("unrelated publication or duplicate suppression failed: metadata=%+v vectors=%d err=%v", metas, vector.Count(), err)
	}
	var unrelatedID string
	for _, meta := range metas {
		if meta.DocID != journalDocID {
			unrelatedID = meta.DocID
		}
	}
	states, err := stm.CheckMemoryEligibility(t.Context(), []string{journalDocID, unrelatedID})
	if err != nil || states[journalDocID].Visible() || !states[journalDocID].PublicationPending || !states[unrelatedID].Visible() {
		t.Fatalf("journal/independent visibility mismatch: journal=%+v unrelated=%+v err=%v", states[journalDocID], states[unrelatedID], err)
	}
}
