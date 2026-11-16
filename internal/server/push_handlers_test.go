package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/push"
	"aurago/internal/security"
)

func TestPushMutationsRequireAdministrator(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("a", 64), t.TempDir()+"/vault.bin")
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := security.NewTokenManager(vault, t.TempDir()+"/tokens.bin")
	if err != nil {
		t.Fatal(err)
	}
	readToken, _, err := tokens.Create("read", []string{"read"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	adminToken, _, err := tokens.Create("admin", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: &config.Config{}, TokenManager: tokens, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.PasswordHash = "configured"
	s.Cfg.Auth.SessionSecret = "push-test-session"
	previous := push.GlobalManager
	push.GlobalManager = nil
	t.Cleanup(func() { push.GlobalManager = previous })
	for _, path := range []string{"/api/push/subscribe", "/api/push/unsubscribe"} {
		if !isAdminProtectedPath(path) {
			t.Fatal("push mutation missing from admin path policy")
		}
		handler := handlePushSubscribe(s)
		if strings.HasSuffix(path, "unsubscribe") {
			handler = handlePushUnsubscribe(s)
		}
		for _, kind := range []string{"anonymous", "reader", "admin", "browser"} {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
			if kind == "reader" {
				req.Header.Set("Authorization", "Bearer "+readToken)
			}
			if kind == "admin" {
				req.Header.Set("Authorization", "Bearer "+adminToken)
			}
			if kind == "browser" {
				req.Header.Set("Origin", "http://example.com")
				req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
			}
			rec := httptest.NewRecorder()
			authMiddleware(s, requireAdmin(s, handler)).ServeHTTP(rec, req)
			want := http.StatusServiceUnavailable
			if kind == "reader" {
				want = http.StatusForbidden
			}
			if kind == "anonymous" {
				want = http.StatusUnauthorized
			}
			if rec.Code != want {
				t.Fatalf("%s/%s = %d, want %d", path, kind, rec.Code, want)
			}
		}
	}
}
