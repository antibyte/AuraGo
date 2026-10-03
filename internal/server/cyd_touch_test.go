package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCYDRateLimitedSnapshotDoesNotTouchToken(t *testing.T) {
	s, raw := testCYDServer(t)
	metas := s.TokenManager.List()
	if len(metas) != 1 {
		t.Fatalf("tokens = %d, want 1", len(metas))
	}
	tokenID := metas[0].ID
	if !cydSnapshotLimiter.allow(tokenID, 0) { // exhaust the 500 ms window
		t.Fatal("priming call was rate limited")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/cyd/snapshot", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	handleCYDSnapshot(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	meta, err := s.TokenManager.Get(tokenID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if meta.LastUsedAt != nil {
		t.Fatalf("rate-limited snapshot recorded token use at %v", meta.LastUsedAt)
	}
}
