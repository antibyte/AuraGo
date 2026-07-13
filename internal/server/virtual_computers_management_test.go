package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/virtualcomputers"

	"github.com/gorilla/websocket"
)

func TestVirtualComputersManagementProxyDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, virtualcomputers.ManagementBasePath+"/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("disabled status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVirtualComputersManagementProxyUnavailableIsSafe(t *testing.T) {
	restore := setVirtualComputersManagementTestHooks(t, "http://127.0.0.1:18081")
	defer restore()
	virtualComputersManagementHealthProbe = func(string) bool { return false }

	cfg := virtualComputersTestConfig("http://127.0.0.1:18080")
	cfg.VirtualComputers.ControlPlane.Mode = virtualcomputers.ControlPlaneLocalHost
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, &Server{Cfg: cfg})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, virtualcomputers.ManagementBasePath+"/", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "run setup install or repair") || strings.Contains(rec.Body.String(), "127.0.0.1") {
		t.Fatalf("unsafe unavailable response: %s", rec.Body.String())
	}
}

func TestVirtualComputersManagementProxyRequiresAuthenticatedSession(t *testing.T) {
	cfg := virtualComputersTestConfig("http://127.0.0.1:18080")
	cfg.VirtualComputers.ControlPlane.Mode = virtualcomputers.ControlPlaneLocalHost
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, &Server{Cfg: cfg})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, virtualcomputers.ManagementBasePath+"/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVirtualComputersManagementProxyRejectsReadTokenMutation(t *testing.T) {
	mutations := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			mutations++
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	restore := setVirtualComputersManagementTestHooks(t, upstream.URL)
	defer restore()

	s, readToken, _ := testDesktopPermissionServer(t)
	s.Cfg.VirtualComputers = virtualComputersTestConfig(upstream.URL).VirtualComputers
	s.Cfg.VirtualComputers.ControlPlane.Mode = virtualcomputers.ControlPlaneLocalHost
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, virtualcomputers.ManagementBasePath+"/boring/v1/machines", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+readToken)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("read-token mutation status = %d body=%s", rec.Code, rec.Body.String())
	}
	if mutations != 0 {
		t.Fatalf("upstream received %d mutations", mutations)
	}
}

func TestVirtualComputersManagementProxyRejectsReadTokenWebSocket(t *testing.T) {
	webSocketRequests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			webSocketRequests++
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	restore := setVirtualComputersManagementTestHooks(t, upstream.URL)
	defer restore()

	s, readToken, _ := testDesktopPermissionServer(t)
	s.Cfg.VirtualComputers = virtualComputersTestConfig(upstream.URL).VirtualComputers
	s.Cfg.VirtualComputers.ControlPlane.Mode = virtualcomputers.ControlPlaneLocalHost
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, virtualcomputers.ManagementBasePath+"/socket", nil)
	req.Header.Set("Authorization", "Bearer "+readToken)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("read-token websocket status = %d body=%s", rec.Code, rec.Body.String())
	}
	if webSocketRequests != 0 {
		t.Fatalf("upstream received %d websocket requests", webSocketRequests)
	}
}

func TestVirtualComputersManagementProxyHonorsReadOnly(t *testing.T) {
	mutations := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			mutations++
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	restore := setVirtualComputersManagementTestHooks(t, upstream.URL)
	defer restore()

	cfg := virtualComputersTestConfig(upstream.URL)
	cfg.VirtualComputers.ControlPlane.Mode = virtualcomputers.ControlPlaneLocalHost
	cfg.VirtualComputers.ReadOnly = true
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, &Server{Cfg: cfg})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, virtualcomputers.ManagementBasePath+"/boring/v1/machines", strings.NewReader(`{}`)))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("read-only mutation status = %d body=%s", rec.Code, rec.Body.String())
	}
	if mutations != 0 {
		t.Fatalf("upstream received %d mutations", mutations)
	}
}

func TestVirtualComputersManagementProxyPreservesBasePath(t *testing.T) {
	var upstreamPath, forwardedHost, forwardedProto string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.RequestURI()
		forwardedHost = r.Header.Get("X-Forwarded-Host")
		forwardedProto = r.Header.Get("X-Forwarded-Proto")
		_, _ = w.Write([]byte("management ok"))
	}))
	defer upstream.Close()
	restore := setVirtualComputersManagementTestHooks(t, upstream.URL)
	defer restore()

	cfg := virtualComputersTestConfig("http://127.0.0.1:18080")
	cfg.VirtualComputers.ControlPlane.Mode = virtualcomputers.ControlPlaneLocalHost
	s := &Server{Cfg: cfg}
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://aurago.test/boring-computers/docs?tab=api", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "management ok" {
		t.Fatalf("proxy status = %d body=%q", rec.Code, rec.Body.String())
	}
	if upstreamPath != "/boring-computers/docs?tab=api" {
		t.Fatalf("upstream request URI = %q", upstreamPath)
	}
	if forwardedHost != "aurago.test" || forwardedProto != "https" {
		t.Fatalf("forwarded host=%q proto=%q", forwardedHost, forwardedProto)
	}
}

func TestVirtualComputersManagementProxyRelaysWebSocket(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == virtualcomputers.ManagementBasePath+"/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path != "/boring-computers/socket" {
			t.Errorf("upstream websocket path = %q", r.URL.Path)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade upstream: %v", err)
			return
		}
		defer conn.Close()
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read upstream: %v", err)
			return
		}
		_ = conn.WriteMessage(messageType, append([]byte("echo:"), message...))
	}))
	defer upstream.Close()
	restore := setVirtualComputersManagementTestHooks(t, upstream.URL)
	defer restore()

	cfg := virtualComputersTestConfig("http://127.0.0.1:18080")
	cfg.VirtualComputers.ControlPlane.Mode = virtualcomputers.ControlPlaneLocalHost
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, &Server{Cfg: cfg})
	proxy := httptest.NewServer(mux)
	defer proxy.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(proxy.URL, "http")+"/boring-computers/socket", nil)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatalf("write proxy: %v", err)
	}
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read proxy: %v", err)
	}
	if string(message) != "echo:hello" {
		t.Fatalf("websocket response = %q", message)
	}
}

func TestVirtualComputersSetupStatusIncludesManagementHealth(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	restore := setVirtualComputersManagementTestHooks(t, upstream.URL)
	defer restore()

	cfg := virtualComputersTestConfig(upstream.URL)
	cfg.VirtualComputers.ControlPlane.Mode = virtualcomputers.ControlPlaneLocalHost
	rec := httptest.NewRecorder()
	handleVirtualComputersSetupStatus(&Server{Cfg: cfg}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/virtual-computers/setup/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["control_plane"].(map[string]interface{}); !ok {
		t.Fatalf("existing control_plane config changed: %#v", body["control_plane"])
	}
	for _, key := range []string{"control_plane_status", "management"} {
		component, ok := body[key].(map[string]interface{})
		if !ok || component["configured"] != true || component["healthy"] != true {
			t.Fatalf("%s = %#v", key, body[key])
		}
	}
	if strings.Contains(rec.Body.String(), "boring-token") {
		t.Fatalf("status leaked token: %s", rec.Body.String())
	}
}

func TestVirtualComputersManagementLocalAccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != virtualcomputers.ManagementBasePath+"/" {
			t.Fatalf("health path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	restore := setVirtualComputersManagementTestHooks(t, upstream.URL)
	defer restore()
	cfg := virtualcomputers.ToolConfig{ControlPlane: virtualcomputers.ControlPlaneConfig{Mode: virtualcomputers.ControlPlaneLocalHost}}

	if err := virtualComputersEnsureManagementAccess(&Server{}, cfg); err != nil {
		t.Fatalf("ensure local management access: %v", err)
	}
	if !virtualComputersManagementHealthy(&Server{}, cfg) {
		t.Fatal("local management service should be healthy")
	}
}

func TestVirtualComputersManagementHealthyDoesNotStartSSHTunnel(t *testing.T) {
	restore := setVirtualComputersManagementTestHooks(t, "http://127.0.0.1:18081")
	defer restore()

	executorCalls := 0
	virtualComputersManagementHealthProbe = func(string) bool { return false }
	virtualComputersManagementSSHExecutor = func(_ *Server, _ virtualcomputers.ToolConfig) (virtualComputersSSHExecutor, error) {
		executorCalls++
		return virtualComputersSSHExecutor{Host: "remote.example", Port: 22}, nil
	}
	cfg := virtualcomputers.ToolConfig{ControlPlane: virtualcomputers.ControlPlaneConfig{Mode: virtualcomputers.ControlPlaneSSHHost, Host: "remote.example"}}

	if virtualComputersManagementHealthy(&Server{}, cfg) {
		t.Fatal("unreachable management service reported healthy")
	}
	if executorCalls != 0 {
		t.Fatalf("passive health check requested SSH executor %d times", executorCalls)
	}
}

func TestVirtualComputersManagementRemoteTunnelReuseAndReplacement(t *testing.T) {
	restore := setVirtualComputersManagementTestHooks(t, "http://127.0.0.1:18081")
	defer restore()

	started := false
	starts := 0
	closes := 0
	virtualComputersManagementHealthProbe = func(string) bool { return started }
	virtualComputersManagementSSHExecutor = func(_ *Server, cfg virtualcomputers.ToolConfig) (virtualComputersSSHExecutor, error) {
		return virtualComputersSSHExecutor{Host: cfg.ControlPlane.Host, Port: 22}, nil
	}
	virtualComputersManagementTunnelStarter = func(_ virtualComputersSSHExecutor, localAddr, remoteAddr string, _ *Server) (func(), error) {
		starts++
		started = true
		if localAddr != "127.0.0.1:18081" || remoteAddr != virtualcomputers.ManagementListenAddr {
			t.Fatalf("tunnel = %q -> %q", localAddr, remoteAddr)
		}
		return func() { closes++; started = false }, nil
	}

	cfg := virtualcomputers.ToolConfig{ControlPlane: virtualcomputers.ControlPlaneConfig{Mode: virtualcomputers.ControlPlaneSSHHost, Host: "first.example"}}
	if err := virtualComputersEnsureManagementAccess(&Server{}, cfg); err != nil {
		t.Fatalf("first access: %v", err)
	}
	if err := virtualComputersEnsureManagementAccess(&Server{}, cfg); err != nil {
		t.Fatalf("reused access: %v", err)
	}
	if starts != 1 || closes != 0 {
		t.Fatalf("reuse starts=%d closes=%d", starts, closes)
	}

	cfg.ControlPlane.Host = "second.example"
	if err := virtualComputersEnsureManagementAccess(&Server{}, cfg); err != nil {
		t.Fatalf("replacement access: %v", err)
	}
	if starts != 2 || closes != 1 {
		t.Fatalf("replacement starts=%d closes=%d", starts, closes)
	}
}

func TestVirtualComputersManagementFailedHealthClosesNewTunnel(t *testing.T) {
	restore := setVirtualComputersManagementTestHooks(t, "http://127.0.0.1:18081")
	defer restore()

	closes := 0
	virtualComputersManagementHealthProbe = func(string) bool { return false }
	virtualComputersManagementSSHExecutor = func(_ *Server, _ virtualcomputers.ToolConfig) (virtualComputersSSHExecutor, error) {
		return virtualComputersSSHExecutor{Host: "remote.example", Port: 22}, nil
	}
	virtualComputersManagementTunnelStarter = func(_ virtualComputersSSHExecutor, _, _ string, _ *Server) (func(), error) {
		return func() { closes++ }, nil
	}
	cfg := virtualcomputers.ToolConfig{ControlPlane: virtualcomputers.ControlPlaneConfig{Mode: virtualcomputers.ControlPlaneSSHHost, Host: "remote.example"}}

	err := virtualComputersEnsureManagementAccess(&Server{}, cfg)
	if err == nil || err.Error() != "Boring Computers management service is unavailable; run setup install or repair" {
		t.Fatalf("error = %v", err)
	}
	if closes != 1 {
		t.Fatalf("new tunnel close count = %d", closes)
	}
	virtualComputersManagementTunnel.Lock()
	defer virtualComputersManagementTunnel.Unlock()
	if virtualComputersManagementTunnel.close != nil || virtualComputersManagementTunnel.key != "" {
		t.Fatalf("failed tunnel retained: key=%q close=%v", virtualComputersManagementTunnel.key, virtualComputersManagementTunnel.close != nil)
	}
}

func setVirtualComputersManagementTestHooks(t *testing.T, managementURL string) func() {
	t.Helper()
	oldURL := virtualComputersManagementURL
	oldProbe := virtualComputersManagementHealthProbe
	oldExecutor := virtualComputersManagementSSHExecutor
	oldStarter := virtualComputersManagementTunnelStarter
	virtualComputersManagementURL = managementURL
	virtualComputersManagementHealthProbe = func(rawURL string) bool {
		resp, err := http.Get(virtualcomputers.ManagementHealthURL(rawURL))
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode >= 200 && resp.StatusCode < 300
	}
	virtualComputersManagementSSHExecutor = func(_ *Server, _ virtualcomputers.ToolConfig) (virtualComputersSSHExecutor, error) {
		return virtualComputersSSHExecutor{}, fmt.Errorf("unexpected SSH executor request")
	}
	virtualComputersManagementTunnelStarter = func(_ virtualComputersSSHExecutor, _, _ string, _ *Server) (func(), error) {
		return nil, fmt.Errorf("unexpected tunnel start")
	}
	resetVirtualComputersManagementTunnel()
	return func() {
		resetVirtualComputersManagementTunnel()
		virtualComputersManagementURL = oldURL
		virtualComputersManagementHealthProbe = oldProbe
		virtualComputersManagementSSHExecutor = oldExecutor
		virtualComputersManagementTunnelStarter = oldStarter
	}
}
