package tsnetnode

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"

	"github.com/gorilla/websocket"
	"tailscale.com/tsnet"
)

func TestManifestProxyTarget(t *testing.T) {
	cfg := &config.Config{}
	cfg.Manifest.Port = 2099
	cfg.Manifest.HostPort = 2099

	cfg.Runtime.IsDocker = false
	if got := manifestProxyTarget(cfg); got != "http://127.0.0.1:2099" {
		t.Fatalf("manifestProxyTarget(host) = %q, want loopback target", got)
	}

	cfg.Runtime.IsDocker = true
	if got := manifestProxyTarget(cfg); got != "http://manifest:2099" {
		t.Fatalf("manifestProxyTarget(docker) = %q, want Docker service target", got)
	}

	cfg.Runtime.IsDocker = false
	cfg.Manifest.HostPort = 3109
	if got := manifestProxyTarget(cfg); got != "http://127.0.0.1:3109" {
		t.Fatalf("manifestProxyTarget(custom host port) = %q, want published host port", got)
	}
}

func TestManifestHostnameDefault(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tailscale.TsNet.Hostname = "aurago"
	m := NewManager(cfg, nil)

	if got := m.effectiveManifestHostname(); got != "aurago-manifest" {
		t.Fatalf("effectiveManifestHostname() = %q, want aurago-manifest", got)
	}

	cfg.Tailscale.TsNet.ManifestHostname = "custom-manifest"
	if got := m.effectiveManifestHostname(); got != "custom-manifest" {
		t.Fatalf("effectiveManifestHostname() = %q, want custom-manifest", got)
	}
}

func TestManifestTsNetPortUsesHTTPSDefault(t *testing.T) {
	tests := []struct {
		name string
		port int
		want int
	}{
		{name: "empty config", port: 0, want: 443},
		{name: "legacy default", port: 8444, want: 443},
		{name: "custom port", port: 2099, want: 2099},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := manifestTsNetPort(tt.port); got != tt.want {
				t.Fatalf("manifestTsNetPort(%d) = %d, want %d", tt.port, got, tt.want)
			}
		})
	}
}

func TestHomepageProxyBackendReachableReportsClosedPort(t *testing.T) {
	oldDial := tcpDialTimeout
	defer func() { tcpDialTimeout = oldDial }()

	tcpDialTimeout = func(network, address string, timeout time.Duration) (net.Conn, error) {
		if network != "tcp" || address != "127.0.0.1:8080" {
			t.Fatalf("unexpected dial target network=%q address=%q", network, address)
		}
		return nil, errors.New("connection refused")
	}

	if homepageProxyBackendReachable(8080, time.Second) {
		t.Fatal("closed homepage backend port should not be reported reachable")
	}
}

func TestReconfigureExposureRetriesHomepageAfterBackendBecomesReachable(t *testing.T) {
	oldDial := tcpDialTimeout
	oldListen := listenTLSWithTimeoutFn
	oldRetryDelay := homepageExposureRetryDelay
	defer func() {
		tcpDialTimeout = oldDial
		listenTLSWithTimeoutFn = oldListen
		homepageExposureRetryDelay = oldRetryDelay
	}()

	backendUp := make(chan struct{})
	tcpDialTimeout = func(network, address string, timeout time.Duration) (net.Conn, error) {
		if network != "tcp" || address != "127.0.0.1:8080" {
			t.Fatalf("unexpected dial target network=%q address=%q", network, address)
		}
		select {
		case <-backendUp:
			serverConn, clientConn := net.Pipe()
			_ = serverConn.Close()
			return clientConn, nil
		default:
			return nil, errors.New("connection refused")
		}
	}

	listener := newBlockingListener("tailnet-homepage")
	listenTLSWithTimeoutFn = func(srv *tsnet.Server, addr string, timeout time.Duration) (net.Listener, error) {
		if srv == nil {
			t.Fatal("expected tsnet server")
		}
		if addr != ":8443" {
			t.Fatalf("expected homepage listener on :8443, got %q", addr)
		}
		return listener, nil
	}
	homepageExposureRetryDelay = 5 * time.Millisecond

	cfg := &config.Config{}
	cfg.Tailscale.TsNet.ExposeHomepage = true
	cfg.Homepage.WebServerEnabled = true
	cfg.Homepage.WebServerPort = 8080
	m := NewManager(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.server = &tsnet.Server{}
	m.running = true

	if err := m.ReconfigureExposure(http.NewServeMux()); err != nil {
		t.Fatalf("ReconfigureExposure returned error: %v", err)
	}
	if m.GetStatus().HomepageServing {
		t.Fatal("homepage should not be serving while backend is still down")
	}

	close(backendUp)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if m.GetStatus().HomepageServing {
			_ = listener.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = listener.Close()
	t.Fatal("homepage exposure was not retried after backend became reachable")
}

func TestStoreAppProxyResponseRemovesFrameHeadersAndDisablesHTMLCache(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("X-Frame-Options", "SAMEORIGIN")
	resp.Header.Set("Content-Type", "text/html; charset=utf-8")
	resp.Header.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'self'; script-src 'self'")
	resp.Header.Set("Content-Security-Policy-Report-Only", "frame-ancestors 'none'; default-src 'self'")
	resp.Header.Set("ETag", `W/"old"`)
	resp.Header.Set("Last-Modified", "Thu, 21 May 2026 19:00:00 GMT")

	if err := sanitizeStoreAppProxyResponse(resp); err != nil {
		t.Fatalf("sanitizeStoreAppProxyResponse returned error: %v", err)
	}
	if got := resp.Header.Get("X-Frame-Options"); got != "" {
		t.Fatalf("X-Frame-Options = %q, want stripped for embedded store app proxy responses", got)
	}
	if got := resp.Header.Get("Content-Security-Policy"); strings.Contains(got, "frame-ancestors") || !strings.Contains(got, "default-src 'self'") {
		t.Fatalf("Content-Security-Policy = %q, want frame-ancestors stripped and other directives preserved", got)
	}
	if got := resp.Header.Get("Content-Security-Policy-Report-Only"); strings.Contains(got, "frame-ancestors") || !strings.Contains(got, "default-src 'self'") {
		t.Fatalf("Content-Security-Policy-Report-Only = %q, want frame-ancestors stripped and other directives preserved", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store for embedded store app documents", got)
	}
	if got := resp.Header.Get("ETag"); got != "" {
		t.Fatalf("ETag = %q, want stripped so stale frame headers cannot be reused", got)
	}
	if got := resp.Header.Get("Last-Modified"); got != "" {
		t.Fatalf("Last-Modified = %q, want stripped so stale frame headers cannot be reused", got)
	}
}

func TestStoreAppProxyRequestDropsConditionalCacheHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://example.test/dashboard", nil)
	req.Header.Set("If-None-Match", `W/"old"`)
	req.Header.Set("If-Modified-Since", "Thu, 21 May 2026 19:00:00 GMT")
	req.Header.Set("If-Range", `W/"old"`)

	sanitizeStoreAppProxyRequest(req)

	for _, header := range []string{"If-None-Match", "If-Modified-Since", "If-Range"} {
		if got := req.Header.Get(header); got != "" {
			t.Fatalf("%s = %q, want stripped", header, got)
		}
	}
	if got := req.Header.Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache for fresh embedded documents", got)
	}
}

func TestStoreAppProxyRoutesAPIPrefixToAPITarget(t *testing.T) {
	uiBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ui:" + r.URL.Path))
	}))
	defer uiBackend.Close()
	apiBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("api:" + r.URL.Path))
	}))
	defer apiBackend.Close()

	handler, err := newStoreAppProxyHandler(StoreAppProxySpec{
		ID:           "dograh",
		Port:         3010,
		TargetURL:    uiBackend.URL + "/",
		APITargetURL: apiBackend.URL + "/",
		Enabled:      true,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("newStoreAppProxyHandler() error = %v", err)
	}

	tests := []struct {
		path string
		want string
	}{
		{path: "/", want: "ui:/"},
		{path: "/_next/static/app.js", want: "ui:/_next/static/app.js"},
		{path: "/api/v1/ws/signaling/1/1", want: "api:/api/v1/ws/signaling/1/1"},
		{path: "/api/v1/node-types", want: "api:/api/v1/node-types"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://dograh.tailnet.test"+tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			if got := rec.Body.String(); got != tt.want {
				t.Fatalf("body = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStoreAppProxyForwardsWebSocketUpgrade(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/comms" {
			t.Fatalf("backend path = %q, want /comms", r.URL.Path)
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("backend websocket upgrade: %v", err)
			return
		}
		defer conn.Close()
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("backend read websocket message: %v", err)
			return
		}
		if err := conn.WriteMessage(messageType, append([]byte("echo:"), payload...)); err != nil {
			t.Errorf("backend write websocket message: %v", err)
		}
	}))
	defer backend.Close()

	handler, err := newStoreAppProxyHandler(StoreAppProxySpec{
		ID:        "node-red",
		Port:      1880,
		TargetURL: backend.URL + "/",
		Enabled:   true,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("newStoreAppProxyHandler() error = %v", err)
	}
	proxy := httptest.NewServer(handler)
	defer proxy.Close()

	wsURL := "ws" + strings.TrimPrefix(proxy.URL, "http") + "/comms"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{"https://aurago.taild1480.ts.net"}})
	if err != nil {
		t.Fatalf("dial proxied websocket: %v", err)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatalf("write proxied websocket: %v", err)
	}
	messageType, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read proxied websocket: %v", err)
	}
	if messageType != websocket.TextMessage || string(payload) != "echo:hello" {
		t.Fatalf("proxied websocket message = type %d payload %q, want text echo:hello", messageType, payload)
	}
}

func TestStoreAppProxyReturnsQuicklyWhenBackendAcceptsButDoesNotRespond(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	handler, err := newStoreAppProxyHandler(StoreAppProxySpec{
		ID:        "slow-app",
		Port:      18080,
		TargetURL: "http://" + listener.Addr().String() + "/",
		Enabled:   true,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("newStoreAppProxyHandler() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "https://slow-app.tailnet.test/", nil)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case conn := <-accepted:
		defer conn.Close()
	case <-time.After(time.Second):
		t.Fatal("proxy did not connect to slow backend")
	}

	select {
	case <-done:
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502", rec.Code)
		}
	case <-time.After(storeAppProxyBackendTimeout + time.Second):
		t.Fatal("proxy request stayed blocked after backend response timeout")
	}
}

type blockingListener struct {
	closed chan struct{}
	addr   net.Addr
}

func newBlockingListener(addr string) *blockingListener {
	return &blockingListener{
		closed: make(chan struct{}),
		addr:   fakeAddr(addr),
	}
}

func (l *blockingListener) Accept() (net.Conn, error) {
	<-l.closed
	return nil, net.ErrClosed
}

func (l *blockingListener) Close() error {
	select {
	case <-l.closed:
	default:
		close(l.closed)
	}
	return nil
}

func (l *blockingListener) Addr() net.Addr {
	return l.addr
}

type fakeAddr string

func (a fakeAddr) Network() string { return "tcp" }
func (a fakeAddr) String() string  { return string(a) }
