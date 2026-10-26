package security

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// countedServer is an httptest server that counts TCP connections, so a probe
// that never completes an HTTP request still shows up.
type countedServer struct {
	*httptest.Server
	accepts atomic.Int64
	mu      sync.Mutex
	headers http.Header
}

type countingListener struct {
	net.Listener
	n *atomic.Int64
}

func (l *countingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err == nil {
		l.n.Add(1)
	}
	return c, err
}

func newCountedServer(t *testing.T, handler http.HandlerFunc) *countedServer {
	t.Helper()
	s := &countedServer{}
	s.Server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.headers = r.Header.Clone()
		s.mu.Unlock()
		handler(w, r)
	}))
	s.Server.Listener = &countingListener{Listener: s.Server.Listener, n: &s.accepts}
	s.Server.Start()
	t.Cleanup(s.Server.Close)
	return s
}

func (s *countedServer) lastHeaders() http.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.headers
}

func (s *countedServer) hostPort() string { return strings.TrimPrefix(s.URL, "http://") }

func captureProxyTestOrigins(t *testing.T) (allowed, internal *countedServer) {
	t.Helper()
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "")
	allowed = newCountedServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "http://"+r.Header.Get("X-Target")+"/secret", http.StatusFound)
		case "/close":
			w.Header().Set("Connection", "close") // the proxy cannot pool this upstream connection
			_, _ = io.WriteString(w, "closing")
		case "/stream":
			for i := 0; i < 10; i++ {
				_, _ = io.WriteString(w, "chunk ")
				w.(http.Flusher).Flush()
				time.Sleep(100 * time.Millisecond)
			}
		case "/big":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(make([]byte, 1<<20))
		default:
			_, _ = io.WriteString(w, "allowed "+r.URL.Path)
		}
	})
	internal = newCountedServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "internal")
	})
	return allowed, internal
}

func startTestCaptureProxy(t *testing.T, allowed *countedServer, limits CaptureProxyLimits) *CaptureProxy {
	t.Helper()
	var origin *url.URL
	if allowed != nil {
		origin = mustParseURL(t, allowed.URL)
	}
	p, err := StartCaptureProxy(origin, limits)
	if err != nil {
		t.Fatalf("StartCaptureProxy: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return parsed
}

// proxyClient talks plain HTTP through p and hands redirects back to the caller.
func proxyClient(t *testing.T, p *CaptureProxy) *http.Client {
	t.Helper()
	proxyURL := mustParseURL(t, "http://"+p.Addr())
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{
		Transport:     transport,
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// rawConnect sends CONNECT target to the proxy and returns the connection, the
// reader positioned after the reply, and the status code.
func rawConnect(t *testing.T, p *CaptureProxy, target string) (net.Conn, *bufio.Reader, int) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", p.Addr(), 5*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target)
	br := bufio.NewReader(conn)
	res, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("read CONNECT reply for %s: %v", target, err)
	}
	return conn, br, res.StatusCode
}

func TestCaptureProxyPlainHTTPReachesOnlyTheAllowedOrigin(t *testing.T) {
	allowed, internal := captureProxyTestOrigins(t)
	p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{})
	client := proxyClient(t, p)

	res, err := client.Get(allowed.URL + "/page")
	if err != nil {
		t.Fatalf("allowed origin: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || string(body) != "allowed /page" {
		t.Fatalf("allowed origin = %d %q", res.StatusCode, body)
	}

	res, err = client.Get(internal.URL + "/secret")
	if err != nil {
		t.Fatalf("internal origin: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("internal origin status = %d, want 403", res.StatusCode)
	}
	if got := internal.accepts.Load(); got != 0 {
		t.Fatalf("the internal server accepted %d connections", got)
	}
}

func TestCaptureProxyConnectTunnelsToTheAllowedOriginOnly(t *testing.T) {
	allowed, internal := captureProxyTestOrigins(t)
	p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{})

	conn, br, status := rawConnect(t, p, allowed.hostPort())
	if status != http.StatusOK {
		t.Fatalf("CONNECT allowed status = %d, want 200", status)
	}
	fmt.Fprintf(conn, "GET /tunneled HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", allowed.hostPort())
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("read through the tunnel: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || string(body) != "allowed /tunneled" {
		t.Fatalf("tunneled response = %d %q", res.StatusCode, body)
	}

	_, _, status = rawConnect(t, p, internal.hostPort())
	if status != http.StatusForbidden {
		t.Fatalf("CONNECT internal status = %d, want 403", status)
	}
	if got := internal.accepts.Load(); got != 0 {
		t.Fatalf("the internal server accepted %d connections", got)
	}
}

func TestCaptureProxyDoesNotFollowRedirects(t *testing.T) {
	allowed, internal := captureProxyTestOrigins(t)
	p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{})
	client := proxyClient(t, p)

	req, _ := http.NewRequest(http.MethodGet, allowed.URL+"/redirect", nil)
	req.Header.Set("X-Target", internal.hostPort())
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("redirecting origin: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusFound || !strings.Contains(res.Header.Get("Location"), internal.hostPort()) {
		t.Fatalf("redirect was not handed to the client: %d %q", res.StatusCode, res.Header.Get("Location"))
	}
	if got := internal.accepts.Load(); got != 0 {
		t.Fatalf("the proxy followed the redirect: internal accepted %d connections", got)
	}
}

func TestCaptureProxyStripsHopByHopHeaders(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{})
	client := proxyClient(t, p)

	req, _ := http.NewRequest(http.MethodGet, allowed.URL+"/headers", nil)
	req.Header.Set("Proxy-Connection", "keep-alive")
	req.Header.Set("Proxy-Authorization", "Basic c2VjcmV0")
	req.Header.Set("Connection", "X-Drop-Me, Keep-Alive")
	req.Header.Set("X-Drop-Me", "1")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("X-Keep-Me", "1")
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	_ = res.Body.Close()
	got := allowed.lastHeaders()
	for _, name := range []string{"Proxy-Connection", "Proxy-Authorization", "X-Drop-Me", "Upgrade"} {
		if got.Get(name) != "" {
			t.Errorf("hop-by-hop header %s reached the origin", name)
		}
	}
	if got.Get("X-Keep-Me") != "1" {
		t.Error("an ordinary header was dropped")
	}
}

func TestCaptureProxyRefusesInternalAddressForms(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{})
	for _, target := range []string{
		"169.254.169.254:80", "10.0.0.7:80", "192.168.1.1:443", "172.16.0.1:80", "100.64.0.1:80", "0.0.0.0:80",
		"[::1]:80", "[fd00::1]:80", "[fe80::1]:80", "[::ffff:10.0.0.1]:80", "[::ffff:127.0.0.1]:80",
		"127.0.0.1:1", "localhost:" + strings.Split(allowed.hostPort(), ":")[1],
	} {
		if _, _, status := rawConnect(t, p, target); status != http.StatusForbidden {
			t.Errorf("CONNECT %s status = %d, want 403", target, status)
		}
	}
}

func TestCaptureProxyWithoutAllowedOriginBlocksLoopback(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startTestCaptureProxy(t, nil, CaptureProxyLimits{})
	if _, _, status := rawConnect(t, p, allowed.hostPort()); status != http.StatusForbidden {
		t.Fatalf("CONNECT status = %d, want 403 without an allowed origin", status)
	}
}

func TestCaptureProxyKeepsTheLoopbackEscapeHatchMeaning(t *testing.T) {
	allowed, internal := captureProxyTestOrigins(t)
	p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{})
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	if _, _, status := rawConnect(t, p, internal.hostPort()); status != http.StatusOK {
		t.Fatalf("with AURAGO_SSRF_ALLOW_LOOPBACK=1 CONNECT status = %d, want 200", status)
	}
	if _, _, status := rawConnect(t, p, "10.0.0.7:80"); status != http.StatusForbidden {
		t.Fatalf("the escape hatch must stay loopback-only: status = %d", status)
	}
}

func TestCaptureProxyRejectsUnsupportedRequests(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{})
	cases := map[string]string{
		"origin form":     "GET /index.html HTTP/1.1\r\nHost: " + allowed.hostPort() + "\r\n\r\n",
		"absolute https":  "GET https://" + allowed.hostPort() + "/ HTTP/1.1\r\nHost: " + allowed.hostPort() + "\r\n\r\n",
		"absolute ws":     "GET ws://" + allowed.hostPort() + "/ HTTP/1.1\r\nHost: " + allowed.hostPort() + "\r\n\r\n",
		"connect no port": "CONNECT example.com HTTP/1.1\r\nHost: example.com\r\n\r\n",
		"connect port 0":  "CONNECT example.com:0 HTTP/1.1\r\nHost: example.com:0\r\n\r\n",
		"connect text":    "CONNECT example.com:http HTTP/1.1\r\nHost: example.com:http\r\n\r\n",
	}
	for name, raw := range cases {
		conn, err := net.DialTimeout("tcp", p.Addr(), 5*time.Second)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		_, _ = io.WriteString(conn, raw)
		res, err := http.ReadResponse(bufio.NewReader(conn), nil)
		_ = conn.Close()
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, res.StatusCode)
		}
	}
}

func TestCaptureProxyRefusesConnectionsBeyondTheLifetimeLimit(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{MaxConnections: 2})
	for i := 1; i <= 2; i++ {
		conn, _, status := rawConnect(t, p, allowed.hostPort())
		if status != http.StatusOK {
			t.Fatalf("connection %d status = %d, want 200", i, status)
		}
		_ = conn.Close() // finished connections still count against the lifetime limit
	}
	if _, _, status := rawConnect(t, p, allowed.hostPort()); status != http.StatusTooManyRequests {
		t.Fatalf("third connection status = %d, want 429", status)
	}
	if got := allowed.accepts.Load(); got != 2 {
		t.Fatalf("the origin accepted %d connections, want 2", got)
	}
}

func TestCaptureProxyLimitsConnectionsOnThePlainPathToo(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{MaxConnections: 1})
	client := proxyClient(t, p)
	statuses := make([]int, 0, 2)
	for i := 0; i < 2; i++ {
		res, err := client.Get(allowed.URL + "/close") // Connection: close: one upstream connection each
		if err != nil {
			t.Fatalf("get %d: %v", i, err)
		}
		_ = res.Body.Close()
		statuses = append(statuses, res.StatusCode)
	}
	if statuses[0] != http.StatusOK || statuses[1] != http.StatusTooManyRequests {
		t.Fatalf("statuses = %v, want [200 429]", statuses)
	}
}

func TestCaptureProxyCutsAResponseAtTheByteLimit(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	const limit = 64 << 10

	t.Run("plain http", func(t *testing.T) {
		p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{MaxBytes: limit})
		res, err := proxyClient(t, p).Get(allowed.URL + "/big")
		if err != nil {
			return // the cut may also surface before the headers
		}
		defer res.Body.Close()
		n, err := io.Copy(io.Discard, res.Body)
		if err == nil || n >= 1<<20 {
			t.Fatalf("read %d bytes with err %v; the response was not cut", n, err)
		}
	})
	t.Run("tunnel", func(t *testing.T) {
		p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{MaxBytes: limit})
		conn, br, status := rawConnect(t, p, allowed.hostPort())
		if status != http.StatusOK {
			t.Fatalf("CONNECT status = %d", status)
		}
		fmt.Fprintf(conn, "GET /big HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", allowed.hostPort())
		n, _ := io.Copy(io.Discard, br)
		if n >= 1<<20 {
			t.Fatalf("read %d bytes; the tunnel was not cut", n)
		}
		// The budget is spent: nothing new is dialed.
		if _, _, status := rawConnect(t, p, allowed.hostPort()); status != http.StatusTooManyRequests {
			t.Fatalf("CONNECT after the byte limit: status = %d, want 429", status)
		}
	})
}

func TestCaptureProxyWaitsForAFreeSlot(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startTestCaptureProxy(t, allowed, CaptureProxyLimits{MaxConcurrent: 1})
	first, _, status := rawConnect(t, p, allowed.hostPort())
	if status != http.StatusOK {
		t.Fatalf("first CONNECT status = %d", status)
	}

	secondStatus := make(chan int, 1)
	go func() {
		conn, err := net.DialTimeout("tcp", p.Addr(), 5*time.Second)
		if err != nil {
			secondStatus <- -1
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", allowed.hostPort(), allowed.hostPort())
		res, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
		if err != nil {
			secondStatus <- -2
			return
		}
		secondStatus <- res.StatusCode
	}()
	select {
	case got := <-secondStatus:
		t.Fatalf("the second tunnel got status %d while the only slot was taken", got)
	case <-time.After(400 * time.Millisecond):
	}
	_ = first.Close()
	select {
	case got := <-secondStatus:
		if got != http.StatusOK {
			t.Fatalf("second tunnel status = %d, want 200 once the slot is free", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the second tunnel never got the freed slot")
	}
}

func TestCaptureProxyCloseCutsEverythingAndIsIdempotent(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p, err := StartCaptureProxy(mustParseURL(t, allowed.URL), CaptureProxyLimits{})
	if err != nil {
		t.Fatalf("StartCaptureProxy: %v", err)
	}
	addr := p.Addr()
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Fatalf("Addr() = %q, want a loopback address", addr)
	}
	conn, _, status := rawConnect(t, p, allowed.hostPort())
	if status != http.StatusOK {
		t.Fatalf("CONNECT status = %d", status)
	}

	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("a live tunnel survived Close")
	} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("a live tunnel survived Close (read timed out)")
	}
	if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		_ = c.Close()
		t.Fatal("the proxy still accepts connections after Close")
	}
}

func TestCaptureProxyRejectsNonHTTPAllowedOrigin(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.com", "http://"} {
		if p, err := StartCaptureProxy(mustParseURL(t, raw), CaptureProxyLimits{}); err == nil {
			_ = p.Close()
			t.Errorf("origin %q was accepted", raw)
		}
	}
}

func TestOriginHostPortFillsDefaultPortsAndLowercases(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:8080/about?x=1": "127.0.0.1:8080",
		"http://127.0.0.1:8081/":          "127.0.0.1:8081",
		"https://127.0.0.1:8080/":         "127.0.0.1:8080", // the port, not the scheme, selects the destination
		"http://127.0.0.1":                "127.0.0.1:80",
		"https://127.0.0.1":               "127.0.0.1:443",
		"http://localhost:8080/":          "localhost:8080",
		"https://Example.com:443/x":       "example.com:443",
		"https://Example.com":             "example.com:443",
		"HTTP://EXAMPLE.com":              "example.com:80",
		"http://[::1]:9000/":              "[::1]:9000",
		"http://[0:0:0:0:0:0:0:1]/":       "[::1]:80",
	}
	for raw, want := range cases {
		if got := originHostPort(mustParseURL(t, raw)); got != want {
			t.Errorf("originHostPort(%q) = %q, want %q", raw, got, want)
		}
	}
	if originHostPort(mustParseURL(t, "http://127.0.0.1:8080")) == originHostPort(mustParseURL(t, "http://localhost:8080")) {
		t.Error("localhost and 127.0.0.1 are different origins")
	}
	if originHostPort(mustParseURL(t, "http://example.com")) == originHostPort(mustParseURL(t, "https://example.com")) {
		t.Error("the default http and https ports must differ")
	}
}

func TestCaptureProxyBoundsConcurrentUpstreamRequests(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "")
	var active, peak atomic.Int64
	slow := newCountedServer(t, func(w http.ResponseWriter, _ *http.Request) {
		now := active.Add(1)
		defer active.Add(-1)
		for {
			old := peak.Load()
			if now <= old || peak.CompareAndSwap(old, now) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		_, _ = io.WriteString(w, "ok")
	})
	const slots, clients = 4, 40
	p := startTestCaptureProxy(t, slow, CaptureProxyLimits{MaxConcurrent: slots})
	client := proxyClient(t, p)

	var wg sync.WaitGroup
	var failed atomic.Int64
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := client.Get(slow.URL + "/slow")
			if err != nil {
				failed.Add(1)
				return
			}
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
			if res.StatusCode != http.StatusOK {
				failed.Add(1)
			}
		}()
	}
	wg.Wait()
	if failed.Load() != 0 {
		t.Fatalf("%d of %d requests failed", failed.Load(), clients)
	}
	if got := peak.Load(); got > slots {
		t.Fatalf("%d requests reached the origin at once, limit is %d", got, slots)
	}
}

func startIdleTestCaptureProxy(t *testing.T, allowed *countedServer, limits CaptureProxyLimits, idle time.Duration) *CaptureProxy {
	t.Helper()
	p, err := startCaptureProxy(mustParseURL(t, allowed.URL), limits, idle)
	if err != nil {
		t.Fatalf("startCaptureProxy: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func TestCaptureProxyClosesIdleTunnelsAndFreesTheirSlot(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startIdleTestCaptureProxy(t, allowed, CaptureProxyLimits{MaxConcurrent: 1}, 300*time.Millisecond)
	idle, _, status := rawConnect(t, p, allowed.hostPort())
	if status != http.StatusOK {
		t.Fatalf("first CONNECT status = %d", status)
	}
	_ = idle.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := idle.Read(make([]byte, 1)); err == nil {
		t.Fatal("an idle tunnel received data")
	} else if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("an idle tunnel was not closed")
	}
	// The only slot is free again.
	if _, _, status := rawConnect(t, p, allowed.hostPort()); status != http.StatusOK {
		t.Fatalf("second CONNECT status = %d, want 200 after the idle tunnel closed", status)
	}
}

func TestCaptureProxyKeepsATunnelAliveWhileOneDirectionIsBusy(t *testing.T) {
	allowed, _ := captureProxyTestOrigins(t)
	p := startIdleTestCaptureProxy(t, allowed, CaptureProxyLimits{}, 300*time.Millisecond)
	conn, br, status := rawConnect(t, p, allowed.hostPort())
	if status != http.StatusOK {
		t.Fatalf("CONNECT status = %d", status)
	}
	// The origin streams for about a second; the client says nothing meanwhile.
	fmt.Fprintf(conn, "GET /stream HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", allowed.hostPort())
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil || strings.Count(string(body), "chunk") != 10 {
		t.Fatalf("streamed body = %q, err %v; the busy tunnel was cut", body, err)
	}
}
