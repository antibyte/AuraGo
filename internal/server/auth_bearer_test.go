package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

const bearerSchemeTestSessionSecret = "bearer-scheme-session-secret"

func newBearerSchemeTestServer(t *testing.T) (s *Server, adminToken, desktopToken, readToken string) {
	t.Helper()
	dir := t.TempDir()
	vault, err := security.NewVault(strings.Repeat("d", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	tokens, err := security.NewTokenManager(vault, filepath.Join(dir, "tokens.bin"))
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	if adminToken, _, err = tokens.Create("admin", []string{"admin"}, nil); err != nil {
		t.Fatalf("Create(admin): %v", err)
	}
	if desktopToken, _, err = tokens.Create("desktop", []string{desktopScopeAdmin}, nil); err != nil {
		t.Fatalf("Create(desktop): %v", err)
	}
	if readToken, _, err = tokens.Create("reader", []string{"read"}, nil); err != nil {
		t.Fatalf("Create(read): %v", err)
	}
	s = &Server{Cfg: &config.Config{}, TokenManager: tokens}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = bearerSchemeTestSessionSecret
	s.Cfg.Auth.PasswordHash = "configured"
	return s, adminToken, desktopToken, readToken
}

func TestBearerSchemeIsCaseInsensitiveAcrossGates(t *testing.T) {
	t.Parallel()
	s, adminToken, desktopToken, _ := newBearerSchemeTestServer(t)
	adminGate := authMiddleware(s, requireAdmin(s, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	for _, scheme := range []string{"Bearer ", "bearer ", "BEARER  ", "Bearer\t"} {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/stop", nil)
		req.Header.Set("Authorization", scheme+adminToken)
		rec := httptest.NewRecorder()
		adminGate.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%q: requireAdmin status = %d, want 204; body=%s", scheme, rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/api/desktop/files", nil)
		req.Header.Set("Authorization", scheme+desktopToken)
		if !requireDesktopPermission(s, httptest.NewRecorder(), req, desktopScopeAdmin) {
			t.Fatalf("%q: requireDesktopPermission rejected a desktop:admin token", scheme)
		}

		req = httptest.NewRequest(http.MethodGet, "/api/daemons", nil)
		req.Header.Set("Authorization", scheme+adminToken)
		if !isDaemonAuthOK(s, req) {
			t.Fatalf("%q: daemon API rejected an admin-scope token", scheme)
		}

		req = httptest.NewRequest(http.MethodGet, "/api/cyd/snapshot", nil)
		req.Header.Set("Authorization", scheme+"K7M 2PQ 9XH")
		if got := cydBearerToken(req); got != "K7M 2PQ 9XH" {
			t.Fatalf("%q: cydBearerToken = %q, want the grouped code", scheme, got)
		}
	}
}

func TestDaemonAuthRequiresAdminScopeForBearer(t *testing.T) {
	t.Parallel()
	s, _, _, readToken := newBearerSchemeTestServer(t)
	session := &http.Cookie{Name: sessionCookieName, Value: createSessionValue(bearerSchemeTestSessionSecret, time.Now().Add(time.Hour))}

	req := httptest.NewRequest(http.MethodGet, "/api/daemons", nil)
	req.Header.Set("Authorization", "Bearer "+readToken)
	req.AddCookie(session)
	if isDaemonAuthOK(s, req) {
		t.Fatal("a non-admin bearer token must not fall back to the session cookie")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/daemons", nil)
	req.AddCookie(session)
	if !isDaemonAuthOK(s, req) {
		t.Fatal("valid browser session rejected")
	}
}

func TestBearerCredentialParsing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		header, token string
		present       bool
	}{
		{"Bearer abc", "abc", true},
		{"bearer abc", "abc", true},
		{"  BEARER\tabc  ", "abc", true},
		{"Bearer", "", true},
		{"Bearer a b", "", true},
		{"Bearerabc", "", false},
		{"Basic abc", "", false},
		{"", "", false},
	} {
		token, present := bearerCredential(tc.header)
		if token != tc.token || present != tc.present {
			t.Fatalf("bearerCredential(%q) = %q, %v; want %q, %v", tc.header, token, present, tc.token, tc.present)
		}
	}
}
