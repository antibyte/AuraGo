package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

func newAutomaticPublicationVector(t *testing.T, dir string, stm *SQLiteMemory, embed chromem.EmbeddingFunc) *ChromemVectorDB {
	return newAutomaticPublicationVectorWithFingerprint(t, dir, stm, embed, "test|embedding|3")
}

func newAutomaticPublicationVectorWithFingerprint(t *testing.T, dir string, stm *SQLiteMemory, embed chromem.EmbeddingFunc, fingerprint string) *ChromemVectorDB {
	t.Helper()
	cv := newTestChromemVectorDB(t, embed)
	cv.embeddingFingerprint = fingerprint
	corpus, err := openDocumentCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	cv.corpus = corpus
	cv.dataDir = dir
	cv.embeddingDimension.Store(3)
	cv.BindMemoryMetadata(stm)
	if err := cv.loadCorpusIndex(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cv.Close() })
	return cv
}

func automaticPublicationFixture(t *testing.T, embed chromem.EmbeddingFunc) (*SQLiteMemory, *ChromemVectorDB, string) {
	t.Helper()
	root := t.TempDir()
	stm, err := NewSQLiteMemory(filepath.Join(root, "memory.db"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	vectorDir := filepath.Join(root, "vectors")
	return stm, newAutomaticPublicationVector(t, vectorDir, stm, embed), vectorDir
}

func publishTestMemory(t *testing.T, stm *SQLiteMemory, vector VectorDB, content string, expiresAt *time.Time) (AutomaticMemoryStoreResult, error) {
	t.Helper()
	return StoreAutomaticMemoryDocumentWithOptionsContext(t.Context(), stm, vector, "[preference] User prefers Vim", content, MemoryMetaUpdate{
		SourceType: "memory_analysis", SourceSessionID: "default", VerificationStatus: "unverified",
	}, AutomaticMemoryWriteOptions{ExpiresAt: expiresAt, Validate: func() error { return nil }})
}

type postActivationPolicyFlipVector struct {
	*ChromemVectorDB
	flip func()
}

func (v *postActivationPolicyFlipVector) ActivateAutomaticMemoryWriteContext(ctx context.Context, publicationID string, receipts []MemoryPublicationDocument) error {
	if err := v.ChromemVectorDB.ActivateAutomaticMemoryWriteContext(ctx, publicationID, receipts); err != nil {
		return err
	}
	if v.flip != nil {
		flip := v.flip
		v.flip = nil
		flip()
	}
	return nil
}

func TestAutomaticMemoryPublicationRollsBackAfterMetadataAbort(t *testing.T) {
	stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	if _, err := stm.db.Exec(`CREATE TRIGGER fail_memory_source BEFORE INSERT ON memory_extraction_sources BEGIN SELECT RAISE(ABORT,'injected source failure'); END`); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(24 * time.Hour)
	result, err := publishTestMemory(t, stm, vector, "Vim is the preferred editor.", &expires)
	if err == nil || len(result.CreatedIDs) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var metadata, sources, identities, expiry, journals, corpus int
	for query, dest := range map[string]*int{
		`SELECT COUNT(*) FROM memory_meta`:                                                   &metadata,
		`SELECT COUNT(*) FROM memory_extraction_sources`:                                     &sources,
		`SELECT COUNT(*) FROM memory_document_identities`:                                    &identities,
		`SELECT COUNT(*) FROM memory_expiry`:                                                 &expiry,
		`SELECT COUNT(*) FROM memory_maintenance_meta WHERE key LIKE 'memory.publication.%'`: &journals,
	} {
		if err := stm.db.QueryRow(query).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if err := vector.corpus.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE collection='aurago_memories'`).Scan(&corpus); err != nil {
		t.Fatal(err)
	}
	if metadata+sources+identities+expiry+journals+corpus != 0 || vector.Count() != 0 {
		t.Fatalf("failed transaction left state: metadata=%d sources=%d identities=%d expiry=%d journals=%d corpus=%d index=%d", metadata, sources, identities, expiry, journals, corpus, vector.Count())
	}
}

func TestAutomaticMemoryPublicationReuseRenewsAndClearsExpiryWithoutChangingCuration(t *testing.T) {
	stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	firstExpiry := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	first, err := publishTestMemory(t, stm, vector, "Vim is the preferred editor.", &firstExpiry)
	if err != nil || len(first.CreatedIDs) != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	id := first.CreatedIDs[0]
	var corpusMetadata string
	if err := vector.corpus.db.QueryRow(`SELECT metadata FROM documents WHERE collection='aurago_memories' AND id=?`, id).Scan(&corpusMetadata); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(corpusMetadata, "memory_publication_id") {
		t.Fatalf("successful live publication retained its journal token: %s", corpusMetadata)
	}
	if err := stm.ApplyMemoryCurationAction(MemoryCurationAction{DocID: id, Action: "confirm", Reason: "reviewed"}, "user", false); err != nil {
		t.Fatal(err)
	}
	before, err := stm.GetMemoryMeta(id)
	if err != nil {
		t.Fatal(err)
	}
	renewed := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
	second, err := publishTestMemory(t, stm, vector, "Vim is the preferred editor.", &renewed)
	if err != nil || len(second.ReusedIDs) != 1 || second.ReusedIDs[0] != id {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	after, err := stm.GetMemoryMeta(id)
	if err != nil || after.VerificationStatus != before.VerificationStatus || after.CurationRevision != before.CurationRevision {
		t.Fatalf("curation changed: before=%+v after=%+v err=%v", before, after, err)
	}
	got, ok, err := stm.MemoryExpiry(t.Context(), id)
	if err != nil || !ok || !got.Equal(renewed) {
		t.Fatalf("renewed expiry=%v ok=%v err=%v", got, ok, err)
	}
	if _, err := publishTestMemory(t, stm, vector, "Vim is the preferred editor.", &time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := stm.MemoryExpiry(t.Context(), id); err != nil || ok {
		t.Fatalf("expiry not cleared: ok=%v err=%v", ok, err)
	}
}

func TestAutomaticMemoryPublicationReuseExpiryAbortPreservesPreviousState(t *testing.T) {
	stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	firstExpiry := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	first, err := publishTestMemory(t, stm, vector, "Vim is the preferred editor.", &firstExpiry)
	if err != nil {
		t.Fatal(err)
	}
	id := first.CreatedIDs[0]
	if err := stm.ApplyMemoryCurationAction(MemoryCurationAction{DocID: id, Action: "confirm", Reason: "reviewed"}, "user", false); err != nil {
		t.Fatal(err)
	}
	before, _ := stm.GetMemoryMeta(id)
	if _, err := stm.db.Exec(`CREATE TRIGGER fail_expiry BEFORE INSERT ON memory_expiry BEGIN SELECT RAISE(ABORT,'injected expiry failure'); END`); err != nil {
		t.Fatal(err)
	}
	renewed := time.Now().Add(96 * time.Hour)
	result, err := publishTestMemory(t, stm, vector, "Vim is the preferred editor.", &renewed)
	if err == nil || len(result.ReusedIDs) != 1 || result.ReusedIDs[0] != id {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	after, _ := stm.GetMemoryMeta(id)
	if after.VerificationStatus != before.VerificationStatus || after.CurationRevision != before.CurationRevision {
		t.Fatalf("curation changed after rollback: before=%+v after=%+v", before, after)
	}
	got, ok, err := stm.MemoryExpiry(t.Context(), id)
	if err != nil || !ok || !got.Equal(firstExpiry) {
		t.Fatalf("previous expiry lost: %v ok=%v err=%v", got, ok, err)
	}
}

func TestAutomaticMemoryPublicationRechecksPermissionAfterActivation(t *testing.T) {
	stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	var denied atomic.Bool
	wrapped := &postActivationPolicyFlipVector{ChromemVectorDB: vector, flip: func() { denied.Store(true) }}
	validate := func() error {
		if denied.Load() {
			return errors.New("permission revoked during activation")
		}
		return nil
	}
	result, err := StoreAutomaticMemoryDocumentWithOptionsContext(t.Context(), stm, wrapped, "[preference] User prefers Vim", "Vim is the preferred editor.", MemoryMetaUpdate{SourceType: "memory_analysis", SourceSessionID: "default"}, AutomaticMemoryWriteOptions{Validate: validate})
	if err == nil || len(result.CreatedIDs) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var journals int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM memory_maintenance_meta WHERE key LIKE 'memory.publication.%'`).Scan(&journals); err != nil || journals != 1 {
		t.Fatalf("revoked publication fence=%d err=%v", journals, err)
	}
	results, err := vector.queryVisibleCollection(t.Context(), vector.collection, []float32{1, 0, 0}, 1)
	if err != nil || len(results) != 0 {
		t.Fatalf("revoked publication visible=%v err=%v", results, err)
	}
}

func prepareReadyPublicationForRecovery(t *testing.T, stm *SQLiteMemory, vector *ChromemVectorDB, session string) (string, []MemoryPublicationDocument) {
	t.Helper()
	const concept, content = "[preference] User prefers local tools", "Use local tools for private files."
	prepared, err := vector.PrepareAutomaticMemoryWriteContext(t.Context(), concept, content)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Release()
	if len(prepared.Documents) != 1 {
		t.Fatalf("prepared documents=%d", len(prepared.Documents))
	}
	key := automaticMemoryPublicationPrefix + "recovery-test"
	document := prepared.Documents[0]
	receipts := []MemoryPublicationDocument{{ID: document.ID, Hash: contentSHA256(document.Content)}}
	staged, _ := jsonString(automaticMemoryPublicationJournal{Version: 1, Status: automaticMemoryPublicationStaged, RequiresValidation: true, Fingerprint: document.Metadata["embedding_fingerprint"], Documents: receipts})
	if _, err := stm.db.Exec(`INSERT INTO memory_maintenance_meta(key,value) VALUES(?,?)`, key, staged); err != nil {
		t.Fatal(err)
	}
	if err := vector.StageAutomaticMemoryWriteContext(t.Context(), key, prepared.Documents); err != nil {
		t.Fatal(err)
	}
	ready, _ := jsonString(automaticMemoryPublicationJournal{Version: 1, Status: automaticMemoryPublicationReady, RequiresValidation: true, Fingerprint: document.Metadata["embedding_fingerprint"], Documents: receipts})
	identity := memoryDocumentIdentity(document.Content, "")
	if err := publishPreparedAutomaticMemoryMetadata(t.Context(), stm, []string{document.ID}, MemoryMetaUpdate{SourceType: "memory_analysis", SourceSessionID: session}, session, identity, identity, CorrectionFactText(document.Content), nil, key, staged, ready); err != nil {
		t.Fatal(err)
	}
	return key, receipts
}

func TestReadyAutomaticMemoryPublicationRecoversWithoutReembedding(t *testing.T) {
	var initialCalls, restartedCalls atomic.Int32
	stm, vector, vectorDir := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) {
		initialCalls.Add(1)
		return []float32{1, 0, 0}, nil
	})
	key, receipts := prepareReadyPublicationForRecovery(t, stm, vector, "default")
	if initialCalls.Load() == 0 {
		t.Fatal("fixture did not embed the staged document")
	}
	if err := vector.Close(); err != nil {
		t.Fatal(err)
	}
	vector = newAutomaticPublicationVector(t, vectorDir, stm, func(context.Context, string) ([]float32, error) {
		restartedCalls.Add(1)
		return []float32{1, 0, 0}, nil
	})
	if err := stm.RecoverAutomaticMemoryPublicationsContext(t.Context(), vector, nil); err == nil {
		t.Fatal("ready publication without its permission validator was activated")
	}
	if err := stm.RecoverAutomaticMemoryPublicationsContext(t.Context(), vector, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if restartedCalls.Load() != 0 {
		t.Fatalf("recovery embedded %d documents", restartedCalls.Load())
	}
	var journals, pending int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM memory_maintenance_meta WHERE key=?`, key).Scan(&journals); err != nil {
		t.Fatal(err)
	}
	if err := vector.corpus.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE collection='aurago_memories' AND id=? AND pending=1`, receipts[0].ID).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if journals != 0 || pending != 0 || vector.Count() != 1 {
		t.Fatalf("recovery state journal=%d pending=%d index=%d", journals, pending, vector.Count())
	}
}

func TestCompletedAutomaticMemoryPublicationRebuildsAfterModelChange(t *testing.T) {
	stm, vector, vectorDir := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	key, receipts := prepareReadyPublicationForRecovery(t, stm, vector, "default")
	if err := vector.ActivateAutomaticMemoryWriteContext(t.Context(), key, receipts); err != nil {
		t.Fatal(err)
	}
	if _, err := stm.db.Exec(`INSERT OR IGNORE INTO deleted_chat_sessions(session_id) VALUES('default')`); err != nil {
		t.Fatal(err)
	}
	// Simulate a process exit after journal deletion and before completed-token
	// retirement. The canonical row is complete, so recovery can prove ownership
	// from pending=0 plus the durable metadata, identity and provenance.
	if _, err := stm.db.Exec(`DELETE FROM memory_maintenance_meta WHERE key=?`, key); err != nil {
		t.Fatal(err)
	}
	var token string
	if err := vector.corpus.db.QueryRow(`SELECT json_extract(metadata,'$.memory_publication_id') FROM documents WHERE collection='aurago_memories' AND id=?`, receipts[0].ID).Scan(&token); err != nil || token != key {
		t.Fatalf("simulated crash gap token=%q err=%v", token, err)
	}
	if err := vector.Close(); err != nil {
		t.Fatal(err)
	}
	var embeddingCalls atomic.Int32
	restarted := newAutomaticPublicationVectorWithFingerprint(t, vectorDir, stm, func(context.Context, string) ([]float32, error) {
		embeddingCalls.Add(1)
		return []float32{0, 1, 0}, nil
	}, "test|changed-model|3")
	if restarted.collection.Count() != 0 {
		var metadata, fingerprint string
		_ = restarted.corpus.db.QueryRow(`SELECT metadata,fingerprint FROM documents WHERE collection='aurago_memories' AND id=?`, receipts[0].ID).Scan(&metadata, &fingerprint)
		t.Fatalf("stale model vector was loaded: index=%d current=%q stored=%q metadata=%s", restarted.collection.Count(), restarted.EmbeddingFingerprint(), fingerprint, metadata)
	}
	if err := stm.RecoverAutomaticMemoryPublicationsContext(t.Context(), restarted, nil); err != nil {
		t.Fatal(err)
	}
	if embeddingCalls.Load() != 1 {
		t.Fatalf("completed document re-embedded %d times, want 1 model rebuild", embeddingCalls.Load())
	}
	var pending int
	var metadata string
	if err := restarted.corpus.db.QueryRow(`SELECT pending,metadata FROM documents WHERE collection='aurago_memories' AND id=?`, receipts[0].ID).Scan(&pending, &metadata); err != nil {
		t.Fatal(err)
	}
	if pending != 0 || strings.Contains(metadata, "memory_publication_id") || restarted.Count() != 1 {
		t.Fatalf("rebuilt completed memory pending=%d metadata=%s index=%d", pending, metadata, restarted.Count())
	}
}

func TestPublicationRecoveryWithNoJournalDoesNotWaitForUnavailableVector(t *testing.T) {
	stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	vector.ready.Store(false)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	if err := stm.RecoverAutomaticMemoryPublicationsContext(ctx, vector, nil); err != nil {
		t.Fatalf("no-journal recovery waited on unavailable vector: %v", err)
	}
}

func TestReadyAutomaticMemoryPublicationKeepsGenerationFencedDocumentHidden(t *testing.T) {
	stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	_, receipts := prepareReadyPublicationForRecovery(t, stm, vector, "sess_old")
	if _, err := stm.db.Exec(`CREATE TABLE chat_session_generations(base_hash TEXT PRIMARY KEY,session_id TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	baseHash := sha256.Sum256([]byte("default"))
	if _, err := stm.db.Exec(`INSERT INTO chat_session_generations(base_hash,session_id) VALUES(?,?)`, hex.EncodeToString(baseHash[:]), "sess_new"); err != nil {
		t.Fatal(err)
	}
	if err := stm.RecoverAutomaticMemoryPublicationsContext(t.Context(), vector, func() error { return nil }); err == nil {
		t.Fatal("superseded source generation was activated")
	}
	if vector.collection.Count() != 0 {
		t.Fatal("superseded ready document entered the index")
	}
	var pending int
	if err := vector.corpus.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE id=? AND pending=1 AND blocked_reason=?`, receipts[0].ID, automaticMemoryPublicationPending).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("fence lost: pending=%d err=%v", pending, err)
	}
}

func TestReadyAutomaticMemoryPublicationModelMismatchStaysHidden(t *testing.T) {
	stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	_, receipts := prepareReadyPublicationForRecovery(t, stm, vector, "default")
	vector.embeddingFingerprint = "different-model"
	if err := stm.RecoverAutomaticMemoryPublicationsContext(t.Context(), vector, func() error { return nil }); err == nil {
		t.Fatal("incompatible embedding model was activated")
	}
	if vector.collection.Count() != 0 {
		t.Fatal("incompatible ready document entered the index")
	}
	var pending int
	if err := vector.corpus.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE id=? AND pending=1`, receipts[0].ID).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("mismatch fence lost: pending=%d err=%v", pending, err)
	}
}

func TestExpiredReadyPublicationReleasesFenceForNormalArchival(t *testing.T) {
	stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	key, receipts := prepareReadyPublicationForRecovery(t, stm, vector, "default")
	if _, err := stm.db.Exec(`INSERT INTO memory_expiry(doc_id,expires_at) VALUES(?,?)`, receipts[0].ID, time.Now().Add(-time.Hour).UTC().Format(memoryExpiryLayout)); err != nil {
		t.Fatal(err)
	}
	if err := stm.RecoverAutomaticMemoryPublicationsContext(t.Context(), vector, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	var journals int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM memory_maintenance_meta WHERE key=?`, key).Scan(&journals); err != nil || journals != 0 {
		t.Fatalf("expired ready publication still fences snapshots: journals=%d err=%v", journals, err)
	}
	results, err := vector.queryVisibleCollection(t.Context(), vector.collection, []float32{1, 0, 0}, 1)
	if err != nil || len(results) != 0 {
		t.Fatalf("expired ready document became visible: results=%v err=%v", results, err)
	}
	archived, err := stm.ArchiveExpiredMemoriesContext(t.Context(), vector)
	if err != nil || archived != 1 {
		t.Fatalf("normal expiry archival did not progress: archived=%d err=%v", archived, err)
	}
}

func TestAlreadyArchivedReadyPublicationConditionallyCleansItsCorpus(t *testing.T) {
	stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	key, receipts := prepareReadyPublicationForRecovery(t, stm, vector, "default")
	if _, err := stm.db.Exec(`UPDATE memory_meta SET verification_status='archived',archived_at=CURRENT_TIMESTAMP,archived_reason='user archive' WHERE doc_id=?`, receipts[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := stm.RecoverAutomaticMemoryPublicationsContext(t.Context(), vector, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	var journals, corpus int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM memory_maintenance_meta WHERE key=?`, key).Scan(&journals); err != nil {
		t.Fatal(err)
	}
	if err := vector.corpus.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE id=?`, receipts[0].ID).Scan(&corpus); err != nil {
		t.Fatal(err)
	}
	meta, err := stm.GetMemoryMeta(receipts[0].ID)
	if err != nil || !IsMemoryArchived(meta) || journals != 0 || corpus != 0 || vector.collection.Count() != 0 {
		t.Fatalf("archived recovery state: archived=%v journals=%d corpus=%d index=%d err=%v", IsMemoryArchived(meta), journals, corpus, vector.collection.Count(), err)
	}
}

func TestCompletedArchivedPublicationTokenIsRetiredWithoutIndexing(t *testing.T) {
	stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	key, receipts := prepareReadyPublicationForRecovery(t, stm, vector, "default")
	if err := vector.ActivateAutomaticMemoryWriteContext(t.Context(), key, receipts); err != nil {
		t.Fatal(err)
	}
	if _, err := stm.db.Exec(`DELETE FROM memory_maintenance_meta WHERE key=?`, key); err != nil {
		t.Fatal(err)
	}
	if _, err := stm.db.Exec(`UPDATE memory_meta SET verification_status='archived',archived_at=CURRENT_TIMESTAMP,archived_reason='user archive' WHERE doc_id=?`, receipts[0].ID); err != nil {
		t.Fatal(err)
	}
	vector.embeddingFingerprint = "test|changed-model|3"
	if vector.collection.Count() != 1 {
		t.Fatal("fixture did not activate the completed vector")
	}
	if err := stm.RecoverAutomaticMemoryPublicationsContext(t.Context(), vector, nil); err != nil {
		t.Fatal(err)
	}
	if vector.collection.Count() != 0 {
		t.Fatal("retiring an archived completed token reinserted its incompatible embedding")
	}
	var pending int
	var metadata string
	if err := vector.corpus.db.QueryRow(`SELECT pending,metadata FROM documents WHERE collection='aurago_memories' AND id=?`, receipts[0].ID).Scan(&pending, &metadata); err != nil {
		t.Fatal(err)
	}
	if pending != 0 || strings.Contains(metadata, "memory_publication_id") {
		t.Fatalf("archived token state pending=%d metadata=%s", pending, metadata)
	}
}

func TestReadyAutomaticMemoryPublicationDeletedSourceStaysHidden(t *testing.T) {
	stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	_, receipts := prepareReadyPublicationForRecovery(t, stm, vector, "private-session")
	if _, err := stm.db.Exec(`INSERT OR REPLACE INTO deleted_chat_sessions(session_id) VALUES('private-session')`); err != nil {
		t.Fatal(err)
	}
	if err := stm.RecoverAutomaticMemoryPublicationsContext(t.Context(), vector, func() error { return nil }); err == nil {
		t.Fatal("deleted source was activated")
	}
	if vector.collection.Count() != 0 {
		t.Fatal("deleted source document entered the index")
	}
	var pending int
	if err := vector.corpus.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE id=? AND pending=1 AND blocked_reason=?`, receipts[0].ID, automaticMemoryPublicationPending).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("deletion fence lost: pending=%d err=%v", pending, err)
	}
}

func TestStagedAutomaticMemoryPublicationRecoveryCleansOwnRows(t *testing.T) {
	stm, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	prepared, err := vector.PrepareAutomaticMemoryWriteContext(t.Context(), "staged fact", "staged body")
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Release()
	key := automaticMemoryPublicationPrefix + "staged-recovery"
	receipts := []MemoryPublicationDocument{{ID: prepared.Documents[0].ID, Hash: contentSHA256(prepared.Documents[0].Content)}}
	value, _ := jsonString(automaticMemoryPublicationJournal{Version: 1, Status: automaticMemoryPublicationStaged, Documents: receipts})
	if _, err := stm.db.Exec(`INSERT INTO memory_maintenance_meta(key,value) VALUES(?,?)`, key, value); err != nil {
		t.Fatal(err)
	}
	if err := vector.StageAutomaticMemoryWriteContext(t.Context(), key, prepared.Documents); err != nil {
		t.Fatal(err)
	}
	if err := stm.RecoverAutomaticMemoryPublicationsContext(t.Context(), vector, nil); err != nil {
		t.Fatal(err)
	}
	var journals, corpus int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM memory_maintenance_meta WHERE key=?`, key).Scan(&journals); err != nil {
		t.Fatal(err)
	}
	if err := vector.corpus.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE collection='aurago_memories'`).Scan(&corpus); err != nil {
		t.Fatal(err)
	}
	if journals != 0 || corpus != 0 || vector.collection.Count() != 0 {
		t.Fatalf("staged orphan retained: journals=%d corpus=%d index=%d", journals, corpus, vector.collection.Count())
	}
}

func TestAutomaticMemoryPublicationCleanupDoesNotDeleteRecycledID(t *testing.T) {
	_, vector, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	const id = "mem_recycled"
	oldPub := automaticMemoryPublicationPrefix + "old"
	oldDoc := PreparedMemoryDocument{ID: id, Content: "old content", Embedding: []float32{1, 0, 0}, Metadata: map[string]string{"embedding_fingerprint": vector.EmbeddingFingerprint()}}
	if err := vector.StageAutomaticMemoryWriteContext(t.Context(), oldPub, []PreparedMemoryDocument{oldDoc}); err != nil {
		t.Fatal(err)
	}
	if _, err := vector.corpus.db.Exec(`UPDATE documents SET content=?,metadata=?,pending=0,blocked_reason='' WHERE collection='aurago_memories' AND id=?`, "new content", `{"memory_publication_id":"memory.publication.new"}`, id); err != nil {
		t.Fatal(err)
	}
	if err := vector.CleanupAutomaticMemoryWriteContext(t.Context(), oldPub, []MemoryPublicationDocument{{ID: id, Hash: contentSHA256(oldDoc.Content)}}); err != nil {
		t.Fatal(err)
	}
	var content string
	if err := vector.corpus.db.QueryRow(`SELECT content FROM documents WHERE collection='aurago_memories' AND id=?`, id).Scan(&content); err != nil || content != "new content" {
		t.Fatalf("recycled document removed: content=%q err=%v", content, err)
	}
}

func TestMemoryPublicationFenceBlocksSnapshotCertification(t *testing.T) {
	cfg, stm, _ := publicationSnapshotFixture(t)
	if _, err := stm.db.Exec(`INSERT INTO memory_maintenance_meta(key,value) VALUES('memory.publication.pending','{}')`); err != nil {
		t.Fatal(err)
	}
	_, err := FreezeMemorySnapshots(t.Context(), cfg)
	if !errors.Is(err, ErrMemorySnapshotBlocked) {
		t.Fatalf("freeze error=%v", err)
	}
}

func TestMissingPublicationJournalLeavesPendingCorpusDocumentHidden(t *testing.T) {
	cfg, stm, vector := publicationSnapshotFixture(t)
	prepared, err := vector.PrepareAutomaticMemoryWriteContext(t.Context(), "pending fact", "pending body")
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Release()
	key := automaticMemoryPublicationPrefix + "missing-journal"
	if err := vector.StageAutomaticMemoryWriteContext(t.Context(), key, prepared.Documents); err != nil {
		t.Fatal(err)
	}
	document, err := vector.corpusDocument(t.Context(), "aurago_memories", prepared.Documents[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := vector.collection.AddDocuments(t.Context(), []chromem.Document{document}, 8); err != nil {
		t.Fatal(err)
	}
	if err := stm.RecoverAutomaticMemoryPublicationsContext(t.Context(), vector, nil); err != nil {
		t.Fatal(err)
	}
	eligibility, err := stm.CheckMemoryEligibility(t.Context(), []string{document.ID})
	if err != nil || !eligibility[document.ID].PublicationPending || eligibility[document.ID].Visible() {
		t.Fatalf("orphan eligibility=%+v err=%v", eligibility[document.ID], err)
	}
	content, err := stm.ReadEligibleMemoriesByID(t.Context(), vector, []MemoryEligibility{eligibility[document.ID]})
	if err == nil || len(content) != 0 {
		t.Fatalf("orphan raw content leaked: content=%v err=%v", content, err)
	}
	var pending int
	if err := vector.corpus.db.QueryRow(`SELECT COUNT(*) FROM documents WHERE id=? AND pending=1 AND blocked_reason=?`, document.ID, automaticMemoryPublicationPending).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("orphan row was unstaged: pending=%d err=%v", pending, err)
	}
	_, err = FreezeMemorySnapshots(t.Context(), cfg)
	if !errors.Is(err, ErrMemorySnapshotBlocked) {
		t.Fatalf("snapshot accepted an orphan pending publication: %v", err)
	}
}

func TestCheckMemoryEligibilityChunksMoreThanThreeHundredIDs(t *testing.T) {
	stm, _, _ := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	ids := make([]string, 0, 601)
	for i := 0; i < 601; i++ {
		id := " raw-id-" + strings.Repeat("x", i%3) + " " + time.Unix(int64(i), 0).UTC().Format("150405")
		ids = append(ids, id)
		if err := stm.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	candidates := make([]MemoryEligibilityCandidate, 0, len(ids)+2)
	for _, id := range ids {
		candidates = append(candidates, MemoryEligibilityCandidate{DocID: id})
	}
	candidates = append(candidates, MemoryEligibilityCandidate{DocID: ids[0]}, MemoryEligibilityCandidate{DocID: ids[300]})
	var stats memoryEligibilityQueryStats
	states, err := stm.checkMemoryDocumentEligibilityWithStats(t.Context(), candidates, &stats)
	if err != nil || len(states) != len(ids) {
		t.Fatalf("eligible=%d want=%d err=%v", len(states), len(ids), err)
	}
	if stats.metadataQueries != 3 || stats.fenceQueries != 3 || stats.corpusQueries != 3 {
		t.Fatalf("601 unique IDs used %d metadata, %d fence, and %d corpus queries; want 3 each", stats.metadataQueries, stats.fenceQueries, stats.corpusQueries)
	}
	for _, id := range ids {
		if state, ok := states[id]; !ok || !state.MetadataExists || !state.Visible() {
			t.Fatalf("raw ID %q was not returned as visible: %+v ok=%v", id, state, ok)
		}
	}
}

func BenchmarkCheckMemoryEligibility600IDs(b *testing.B) {
	stm, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		b.Fatal(err)
	}
	defer stm.Close()
	ids := make([]string, 600)
	tx, err := stm.db.Begin()
	if err != nil {
		b.Fatal(err)
	}
	for i := range ids {
		ids[i] = "benchmark-" + time.Unix(int64(i), 0).UTC().Format("150405")
		if _, err := tx.Exec(`INSERT INTO memory_meta(doc_id) VALUES(?)`, ids[i]); err != nil {
			b.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	var lastStats memoryEligibilityQueryStats
	for i := 0; i < b.N; i++ {
		lastStats = memoryEligibilityQueryStats{}
		candidates := make([]MemoryEligibilityCandidate, len(ids))
		for index, id := range ids {
			candidates[index] = MemoryEligibilityCandidate{DocID: id}
		}
		if _, err := stm.checkMemoryDocumentEligibilityWithStats(context.Background(), candidates, &lastStats); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(lastStats.metadataQueries), "metadata-batches/op")
	b.ReportMetric(float64(lastStats.fenceQueries), "fence-queries/op")
	b.ReportMetric(float64(lastStats.corpusQueries), "corpus-queries/op")
	b.ReportMetric(float64(lastStats.metadataQueries+lastStats.fenceQueries+lastStats.corpusQueries), "sql-queries/op")
}

func TestMemorySourceGenerationAcceptsUnchangedBaseAndRejectsReusedSession(t *testing.T) {
	stm := automaticTestStore(t)
	if _, err := stm.db.Exec(`CREATE TABLE chat_session_generations(base_hash TEXT PRIMARY KEY,session_id TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	baseHash := sha256.Sum256([]byte("default"))
	if _, err := stm.db.Exec(`INSERT INTO chat_session_generations(base_hash,session_id) VALUES(?,?)`, hex.EncodeToString(baseHash[:]), "sess_active"); err != nil {
		t.Fatal(err)
	}
	for session, want := range map[string]bool{"default": false, "sess_active": true} {
		got, err := memorySourceSessionGenerationCurrent(t.Context(), stm.db, session)
		if err != nil || got != want {
			t.Fatalf("session=%s current=%v err=%v want=%v", session, got, err, want)
		}
	}
}
