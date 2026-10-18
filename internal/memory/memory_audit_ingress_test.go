package memory

import (
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func TestDurableMemoryCredentialFilterKeepsPersonalFacts(t *testing.T) {
	for _, text := range []string{"User's birthday is 1990-04-17.", "User is disabled and uses a wheelchair.", "User prefers API debugging guides."} {
		if HasDurableCredential(text) || ValidateCoreMemoryFact(text) != nil {
			t.Fatalf("personal fact rejected: %s", text)
		}
	}
	for _, text := range []string{"api_key=synthetic-secret-123456", "https://alice:synthetic-pass@example.com/"} {
		if !HasDurableCredential(text) || ValidateCoreMemoryFact(text) == nil {
			t.Fatal("credential accepted")
		}
	}
}

func TestConsolidationInfrastructureBackoffPreservesRetriesAndExclusions(t *testing.T) {
	stm, cleanup := newTestSQLiteMemory(t)
	defer cleanup()
	if _, err := stm.db.Exec(`INSERT INTO archived_messages(session_id,role,content,original_timestamp,is_internal,consolidated,consolidation_status,consolidation_retries) VALUES('real','user','complete source',CURRENT_TIMESTAMP,0,0,'in_progress',2)`); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := stm.db.QueryRow(`SELECT id FROM archived_messages WHERE session_id='real'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err := stm.DeferConsolidationClaims([]int64{id}); err != nil {
		t.Fatal(err)
	}
	var status string
	var retries int
	if err := stm.db.QueryRow(`SELECT consolidation_status,consolidation_retries FROM archived_messages WHERE id=?`, id).Scan(&status, &retries); err != nil || status != "failed" || retries != 2 {
		t.Fatalf("status=%s retries=%d err=%v", status, retries, err)
	}
	if claimed, err := stm.ClaimConsolidationCandidates(10, 3); err != nil || len(claimed) != 0 {
		t.Fatalf("backoff bypassed: %+v %v", claimed, err)
	}
	if _, err := stm.db.Exec(`UPDATE archived_messages SET next_retry_at=datetime('now','-1 minute') WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	if claimed, err := stm.ClaimConsolidationCandidates(10, 3); err != nil || len(claimed) != 1 || claimed[0].ConsolidationStatus != "in_progress" {
		t.Fatalf("retry not claimed: %+v %v", claimed, err)
	}
	if err := stm.ReleaseConsolidationClaims([]int64{id}); err != nil {
		t.Fatal(err)
	}
	if err := stm.db.QueryRow(`SELECT consolidation_status,consolidation_retries FROM archived_messages WHERE id=?`, id).Scan(&status, &retries); err != nil || status != "pending" || retries != 2 {
		t.Fatalf("cancel retries=%d status=%s err=%v", retries, status, err)
	}
}

func TestExcludedConsolidationStateSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO archived_messages(session_id,role,content,is_internal,consolidated,consolidation_status) VALUES('excluded','user','excluded source',0,1,'excluded')`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var status string
	if err := s.db.QueryRow(`SELECT consolidation_status FROM archived_messages WHERE session_id='excluded'`).Scan(&status); err != nil || status != "excluded" {
		t.Fatalf("status=%s err=%v", status, err)
	}
}

func TestPendingInfrastructureOutagesNeverExhaustValidationBudget(t *testing.T) {
	s, cleanup := newTestSQLiteMemory(t)
	defer cleanup()
	if err := s.EnqueuePendingMemoryWrite(PendingMemoryWrite{Concept: "profile", Content: "personal fact"}, errors.New("provider unavailable")); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM pending_memory_writes`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for range 10 {
		if err := s.DeferPendingMemoryWrite(id, now); err != nil {
			t.Fatal(err)
		}
	}
	due, err := s.GetDuePendingMemoryWrites(now.Add(6*time.Minute), 10)
	if err != nil || len(due) != 1 || due[0].Attempts != 0 {
		t.Fatalf("budget consumed: %+v %v", due, err)
	}
}

func TestAnalysisRoleEnvelopeDoesNotInventHumanProvenance(t *testing.T) {
	if _, ok := MemoryAnalysisSession("source:memory_analysis role:confirmed-human"); ok {
		t.Fatal("invented role accepted")
	}
	if session, ok := MemoryAnalysisSession("source:memory_analysis session:human role:user_and_assistant"); !ok || session != "human" {
		t.Fatalf("session=%s ok=%v", session, ok)
	}
}
