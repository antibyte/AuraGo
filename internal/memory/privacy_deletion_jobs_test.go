package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type privacyJobsVectors struct {
	VectorDB
	documents map[string]string
	getErrors map[string]error
	deleteErr error
	deleted   []string
}

func (v *privacyJobsVectors) IsDisabled() bool { return false }

func (v *privacyJobsVectors) GetByID(id string) (string, error) {
	if err := v.getErrors[id]; err != nil {
		return "", err
	}
	doc, ok := v.documents[id]
	if !ok {
		return "", fmt.Errorf("document with ID '%s' not found", id)
	}
	return doc, nil
}

func (v *privacyJobsVectors) DeleteDocument(id string) error {
	if v.deleteErr != nil {
		return v.deleteErr
	}
	v.deleted = append(v.deleted, id)
	delete(v.documents, id)
	return nil
}

func openPrivacyJobsStore(t *testing.T, path string) *SQLiteMemory {
	t.Helper()
	store, err := NewSQLiteMemory(path, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func openPrivacyJobsGraph(t *testing.T) *KnowledgeGraph {
	t.Helper()
	kg, err := NewKnowledgeGraph(filepath.Join(t.TempDir(), "knowledge.db"), "", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	return kg
}

func addPrivacyJobTranscript(t *testing.T, store *SQLiteMemory, sessionID string) {
	t.Helper()
	if _, err := store.db.Exec(`INSERT INTO chat_sessions(id) VALUES(?)`, sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO messages(session_id,role,content) VALUES(?,'user','private transcript')`, sessionID); err != nil {
		t.Fatal(err)
	}
}

func addPrivacyJobOwnedMemory(t *testing.T, store *SQLiteMemory, docID, sessionID string) {
	t.Helper()
	if err := store.UpsertMemoryMetaWithDetails(docID, MemoryMetaUpdate{SourceType: "memory_analysis"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordMemoryExtractionSource(docID, "memory_analysis", sessionID); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareChatPrivacyDeletionFailureBeforeAcceptancePreservesTranscript(t *testing.T) {
	store := openPrivacyJobsStore(t, filepath.Join(t.TempDir(), "memory.db"))
	addPrivacyJobTranscript(t, store, "chat-failed-before-acceptance")
	addPrivacyJobOwnedMemory(t, store, "owned-fact", "chat-failed-before-acceptance")
	vectors := &privacyJobsVectors{
		documents: map[string]string{"owned-fact": "private fact"},
		getErrors: map[string]error{"owned-fact": errors.New("injected vector read failure")},
	}

	if _, err := store.PrepareChatPrivacyDeletion(context.Background(), "chat-failed-before-acceptance", vectors, nil, true); err == nil {
		t.Fatal("expected preparation to fail before accepting the deletion")
	}
	var messages, fences, jobs int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE session_id=?`, "chat-failed-before-acceptance").Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM deleted_chat_sessions WHERE session_id=?`, "chat-failed-before-acceptance").Scan(&fences); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM privacy_deletion_jobs`).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if messages != 1 || fences != 0 || jobs != 0 {
		t.Fatalf("pre-acceptance failure changed state: messages=%d fences=%d jobs=%d", messages, fences, jobs)
	}
}

func TestPrepareChatPrivacyDeletionJobInsertFailureRollsBackFence(t *testing.T) {
	store := openPrivacyJobsStore(t, filepath.Join(t.TempDir(), "memory.db"))
	const sessionID = "chat-job-insert-fails"
	addPrivacyJobTranscript(t, store, sessionID)
	if _, err := store.db.Exec(`CREATE TRIGGER fail_privacy_deletion_job_insert BEFORE INSERT ON privacy_deletion_jobs
		BEGIN SELECT RAISE(ABORT, 'injected job insert failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PrepareChatPrivacyDeletion(context.Background(), sessionID, nil, nil, false); err == nil {
		t.Fatal("expected the SQLite job insert trigger to fail preparation")
	}
	var messages, fences, jobs int
	for query, dest := range map[string]*int{
		`SELECT COUNT(*) FROM messages WHERE session_id=?`:              &messages,
		`SELECT COUNT(*) FROM deleted_chat_sessions WHERE session_id=?`: &fences,
		`SELECT COUNT(*) FROM privacy_deletion_jobs WHERE kind='chat'`:  &jobs,
	} {
		var err error
		if strings.Contains(query, "session_id=?") {
			err = store.db.QueryRow(query, sessionID).Scan(dest)
		} else {
			err = store.db.QueryRow(query).Scan(dest)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if messages != 1 || fences != 0 || jobs != 0 {
		t.Fatalf("failed job INSERT partially committed deletion: messages=%d fences=%d jobs=%d", messages, fences, jobs)
	}
}

func TestPrivacyDeletionPublicTerminalHeaderOmitsSessionHashAndTargets(t *testing.T) {
	store := openPrivacyJobsStore(t, filepath.Join(t.TempDir(), "memory.db"))
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.db.Exec(`INSERT INTO privacy_deletion_jobs(
		deletion_job_id,kind,deletion_status,delete_memories,contact_source_id,contact_operation_token,session_hash,transcript_deleted,
		created_at,updated_at,next_retry_at,pending_count,deleted_count,retained_count)
		VALUES('terminal-header','chat',?,1,'','','private-session-hash',1,?,?, '',0,1,0)`, PrivacyDeletionCompleted, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO privacy_deletion_targets(deletion_job_id,store,target_id,content_sha256,source_marker,state)
		VALUES('terminal-header','ltm','private-target-id','sensitive-content-hash','deleted:private-session-marker','deleted')`); err != nil {
		t.Fatal(err)
	}
	job, err := store.GetPrivacyDeletionJob(context.Background(), "terminal-header")
	if err != nil {
		t.Fatal(err)
	}
	if !job.TranscriptDeleted || job.DeletionStatus != PrivacyDeletionCompleted {
		t.Fatalf("terminal job header lost transcript/status state: %+v", job)
	}
	listed, err := store.ListPrivacyDeletionJobs(context.Background(), 10)
	if err != nil || len(listed) != 1 {
		t.Fatalf("list terminal job headers: jobs=%+v err=%v", listed, err)
	}
	for name, value := range map[string]any{"get": job, "list": listed} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"private-session-hash", "private-target-id", "sensitive-content-hash", "deleted:private-session-marker"} {
			if strings.Contains(string(encoded), secret) {
				t.Errorf("%s API record exposed private deletion metadata %q: %s", name, secret, encoded)
			}
		}
	}
}

func TestPrivacyDeletionQueueMigrationBacksUpLegacyTranscript(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	path := filepath.Join(t.TempDir(), "memory.db")
	store, err := NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	addPrivacyJobTranscript(t, store, "legacy-chat")
	if _, err := store.db.Exec(`DROP TABLE privacy_deletion_targets; DROP TABLE privacy_deletion_jobs;`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`PRAGMA user_version=11`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var queueTables int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('privacy_deletion_jobs','privacy_deletion_targets')`).Scan(&queueTables); err != nil || queueTables != 2 {
		t.Fatalf("privacy deletion migration did not create both queue tables: %d %v", queueTables, err)
	}
	backups, err := filepath.Glob(path + ".privacy-deletions-v1-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("privacy deletion migration backups=%v err=%v", backups, err)
	}
	backup, err := sql.Open("sqlite", snapshotSQLiteURI(backups[0], "mode=ro&immutable=1"))
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var messages, queueTablesInBackup int
	if err := backup.QueryRow(`SELECT COUNT(*) FROM messages WHERE session_id='legacy-chat' AND content='private transcript'`).Scan(&messages); err != nil || messages != 1 {
		t.Fatalf("pre-migration backup lost transcript: messages=%d err=%v", messages, err)
	}
	if err := backup.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('privacy_deletion_jobs','privacy_deletion_targets')`).Scan(&queueTablesInBackup); err != nil || queueTablesInBackup != 0 {
		t.Fatalf("backup was not captured before queue creation: queue tables=%d err=%v", queueTablesInBackup, err)
	}
}

func TestChatPrivacyDeletionReopensAndAppliesOnlyJournaledTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	store := openPrivacyJobsStore(t, path)
	kg := openPrivacyJobsGraph(t)
	addPrivacyJobTranscript(t, store, "chat-resume")
	addPrivacyJobOwnedMemory(t, store, "authorized-fact", "chat-resume")
	vectors := &privacyJobsVectors{documents: map[string]string{
		"authorized-fact": "private transcript-derived fact",
		"unrelated-fact":  "unrelated data",
	}}
	job, err := store.PrepareChatPrivacyDeletion(context.Background(), "chat-resume", vectors, kg, true)
	if err != nil || job.DeletionStatus != PrivacyDeletionPending {
		t.Fatalf("prepare job=%+v err=%v", job, err)
	}
	var targets int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM privacy_deletion_targets WHERE deletion_job_id=? AND store='ltm' AND state='pending'`, job.DeletionJobID).Scan(&targets); err != nil || targets != 1 {
		t.Fatalf("journaled targets=%d err=%v", targets, err)
	}
	var targetID, contentHash string
	if err := store.db.QueryRow(`SELECT target_id,content_sha256 FROM privacy_deletion_targets WHERE deletion_job_id=? AND store='ltm'`, job.DeletionJobID).Scan(&targetID, &contentHash); err != nil {
		t.Fatal(err)
	}
	if targetID != "authorized-fact" || contentHash == "" || contentHash == "private transcript-derived fact" {
		t.Fatalf("privacy manifest is not an opaque exact target: id=%q hash=%q", targetID, contentHash)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store = openPrivacyJobsStore(t, path)
	if err := store.ProcessDuePrivacyDeletionJobs(context.Background(), vectors, kg, time.Now().UTC().Add(time.Minute), 10); err != nil {
		t.Fatal(err)
	}
	if len(vectors.deleted) != 1 || vectors.deleted[0] != "authorized-fact" {
		t.Fatalf("reopen applied unexpected vector IDs: %v", vectors.deleted)
	}
	if _, ok := vectors.documents["unrelated-fact"]; !ok {
		t.Fatal("reopen removed a vector not present in the journal")
	}
	job, err = store.GetPrivacyDeletionJob(context.Background(), job.DeletionJobID)
	if err != nil || job.DeletionStatus != PrivacyDeletionCompleted || job.DeletedCount != 1 {
		t.Fatalf("resumed job=%+v err=%v", job, err)
	}
}

func TestChatPrivacyDeletionRetainsProtectedAndNewlySharedTargets(t *testing.T) {
	store := openPrivacyJobsStore(t, filepath.Join(t.TempDir(), "memory.db"))
	kg := openPrivacyJobsGraph(t)
	addPrivacyJobTranscript(t, store, "chat-protection")
	for _, id := range []string{"allowed-fact", "protected-fact", "became-shared"} {
		addPrivacyJobOwnedMemory(t, store, id, "chat-protection")
	}
	if _, err := store.db.Exec(`UPDATE memory_meta SET protected=1 WHERE doc_id='protected-fact'`); err != nil {
		t.Fatal(err)
	}
	vectors := &privacyJobsVectors{documents: map[string]string{
		"allowed-fact": "allowed content", "protected-fact": "protected content", "became-shared": "shared content",
	}}
	job, err := store.PrepareChatPrivacyDeletion(context.Background(), "chat-protection", vectors, kg, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordMemoryExtractionSource("became-shared", "memory_analysis", "other-chat"); err != nil {
		t.Fatal(err)
	}
	if err := store.ProcessDuePrivacyDeletionJobs(context.Background(), vectors, kg, time.Now().UTC().Add(time.Minute), 10); err != nil {
		t.Fatal(err)
	}
	if len(vectors.deleted) != 1 || vectors.deleted[0] != "allowed-fact" {
		t.Fatalf("deletion ignored protection or new sharing: %v", vectors.deleted)
	}
	for _, retainedID := range []string{"protected-fact", "became-shared"} {
		if _, ok := vectors.documents[retainedID]; !ok {
			t.Fatalf("retained vector %q was deleted", retainedID)
		}
	}
	job, err = store.GetPrivacyDeletionJob(context.Background(), job.DeletionJobID)
	if err != nil || job.DeletionStatus != PrivacyDeletionRetainedReview || job.DeletedCount != 1 || job.RetainedCount != 2 || job.PendingCount != 0 {
		t.Fatalf("retention result=%+v err=%v", job, err)
	}
}

func TestChatPrivacyDeletionMissingGraphTargetCompletes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := openPrivacyJobsStore(t, filepath.Join(t.TempDir(), "memory.db"))
	addPrivacyJobTranscript(t, store, "chat-missing-graph-target")
	kg, err := NewKnowledgeGraph(filepath.Join(t.TempDir(), "knowledge.db"), "", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	if _, err := kg.db.Exec(`INSERT INTO kg_nodes(id,label,properties) VALUES('owned-node','derived fact','{"source":"memory_analysis","session_id":"chat-missing-graph-target"}')`); err != nil {
		t.Fatal(err)
	}
	job, err := store.PrepareChatPrivacyDeletion(context.Background(), "chat-missing-graph-target", nil, kg, true)
	if err != nil || job.DeletionStatus != PrivacyDeletionPending {
		t.Fatalf("prepare job=%+v err=%v", job, err)
	}
	if _, err := kg.db.Exec(`DELETE FROM kg_nodes WHERE id='owned-node'`); err != nil {
		t.Fatal(err)
	}
	if err := store.ProcessDuePrivacyDeletionJobs(context.Background(), nil, kg, time.Now().UTC().Add(time.Minute), 10); err != nil {
		t.Fatal(err)
	}
	job, err = store.GetPrivacyDeletionJob(context.Background(), job.DeletionJobID)
	if err != nil || job.DeletionStatus != PrivacyDeletionCompleted || job.DeletedCount != 1 || job.PendingCount != 0 {
		t.Fatalf("missing graph target did not complete: %+v err=%v", job, err)
	}
}

func TestChatPrivacyDeletionMissingVectorTargetCompletes(t *testing.T) {
	store := openPrivacyJobsStore(t, filepath.Join(t.TempDir(), "memory.db"))
	kg := openPrivacyJobsGraph(t)
	addPrivacyJobTranscript(t, store, "chat-missing-vector-target")
	addPrivacyJobOwnedMemory(t, store, "missing-vector", "chat-missing-vector-target")
	vectors := &privacyJobsVectors{documents: map[string]string{"missing-vector": "removed before resume"}}
	job, err := store.PrepareChatPrivacyDeletion(context.Background(), "chat-missing-vector-target", vectors, kg, true)
	if err != nil || job.DeletionStatus != PrivacyDeletionPending {
		t.Fatalf("prepare job=%+v err=%v", job, err)
	}
	delete(vectors.documents, "missing-vector")
	if err := store.ProcessDuePrivacyDeletionJobs(context.Background(), vectors, kg, time.Now().UTC().Add(time.Minute), 10); err != nil {
		t.Fatal(err)
	}
	job, err = store.GetPrivacyDeletionJob(context.Background(), job.DeletionJobID)
	if err != nil || job.DeletionStatus != PrivacyDeletionCompleted || job.DeletedCount != 1 || job.PendingCount != 0 {
		t.Fatalf("missing vector target did not complete: %+v err=%v", job, err)
	}
	var metadata, sources int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM memory_meta WHERE doc_id='missing-vector'`).Scan(&metadata); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM memory_extraction_sources WHERE doc_id='missing-vector'`).Scan(&sources); err != nil {
		t.Fatal(err)
	}
	if metadata != 0 || sources != 0 || len(vectors.deleted) != 0 {
		t.Fatalf("missing vector reconciliation incomplete: metadata=%d sources=%d deleted=%v", metadata, sources, vectors.deleted)
	}
}

func TestChatPrivacyDeletionFailureBackoffAndManualRetry(t *testing.T) {
	store := openPrivacyJobsStore(t, filepath.Join(t.TempDir(), "memory.db"))
	kg := openPrivacyJobsGraph(t)
	addPrivacyJobTranscript(t, store, "chat-retry")
	addPrivacyJobOwnedMemory(t, store, "retry-fact", "chat-retry")
	vectors := &privacyJobsVectors{documents: map[string]string{"retry-fact": "retryable content"}, deleteErr: errors.New("injected vector delete failure")}
	job, err := store.PrepareChatPrivacyDeletion(context.Background(), "chat-retry", vectors, kg, true)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	if err := store.ProcessDuePrivacyDeletionJobs(context.Background(), vectors, kg, now, 10); err == nil {
		t.Fatal("expected target failure")
	}
	job, err = store.GetPrivacyDeletionJob(context.Background(), job.DeletionJobID)
	if err != nil || job.DeletionStatus != PrivacyDeletionRetryWait || job.AttemptCount != 1 {
		t.Fatalf("first failure job=%+v err=%v", job, err)
	}
	firstRetry, err := time.Parse(time.RFC3339Nano, job.NextRetryAt)
	if err != nil || firstRetry.Sub(now) != privacyDeletionRetryInitial {
		t.Fatalf("first backoff=%s err=%v", job.NextRetryAt, err)
	}
	if err := store.ProcessDuePrivacyDeletionJobs(context.Background(), vectors, kg, now.Add(4*time.Minute), 10); err != nil {
		t.Fatal(err)
	}
	if err := store.ProcessDuePrivacyDeletionJobs(context.Background(), vectors, kg, firstRetry, 10); err == nil {
		t.Fatal("expected second target failure after backoff")
	}
	job, err = store.GetPrivacyDeletionJob(context.Background(), job.DeletionJobID)
	if err != nil || job.AttemptCount != 2 {
		t.Fatalf("second failure job=%+v err=%v", job, err)
	}
	secondRetry, err := time.Parse(time.RFC3339Nano, job.NextRetryAt)
	if err != nil || secondRetry.Sub(firstRetry) != 2*privacyDeletionRetryInitial {
		t.Fatalf("second backoff=%s err=%v", job.NextRetryAt, err)
	}
	manualAt := firstRetry.Add(time.Minute)
	job, err = store.RetryPrivacyDeletionJob(context.Background(), job.DeletionJobID, manualAt)
	if err != nil || job.DeletionStatus != PrivacyDeletionPending || job.NextRetryAt != "" {
		t.Fatalf("manual retry job=%+v err=%v", job, err)
	}
	vectors.deleteErr = nil
	if err := store.ProcessDuePrivacyDeletionJobs(context.Background(), vectors, kg, manualAt, 10); err != nil {
		t.Fatal(err)
	}
	job, err = store.GetPrivacyDeletionJob(context.Background(), job.DeletionJobID)
	if err != nil || job.DeletionStatus != PrivacyDeletionCompleted || job.AttemptCount != 2 {
		t.Fatalf("manual retry result=%+v err=%v", job, err)
	}
}

func TestPrivacyDeletionWaitsForCorrectionLockWithCallerContext(t *testing.T) {
	store := newTestSTM(t)
	store.correctionMu.Lock()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	_, err := store.PrepareChatPrivacyDeletion(ctx, "privacy-lock-prepare", nil, nil, false)
	cancel()
	store.correctionMu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("prepare waiting on correction lock = %v, want caller deadline", err)
	}

	job, err := store.PrepareChatPrivacyDeletion(t.Context(), "privacy-lock-process", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	store.correctionMu.Lock()
	ctx, cancel = context.WithTimeout(t.Context(), 30*time.Millisecond)
	err = store.ProcessDuePrivacyDeletionJobs(ctx, nil, nil, time.Now().UTC().Add(time.Minute), 10)
	cancel()
	store.correctionMu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("worker waiting on correction lock = %v, want caller deadline", err)
	}
	job, err = store.GetPrivacyDeletionJob(t.Context(), job.DeletionJobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.AttemptCount != 0 || job.DeletionStatus != PrivacyDeletionPending {
		t.Fatalf("caller cancellation consumed a retry or changed durable job state: %+v", job)
	}
}

func TestPrivacyDeletionFinalizationContextIsDetachedAndBounded(t *testing.T) {
	type contextKey struct{}
	ctx, cancel := context.WithCancel(context.WithValue(t.Context(), contextKey{}, "kept"))
	cancel()
	finalizeCtx, finalizeCancel := boundedPrivacyDeletionFinalizationContext(ctx)
	defer finalizeCancel()
	if err := finalizeCtx.Err(); err != nil {
		t.Fatalf("detached finalization context inherited cancellation: %v", err)
	}
	if got := finalizeCtx.Value(contextKey{}); got != "kept" {
		t.Fatalf("finalization context lost caller values: %v", got)
	}
	deadline, ok := finalizeCtx.Deadline()
	if !ok {
		t.Fatal("finalization context has no deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > privacyDeletionFinalizeTimeout {
		t.Fatalf("finalization deadline is not bounded: %v remaining", remaining)
	}
}

func TestPrivacyDeletionTerminalRetentionRemovesTargets(t *testing.T) {
	store := openPrivacyJobsStore(t, filepath.Join(t.TempDir(), "memory.db"))
	if _, err := store.db.Exec(`INSERT INTO privacy_deletion_jobs(deletion_job_id,kind,deletion_status,session_hash,transcript_deleted,created_at,updated_at,next_retry_at) VALUES('expired','chat',?,'expired-session-hash',1,'2020-01-01T00:00:00Z','2020-01-01T00:00:00Z','')`, PrivacyDeletionCompleted); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO privacy_deletion_targets(deletion_job_id,store,target_id,state) VALUES('expired','ltm','target-secret','deleted')`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO privacy_deletion_jobs(deletion_job_id,kind,deletion_status,created_at,updated_at,next_retry_at) VALUES('active','chat',?, '2020-01-01T00:00:00Z','2020-01-01T00:00:00Z','')`, PrivacyDeletionRetryWait); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO privacy_deletion_targets(deletion_job_id,store,target_id,state) VALUES('active','ltm','keep-active-target','pending')`); err != nil {
		t.Fatal(err)
	}
	if err := store.cleanupPrivacyDeletionHistory(context.Background(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var expiredJobs, expiredTargets, activeJobs, activeTargets int
	for query, dest := range map[string]*int{
		`SELECT COUNT(*) FROM privacy_deletion_jobs WHERE deletion_job_id='expired'`:    &expiredJobs,
		`SELECT COUNT(*) FROM privacy_deletion_targets WHERE deletion_job_id='expired'`: &expiredTargets,
		`SELECT COUNT(*) FROM privacy_deletion_jobs WHERE deletion_job_id='active'`:     &activeJobs,
		`SELECT COUNT(*) FROM privacy_deletion_targets WHERE deletion_job_id='active'`:  &activeTargets,
	} {
		if err := store.db.QueryRow(query).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if expiredJobs != 0 || expiredTargets != 0 || activeJobs != 1 || activeTargets != 1 {
		t.Fatalf("terminal retention job/target counts: expired=%d/%d active=%d/%d", expiredJobs, expiredTargets, activeJobs, activeTargets)
	}
}
