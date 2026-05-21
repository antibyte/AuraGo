package tsnetnode

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"aurago/internal/config"

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

	if err := sanitizeStoreAppProxyResponse(resp); err != nil {
		t.Fatalf("sanitizeStoreAppProxyResponse returned error: %v", err)
	}
	if got := resp.Header.Get("X-Frame-Options"); got != "" {
		t.Fatalf("X-Frame-Options = %q, want stripped for embedded store app proxy responses", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store for embedded store app documents", got)
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
