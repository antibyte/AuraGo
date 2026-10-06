package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

func TestContainerTerminalRejectsDockerDisabledBeforeUpgrade(t *testing.T) {
	s := testContainerServer(false, false)
	rec := httptest.NewRecorder()
	req := newContainerTerminalUpgradeRequest("/api/containers/demo/terminal")

	handleContainerAction(s)(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if rec.Header().Get("Upgrade") != "" {
		t.Fatal("handler upgraded websocket while Docker was disabled")
	}
}

func TestContainerTerminalRejectsDockerReadOnlyBeforeUpgrade(t *testing.T) {
	s := testContainerServer(true, true)
	rec := httptest.NewRecorder()
	req := newContainerTerminalUpgradeRequest("/api/containers/demo/terminal")

	handleContainerAction(s)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if rec.Header().Get("Upgrade") != "" {
		t.Fatal("handler upgraded websocket while Docker was read-only")
	}
}

func TestContainerUpdateRejectsDockerReadOnly(t *testing.T) {
	s := testContainerServer(true, true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/containers/demo/update", nil)

	handleContainerAction(s)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestContainerTerminalRejectsStoppedContainerBeforeUpgrade(t *testing.T) {
	s := testContainerServer(true, false)
	fake := &fakeContainerTerminalBackend{running: false}
	restore := replaceContainerTerminalBackend(fake)
	defer restore()
	defer replaceContainerProtection(containerProtection{})()

	rec := httptest.NewRecorder()
	req := newContainerTerminalUpgradeRequest("/api/containers/demo/terminal")

	handleContainerAction(s)(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if rec.Header().Get("Upgrade") != "" {
		t.Fatal("handler upgraded websocket for a stopped container")
	}
	if fake.createCalls != 0 {
		t.Fatalf("terminal session was created %d times for stopped container", fake.createCalls)
	}
}

func TestContainerTerminalRejectsCrossOriginBeforeUpgrade(t *testing.T) {
	s := testContainerServer(true, false)
	fake := &fakeContainerTerminalBackend{running: true}
	restore := replaceContainerTerminalBackend(fake)
	defer restore()
	defer replaceContainerProtection(containerProtection{})()

	rec := httptest.NewRecorder()
	req := newContainerTerminalUpgradeRequest("/api/containers/demo/terminal")
	req.Header.Set("Origin", "http://evil.example")

	handleContainerAction(s)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if rec.Header().Get("Upgrade") != "" {
		t.Fatal("handler upgraded cross-origin websocket")
	}
	if fake.createCalls != 0 {
		t.Fatalf("terminal session was created %d times for rejected origin", fake.createCalls)
	}
}

func TestContainerTerminalResizeControlCallsBackend(t *testing.T) {
	s := testContainerServer(true, false)
	session := newFakeContainerTerminalSession()
	fake := &fakeContainerTerminalBackend{running: true, session: session}
	restore := replaceContainerTerminalBackend(fake)
	defer restore()
	defer replaceContainerProtection(containerProtection{})()

	ts := httptest.NewServer(handleContainerAction(s))
	defer ts.Close()

	wsURL := "ws" + ts.URL[len("http"):] + "/api/containers/demo/terminal"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial terminal websocket: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]interface{}{"type": "resize", "cols": 120, "rows": 32}); err != nil {
		t.Fatalf("write resize control: %v", err)
	}

	select {
	case got := <-session.resizeCalls:
		if got.cols != 120 || got.rows != 32 {
			t.Fatalf("resize = %+v, want cols=120 rows=32", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for resize call")
	}
}

func testContainerServer(dockerEnabled, dockerReadOnly bool) *Server {
	cfg := &config.Config{}
	cfg.Docker.Enabled = dockerEnabled
	cfg.Docker.ReadOnly = dockerReadOnly
	return &Server{Cfg: cfg, Logger: slog.Default()}
}

func newContainerTerminalUpgradeRequest(path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-Websocket-Version", "13")
	req.Header.Set("Sec-Websocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	return req
}

func replaceContainerTerminalBackend(next containerTerminalBackend) func() {
	old := activeContainerTerminalBackend
	activeContainerTerminalBackend = next
	return func() {
		activeContainerTerminalBackend = old
	}
}

type fakeContainerTerminalBackend struct {
	running         bool
	session         *fakeContainerTerminalSession
	createCalls     int
	lastContainerID string
	lastExecCmd     []string
}

func (f *fakeContainerTerminalBackend) ContainerRunning(ctx context.Context, cfg tools.DockerConfig, containerID string) (bool, error) {
	return f.running, nil
}

func (f *fakeContainerTerminalBackend) CreateSession(ctx context.Context, cfg tools.DockerConfig, containerID string, cols, rows int, execCmd []string) (containerTerminalSession, error) {
	f.createCalls++
	f.lastContainerID = containerID
	f.lastExecCmd = append([]string(nil), execCmd...)
	if f.session == nil {
		f.session = newFakeContainerTerminalSession()
	}
	return f.session, nil
}

type terminalResizeCall struct {
	cols int
	rows int
}

type fakeContainerTerminalSession struct {
	closeOnce   sync.Once
	closed      chan struct{}
	resizeCalls chan terminalResizeCall
}

func newFakeContainerTerminalSession() *fakeContainerTerminalSession {
	return &fakeContainerTerminalSession{
		closed:      make(chan struct{}),
		resizeCalls: make(chan terminalResizeCall, 4),
	}
}

func (s *fakeContainerTerminalSession) Read(p []byte) (int, error) {
	<-s.closed
	return 0, io.EOF
}

func (s *fakeContainerTerminalSession) Write(p []byte) (int, error) {
	return len(p), nil
}

func (s *fakeContainerTerminalSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
	})
	return nil
}

func (s *fakeContainerTerminalSession) Resize(ctx context.Context, cols, rows int) error {
	s.resizeCalls <- terminalResizeCall{cols: cols, rows: rows}
	return nil
}

// newContainerAuthChain registers the tool API routes (which include the
// container routes) behind authMiddleware with session auth enabled.
func newContainerAuthChain(t *testing.T) (s *Server, chain http.Handler, adminToken, desktopToken, readToken string) {
	t.Helper()
	s, adminToken, desktopToken, readToken = newBearerSchemeTestServer(t)
	s.Cfg.WebConfig.Enabled = true
	s.Logger = slog.Default()
	mux := http.NewServeMux()
	s.registerToolAPIRoutes(mux)
	return s, authMiddleware(s, mux), adminToken, desktopToken, readToken
}

// TestContainerRoutesKeepAdminScopeThroughAuthMiddleware pins today's gate:
// read and desktop:admin tokens are refused with invalid_bearer_scope, a
// browser session and an admin token reach the handler, anonymous gets 401.
func TestContainerRoutesKeepAdminScopeThroughAuthMiddleware(t *testing.T) {
	_, chain, adminToken, desktopToken, readToken := newContainerAuthChain(t)

	requests := []func() *http.Request{
		func() *http.Request { return httptest.NewRequest(http.MethodGet, "/api/containers", nil) },
		func() *http.Request { return newContainerTerminalUpgradeRequest("/api/containers/demo/terminal") },
		func() *http.Request { return httptest.NewRequest(http.MethodPost, "/api/containers/demo/restart", nil) },
	}
	for _, build := range requests {
		for _, tc := range []struct {
			name  string
			token string
			want  int
		}{
			{"read scope", readToken, http.StatusForbidden},
			{"desktop admin scope", desktopToken, http.StatusForbidden},
			// Docker is disabled in this fixture: an admitted request reaches
			// the handler's own 503 "Docker is not enabled".
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
			if tc.want == http.StatusServiceUnavailable && !strings.Contains(rec.Body.String(), "Docker is not enabled") {
				t.Fatalf("%s %s with %s: body = %s, want the handler's Docker is not enabled", req.Method, req.URL.Path, tc.name, rec.Body.String())
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
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "Docker is not enabled") {
			t.Fatalf("%s %s with a browser session: status = %d, want 503 Docker is not enabled from the handler; body=%s", req.Method, req.URL.Path, rec.Code, rec.Body.String())
		}

		req = build()
		rec = httptest.NewRecorder()
		chain.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous %s %s: status = %d, want 401", req.Method, req.URL.Path, rec.Code)
		}
	}
}

// TestContainerRoutesStayInTheAdminBearerCatchAll fails when a future change
// moves the container paths into a list that would admit weaker credentials.
func TestContainerRoutesStayInTheAdminBearerCatchAll(t *testing.T) {
	s, _, adminToken, desktopToken, readToken := newContainerAuthChain(t)
	tokens := map[string]string{"read": readToken, desktopScopeAdmin: desktopToken}
	for _, scope := range []string{desktopScopeRead, desktopScopeWrite, go2RTCViewScope, "cyd"} {
		raw, _, err := s.TokenManager.Create(scope, []string{scope}, nil)
		if err != nil {
			t.Fatalf("create %s token: %v", scope, err)
		}
		tokens[scope] = raw
	}

	for _, path := range []string{"/api/containers", "/api/containers/", "/api/containers/demo/terminal", "/api/containers/demo/update", "/api/containers/demo"} {
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
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
			if !validRouteBearer(s, adminToken, path, method) {
				t.Fatalf("%s %s: admin token refused", method, path)
			}
			for scope, raw := range tokens {
				if validRouteBearer(s, raw, path, method) {
					t.Fatalf("%s %s: a %q token is admitted; container routes require the admin scope", method, path, scope)
				}
			}
		}
	}
}

// TestContainerRoutesStayOpenWhenAuthIsDisabled pins installs without login:
// a request without credentials still reaches the handler.
func TestContainerRoutesStayOpenWhenAuthIsDisabled(t *testing.T) {
	s := testContainerServer(false, false)
	s.Cfg.WebConfig.Enabled = true
	mux := http.NewServeMux()
	s.registerToolAPIRoutes(mux)

	rec := httptest.NewRecorder()
	authMiddleware(s, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "Docker is not enabled") {
		t.Fatalf("auth-disabled status = %d, want 503 Docker is not enabled from the handler; body=%s", rec.Code, rec.Body.String())
	}
}

// TestContainerRoutesAuthDisabledIgnoreBearerScope pins that, with
// auth.enabled=false, a Bearer token of any scope still reaches the container
// handlers. A requireAdmin-style wrapper on the routes would refuse the
// non-admin tokens here and change that behaviour.
func TestContainerRoutesAuthDisabledIgnoreBearerScope(t *testing.T) {
	s, chain, _, desktopToken, readToken := newContainerAuthChain(t)
	s.Cfg.Auth.Enabled = false // authMiddleware reads cfg per request
	for _, tok := range []string{readToken, desktopToken} {
		for _, req := range []*http.Request{
			httptest.NewRequest(http.MethodGet, "/api/containers", nil),
			httptest.NewRequest(http.MethodPost, "/api/containers/demo/restart", nil),
		} {
			req.Header.Set("Authorization", "Bearer "+tok)
			rec := httptest.NewRecorder()
			chain.ServeHTTP(rec, req)
			if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "Docker is not enabled") {
				t.Fatalf("%s %s: status=%d body=%s", req.Method, req.URL.Path, rec.Code, rec.Body.String())
			}
		}
	}
}

// TestContainerTerminalWebSocketWorksThroughAuthChain pins that a browser
// session opens the terminal WebSocket through authMiddleware.
func TestContainerTerminalWebSocketWorksThroughAuthChain(t *testing.T) {
	s, chain, _, _, _ := newContainerAuthChain(t)
	s.Cfg.Docker.Enabled = true
	fake := &fakeContainerTerminalBackend{running: true}
	restore := replaceContainerTerminalBackend(fake)
	defer restore()
	defer replaceContainerProtection(containerProtection{})()
	ts := httptest.NewServer(chain)
	defer ts.Close()

	header := http.Header{}
	header.Set("Cookie", sessionCookieName+"="+createSessionValue(bearerSchemeTestSessionSecret, time.Now().Add(time.Hour)))
	conn, resp, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):]+"/api/containers/demo/terminal", header)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("session terminal through authMiddleware: %v (HTTP %d)", err, status)
	}
	_ = conn.Close()
	if fake.createCalls != 1 {
		t.Fatalf("terminal sessions created = %d, want 1", fake.createCalls)
	}
}
