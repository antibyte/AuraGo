package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/gorilla/websocket"
)

func TestDesktopSerialGuardPermissions(t *testing.T) {
	s, readToken, _ := testDesktopPermissionServer(t)
	adminToken, _, err := s.TokenManager.Create("serial admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, token                            string
		auth, enabled, host, readonly, connect bool
		status                                 int
	}{
		{"anonymous", "", true, true, true, false, false, 401},
		{"read scope", readToken, true, true, true, false, false, 403},
		{"admin list", adminToken, true, true, true, false, false, 204},
		{"admin connect", adminToken, true, true, true, false, true, 204},
		{"host disabled", adminToken, true, true, false, false, true, 403},
		{"desktop disabled", adminToken, true, false, true, false, true, 403},
		{"readonly open", adminToken, true, true, true, true, true, 403},
		{"readonly list", adminToken, true, true, true, true, false, 204},
		{"auth disabled", "", false, true, true, false, true, 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s.CfgMu.Lock()
			s.Cfg.Auth.Enabled = tc.auth
			s.Cfg.VirtualDesktop.Enabled = tc.enabled
			s.Cfg.VirtualDesktop.SerialHostEnabled = tc.host
			s.Cfg.VirtualDesktop.ReadOnly = tc.readonly
			s.CfgMu.Unlock()
			r := httptest.NewRequest(http.MethodGet, "/api/desktop/serial/connect", nil)
			ctx, cancel := context.WithCancel(r.Context())
			defer cancel()
			r = r.WithContext(ctx)
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			withDesktopSerialGuard(s, tc.connect, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d, want=%d: %s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}

func TestDesktopSerialPolicyProjectsLiveConfiguration(t *testing.T) {
	s, readToken, _ := testDesktopPermissionServer(t)
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.VirtualDesktop.SerialHostEnabled = true
	s.Cfg.VirtualDesktop.SerialBrowserEnabled = true
	s.Cfg.VirtualDesktop.RemoteMaxSessionMinutes = 15
	s.Cfg.VirtualDesktop.RemoteIdleTimeoutMinutes = 2
	policy := s.desktopSerialPolicy(nil)
	if !policy.SerialHostEnabled || !policy.SerialBrowserEnabled || policy.RemoteMaxSessionMinutes != 15 || policy.RemoteIdleTimeoutMinutes != 2 {
		t.Fatalf("policy: %+v", policy)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/desktop/bootstrap", nil)
	r.Header.Set("Authorization", "Bearer "+readToken)
	if scoped := s.desktopSerialPolicy(r); scoped.SerialHostEnabled || scoped.SerialBrowserEnabled || !scoped.ReadOnly {
		t.Fatalf("read token policy: %+v", scoped)
	}
	payload := filterDesktopBootstrap(s, r, desktop.BootstrapPayload{SerialBrowserEnabled: true, SerialHostEnabled: true})
	if payload.SerialBrowserEnabled || payload.SerialHostEnabled {
		t.Fatal("serial capabilities leaked to a non-admin bootstrap")
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	if policy := s.desktopSerialPolicy(nil); policy.SerialBrowserEnabled || policy.SerialHostEnabled {
		t.Fatal("readonly still allows serial connections")
	}
}

func TestDesktopSerialRevocationClosesIdleRun(t *testing.T) {
	for _, reason := range []string{"feature", "readonly", "token", "shutdown"} {
		t.Run(reason, func(t *testing.T) {
			s, _, _ := testDesktopPermissionServer(t)
			token, meta, err := s.TokenManager.Create("serial admin", []string{desktopScopeAdmin}, nil)
			if err != nil {
				t.Fatal(err)
			}
			s.Cfg.VirtualDesktop.Enabled = true
			s.Cfg.VirtualDesktop.SerialHostEnabled = true
			integrationCtx, shutdown := context.WithCancel(context.Background())
			defer shutdown()
			s.integrationCtx = integrationCtx
			entered, finished := make(chan struct{}), make(chan struct{})
			handler := withDesktopSerialGuard(s, true, func(_ http.ResponseWriter, r *http.Request) {
				close(entered)
				<-r.Context().Done()
			})
			r := httptest.NewRequest(http.MethodGet, "/api/desktop/serial/connect", nil)
			ctx, cancel := context.WithCancel(r.Context())
			defer cancel()
			r = r.WithContext(ctx)
			r.Header.Set("Authorization", "Bearer "+token)
			go func() { defer close(finished); handler(httptest.NewRecorder(), r) }()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("run was not admitted")
			}
			s.CfgMu.Lock()
			switch reason {
			case "feature":
				s.Cfg.VirtualDesktop.SerialHostEnabled = false
			case "readonly":
				s.Cfg.VirtualDesktop.ReadOnly = true
			case "token":
				if err := s.TokenManager.Delete(meta.ID); err != nil {
					t.Error(err)
				}
			case "shutdown":
				shutdown()
			}
			s.CfgMu.Unlock()
			select {
			case <-finished:
			case <-time.After(3 * time.Second):
				t.Fatal("idle serial run survived revocation")
			}
		})
	}
}

func TestDesktopSerialPermissionsPolicy(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }), false, false)
	for path, want := range map[string]string{
		"/desktop": "serial=(self)", "/desktop.html": "serial=(self)",
		"/files/desktop/Apps/example/index.html": "serial=()", "/api/game-maker/preview/example": "serial=()", "/": "serial=()",
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if got := w.Header().Get("Permissions-Policy"); got != want {
			t.Errorf("%s: %q, want %q", path, got, want)
		}
	}
}

func TestDesktopSerialCookieRevocationAndExpiry(t *testing.T) {
	for _, reason := range []string{"logout", "expiry"} {
		t.Run(reason, func(t *testing.T) {
			s, _, _ := testDesktopPermissionServer(t)
			s.Cfg.Auth.SessionSecret = t.Name()
			s.Cfg.VirtualDesktop.Enabled = true
			s.Cfg.VirtualDesktop.SerialHostEnabled = true
			expires := time.Now().Add(time.Hour)
			if reason == "expiry" {
				expires = time.Now().Add(2 * time.Second)
			}
			r := httptest.NewRequest(http.MethodGet, "/api/desktop/serial/connect", nil)
			ctx, cancel := context.WithCancel(r.Context())
			defer cancel()
			r = r.WithContext(ctx)
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, expires)})
			entered, finished := make(chan struct{}), make(chan struct{})
			handler := withDesktopSerialGuard(s, true, func(_ http.ResponseWriter, request *http.Request) {
				close(entered)
				<-request.Context().Done()
			})
			go func() { defer close(finished); handler(httptest.NewRecorder(), r) }()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("valid cookie was not admitted")
			}
			if reason == "logout" && !revokeRequestSession(s, r) {
				t.Fatal("could not revoke serial session")
			}
			select {
			case <-finished:
			case <-time.After(4 * time.Second):
				t.Fatal("serial connection survived " + reason)
			}
		})
	}
}

func TestDesktopSerialPolicyUpdatesWithoutRecreatingService(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	s.Cfg.VirtualDesktop.SerialBrowserEnabled = true
	s.Cfg.VirtualDesktop.SerialHostEnabled = true
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	host := httptest.NewServer(handleDesktopWS(s))
	defer host.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(host.URL, "http")+"/api/desktop/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	var event struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatal(err)
	}
	var boot desktop.BootstrapPayload
	if err := json.Unmarshal(event.Payload, &boot); err != nil || event.Type != "welcome" || !boot.SerialBrowserEnabled || !boot.SerialHostEnabled {
		t.Fatalf("welcome lacks serial capabilities: %s (%v)", event.Payload, err)
	}
	s.CfgMu.Lock()
	s.Cfg.VirtualDesktop.SerialBrowserEnabled = false
	s.Cfg.VirtualDesktop.SerialHostEnabled = false
	s.CfgMu.Unlock()
	for {
		if err := conn.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if event.Type == "desktop_policy" {
			break
		}
	}
	var policy desktopLivePolicy
	if err := json.Unmarshal(event.Payload, &policy); err != nil || policy.SerialBrowserEnabled || policy.SerialHostEnabled || !policy.Enabled {
		t.Fatalf("grant revocation not projected: %s (%v)", event.Payload, err)
	}
	after, _, err := s.getDesktopService(context.Background())
	if err != nil || svc != after {
		t.Fatalf("serial toggle recreated desktop service: %v", err)
	}
}

func TestDesktopSerialRoutesRejectUnsafeHandshake(t *testing.T) {
	s, _, _ := testDesktopPermissionServer(t)
	s.Cfg.Auth.Enabled = false
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.VirtualDesktop.SerialHostEnabled = true
	mux := http.NewServeMux()
	registerDesktopSerialRoutes(mux, s)
	server := httptest.NewServer(mux)
	defer server.Close()
	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/desktop/serial/connect", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", server.URL)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("plain GET = %d, want 400", response.StatusCode)
	}
	headers := http.Header{"Origin": []string{"https://foreign.example"}}
	conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/desktop/serial/connect", headers)
	if conn != nil {
		conn.Close()
	}
	if response != nil {
		defer response.Body.Close()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin was not rejected: %v, %v", response, err)
	}
}
