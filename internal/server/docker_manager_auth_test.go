package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestDockerManagerRoutesKeepAdminScopeThroughAuthMiddleware pins that the
// Docker Manager API sits behind the same gate as /api/containers: read and
// desktop:admin tokens are refused with invalid_bearer_scope, a browser session
// and an admin token reach the handler, anonymous gets 401.
func TestDockerManagerRoutesKeepAdminScopeThroughAuthMiddleware(t *testing.T) {
	_, chain, adminToken, desktopToken, readToken := newContainerAuthChain(t)

	requests := []func() *http.Request{
		func() *http.Request { return httptest.NewRequest(http.MethodGet, "/api/docker-manager/hosts", nil) },
		func() *http.Request {
			return httptest.NewRequest(http.MethodPost, "/api/docker-manager/hosts/local/containers/web1/restart", nil)
		},
		func() *http.Request {
			return httptest.NewRequest(http.MethodDelete, "/api/docker-manager/hosts/local/containers/web1", nil)
		},
	}
	for _, build := range requests {
		for _, tc := range []struct {
			name  string
			token string
			want  int
		}{
			{"read scope", readToken, http.StatusForbidden},
			{"desktop admin scope", desktopToken, http.StatusForbidden},
			// Docker is disabled in this fixture: an admitted request reaches the
			// handler's own 503 docker_disabled.
			{"admin scope", adminToken, http.StatusServiceUnavailable},
		} {
			req := build()
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rec := httptest.NewRecorder()
			chain.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("%s %s with %s: status = %d, want %d; body=%s", req.Method, req.URL.Path, tc.name, rec.Code, tc.want, rec.Body.String())
			}
			if tc.want == http.StatusForbidden && !strings.Contains(rec.Body.String(), "invalid_bearer_scope") {
				t.Fatalf("%s %s with %s: body = %s, want invalid_bearer_scope", req.Method, req.URL.Path, tc.name, rec.Body.String())
			}
			if tc.want == http.StatusServiceUnavailable && !strings.Contains(rec.Body.String(), "docker_disabled") {
				t.Fatalf("%s %s with %s: body = %s, want the handler's docker_disabled", req.Method, req.URL.Path, tc.name, rec.Body.String())
			}
		}

		session := &http.Cookie{Name: sessionCookieName, Value: createSessionValue(bearerSchemeTestSessionSecret, time.Now().Add(time.Hour))}
		req := build()
		req.AddCookie(session)
		if req.Method != http.MethodGet {
			req.Header.Set("Origin", "http://example.com")
		}
		rec := httptest.NewRecorder()
		chain.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "docker_disabled") {
			t.Fatalf("%s %s with a browser session: status = %d, want 503 docker_disabled from the handler; body=%s", req.Method, req.URL.Path, rec.Code, rec.Body.String())
		}

		req = build()
		rec = httptest.NewRecorder()
		chain.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous %s %s: status = %d, want 401", req.Method, req.URL.Path, rec.Code)
		}
	}
}

// TestDockerManagerRoutesStayInTheAdminBearerCatchAll fails when a future change
// moves the Docker Manager paths into a list that would admit weaker credentials.
func TestDockerManagerRoutesStayInTheAdminBearerCatchAll(t *testing.T) {
	s, _, adminToken, desktopToken, readToken := newContainerAuthChain(t)
	tokens := map[string]string{"read": readToken, desktopScopeAdmin: desktopToken}
	for _, scope := range []string{desktopScopeRead, desktopScopeWrite, go2RTCViewScope, "cyd"} {
		raw, _, err := s.TokenManager.Create(scope, []string{scope}, nil)
		if err != nil {
			t.Fatalf("create %s token: %v", scope, err)
		}
		tokens[scope] = raw
	}

	for _, path := range []string{"/api/docker-manager/", "/api/docker-manager/hosts", "/api/docker-manager/hosts/local/containers", "/api/docker-manager/hosts/import-nest/n1"} {
		if isDesktopScopedAPIPath(path) {
			t.Fatalf("%s must not become a desktop-scoped path: desktop:read/write tokens would reach Docker", path)
		}
		if isAuthBypassed(path) {
			t.Fatalf("%s must not bypass session authentication", path)
		}
		if isAllowedWithoutPassword(path) {
			t.Fatalf("%s must not be reachable during the password lockdown", path)
		}
		if isDesktopEmbedResourcePath(path) {
			t.Fatalf("%s must not accept desktop embed tickets", path)
		}
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
			if !validRouteBearer(s, adminToken, path, method) {
				t.Fatalf("%s %s: admin token refused", method, path)
			}
			for scope, raw := range tokens {
				if validRouteBearer(s, raw, path, method) {
					t.Fatalf("%s %s: a %q token is admitted; Docker Manager routes require the admin scope", method, path, scope)
				}
			}
		}
	}
}
