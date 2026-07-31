package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/planner"
)

func testOperationalIssueServer(t *testing.T) *Server {
	t.Helper()
	db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatalf("planner.InitDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &Server{PlannerDB: db}
}

func TestOperationalIssueListUsesSanitizedOpaqueIDs(t *testing.T) {
	s := testOperationalIssueServer(t)
	fingerprint := "heartbeat|heartbeat|tool|save_note"
	if _, err := planner.RecordOperationalIssue(s.PlannerDB, planner.OperationalIssue{
		Fingerprint: fingerprint, Source: "heartbeat", Context: "heartbeat",
		Title: "Tool failed", Detail: "api_key=secret-value", Severity: "warning",
		OccurredAt: time.Now(),
	}); err != nil {
		t.Fatalf("RecordOperationalIssue() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/operational-issues?status=all", nil)
	rec := httptest.NewRecorder()
	handleOperationalIssues(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, forbidden := range []string{fingerprint, "secret-value", "api_key=secret-value"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response contains forbidden value %q: %s", forbidden, body)
		}
	}
	var response struct {
		Items []struct {
			ID     string `json:"id"`
			Detail string `json:"detail"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 || len(response.Items[0].ID) != 64 {
		t.Fatalf("items = %#v, want one opaque SHA-256 ID", response.Items)
	}
	if !strings.Contains(response.Items[0].Detail, "[redacted]") {
		t.Fatalf("detail = %q, want redaction", response.Items[0].Detail)
	}
}

func TestOperationalIssueMutationsRequireSameOriginAndStrictJSON(t *testing.T) {
	s := testOperationalIssueServer(t)
	fingerprint, err := planner.RecordOperationalIssue(s.PlannerDB, planner.OperationalIssue{
		Fingerprint: "maintenance|nightly", Source: "maintenance", Context: "nightly",
		Title: "Maintenance warning", Detail: "temporary failure", Severity: "warning",
		OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("RecordOperationalIssue() error = %v", err)
	}
	publicID := planner.OperationalIssuePublicID(fingerprint)
	url := "http://example.test/api/operational-issues/" + publicID + "/archive"

	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(`{"reason":"reviewed"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://attacker.test")
	rec := httptest.NewRecorder()
	handleOperationalIssues(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	req = httptest.NewRequest(http.MethodPost, url, strings.NewReader(`{"reason":"reviewed","unknown":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://example.test")
	rec = httptest.NewRecorder()
	handleOperationalIssues(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown-field status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, url, strings.NewReader(`{"reason":"reviewed"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://example.test")
	rec = httptest.NewRecorder()
	handleOperationalIssues(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("archive status = %d, body = %s", rec.Code, rec.Body.String())
	}
	page, err := planner.ListOperationalIssues(s.PlannerDB, planner.OperationalIssueListFilter{Status: "archived"})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("archived page = %#v, err %v", page, err)
	}
}

func TestOperationalIssueBulkArchiveRequiresExplicitConfirmation(t *testing.T) {
	s := testOperationalIssueServer(t)
	if _, err := planner.RecordOperationalIssue(s.PlannerDB, planner.OperationalIssue{
		Fingerprint: "heartbeat|old-warning", Source: "heartbeat", Context: "heartbeat",
		Title: "Old warning", Detail: "one old failure", Severity: "warning",
		OccurredAt: time.Now().Add(-8 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("RecordOperationalIssue() error = %v", err)
	}

	for _, tc := range []struct {
		body       string
		wantStatus int
	}{
		{body: `{}`, wantStatus: http.StatusBadRequest},
		{body: `{"confirm":"ARCHIVE_STALE_ISSUES"}`, wantStatus: http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodPost, "http://example.test/api/operational-issues/archive-stale", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://example.test")
		rec := httptest.NewRecorder()
		handleOperationalIssues(s).ServeHTTP(rec, req)
		if rec.Code != tc.wantStatus {
			t.Fatalf("body %s status = %d, body = %s, want %d", tc.body, rec.Code, rec.Body.String(), tc.wantStatus)
		}
	}
}
