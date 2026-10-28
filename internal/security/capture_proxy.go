package security

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Defaults for CaptureProxyLimits fields that are zero or negative.
const (
	defaultCaptureProxyConcurrent  = 16
	defaultCaptureProxyConnections = 600
	defaultCaptureProxyBytes       = 64 << 20

	captureProxyDialTimeout = 10 * time.Second
	// captureProxyIdleTimeout closes a tunnel that moved no bytes in either
	// direction. Chrome keeps finished HTTPS connections open in its socket
	// pool; without this they would hold concurrency slots for the whole
	// capture and starve later requests.
	captureProxyIdleTimeout     = 10 * time.Second
	captureProxyResponseTimeout = 20 * time.Second
	captureProxyBufferBytes     = 32 << 10
)

var (
	errCaptureDestinationBlocked = errors.New("destination is not allowed")
	errCaptureConnLimit          = errors.New("upstream connection limit reached")
	errCaptureByteLimit          = errors.New("relayed byte limit reached")
	errCaptureProxyClosed        = errors.New("capture proxy is closed")
)

// hopByHopHeaders are connection-scoped (RFC 9110 section 7.6.1) plus the
// legacy Proxy-Connection; a forwarding proxy must not pass them on.
var hopByHopHeaders = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
	"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

// CaptureProxyLimits bound what one page capture can make the AuraGo host do.
// A zero or negative field selects its default.
type CaptureProxyLimits struct {
	// MaxConcurrent is the number of requests and tunnels that may hold an
	// upstream connection at the same time; further ones wait for a slot.
	MaxConcurrent int
	// MaxConnections is how many outbound attempts (DNS lookup plus dial) the
	// proxy makes over its whole lifetime; further ones are refused with 429.
	MaxConnections int
	// MaxBytes is the total number of bytes relayed in both directions; the
	// connections are cut when it is exceeded.
	MaxBytes int64
}

// CaptureProxy is an in-process forward proxy for one headless-browser capture.
// The browser sends all of its traffic here (including loopback, service
// workers, WebSockets and prerenders), so the destination policy holds for every
// network path of the browser and not just the requests of one page target.
//
// Destination policy: the allowed origin's host:port is dialed as given;
// every other destination is resolved and validated with the SSRF rules
// (private, loopback, link-local, CGNAT and IPv6 forms are refused) and the
// validated IP itself is dialed, so there is no second lookup to rebind.
type CaptureProxy struct {
	allowed string // canonical host:port of the trusted origin, "" for none
	limits  CaptureProxyLimits
	idle    time.Duration // tunnel idle timeout

	listener  net.Listener
	server    *http.Server
	transport *http.Transport
	dialer    net.Dialer

	slots  chan struct{}
	ctx    context.Context
	cancel context.CancelFunc

	dialed  atomic.Int64
	relayed atomic.Int64

	mu     sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool

	closeOnce sync.Once
}

// StartCaptureProxy listens on 127.0.0.1 and starts serving. allowed, when not
// nil, is the one origin (typically the owner's loopback preview) the proxy may
// dial without the SSRF check; nil sends every destination through the check.
func StartCaptureProxy(allowed *url.URL, limits CaptureProxyLimits) (*CaptureProxy, error) {
	return startCaptureProxy(allowed, limits, captureProxyIdleTimeout)
}

func startCaptureProxy(allowed *url.URL, limits CaptureProxyLimits, idle time.Duration) (*CaptureProxy, error) {
	allowedHostPort := ""
	if allowed != nil {
		if (allowed.Scheme != "http" && allowed.Scheme != "https") || allowed.Hostname() == "" {
			return nil, fmt.Errorf("allowed origin must be an http(s) URL with a host")
		}
		allowedHostPort = originHostPort(allowed)
	}
	if limits.MaxConcurrent <= 0 {
		limits.MaxConcurrent = defaultCaptureProxyConcurrent
	}
	if limits.MaxConnections <= 0 {
		limits.MaxConnections = defaultCaptureProxyConnections
	}
	if limits.MaxBytes <= 0 {
		limits.MaxBytes = defaultCaptureProxyBytes
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("capture proxy listen: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &CaptureProxy{
		allowed:  allowedHostPort,
		limits:   limits,
		idle:     idle,
		listener: listener,
		dialer:   net.Dialer{Timeout: captureProxyDialTimeout},
		slots:    make(chan struct{}, limits.MaxConcurrent),
		ctx:      ctx,
		cancel:   cancel,
		conns:    make(map[net.Conn]struct{}),
	}
	p.transport = &http.Transport{
		Proxy:                  nil,
		DialContext:            p.dialUpstream,
		DisableCompression:     true,
		MaxIdleConns:           limits.MaxConcurrent,
		MaxIdleConnsPerHost:    2,
		IdleConnTimeout:        idle,
		ResponseHeaderTimeout:  captureProxyResponseTimeout,
		MaxResponseHeaderBytes: 256 << 10,
	}
	p.server = &http.Server{
		Handler:           p,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          log.New(io.Discard, "", 0),
		// Track every client connection so Close and the byte limit can cut
		// them, including the ones a CONNECT handler hijacks.
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			if !p.track(c) {
				_ = c.Close()
			}
			return ctx
		},
		ConnState: func(c net.Conn, state http.ConnState) {
			if state == http.StateClosed {
				p.untrack(c)
			}
		},
	}
	go func() { _ = p.server.Serve(listener) }()
	return p, nil
}

// Addr returns the proxy's listen address, for example 127.0.0.1:53211.
func (p *CaptureProxy) Addr() string { return p.listener.Addr().String() }

// Close stops the listener and cuts every live client and upstream connection.
// It is safe to call more than once.
func (p *CaptureProxy) Close() error {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
		p.cancel()
		_ = p.server.Close()
		p.closeConns()
		p.transport.CloseIdleConnections()
	})
	return nil
}

// ServeHTTP implements http.Handler for CONNECT and absolute-form requests.
func (p *CaptureProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.serveConnect(w, r)
		return
	}
	p.servePlain(w, r)
}

// track registers c for Close; it reports false when the proxy is already closed.
func (p *CaptureProxy) track(c net.Conn) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false
	}
	p.conns[c] = struct{}{}
	return true
}

func (p *CaptureProxy) untrack(c net.Conn) {
	p.mu.Lock()
	delete(p.conns, c)
	p.mu.Unlock()
}

func (p *CaptureProxy) closeConns() {
	p.mu.Lock()
	conns := make([]net.Conn, 0, len(p.conns))
	for c := range p.conns {
		conns = append(conns, c)
	}
	p.conns = make(map[net.Conn]struct{})
	p.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

// relay accounts n relayed bytes. Beyond MaxBytes it cuts every connection and
// reports errCaptureByteLimit.
func (p *CaptureProxy) relay(n int) error {
	if p.relayed.Add(int64(n)) > p.limits.MaxBytes {
		p.closeConns()
		return errCaptureByteLimit
	}
	return nil
}

func (p *CaptureProxy) acquire(ctx context.Context) bool {
	select {
	case p.slots <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	case <-p.ctx.Done():
		return false
	}
}

func (p *CaptureProxy) release() { <-p.slots }

// requestContext ends when the client goes away or the proxy closes.
func (p *CaptureProxy) requestContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(p.ctx, cancel)
	return ctx, func() { stop(); cancel() }
}

// destination applies the destination policy to a host:port and returns the
// address to dial.
func (p *CaptureProxy) destination(ctx context.Context, hostPort string) (string, error) {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errCaptureDestinationBlocked, err)
	}
	if p.allowed != "" && captureHostPort(host, port) == p.allowed {
		return p.allowed, nil
	}
	target, _, err := validatedSSRFDialTarget(ctx, hostPort)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			return "", err // not a policy refusal: the name does not resolve
		}
		return "", fmt.Errorf("%w: %v", errCaptureDestinationBlocked, err)
	}
	return target, nil
}

// dialUpstream is the only way the proxy opens an outbound connection: lifetime
// limits first, then the destination policy, then a dial of the validated
// address.
func (p *CaptureProxy) dialUpstream(ctx context.Context, _, hostPort string) (net.Conn, error) {
	if p.ctx.Err() != nil {
		return nil, errCaptureProxyClosed
	}
	if p.relayed.Load() >= p.limits.MaxBytes {
		return nil, errCaptureByteLimit
	}
	if p.dialed.Add(1) > int64(p.limits.MaxConnections) {
		return nil, errCaptureConnLimit
	}
	dialCtx, cancel := p.requestContext(ctx)
	defer cancel()
	target, err := p.destination(dialCtx, hostPort)
	if err != nil {
		return nil, err
	}
	raw, err := p.dialer.DialContext(dialCtx, "tcp", target)
	if err != nil {
		return nil, err
	}
	conn := &captureUpstreamConn{Conn: raw, proxy: p}
	if !p.track(conn) {
		_ = raw.Close()
		return nil, errCaptureProxyClosed
	}
	return conn, nil
}

// captureUpstreamConn untracks itself on Close and supports half-close.
type captureUpstreamConn struct {
	net.Conn
	proxy *CaptureProxy
	once  sync.Once
}

func (c *captureUpstreamConn) Close() error {
	var err error
	c.once.Do(func() {
		c.proxy.untrack(c)
		err = c.Conn.Close()
	})
	return err
}

func (c *captureUpstreamConn) CloseWrite() error {
	if cw, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return c.Close()
}

func (p *CaptureProxy) refuse(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	text := "capture proxy: upstream unreachable"
	switch {
	case errors.Is(err, errCaptureDestinationBlocked):
		status, text = http.StatusForbidden, "capture proxy: destination is not allowed"
	case errors.Is(err, errCaptureConnLimit), errors.Is(err, errCaptureByteLimit):
		status, text = http.StatusTooManyRequests, "capture proxy: limit reached"
	case errors.Is(err, errCaptureProxyClosed):
		status, text = http.StatusServiceUnavailable, "capture proxy: closed"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Connection", "close")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, text+"\n")
}

func (p *CaptureProxy) serveConnect(w http.ResponseWriter, r *http.Request) {
	host, port, err := net.SplitHostPort(r.Host)
	if n, convErr := strconv.Atoi(port); err != nil || host == "" || convErr != nil || n < 1 || n > 65535 {
		http.Error(w, "capture proxy: malformed CONNECT target", http.StatusBadRequest)
		return
	}
	if !p.acquire(r.Context()) {
		http.Error(w, "capture proxy: closed", http.StatusServiceUnavailable)
		return
	}
	defer p.release()

	ctx, cancel := p.requestContext(r.Context())
	upstream, err := p.dialUpstream(ctx, "tcp", net.JoinHostPort(host, port))
	cancel()
	if err != nil {
		p.refuse(w, err)
		return
	}
	defer upstream.Close()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "capture proxy: cannot take over the connection", http.StatusInternalServerError)
		return
	}
	client, rw, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer func() {
		p.untrack(client)
		_ = client.Close()
	}()
	if _, err := rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := rw.Flush(); err != nil {
		return
	}
	p.tunnel(client, rw.Reader, upstream)
}

// tunnel copies both directions until both are done or the tunnel is idle.
// clientReader is the client's buffered reader, so bytes that arrived with the
// CONNECT request are not lost.
func (p *CaptureProxy) tunnel(client net.Conn, clientReader io.Reader, upstream net.Conn) {
	var lastActive atomic.Int64
	lastActive.Store(time.Now().UnixNano())
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		p.pipe(upstream, client, clientReader, &lastActive)
	}()
	go func() {
		defer wg.Done()
		p.pipe(client, upstream, upstream, &lastActive)
	}()
	wg.Wait()
}

// pipe copies src to dst. A clean EOF half-closes dst so the other direction
// can finish; any other end (error, idle timeout, byte limit) cuts both sides.
func (p *CaptureProxy) pipe(dst, src net.Conn, reader io.Reader, lastActive *atomic.Int64) {
	buf := make([]byte, captureProxyBufferBytes)
	for {
		_ = src.SetReadDeadline(time.Now().Add(p.idle))
		n, err := reader.Read(buf)
		if n > 0 {
			lastActive.Store(time.Now().UnixNano())
			if p.relay(n) != nil {
				break
			}
			_ = dst.SetWriteDeadline(time.Now().Add(p.idle))
			if _, werr := dst.Write(buf[:n]); werr != nil {
				break
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			if cw, ok := dst.(interface{ CloseWrite() error }); ok {
				_ = cw.CloseWrite()
				return
			}
			break
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() &&
			time.Since(time.Unix(0, lastActive.Load())) < p.idle {
			continue // the other direction is busy: the tunnel is not idle
		}
		break
	}
	_ = src.Close()
	_ = dst.Close()
}

func (p *CaptureProxy) servePlain(w http.ResponseWriter, r *http.Request) {
	target := r.URL
	if r.RequestURI == "" || !target.IsAbs() || target.Scheme != "http" || target.Hostname() == "" {
		http.Error(w, "capture proxy: only CONNECT and absolute-form http requests are supported", http.StatusBadRequest)
		return
	}
	if !p.acquire(r.Context()) {
		http.Error(w, "capture proxy: closed", http.StatusServiceUnavailable)
		return
	}
	defer p.release()

	ctx, cancel := p.requestContext(r.Context())
	defer cancel()
	out := r.Clone(ctx)
	out.RequestURI = ""
	out.Host = target.Host
	out.Close = false
	removeHopByHopHeaders(out.Header)
	if _, ok := out.Header["User-Agent"]; !ok {
		out.Header["User-Agent"] = []string{""} // keep net/http from inventing one
	}
	if r.Body != nil && r.Body != http.NoBody {
		out.Body = &captureCountingBody{ReadCloser: r.Body, proxy: p}
	}

	res, err := p.transport.RoundTrip(out)
	if err != nil {
		if ctx.Err() == nil {
			p.refuse(w, err)
		}
		return
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusSwitchingProtocols {
		p.refuse(w, errors.New("upgrade is not supported"))
		return
	}
	removeHopByHopHeaders(res.Header)
	header := w.Header()
	for name, values := range res.Header {
		header[name] = append([]string(nil), values...)
	}
	w.WriteHeader(res.StatusCode)

	buf := make([]byte, captureProxyBufferBytes)
	for {
		n, readErr := res.Body.Read(buf)
		if n > 0 {
			if p.relay(n) != nil {
				panic(http.ErrAbortHandler)
			}
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return
			}
		}
		if readErr == nil {
			continue
		}
		if !errors.Is(readErr, io.EOF) {
			// Cut the client connection so a truncated body is not mistaken
			// for a complete one.
			panic(http.ErrAbortHandler)
		}
		return
	}
}

// captureCountingBody charges a request body against the byte limit.
type captureCountingBody struct {
	io.ReadCloser
	proxy *CaptureProxy
}

func (b *captureCountingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		if limitErr := b.proxy.relay(n); limitErr != nil {
			return n, limitErr
		}
	}
	return n, err
}

// removeHopByHopHeaders drops connection-scoped headers, including any header
// the Connection header names.
func removeHopByHopHeaders(h http.Header) {
	for _, value := range h.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			if name = strings.TrimSpace(name); name != "" {
				h.Del(name)
			}
		}
	}
	for _, name := range hopByHopHeaders {
		h.Del(name)
	}
}

// originHostPort returns host:port of u with the scheme's default port filled in.
func originHostPort(u *url.URL) string {
	port := u.Port()
	if port == "" {
		port = "80"
		if strings.EqualFold(u.Scheme, "https") {
			port = "443"
		}
	}
	return captureHostPort(u.Hostname(), port)
}

// captureHostPort is the canonical host:port used for origin matching: lower
// case names and the normalized text form of IP literals.
func captureHostPort(host, port string) string {
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	}
	return net.JoinHostPort(strings.ToLower(host), port)
}
