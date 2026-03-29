package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestHandleTrueNASPoolsHidesClientInitDetails(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.TrueNAS.Enabled = true

	s := &Server{
		Cfg:    cfg,
		Logger: slog.Default(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/truenas/pools", nil)
	rec := httptest.NewRecorder()

	handleTrueNASPools(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Failed to initialize TrueNAS client") || strings.Contains(strings.ToLower(body), "host is required") {
		t.Fatalf("expected generic TrueNAS init error, got %q", body)
	}
}
