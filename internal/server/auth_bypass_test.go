package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestAuthBypassMatchesExactPathsOrSegmentSubtrees(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"/api/health", "/api/ready", "/api/i18n", "/api/setup", "/api/setup/status", "/api/setup/local-llm/job",
		"/auth/login", "/webhook/hook-1", "/mcp", "/js/app.js", "/css/site.css", "/shared.css",
		"/api/cyd/speak/ntf_x", "/api/agodesk/media/upload/a1", "/api/game-maker/preview/token/index.html", "/favicon.ico",
	} {
		if !isAuthBypassed(path) {
			t.Fatalf("%s should bypass session auth", path)
		}
	}
	for _, path := range []string{
		"/api/health/discord", "/api/healthz", "/api/readyz", "/api/i18n/de", "/api/setupx", "/mcpx", "/mcp/tools",
		"/setupx", "/setup/x", "/api/auth/status/x", "/api/cyd/snapshotx", "/api/remote/ws2", "/shared.css.map", "/webhook",
	} {
		if isAuthBypassed(path) {
			t.Fatalf("%s must require a session", path)
		}
	}
}

func TestAuthBypassSubtreesEndWithSlash(t *testing.T) {
	t.Parallel()
	for _, list := range [][]string{authBypassSubtrees, noPasswordSubtrees} {
		for _, prefix := range list {
			if !strings.HasSuffix(prefix, "/") {
				t.Fatalf("subtree %q must end with / so it cannot match sibling paths", prefix)
			}
		}
	}
}

func TestAuthLockdownAllowsOnlyExactSetupDependencies(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/api/auth/password", "/api/setup", "/api/setup/status", "/setup", "/js/setup/main.js", "/api/personalities", "/auth/login", "/api/i18n"} {
		if !isAllowedWithoutPassword(path) {
			t.Fatalf("%s must stay reachable during the lockdown", path)
		}
	}
	for _, path := range []string{"/api/auth/passwordx", "/api/personalitiesx", "/api/setupx", "/setupx", "/api/i18n/x", "/api/health/discord"} {
		if isAllowedWithoutPassword(path) {
			t.Fatalf("%s must stay blocked during the lockdown", path)
		}
	}
}

func TestAuthMiddlewareRequiresSessionForDiscordHealth(t *testing.T) {
	t.Parallel()
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	s.Cfg.Auth.PasswordHash = "configured"
	handler := authMiddleware(s, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for path, want := range map[string]int{"/api/health": http.StatusNoContent, "/api/health/discord": http.StatusUnauthorized} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, want)
		}
	}
}
