package memory

import (
	"context"
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

func TestPrivacyPurgeSerializesConcurrentLearningWriter(t *testing.T) {
	store := newTestSTM(t)
	for round := 0; round < 12; round++ {
		sessionID := fmt.Sprintf("racing-session-%d", round)
		start := make(chan struct{})
		writeDone := make(chan error, 1)
		purgeDone := make(chan error, 1)
		go func() {
			<-start
			_, err := store.ObserveLearning(context.Background(), LearningDraft{
				Statement: fmt.Sprintf("Private concurrent rule %d", round), Kind: "decision", Confidence: .99,
			}, LearningScope{Project: "privacy-race"}, learningSource(sessionID, "private concurrent evidence"), "race-fingerprint")
			if errors.Is(err, ErrChatSessionDeleted) {
				err = nil
			}
			writeDone <- err
		}()
		go func() {
			<-start
			purgeDone <- store.PurgeChatSession(sessionID)
		}()
		close(start)
		if err := <-writeDone; err != nil {
			t.Fatalf("concurrent learning write failed unexpectedly: %v", err)
		}
		if err := <-purgeDone; err != nil {
			t.Fatalf("concurrent session purge failed: %v", err)
		}
	}

	rows, err := store.db.QueryContext(t.Context(), `SELECT payload FROM memory_learnings`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var payload string
		var learning Learning
		if err := rows.Scan(&payload); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(payload), &learning); err != nil {
			t.Fatal(err)
		}
		for _, source := range learning.Sources {
			if strings.HasPrefix(source.Session, "racing-session-") {
				t.Fatalf("concurrent write left raw deleted-session evidence: %s", payload)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestPrivacyPurgeScrubsLearningEvidenceWithoutRevivingRetiredClaims(t *testing.T) {
	store := newTestSTM(t)
	scope := LearningScope{Project: "project-a"}
	active, err := store.ObserveLearning(t.Context(), LearningDraft{Statement: "Keep the local cache private", Kind: "decision", Confidence: .99}, scope, learningSource("private-session", "Keep the local cache private"), "fingerprint")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ObserveLearning(t.Context(), active.LearningDraft, scope, learningSource("other-session", "Keep the local cache private for this project"), "fingerprint"); err != nil {
		t.Fatal(err)
	}
	archived, err := store.ObserveLearning(t.Context(), LearningDraft{Statement: "Keep backups local", Kind: "decision", Confidence: .99}, scope, learningSource("private-session", "Keep backups local"), "archive-fingerprint")
	if err != nil {
		t.Fatal(err)
	}
	archived, err = store.ReviewLearning(t.Context(), archived.ID, archived.Revision, scope, "archive", "", "retired after private-session review")
	if err != nil {
		t.Fatal(err)
	}
	superseded, err := store.ObserveLearning(t.Context(), LearningDraft{Statement: "Use the old cache", Kind: "decision", Confidence: .99}, scope, learningSource("private-session", "Use the old cache"), "supersede-fingerprint")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.ReviewLearning(t.Context(), superseded.ID, superseded.Revision, scope, "replace", "Use the new cache", "new evidence")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.PurgeChatSession("private-session"); err != nil {
		t.Fatal(err)
	}
	active, err = store.Learning(t.Context(), active.ID, scope)
	if err != nil || active.Status != "revalidation" {
		t.Fatalf("active learning was not sent to revalidation: status=%q err=%v", active.Status, err)
	}
	otherSourceFound := false
	for _, source := range active.Sources {
		if source.Session == "other-session" && source.Quote != "" && source.Hash != "" && source.Reference != "" {
			otherSourceFound = true
		}
		if source.Session == sessionPrivacyMarker("private-session") && (source.Quote != "" || source.Hash != "" || source.Reference != "") {
			t.Fatalf("deleted evidence fields survived: %+v", source)
		}
	}
	if !otherSourceFound {
		t.Fatal("evidence from another session was not preserved")
	}
	archived, err = store.Learning(t.Context(), archived.ID, scope)
	if err != nil || archived.Status != "archived" {
		t.Fatalf("archived learning was revived: status=%q err=%v", archived.Status, err)
	}
	superseded, err = store.Learning(t.Context(), superseded.ID, scope)
	if err != nil || superseded.Status != "superseded" {
		t.Fatalf("superseded learning was revived: status=%q err=%v", superseded.Status, err)
	}

	rows, err := store.db.QueryContext(t.Context(), `SELECT payload FROM memory_learning_revisions ORDER BY id,revision`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var payload string
		var revision LearningRevision
		if err := rows.Scan(&payload); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(payload), &revision); err != nil {
			t.Fatal(err)
		}
		for _, source := range revision.Learning.Sources {
			if source.Session == "private-session" {
				t.Fatalf("revision retained deleted session: %s", payload)
			}
			if source.Session == sessionPrivacyMarker("private-session") && (source.Quote != "" || source.Hash != "" || source.Reference != "") {
				t.Fatalf("revision retained deleted source details: %s", payload)
			}
		}
		if strings.Contains(revision.Reason, "private-session") {
			t.Fatalf("revision reason retained deleted session: %s", payload)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ObserveLearning(t.Context(), LearningDraft{Statement: "New claim", Kind: "decision", Confidence: .99}, scope, learningSource("private-session", "New claim"), "late"); !errors.Is(err, ErrChatSessionDeleted) {
		t.Fatalf("late learning write bypassed session fence: %v", err)
	}
	if _, err := store.db.Exec(`INSERT INTO memory_learning_revisions(id,revision,payload) VALUES('late-learning',1,?)`, `{"learning":{"sources":[{"session":"private-session"}]}}`); err == nil {
		t.Fatal("direct SQL revision write bypassed privacy trigger")
	}
	if _, err := store.db.Exec(`INSERT INTO memory_learnings(id,project,resource,status,revision,payload) VALUES('late-learning','','','proposed',1,?)`, `{"sources":[{"session":"private-session"}]}`); err == nil {
		t.Fatal("direct SQL learning insert bypassed privacy trigger")
	}
}

func TestPrivacyPurgeCancelsCorrectionReplayAndScrubsNestedEvidence(t *testing.T) {
	store := newTestSTM(t)
	privateLearning := Learning{LearningDraft: LearningDraft{Statement: "Use local storage", Kind: "decision"}, ID: "learning-nested", Sources: []LearningSource{learningSource("private-session", "private quote")}, Status: "active", Revision: 1}
	snapshot, err := json.Marshal(correctionSnapshot{Learning: &privateLearning})
	if err != nil {
		t.Fatal(err)
	}
	owned := &MemoryCorrection{
		ID: "correction-private", SessionID: "private-session", RunID: "run-private", Reason: "private correction reason",
		Evidence: []CorrectionEvidence{{Reference: "private-reference", Kind: "user", Quote: "private correction quote", ContentHash: "private-hash"}},
		Status:   "applying", Items: []MemoryCorrectionItem{
			{MemoryCorrectionChange: MemoryCorrectionChange{Reference: MemoryReference{Store: "profile", Category: "prefs", Key: "drink"}, Operation: "replace", Value: "unapplied private value"}, Before: "unapplied prior value", Snapshot: snapshot, State: "pending"},
			{MemoryCorrectionChange: MemoryCorrectionChange{Reference: MemoryReference{Store: "kg_node", ID: "node-17", Field: "label"}, Operation: "replace", Value: "committed derived claim"}, Before: "old derived claim", Snapshot: json.RawMessage(`{"value":"old derived claim"}`), After: json.RawMessage(`{"value":"committed derived claim"}`), State: "done"},
		},
	}
	other := &MemoryCorrection{ID: "correction-other", SessionID: "other-session", Status: "applied", Items: []MemoryCorrectionItem{{MemoryCorrectionChange: MemoryCorrectionChange{Reference: MemoryReference{Store: "learning", ID: "learning-nested"}, Operation: "replace", Value: "Use local storage"}, Snapshot: snapshot, State: "done"}}}
	for _, correction := range []*MemoryCorrection{owned, other} {
		if err := saveMemoryCorrection(t.Context(), store.db, correction); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.PurgeChatSession("private-session"); err != nil {
		t.Fatal(err)
	}
	var storedSession, payload string
	if err := store.db.QueryRow(`SELECT session_id,payload FROM memory_corrections WHERE id=?`, owned.ID).Scan(&storedSession, &payload); err != nil {
		t.Fatal(err)
	}
	var scrubbed MemoryCorrection
	if err := json.Unmarshal([]byte(payload), &scrubbed); err != nil {
		t.Fatal(err)
	}
	if storedSession != sessionPrivacyMarker("private-session") || scrubbed.Status != "canceled" || scrubbed.SessionID != storedSession || scrubbed.RunID != "" || scrubbed.Reason != "" || len(scrubbed.Evidence) != 0 {
		t.Fatalf("correction replay metadata was not scrubbed: session=%q correction=%+v", storedSession, scrubbed)
	}
	if scrubbed.Items[0].State != "canceled" || scrubbed.Items[0].Value != "" || scrubbed.Items[0].Before != "" || len(scrubbed.Items[0].Snapshot) != 0 && string(scrubbed.Items[0].Snapshot) != "null" {
		t.Fatalf("uncommitted correction remains replayable: %+v", scrubbed.Items[0])
	}
	if scrubbed.Items[1].State != "done" || scrubbed.Items[1].MemoryCorrectionChange != (MemoryCorrectionChange{}) || scrubbed.Items[1].Before != "" || len(scrubbed.Items[1].After) != 0 {
		t.Fatalf("committed correction replay payload was not scrubbed: %+v", scrubbed.Items[1])
	}
	if strings.Contains(payload, "private-session") || strings.Contains(payload, "private correction") || strings.Contains(payload, "private-reference") || strings.Contains(payload, "private-hash") || strings.Contains(payload, "run-private") {
		t.Fatalf("correction payload retained source evidence: %s", payload)
	}
	if _, err := store.db.Exec(`INSERT INTO memory_corrections(id,session_id,payload) VALUES('late-correction','private-session',?)`, `{"session_id":"private-session"}`); err == nil {
		t.Fatal("correction session fence did not reject a late write")
	}
	var otherPayload string
	if err := store.db.QueryRow(`SELECT payload FROM memory_corrections WHERE id=?`, other.ID).Scan(&otherPayload); err != nil {
		t.Fatal(err)
	}
	var scrubbedOther MemoryCorrection
	if err := json.Unmarshal([]byte(otherPayload), &scrubbedOther); err != nil {
		t.Fatal(err)
	}
	var otherSnapshot correctionSnapshot
	if err := json.Unmarshal(scrubbedOther.Items[0].Snapshot, &otherSnapshot); err != nil {
		t.Fatal(err)
	}
	if scrubbedOther.Status != "applied" || otherSnapshot.Learning == nil || otherSnapshot.Learning.Sources[0].Session != sessionPrivacyMarker("private-session") || otherSnapshot.Learning.Sources[0].Quote != "" || otherSnapshot.Learning.Sources[0].Hash != "" || otherSnapshot.Learning.Sources[0].Reference != "" {
		t.Fatalf("nested learning evidence was not scrubbed: %+v", scrubbedOther)
	}
	var receiptTargets int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM privacy_deletion_targets WHERE store=? AND source_marker=?`, privacyTargetCorrectionReceipt, owned.ID).Scan(&receiptTargets); err != nil || receiptTargets != 1 {
		t.Fatalf("committed graph correction receipt was not retained for cleanup: %d %v", receiptTargets, err)
	}
	service := &MemoryCorrectionService{Memory: store}
	if _, err := service.Apply(t.Context(), owned.ID, "private-session"); !errors.Is(err, ErrChatSessionDeleted) {
		t.Fatalf("canceled correction replay was not rejected: %v", err)
	}
	if _, err := service.Apply(t.Context(), owned.ID, storedSession); err == nil {
		t.Fatal("canceled correction replay was accepted through its opaque stored scope")
	}
}

func TestPrivacyDeletionPreservesCommittedGraphCorrectionAndRemovesReceipt(t *testing.T) {
	store := newTestSTM(t)
	graph := newTestKG(t)
	if err := graph.AddNode("corrected-node", "Committed corrected claim", map[string]string{"source": "memory_correction"}); err != nil {
		t.Fatal(err)
	}
	correction := &MemoryCorrection{ID: "correction-committed-graph", SessionID: "private-graph-session", Status: "applying", Items: []MemoryCorrectionItem{{
		MemoryCorrectionChange: MemoryCorrectionChange{Reference: MemoryReference{Store: "kg_node", ID: "corrected-node", Field: "label"}, Operation: "replace", Value: "Committed corrected claim"},
		Before:                 "Old claim", Snapshot: json.RawMessage(`{"value":"Old claim"}`), State: "pending",
	}}}
	if err := saveMemoryCorrection(t.Context(), store.db, correction); err != nil {
		t.Fatal(err)
	}
	if _, err := graph.db.Exec(`INSERT INTO kg_correction_receipts(id,after_json) VALUES(?,?)`, correction.ID+":0", `{"value":"Committed corrected claim"}`); err != nil {
		t.Fatal(err)
	}
	job, err := store.PrepareChatPrivacyDeletion(t.Context(), correction.SessionID, nil, graph, false)
	if err != nil {
		t.Fatal(err)
	}
	var storedPayload string
	if err := store.db.QueryRow(`SELECT payload FROM memory_corrections WHERE id=?`, correction.ID).Scan(&storedPayload); err != nil {
		t.Fatal(err)
	}
	var scrubbed MemoryCorrection
	if err := json.Unmarshal([]byte(storedPayload), &scrubbed); err != nil {
		t.Fatal(err)
	}
	if scrubbed.Status != "canceled" || scrubbed.Items[0].State != "done" || scrubbed.Items[0].MemoryCorrectionChange != (MemoryCorrectionChange{}) {
		t.Fatalf("receipt-backed result retained replay data or lost committed state: %+v", scrubbed)
	}
	if err := store.ProcessDuePrivacyDeletionJobs(t.Context(), nil, graph, time.Now().Add(time.Minute), 10); err != nil {
		t.Fatal(err)
	}
	job, err = store.GetPrivacyDeletionJob(t.Context(), job.DeletionJobID)
	if err != nil || job.DeletionStatus != PrivacyDeletionCompleted {
		t.Fatalf("receipt cleanup did not complete: %+v %v", job, err)
	}
	var receipts int
	if err := graph.db.QueryRow(`SELECT COUNT(*) FROM kg_correction_receipts WHERE id=?`, correction.ID+":0").Scan(&receipts); err != nil || receipts != 0 {
		t.Fatalf("correction receipt survived: %d %v", receipts, err)
	}
	var label string
	if err := graph.db.QueryRow(`SELECT label FROM kg_nodes WHERE id='corrected-node'`).Scan(&label); err != nil || label != "Committed corrected claim" {
		t.Fatalf("committed graph effect was lost: %q %v", label, err)
	}
}

func TestPrivacyDeletionDefersKGCorrectionScrubUntilReceiptStoreIsAvailable(t *testing.T) {
	store := newTestSTM(t)
	graph := newTestKG(t)
	const sessionID = "private-graph-resume-session"
	const correctionID = "correction-graph-resume"
	if err := graph.AddNode("resume-corrected-node", "Committed corrected claim", map[string]string{"source": "memory_correction"}); err != nil {
		t.Fatal(err)
	}
	correction := &MemoryCorrection{
		ID: correctionID, SessionID: sessionID, RunID: "private-run", Reason: "private reason",
		Evidence: []CorrectionEvidence{{Reference: "private-reference", Kind: "quote", Quote: "private correction quote", ContentHash: "private-hash"}},
		Status:   "applying",
		Items: []MemoryCorrectionItem{{
			MemoryCorrectionChange: MemoryCorrectionChange{Reference: MemoryReference{Store: "kg_node", ID: "resume-corrected-node", Field: "label"}, Operation: "replace", Value: "Committed corrected claim"},
			Before:                 "private old claim", Snapshot: json.RawMessage(`{"label":"private old claim"}`), State: "pending",
		}},
	}
	if err := saveMemoryCorrection(t.Context(), store.db, correction); err != nil {
		t.Fatal(err)
	}
	if _, err := graph.db.Exec(`INSERT INTO kg_correction_receipts(id,after_json) VALUES(?,?)`, correctionID+":0", `{"label":"Committed corrected claim"}`); err != nil {
		t.Fatal(err)
	}

	job, err := store.PrepareChatPrivacyDeletion(t.Context(), sessionID, nil, nil, false)
	var reconcile *privacyDeletionReconcileError
	if !errors.As(err, &reconcile) {
		t.Fatalf("prepare without KG receipt store error=%v; want durable reconciliation", err)
	}
	if job.DeletionStatus != PrivacyDeletionReconcilePending || job.PendingCount == 0 || job.AttemptCount != 1 {
		t.Fatalf("missing KG receipt store did not retain a retryable job: %+v", job)
	}
	var payload string
	if err := store.db.QueryRow(`SELECT payload FROM memory_corrections WHERE id=?`, correctionID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	for _, retained := range []string{sessionID, "private-run", "private reason", "private correction quote", "private old claim", "Committed corrected claim"} {
		if !strings.Contains(payload, retained) {
			t.Fatalf("correction payload was scrubbed before KG receipt classification (%q missing): %s", retained, payload)
		}
	}
	var label string
	if err := graph.db.QueryRow(`SELECT label FROM kg_nodes WHERE id='resume-corrected-node'`).Scan(&label); err != nil || label != "Committed corrected claim" {
		t.Fatalf("committed graph effect changed before resume: %q %v", label, err)
	}

	resumeAt := time.Now().UTC().Add(privacyDeletionRetryInitial + time.Minute)
	if err := store.ProcessDuePrivacyDeletionJobs(t.Context(), nil, graph, resumeAt, 10); err != nil {
		t.Fatalf("resume privacy deletion with KG receipt store: %v", err)
	}
	job, err = store.GetPrivacyDeletionJob(t.Context(), job.DeletionJobID)
	if err != nil || job.DeletionStatus != PrivacyDeletionCompleted {
		t.Fatalf("resumed privacy deletion did not complete: %+v %v", job, err)
	}
	if err := store.db.QueryRow(`SELECT payload FROM memory_corrections WHERE id=?`, correctionID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var scrubbed MemoryCorrection
	if err := json.Unmarshal([]byte(payload), &scrubbed); err != nil {
		t.Fatal(err)
	}
	if scrubbed.Items[0].State != "done" || scrubbed.Items[0].MemoryCorrectionChange != (MemoryCorrectionChange{}) || scrubbed.Items[0].Before != "" || len(scrubbed.Items[0].Snapshot) != 0 && string(scrubbed.Items[0].Snapshot) != "null" || len(scrubbed.Evidence) != 0 || scrubbed.RunID != "" {
		t.Fatalf("resumed scrub lost committed state or retained replay evidence: %+v", scrubbed)
	}
	for _, private := range []string{sessionID, "private-run", "private reason", "private correction quote", "private old claim"} {
		if strings.Contains(payload, private) {
			t.Fatalf("resumed correction payload retained private value %q: %s", private, payload)
		}
	}
	if err := graph.db.QueryRow(`SELECT label FROM kg_nodes WHERE id='resume-corrected-node'`).Scan(&label); err != nil || label != "Committed corrected claim" {
		t.Fatalf("privacy scrub removed committed graph effect: %q %v", label, err)
	}
	var receipts int
	if err := graph.db.QueryRow(`SELECT COUNT(*) FROM kg_correction_receipts WHERE id=?`, correctionID+":0").Scan(&receipts); err != nil || receipts != 0 {
		t.Fatalf("resumed privacy cleanup left the receipt: %d %v", receipts, err)
	}
}

func TestPrivacyDeletionScopeIsImmutableAndVisibleInJobReads(t *testing.T) {
	store := newTestSTM(t)
	for _, tc := range []struct {
		sessionID string
		first     bool
		second    bool
	}{{"scope-false-first", false, true}, {"scope-true-first", true, false}} {
		addPrivacyJobTranscript(t, store, tc.sessionID)
		first, err := store.PrepareChatPrivacyDeletion(t.Context(), tc.sessionID, nil, nil, tc.first)
		if err != nil {
			t.Fatalf("prepare first scope %v: %v", tc.first, err)
		}
		if first.DeleteMemories != tc.first {
			t.Fatalf("job scan lost scope choice: %+v", first)
		}
		reused, err := store.PrepareChatPrivacyDeletion(t.Context(), tc.sessionID, nil, nil, tc.first)
		if err != nil || reused.DeletionJobID != first.DeletionJobID {
			t.Fatalf("same-scope retry did not reuse its job: %+v %v", reused, err)
		}
		var conflict *PrivacyDeletionScopeConflictError
		if _, err := store.PrepareChatPrivacyDeletion(t.Context(), tc.sessionID, nil, nil, tc.second); !errors.As(err, &conflict) {
			t.Fatalf("opposite scope did not return typed conflict: %v", err)
		}
	}
	jobs, err := store.ListPrivacyDeletionJobs(t.Context(), 10)
	if err != nil || len(jobs) < 2 {
		t.Fatalf("list jobs: %d %v", len(jobs), err)
	}
	for _, job := range jobs {
		if strings.HasPrefix(job.DeletionJobID, "missing") {
			continue
		}
		if job.Kind == "chat" && job.DeleteMemories && job.DeletionStatus == PrivacyDeletionPending && job.AttemptCount == 0 {
			return
		}
	}
	t.Fatal("list job scan omitted the immutable delete_memories choice")
}

func prepareStagedCorrectionVector(t *testing.T, store *SQLiteMemory, vectors *ChromemVectorDB, sessionID, correctionID, docID, content string) {
	t.Helper()
	correction := &MemoryCorrection{ID: correctionID, SessionID: sessionID, Status: "applying", Items: []MemoryCorrectionItem{{
		MemoryCorrectionChange: MemoryCorrectionChange{Reference: MemoryReference{Store: "ltm", ID: "old-document"}, Operation: "replace", Value: content},
		ReplacementID:          docID, State: "pending",
	}}}
	if err := saveMemoryCorrection(t.Context(), store.db, correction); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO memory_correction_vectors(doc_id,correction_id,published) VALUES(?,?,0)`, docID, correctionID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO memory_meta(doc_id,verification_status,archived_at,archived_reason,source_type) VALUES(?,'archived',CURRENT_TIMESTAMP,?,'memory_correction')`, docID, "staged correction "+correctionID); err != nil {
		t.Fatal(err)
	}
	if err := vectors.StoreCorrectionDocument(t.Context(), docID, content); err != nil {
		t.Fatal(err)
	}
}

func TestPrivacyDeletionReconcilesCorrectionVectorOwnershipAndSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	store, err := NewSQLiteMemory(path, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	vectors := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	graph := newTestKG(t)
	const sessionID = "private-staged-vector"
	const correctionID = "correction-staged-vector"
	const docID = "mc_privacy_staged_vector"
	prepareStagedCorrectionVector(t, store, vectors, sessionID, correctionID, docID, "unpublished replacement")
	job, err := store.PrepareChatPrivacyDeletion(t.Context(), sessionID, vectors, graph, false)
	if err != nil {
		t.Fatal(err)
	}
	if job.PendingCount == 0 {
		t.Fatal("staged vector cleanup was not journaled before correction scrub")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = NewSQLiteMemory(path, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.ProcessDuePrivacyDeletionJobs(t.Context(), vectors, graph, time.Now().Add(time.Minute), 10); err != nil {
		t.Fatal(err)
	}
	job, err = store.GetPrivacyDeletionJob(t.Context(), job.DeletionJobID)
	if err != nil || job.DeletionStatus != PrivacyDeletionCompleted {
		t.Fatalf("journal did not complete after restart: %+v %v", job, err)
	}
	if _, err := vectors.GetByIDFromCollection(docID, "aurago_memories"); err == nil {
		t.Fatal("proven unpublished correction vector survived scrub")
	}
	var owners, metadata int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM memory_correction_vectors WHERE doc_id=?`, docID).Scan(&owners); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM memory_meta WHERE doc_id=?`, docID).Scan(&metadata); err != nil {
		t.Fatal(err)
	}
	if owners != 0 || metadata != 0 {
		t.Fatalf("staged vector metadata survived: owners=%d metadata=%d", owners, metadata)
	}
}

func TestPrivacyDeletionUncertainCorrectionVectorOwnershipStaysDurable(t *testing.T) {
	store := newTestSTM(t)
	vectors := newTestChromemVectorDB(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
	graph := newTestKG(t)
	const sessionID = "private-uncertain-vector"
	const correctionID = "correction-uncertain-vector"
	const docID = "mc_privacy_uncertain_vector"
	prepareStagedCorrectionVector(t, store, vectors, sessionID, correctionID, docID, "unpublished replacement")
	job, err := store.PrepareChatPrivacyDeletion(t.Context(), sessionID, vectors, graph, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE memory_correction_vectors SET correction_id='foreign-owner' WHERE doc_id=?`, docID); err != nil {
		t.Fatal(err)
	}
	runAt := time.Now().UTC().Add(time.Minute)
	err = store.ProcessDuePrivacyDeletionJobs(t.Context(), vectors, graph, runAt, 10)
	if err == nil {
		t.Fatal("uncertain owner completed without reconciliation")
	}
	job, getErr := store.GetPrivacyDeletionJob(t.Context(), job.DeletionJobID)
	nextRetry, parseErr := time.Parse(time.RFC3339Nano, job.NextRetryAt)
	if getErr != nil || parseErr != nil || job.DeletionStatus != PrivacyDeletionReconcilePending || job.PendingCount == 0 || job.AttemptCount != 1 || !nextRetry.Equal(runAt.Add(privacyDeletionRetryInitial)) {
		t.Fatalf("uncertain owner was not retained for reconciliation: %+v %v", job, getErr)
	}
	if _, err := vectors.GetByIDFromCollection(docID, "aurago_memories"); err != nil {
		t.Fatalf("uncertain vector was deleted: %v", err)
	}
}
