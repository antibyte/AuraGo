package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/contacts"
	"aurago/internal/memory"
)

func newPrivacyDeletionHTTPFixture(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	stm, err := memory.NewSQLiteMemory(filepath.Join(dir, "stm.db"), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	kg, err := memory.NewKnowledgeGraph(filepath.Join(dir, "kg.db"), "", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	db, err := contacts.InitDB(filepath.Join(dir, "contacts.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &Server{Cfg: &config.Config{}, Logger: slog.Default(), ShortTermMem: stm, KG: kg, ContactsDB: db}
}

func TestContactDeletionKnownFailureReleasesOnlyItsFence(t *testing.T) {
	s := newPrivacyDeletionHTTPFixture(t)
	id, err := contacts.Create(s.ContactsDB, contacts.Contact{Name: "Synthetic privacy test contact"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.KG.SyncContact(t.Context(), "contact_"+id, "Synthetic privacy test contact", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ContactsDB.Exec(`CREATE TRIGGER reject_test_delete BEFORE DELETE ON contacts BEGIN SELECT RAISE(ABORT,'synthetic delete failure'); END`); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handleContactByID(s).ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/api/contacts/"+id+"?delete_memories=true", nil))
	if response.Code < 400 {
		t.Fatalf("known failed DELETE reported success: %d %s", response.Code, response.Body.String())
	}
	if _, err := contacts.GetByID(s.ContactsDB, id); err != nil {
		t.Fatalf("failed DELETE changed contact: %v", err)
	}
	fenced, err := s.KG.IsContactSourceDeleted(t.Context(), "contact_"+id)
	if err != nil || fenced {
		t.Fatalf("known failed DELETE retained its fence: fenced=%v err=%v", fenced, err)
	}
	jobs, err := s.ShortTermMem.ListPrivacyDeletionJobs(t.Context(), 10)
	if err != nil || len(jobs) != 1 || jobs[0].DeletionStatus != memory.PrivacyDeletionAborted {
		t.Fatalf("known failure did not leave an aborted journal: %+v %v", jobs, err)
	}
	update := httptest.NewRecorder()
	handleContactByID(s).ServeHTTP(update, httptest.NewRequest(http.MethodPut, "/api/contacts/"+id, strings.NewReader(`{"name":"Updated synthetic contact"}`)))
	if update.Code != http.StatusOK {
		t.Fatalf("contact remained locked after known failure: %d %s", update.Code, update.Body.String())
	}
	if err := s.KG.SyncContact(t.Context(), "contact_"+id, "Updated synthetic contact", nil); err != nil {
		t.Fatalf("contact sync remained fenced after known failure: %v", err)
	}
}

func TestPrivacyDeletionStatusPersistsWithoutChatAndExcludesSourceIdentity(t *testing.T) {
	s := newPrivacyDeletionHTTPFixture(t)
	session, err := s.ShortTermMem.CreateChatSession()
	if err != nil {
		t.Fatal(err)
	}
	const transcript = "synthetic source text must not appear in privacy status"
	if _, err := s.ShortTermMem.InsertMessage(session.ID, "user", transcript, false, false); err != nil {
		t.Fatal(err)
	}
	job, err := s.ShortTermMem.PrepareChatPrivacyDeletion(t.Context(), session.ID, nil, s.KG, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ShortTermMem.ProcessDuePrivacyDeletionJobs(t.Context(), nil, s.KG, time.Now().UTC(), 10); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/privacy-deletions", "/api/privacy-deletions/" + job.DeletionJobID} {
		response := httptest.NewRecorder()
		if path == "/api/privacy-deletions" {
			handlePrivacyDeletionCollection(s).ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		} else {
			handlePrivacyDeletionItem(s).ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		}
		if response.Code != http.StatusOK || !json.Valid(response.Body.Bytes()) {
			t.Fatalf("status unavailable after deleting chat: %d %s", response.Code, response.Body.String())
		}
		body := response.Body.String()
		if !strings.Contains(body, job.DeletionJobID) || !strings.Contains(body, memory.PrivacyDeletionCompleted) {
			t.Fatalf("missing completed job: %s", body)
		}
		for _, forbidden := range []string{session.ID, transcript, "target_id", "expected_fingerprint", "source_marker", "contact_source_id"} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("privacy status leaked source identity field %q", forbidden)
			}
		}
	}
	retry := httptest.NewRecorder()
	handlePrivacyDeletionItem(s).ServeHTTP(retry, httptest.NewRequest(http.MethodPost, "/api/privacy-deletions/"+job.DeletionJobID+"/retry", nil))
	if retry.Code != http.StatusConflict {
		t.Fatalf("completed job accepted retry: %d %s", retry.Code, retry.Body.String())
	}
}

func TestPrivacyDeletionAdministrativeEndpointsRejectUnprivilegedRequests(t *testing.T) {
	s := newPrivacyDeletionHTTPFixture(t)
	s.Cfg.Auth.Enabled = true
	for _, spec := range []struct {
		method  string
		path    string
		handler http.Handler
	}{
		{http.MethodGet, "/api/privacy-deletions", handlePrivacyDeletionCollection(s)},
		{http.MethodGet, "/api/privacy-deletions/synthetic-job", handlePrivacyDeletionItem(s)},
		{http.MethodPost, "/api/privacy-deletions/synthetic-job/retry", handlePrivacyDeletionItem(s)},
	} {
		t.Run(spec.method+spec.path, func(t *testing.T) {
			for _, bearer := range []bool{false, true} {
				request := httptest.NewRequest(spec.method, spec.path, nil).WithContext(context.Background())
				if bearer {
					request.Header.Set("Authorization", "Bearer synthetic-non-admin")
				}
				response := httptest.NewRecorder()
				requireAdmin(s, spec.handler).ServeHTTP(response, request)
				if response.Code != http.StatusUnauthorized && response.Code != http.StatusForbidden {
					t.Fatalf("unprivileged request reached privacy endpoint: %d", response.Code)
				}
			}
		})
	}
}
