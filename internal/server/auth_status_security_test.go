package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestPublicSecurityStatusOmitsNetworkAndAccountDetails(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Server.HTTPS.Domain = "private.example"
	cfg.Server.HTTPS.Email = "admin@example.com"
	cfg.Server.HTTPS.HTTPSPort = 8443
	s := &Server{Cfg: cfg}
	rec := httptest.NewRecorder()
	handleSecurityStatus(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/security/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	for _, leaked := range []string{"private.example", "admin@example.com", "8443", "domain", "email", "http_port", "https_port", "proto"} {
		if strings.Contains(rec.Body.String(), leaked) {
			t.Fatalf("public status contains %q", leaked)
		}
	}
}

func TestLockdownLogsOnceAcrossRequests(t *testing.T) {
	var logs bytes.Buffer
	s := &Server{Cfg: &config.Config{}, Logger: slog.New(slog.NewTextHandler(&logs, nil))}
	s.Cfg.Auth.Enabled = true
	handler := authMiddleware(s, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	for range 3 {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d", rec.Code)
		}
	}
	if count := strings.Count(logs.String(), "LOCKDOWN"); count != 1 {
		t.Fatalf("lockdown log count = %d", count)
	}
}
