package memory

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func newMaintenanceProgressStore(t *testing.T, path string) *SQLiteMemory {
	t.Helper()
	stm, err := NewSQLiteMemory(path, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	return stm
}

func insertClaimedMaintenanceArchive(t *testing.T, stm *SQLiteMemory, content string) ArchivedMessage {
	t.Helper()
	res, err := stm.db.Exec(`INSERT INTO archived_messages
		(session_id,role,content,is_internal,original_timestamp,consolidated,consolidation_status)
		VALUES('session-private','user',?,0,CURRENT_TIMESTAMP,0,'in_progress')`, content)
	if err != nil {
		t.Fatalf("insert claimed archive: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("read archive id: %v", err)
	}
	var message ArchivedMessage
	if err := stm.db.QueryRow(`SELECT id,session_id,role,content,original_timestamp,consolidation_status,consolidation_retries FROM archived_messages WHERE id=?`, id).Scan(
		&message.ID, &message.SessionID, &message.Role, &message.Content, &message.Timestamp, &message.ConsolidationStatus, &message.ConsolidationRetries,
	); err != nil {
		t.Fatalf("read claimed archive: %v", err)
	}
	return message
}

func TestConsolidationCheckpointFencesSourceAndFinalizesOnlyAfterPublishCursor(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	message := insertClaimedMaintenanceArchive(t, stm, "stable source")
	key := ConsolidationCheckpointKey{
		MessageID: message.ID, SourceHash: ConsolidationSourceHash(message), PromptVersion: "prompt-v1", ModelID: "model-v1",
	}
	checkpoint, err := stm.EnsureConsolidationCheckpoint(context.Background(), key, 1)
	if err != nil || checkpoint.State != "extracting" || checkpoint.PublishCursor != 0 {
		t.Fatalf("EnsureConsolidationCheckpoint = %#v, %v", checkpoint, err)
	}
	if err := stm.SaveConsolidationPartResult(context.Background(), key, 0, `[{"concept":"fact","content":"value"}]`); err != nil {
		t.Fatalf("SaveConsolidationPartResult: %v", err)
	}
	parts, err := stm.ListConsolidationPartResults(context.Background(), key)
	if err != nil || len(parts) != 1 || parts[0].PartIndex != 0 {
		t.Fatalf("ListConsolidationPartResults = %#v, %v", parts, err)
	}
	if err := stm.MarkConsolidationCheckpointReady(context.Background(), key); err != nil {
		t.Fatalf("MarkConsolidationCheckpointReady: %v", err)
	}
	if err := stm.FinalizeConsolidationCheckpoint(context.Background(), key); err == nil {
		t.Fatal("finalized a checkpoint before publishing its fact")
	}
	if advanced, err := stm.AdvanceConsolidationPublishCursor(context.Background(), key, 0); err != nil || !advanced {
		t.Fatalf("AdvanceConsolidationPublishCursor = %v, %v", advanced, err)
	}
	if advanced, err := stm.AdvanceConsolidationPublishCursor(context.Background(), key, 0); err != nil || advanced {
		t.Fatalf("duplicate cursor advance = %v, %v; want false, nil", advanced, err)
	}
	if err := stm.FinalizeConsolidationCheckpoint(context.Background(), key); err != nil {
		t.Fatalf("FinalizeConsolidationCheckpoint: %v", err)
	}
	var consolidated int
	if err := stm.db.QueryRow(`SELECT consolidated FROM archived_messages WHERE id=?`, message.ID).Scan(&consolidated); err != nil || consolidated != 1 {
		t.Fatalf("source consolidated=%d, err=%v", consolidated, err)
	}
	var checkpoints int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM memory_consolidation_checkpoints WHERE message_id=?`, message.ID).Scan(&checkpoints); err != nil || checkpoints != 0 {
		t.Fatalf("completed checkpoint count=%d, err=%v", checkpoints, err)
	}
}

func TestConsolidationCheckpointRejectsChangedOrDeletedSource(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	message := insertClaimedMaintenanceArchive(t, stm, "original source")
	key := ConsolidationCheckpointKey{MessageID: message.ID, SourceHash: ConsolidationSourceHash(message), PromptVersion: "prompt-v1", ModelID: "model-v1"}
	if _, err := stm.EnsureConsolidationCheckpoint(context.Background(), key, 1); err != nil {
		t.Fatalf("EnsureConsolidationCheckpoint: %v", err)
	}
	if _, err := stm.db.Exec(`UPDATE archived_messages SET content='changed source' WHERE id=?`, message.ID); err != nil {
		t.Fatalf("change source: %v", err)
	}
	if err := stm.SaveConsolidationPartResult(context.Background(), key, 0, `[]`); err == nil {
		t.Fatal("saved extraction result after source changed")
	}
	if _, err := stm.db.Exec(`DELETE FROM archived_messages WHERE id=?`, message.ID); err != nil {
		t.Fatalf("delete source: %v", err)
	}
	var checkpoints, parts int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM memory_consolidation_checkpoints WHERE message_id=?`, message.ID).Scan(&checkpoints); err != nil {
		t.Fatal(err)
	}
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM memory_consolidation_parts WHERE message_id=?`, message.ID).Scan(&parts); err != nil {
		t.Fatal(err)
	}
	if checkpoints != 0 || parts != 0 {
		t.Fatalf("delete fence left checkpoint=%d parts=%d", checkpoints, parts)
	}
}

func TestConsolidationCheckpointFenceRejectsDeletedSession(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	message := insertClaimedMaintenanceArchive(t, stm, "deleted session source")
	if _, err := stm.db.Exec(`INSERT INTO deleted_chat_sessions(session_id) VALUES(?)`, message.SessionID); err != nil {
		t.Fatalf("install deleted-session fence: %v", err)
	}
	_, err := stm.db.Exec(`INSERT INTO memory_consolidation_checkpoints(message_id,source_hash,prompt_version,model_id,part_count)
		VALUES(?,?,?,?,1)`, message.ID, ConsolidationSourceHash(message), "prompt-v1", "model-v1")
	if err == nil {
		t.Fatal("checkpoint insert passed after its session was fenced as deleted")
	}
}

func TestRecoverMemoryMaintenanceReclaimsAtMostOneThousandClaims(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	for i := 0; i < 1005; i++ {
		if _, err := stm.db.Exec(`INSERT INTO archived_messages(session_id,role,content,original_timestamp,consolidated,consolidation_status,consolidation_claimed_at)
			VALUES('session','user','payload',CURRENT_TIMESTAMP,0,'in_progress',datetime('now','-2 hours'))`); err != nil {
			t.Fatalf("insert stale claim %d: %v", i, err)
		}
	}
	if err := stm.recoverMemoryMaintenance(context.Background()); err != nil {
		t.Fatalf("recoverMemoryMaintenance: %v", err)
	}
	var pending, inProgress int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM archived_messages WHERE consolidation_status='pending'`).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM archived_messages WHERE consolidation_status='in_progress'`).Scan(&inProgress); err != nil {
		t.Fatal(err)
	}
	if pending != 1000 || inProgress != 5 {
		t.Fatalf("recovery changed pending=%d in_progress=%d; want 1000 and 5", pending, inProgress)
	}
}

func TestMaintenanceProgressQueryPlans(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	plans := []struct {
		name  string
		query string
		args  []any
		want  string
	}{
		{
			name:  "retention keyset",
			query: `SELECT id,session_id,is_pinned FROM messages WHERE id>? ORDER BY id LIMIT ?`,
			args:  []any{0, stmRetentionPageSize},
			want:  "INTEGER PRIMARY KEY",
		},
		{
			name:  "retention timestamp cutoff",
			query: `SELECT id,CAST(timestamp AS TEXT) FROM messages WHERE session_id=? ORDER BY timestamp DESC,id DESC LIMIT 1 OFFSET ?`,
			args:  []any{"plan-session", 9},
			want:  "idx_messages_session_ts",
		},
		{
			name: "stale consolidation claim",
			query: `SELECT id FROM archived_messages WHERE consolidated=0 AND consolidation_status='in_progress'
				AND (consolidation_claimed_at IS NULL OR consolidation_claimed_at<datetime('now','-30 minutes'))
				ORDER BY consolidation_claimed_at,id LIMIT ?`,
			args: []any{100},
			want: "idx_archived_messages_consolidation_claim",
		},
		{
			name: "ineligible consolidation finalization",
			query: `SELECT id FROM archived_messages WHERE consolidated=0 AND consolidation_status IN ('pending','failed')
				AND NOT (` + consolidationCandidateEligibilitySQL + `) ORDER BY id LIMIT ?`,
			args: []any{250},
			want: "idx_archived_messages_consolidated",
		},
	}
	for _, item := range plans {
		t.Run(item.name, func(t *testing.T) {
			rows, err := stm.db.Query(`EXPLAIN QUERY PLAN `+item.query, item.args...)
			if err != nil {
				t.Fatalf("explain query plan: %v", err)
			}
			defer rows.Close()
			var details []string
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatalf("scan query plan: %v", err)
				}
				details = append(details, detail)
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("iterate query plan: %v", err)
			}
			plan := strings.Join(details, " | ")
			t.Logf("EXPLAIN QUERY PLAN: %s", plan)
			if !strings.Contains(plan, item.want) {
				t.Fatalf("query plan=%q; want use of %q", plan, item.want)
			}
		})
	}
}

func TestTerminalConsolidationFailurePersistsUntilExplicitRecovery(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	res, err := stm.db.Exec(`INSERT INTO archived_messages(session_id,role,content,original_timestamp,consolidated,consolidation_status,consolidation_retries)
		VALUES('session','user','terminal row',CURRENT_TIMESTAMP,0,'failed',3)`)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	count, err := stm.CountTerminalConsolidationFailures(context.Background(), 3)
	if err != nil || count != 1 {
		t.Fatalf("terminal failure count=%d err=%v", count, err)
	}
	if err := stm.recoverMemoryMaintenance(context.Background()); err != nil {
		t.Fatal(err)
	}
	count, err = stm.CountTerminalConsolidationFailures(context.Background(), 3)
	if err != nil || count != 1 {
		t.Fatalf("startup recovery cleared terminal failure: count=%d err=%v", count, err)
	}
	if recovered, err := stm.RecoverTerminalConsolidationFailures(context.Background(), []int64{id}); err != nil || recovered != 1 {
		t.Fatalf("explicit terminal recovery=%d err=%v", recovered, err)
	}
	count, err = stm.CountTerminalConsolidationFailures(context.Background(), 3)
	if err != nil || count != 0 {
		t.Fatalf("terminal failures after explicit recovery=%d err=%v", count, err)
	}
}

func TestClaimConsolidationCandidatesContextPreservesDefaultAndCap(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	for index := 0; index < 1001; index++ {
		if _, err := stm.db.Exec(`INSERT INTO archived_messages(session_id,role,content,original_timestamp)
			VALUES('claim-limit','user',?,CURRENT_TIMESTAMP)`, fmt.Sprintf("message-%d", index)); err != nil {
			t.Fatalf("insert archived source %d: %v", index, err)
		}
	}
	defaultPage, err := stm.ClaimConsolidationCandidatesContext(context.Background(), 0, 3)
	if err != nil || len(defaultPage) != 200 {
		t.Fatalf("default claim page=%d err=%v; want legacy default 200", len(defaultPage), err)
	}
	ids := make([]int64, 0, len(defaultPage))
	for _, message := range defaultPage {
		ids = append(ids, message.ID)
	}
	if err := stm.ReleaseConsolidationClaimsContext(context.Background(), ids); err != nil {
		t.Fatalf("release default page: %v", err)
	}
	largePage, err := stm.ClaimConsolidationCandidatesContext(context.Background(), 2000, 3)
	if err != nil || len(largePage) != 1000 {
		t.Fatalf("oversized claim page=%d err=%v; want cap 1000", len(largePage), err)
	}
}

func TestSTMRetentionResumesPersistedKeysetAfterRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	stm := newMaintenanceProgressStore(t, path)
	for i := 1; i <= 5; i++ {
		stamp := time.Date(2026, time.January, i, 0, 0, 0, 0, time.UTC)
		if _, err := stm.db.Exec(`INSERT INTO messages(session_id,role,content,timestamp) VALUES('retention-session','user',?,?)`, strings.Repeat("m", i), stamp); err != nil {
			t.Fatalf("insert message %d: %v", i, err)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := stm.EnforceSTMPRetentionContext(canceled, 2, 3); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled retention error=%v; want context.Canceled", err)
	}
	var before int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&before); err != nil || before != 5 {
		t.Fatalf("canceled pass left %d messages, err=%v", before, err)
	}
	if _, err := stm.EnforceSTMPRetentionContext(context.Background(), 2, 3); err != nil {
		t.Fatalf("first retention page: %v", err)
	}
	var cursor string
	if err := stm.db.QueryRow(`SELECT value FROM memory_maintenance_meta WHERE key=?`, stmRetentionCursorKey).Scan(&cursor); err != nil || cursor != "3" {
		t.Fatalf("persisted cursor=%q, err=%v; want 3", cursor, err)
	}
	if err := stm.Close(); err != nil {
		t.Fatalf("close before restart: %v", err)
	}
	stm = newMaintenanceProgressStore(t, path)
	if _, err := stm.EnforceSTMPRetentionContext(context.Background(), 2, 3); err != nil {
		t.Fatalf("resumed retention page: %v", err)
	}
	var remaining int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&remaining); err != nil || remaining != 2 {
		t.Fatalf("remaining messages=%d, err=%v; want 2", remaining, err)
	}
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM archived_messages WHERE session_id='retention-session'`).Scan(&remaining); err != nil || remaining != 3 {
		t.Fatalf("archived messages=%d, err=%v; want 3", remaining, err)
	}
}

func TestPendingMemoryCleanupLeavesContentFreeTombstone(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	write := PendingMemoryWrite{
		Concept: "private concept", Content: "private source payload", Domain: "test",
		Metadata: &MemoryMetaUpdate{SourceSessionID: "session-private"},
	}
	if err := stm.EnqueuePendingMemoryWrite(write, errors.New("provider unavailable")); err != nil {
		t.Fatalf("EnqueuePendingMemoryWrite: %v", err)
	}
	var id int64
	if err := stm.db.QueryRow(`SELECT id FROM pending_memory_writes WHERE concept=?`, write.Concept).Scan(&id); err != nil {
		t.Fatal(err)
	}
	terminalAt := time.Now().UTC().Add(-31 * 24 * time.Hour)
	if _, err := stm.db.Exec(`UPDATE pending_memory_writes SET status='exhausted',attempts=?,terminal_at=? WHERE id=?`, pendingMemoryWriteMaxAttempts, terminalAt, id); err != nil {
		t.Fatal(err)
	}
	if count, err := stm.CountPendingMemoryWrites(); err != nil || count != 0 {
		t.Fatalf("pending count=%d, err=%v; exhausted item must not count", count, err)
	}
	if count, err := stm.CountExhaustedMemoryWritesContext(context.Background()); err != nil || count != 1 {
		t.Fatalf("exhausted count before payload cleanup=%d, err=%v", count, err)
	}
	now := time.Now().UTC()
	if deleted, err := stm.CleanupExpiredPendingMemoryWrites(context.Background(), now, 100); err != nil || deleted != 1 {
		t.Fatalf("CleanupExpiredPendingMemoryWrites=%d, %v; want 1", deleted, err)
	}
	var payloads, tombstones int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_writes WHERE id=?`, id).Scan(&payloads); err != nil {
		t.Fatal(err)
	}
	fingerprint := pendingMemoryWriteFingerprint(write, PendingMemorySessionFingerprint("session-private"))
	var storedSessionHash string
	if err := stm.db.QueryRow(`SELECT source_session_hash FROM pending_memory_write_tombstones WHERE fingerprint=?`, fingerprint).Scan(&storedSessionHash); err != nil {
		t.Fatal(err)
	}
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_write_tombstones`).Scan(&tombstones); err != nil {
		t.Fatal(err)
	}
	if payloads != 0 || tombstones != 1 || storedSessionHash != PendingMemorySessionFingerprint("session-private") {
		t.Fatalf("payloads=%d tombstones=%d session-hash=%q", payloads, tombstones, storedSessionHash)
	}
	if count, err := stm.CountExhaustedMemoryWritesContext(context.Background()); err != nil || count != 1 {
		t.Fatalf("exhausted count after payload cleanup=%d, err=%v; tombstone must remain counted", count, err)
	}
	if err := stm.EnqueuePendingMemoryWrite(write, nil); !errors.Is(err, ErrAutomaticMemorySkipped) {
		t.Fatalf("re-enqueue tombstoned payload error=%v", err)
	}
	var exposed int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_write_tombstones WHERE CAST(fingerprint AS TEXT) LIKE '%private%' OR CAST(source_session_hash AS TEXT) LIKE '%session-private%'`).Scan(&exposed); err != nil || exposed != 0 {
		t.Fatalf("tombstone retained readable private identity: count=%d err=%v", exposed, err)
	}
}

func TestPendingMemoryCleanupFencesConcurrentLateEnqueue(t *testing.T) {
	stm := newMaintenanceProgressStore(t, filepath.Join(t.TempDir(), "pending-write-fence.db"))
	write := PendingMemoryWrite{Concept: "fenced concept", Content: "fenced payload", Domain: "test"}
	if err := stm.EnqueuePendingMemoryWrite(write, nil); err != nil {
		t.Fatalf("enqueue initial pending write: %v", err)
	}
	var id int64
	if err := stm.db.QueryRow(`SELECT id FROM pending_memory_writes WHERE concept=?`, write.Concept).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := stm.db.Exec(`UPDATE pending_memory_writes SET status='exhausted', attempts=?, terminal_at=? WHERE id=?`, pendingMemoryWriteMaxAttempts, time.Now().UTC().Add(-31*24*time.Hour), id); err != nil {
		t.Fatalf("expire pending write: %v", err)
	}

	const writers = 32
	start := make(chan struct{})
	var workers sync.WaitGroup
	errs := make(chan error, writers+1)
	for i := 0; i < writers; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			if err := stm.EnqueuePendingMemoryWrite(write, nil); err != nil && !errors.Is(err, ErrAutomaticMemorySkipped) {
				errs <- err
			}
		}()
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		<-start
		if deleted, err := stm.CleanupExpiredPendingMemoryWrites(context.Background(), time.Now().UTC(), 100); err != nil {
			errs <- err
		} else if deleted != 1 {
			errs <- fmt.Errorf("cleanup deleted %d rows, want 1", deleted)
		}
	}()
	close(start)
	workers.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent pending write operation: %v", err)
	}

	fingerprint := pendingMemoryWriteFingerprint(write, "")
	var tombstones, payloads int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_write_tombstones WHERE fingerprint=?`, fingerprint).Scan(&tombstones); err != nil {
		t.Fatal(err)
	}
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_writes WHERE source_fingerprint=?`, fingerprint).Scan(&payloads); err != nil {
		t.Fatal(err)
	}
	if tombstones != 1 || payloads != 0 {
		t.Fatalf("concurrent cleanup left tombstones=%d payloads=%d; want one tombstone and no resurrected payload", tombstones, payloads)
	}
	if err := stm.EnqueuePendingMemoryWrite(write, nil); !errors.Is(err, ErrAutomaticMemorySkipped) {
		t.Fatalf("late enqueue after cleanup error=%v; want tombstone rejection", err)
	}
}

func TestPendingMemoryWriteContextMethodsHonorCancellation(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	write := PendingMemoryWrite{Concept: "canceled queue write", Content: "not persisted"}
	if err := stm.EnqueuePendingMemoryWriteContext(canceled, write, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled enqueue error=%v; want context.Canceled", err)
	}
	if writes, err := stm.GetDuePendingMemoryWritesContext(canceled, time.Now().UTC(), 10); !errors.Is(err, context.Canceled) || len(writes) != 0 {
		t.Fatalf("canceled queue read writes=%d err=%v; want no rows and context.Canceled", len(writes), err)
	}
	if err := stm.MarkPendingMemoryWriteFailedContext(canceled, 1, errors.New("failure"), time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled failure update error=%v; want context.Canceled", err)
	}
	if err := stm.DeferPendingMemoryWriteContext(canceled, 1, time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled defer error=%v; want context.Canceled", err)
	}
	if err := stm.CompletePendingMemoryWriteContext(canceled, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled completion error=%v; want context.Canceled", err)
	}
	if count, err := stm.CountPendingMemoryWritesContext(canceled); !errors.Is(err, context.Canceled) || count != 0 {
		t.Fatalf("canceled queue count=%d err=%v; want zero and context.Canceled", count, err)
	}
	var rows int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_writes`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("canceled enqueue persisted %d rows, err=%v", rows, err)
	}
}

func TestReleaseMaintenanceDayClaimOnlyBeforeRunIsPersisted(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	startedAt := time.Date(2026, time.October, 20, 1, 30, 0, 0, time.FixedZone("test", 2*60*60))
	claimed, err := stm.ClaimMaintenanceDay(startedAt)
	if err != nil || !claimed {
		t.Fatalf("ClaimMaintenanceDay=%v err=%v", claimed, err)
	}
	if released, err := stm.ReleaseMaintenanceDayClaim(startedAt); err != nil || !released {
		t.Fatalf("ReleaseMaintenanceDayClaim=%v err=%v; want true", released, err)
	}
	claimed, err = stm.ClaimMaintenanceDay(startedAt)
	if err != nil || !claimed {
		t.Fatalf("reclaim after release=%v err=%v", claimed, err)
	}
	if err := stm.InsertMaintenanceRun(startedAt, startedAt.Add(time.Minute), "completed", MaintenancePhaseResults{}); err != nil {
		t.Fatalf("InsertMaintenanceRun: %v", err)
	}
	if released, err := stm.ReleaseMaintenanceDayClaim(startedAt); err != nil || released {
		t.Fatalf("release after persisted run=%v err=%v; want false", released, err)
	}
}

func TestMemoryMaintenanceFailureRetryPageRotatesAndScrubsCause(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	for _, id := range []string{"a", "b", "c"} {
		if err := stm.RecordMemoryMaintenanceFailureContext(context.Background(), "detect_conflict", id, errors.New("private source text")); err != nil {
			t.Fatalf("RecordMemoryMaintenanceFailureContext(%s): %v", id, err)
		}
	}
	if _, err := stm.db.Exec(`UPDATE memory_maintenance_failures SET last_failed_at=datetime('now','-6 minutes') WHERE action='detect_conflict'`); err != nil {
		t.Fatal(err)
	}
	var lastError string
	if err := stm.db.QueryRow(`SELECT last_error FROM memory_maintenance_failures WHERE action='detect_conflict' AND target_id='a'`).Scan(&lastError); err != nil || lastError != "maintenance_action_failed" {
		t.Fatalf("stored failure reason=%q err=%v", lastError, err)
	}
	page, _, err := stm.GetMemoryMaintenanceFailureRetryPage(context.Background(), "detect_conflict", 2)
	if err != nil || strings.Join(page, ",") != "a,b" {
		t.Fatalf("first retry page=%v err=%v", page, err)
	}
	if err := stm.AdvanceMemoryMaintenanceFailureRetryCursor(context.Background(), "detect_conflict", page[len(page)-1]); err != nil {
		t.Fatal(err)
	}
	page, _, err = stm.GetMemoryMaintenanceFailureRetryPage(context.Background(), "detect_conflict", 2)
	if err != nil || strings.Join(page, ",") != "c" {
		t.Fatalf("second retry page=%v err=%v", page, err)
	}
	if err := stm.AdvanceMemoryMaintenanceFailureRetryCursor(context.Background(), "detect_conflict", page[len(page)-1]); err != nil {
		t.Fatal(err)
	}
	page, _, err = stm.GetMemoryMaintenanceFailureRetryPage(context.Background(), "detect_conflict", 2)
	if err != nil || strings.Join(page, ",") != "a,b" {
		t.Fatalf("rotated retry page=%v err=%v", page, err)
	}
}

func TestMemoryMaintenanceFailureRetryPageOnlyReturnsDueTargets(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	for _, id := range []string{"aged", "fresh"} {
		if err := stm.RecordMemoryMaintenanceFailureContext(context.Background(), "detect_conflict", id, errors.New("failure")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := stm.db.Exec(`UPDATE memory_maintenance_failures SET last_failed_at=datetime('now','-6 minutes') WHERE target_id='aged'`); err != nil {
		t.Fatal(err)
	}
	page, _, err := stm.GetMemoryMaintenanceFailureRetryPage(context.Background(), "detect_conflict", 25)
	if err != nil || strings.Join(page, ",") != "aged" {
		t.Fatalf("due retry page=%v err=%v; fresh target should wait", page, err)
	}
	if err := stm.AdvanceMemoryMaintenanceFailureRetryCursor(context.Background(), "detect_conflict", page[len(page)-1]); err != nil {
		t.Fatal(err)
	}
	if _, err := stm.db.Exec(`UPDATE memory_maintenance_failures SET last_failed_at=datetime('now','-6 minutes') WHERE target_id='fresh'`); err != nil {
		t.Fatal(err)
	}
	page, _, err = stm.GetMemoryMaintenanceFailureRetryPage(context.Background(), "detect_conflict", 25)
	if err != nil || strings.Join(page, ",") != "fresh" {
		t.Fatalf("due retry page after cursor=%v err=%v; fresh target should be considered before wrap", page, err)
	}
	if err := stm.AdvanceMemoryMaintenanceFailureRetryCursor(context.Background(), "detect_conflict", page[len(page)-1]); err != nil {
		t.Fatal(err)
	}
	page, _, err = stm.GetMemoryMaintenanceFailureRetryPage(context.Background(), "detect_conflict", 25)
	if err != nil || strings.Join(page, ",") != "aged,fresh" {
		t.Fatalf("rotated due retry page=%v err=%v", page, err)
	}
}
