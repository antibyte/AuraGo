package server

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/memory"
)

func newChatPrivacyStatusFixture(t *testing.T) (*Server, *sql.DB) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "stm.db")
	stm, err := memory.NewSQLiteMemory(dbPath, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	adminDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	adminDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = adminDB.Close() })
	kg, err := memory.NewKnowledgeGraph(filepath.Join(t.TempDir(), "knowledge.db"), "", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	return &Server{ShortTermMem: stm, KG: kg}, adminDB
}

func createMessageDeleteFailureTrigger(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`CREATE TRIGGER fail_privacy_chat_delete BEFORE DELETE ON messages BEGIN SELECT RAISE(ABORT,'synthetic delete failure'); END`); err != nil {
		t.Fatal(err)
	}
}

func TestChatSessionDeleteReportsPersistedTranscriptStateOnScrubFailure(t *testing.T) {
	s, adminDB := newChatPrivacyStatusFixture(t)
	session, err := s.ShortTermMem.CreateChatSession()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ShortTermMem.InsertMessage(session.ID, "user", "synthetic transcript", false, false); err != nil {
		t.Fatal(err)
	}
	createMessageDeleteFailureTrigger(t, adminDB)
	response := httptest.NewRecorder()
	handleDeleteChatSession(s).ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/chat/sessions/"+session.ID, nil))
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("accepted incomplete deletion status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Status         string `json:"status"`
		DeletionJobID  string `json:"deletion_job_id"`
		DeletionStatus string `json:"deletion_status"`
		Deletion       struct {
			TranscriptDeleted bool `json:"transcript_deleted"`
		} `json:"deletion"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "partial" || body.DeletionJobID == "" || body.DeletionStatus != memory.PrivacyDeletionRetryWait || body.Deletion.TranscriptDeleted {
		t.Fatalf("response claims wrong accepted deletion state: %+v", body)
	}
}

func TestChatSessionDeleteStatusUsesRequestedJobWhenAnotherJobFails(t *testing.T) {
	s, adminDB := newChatPrivacyStatusFixture(t)
	other, err := s.ShortTermMem.CreateChatSession()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ShortTermMem.InsertMessage(other.ID, "user", "other transcript", false, false); err != nil {
		t.Fatal(err)
	}
	createMessageDeleteFailureTrigger(t, adminDB)
	otherJob, prepareErr := s.ShortTermMem.PrepareChatPrivacyDeletion(t.Context(), other.ID, nil, s.KG, false)
	if prepareErr == nil || otherJob.DeletionStatus != memory.PrivacyDeletionRetryWait {
		t.Fatalf("setup did not produce retryable older job: job=%+v err=%v", otherJob, prepareErr)
	}
	if _, err := adminDB.Exec(`DROP TRIGGER fail_privacy_chat_delete`); err != nil {
		t.Fatal(err)
	}

	requested, err := s.ShortTermMem.CreateChatSession()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ShortTermMem.InsertMessage(requested.ID, "user", "requested transcript", false, false); err != nil {
		t.Fatal(err)
	}
	farFuture := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339Nano)
	if _, err := adminDB.Exec(`UPDATE privacy_deletion_jobs SET next_retry_at=? WHERE deletion_job_id=?`, farFuture, otherJob.DeletionJobID); err != nil {
		t.Fatal(err)
	}
	requestedJob, err := s.ShortTermMem.PrepareChatPrivacyDeletion(t.Context(), requested.ID, nil, s.KG, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ShortTermMem.ProcessDuePrivacyDeletionJobs(t.Context(), nil, s.KG, time.Now().UTC(), 10); err != nil {
		t.Fatal(err)
	}
	requestedJob, err = s.ShortTermMem.GetPrivacyDeletionJob(t.Context(), requestedJob.DeletionJobID)
	if err != nil || requestedJob.DeletionStatus != memory.PrivacyDeletionCompleted {
		t.Fatalf("requested setup job=%+v err=%v", requestedJob, err)
	}

	past := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	if _, err := adminDB.Exec(`UPDATE privacy_deletion_jobs SET next_retry_at=? WHERE deletion_job_id=?`, past, otherJob.DeletionJobID); err != nil {
		t.Fatal(err)
	}
	createMessageDeleteFailureTrigger(t, adminDB)
	response := httptest.NewRecorder()
	handleDeleteChatSession(s).ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/chat/sessions/"+requested.ID, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("unrelated older job changed requested terminal response: status=%d body=%s", response.Code, response.Body.String())
	}
}
