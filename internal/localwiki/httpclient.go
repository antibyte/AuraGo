package localwiki

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"
)

var errInsecureRedirect = errors.New("localwiki: redirect to a non-HTTPS URL refused")

// httpsOnlyRedirect returns the redirect policy of every localwiki client: at
// most ten redirects, only to HTTPS URLs without credentials (no mirror can
// downgrade a download to HTTP), and never to a local host name or address
// (isLocalHost) unless it is the host of a trusted URL.
func httpsOnlyRedirect(trusted []*url.URL) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("localwiki: too many redirects")
		}
		if req.URL.Scheme != "https" || req.URL.User != nil {
			return errInsecureRedirect
		}
		if isLocalHost(req.URL.Hostname()) && !isTrustedHost(req.URL, trusted) {
			return fmt.Errorf("redirect to %s: %w", req.URL.Host, errLocalAddress)
		}
		return nil
	}
}

// isTrustedHost reports whether u names the host and port of a trusted URL.
func isTrustedHost(u *url.URL, trusted []*url.URL) bool {
	for _, t := range trusted {
		if t != nil && strings.EqualFold(u.Host, t.Host) {
			return true
		}
	}
	return false
}

// newHTTPClient returns a copy of base with the redirect policy of
// httpsOnlyRedirect, or a default client without an overall timeout:
// downloads take hours, so every request carries its own deadline or stall
// watchdog instead.
//
// The default client also refuses to connect to loopback, private, link-local
// and other local addresses (see dialGuard), so a public mirror name that
// resolves into the LAN (DNS rebinding) cannot reach it. The hosts of the
// trusted URLs (the manager passes its catalog URL) are exempt, and so are the
// proxies chosen from the environment (HTTPS_PROXY and friends). A request
// that goes through such a proxy is resolved and connected by the proxy, so
// the dial guard never sees the mirror's address; only the lexical checks of
// mirror URLs and redirects apply then. A caller's base client keeps its own
// transport and gets no dial guard.
func newHTTPClient(base *http.Client, trusted ...*url.URL) *http.Client {
	if base == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSHandshakeTimeout = 15 * time.Second
		transport.ResponseHeaderTimeout = 60 * time.Second
		guard := newDialGuard(transport.Proxy, trusted)
		transport.Proxy = guard.proxy
		transport.DialContext = guard.dialContext
		return &http.Client{Transport: transport, CheckRedirect: httpsOnlyRedirect(trusted)}
	}
	client := *base
	client.CheckRedirect = httpsOnlyRedirect(trusted)
	return &client
}

// errLocalAddress refuses a connection to an address no public mirror has.
var errLocalAddress = errors.New("localwiki: connection to a local network address refused")

// dialGuard dials trusted hosts and configured proxies directly and everything
// else through a dialer whose Control hook sees the resolved address of every
// connection attempt and refuses local ones (isLocalHost).
type dialGuard struct {
	upstreamProxy func(*http.Request) (*url.URL, error)
	direct        net.Dialer
	checked       net.Dialer

	mu     sync.Mutex
	exempt map[string]bool // lower-case "host:port" as the transport dials it
}

func newDialGuard(proxy func(*http.Request) (*url.URL, error), trusted []*url.URL) *dialGuard {
	g := &dialGuard{
		upstreamProxy: proxy,
		direct:        net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second},
		exempt:        make(map[string]bool),
	}
	g.checked = g.direct
	g.checked.Control = refuseLocalAddress
	for _, u := range trusted {
		g.allow(u)
	}
	return g
}

// allow exempts the address the transport dials for u.
func (g *dialGuard) allow(u *url.URL) {
	if u == nil || u.Hostname() == "" {
		return
	}
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			port = "443"
		case "http":
			port = "80"
		case "socks5", "socks5h":
			port = "1080"
		default:
			return
		}
	}
	address := strings.ToLower(net.JoinHostPort(u.Hostname(), port))
	g.mu.Lock()
	g.exempt[address] = true
	g.mu.Unlock()
}

func (g *dialGuard) isExempt(address string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.exempt[strings.ToLower(address)]
}

// proxy chooses the proxy like the wrapped function and exempts it: a proxy
// the administrator configured may live in the LAN, and the transport dials it
// instead of the mirror.
func (g *dialGuard) proxy(req *http.Request) (*url.URL, error) {
	if g.upstreamProxy == nil {
		return nil, nil
	}
	proxyURL, err := g.upstreamProxy(req)
	if err == nil && proxyURL != nil {
		g.allow(proxyURL)
	}
	return proxyURL, err
}

func (g *dialGuard) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if g.isExempt(address) {
		return g.direct.DialContext(ctx, network, address)
	}
	return g.checked.DialContext(ctx, network, address)
}

// refuseLocalAddress is a net.Dialer Control hook; address is the resolved
// "ip:port" of one connection attempt.
func refuseLocalAddress(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return errLocalAddress
	}
	if _, err := netip.ParseAddr(host); err != nil || isLocalHost(host) {
		return errLocalAddress
	}
	return nil
}

// requireHTTPS accepts absolute https URLs without user information.
func requireHTTPS(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("localwiki: %q is not an https URL", raw)
	}
	return parsed, nil
}

// meta4HostAllowed limits edition metadata to Kiwix hosts or the configured
// catalog host (tests run a local catalog).
func meta4HostAllowed(target, catalog *url.URL) bool {
	if catalog != nil && strings.EqualFold(target.Host, catalog.Host) {
		return true
	}
	host := strings.ToLower(target.Hostname())
	return host == "kiwix.org" || strings.HasSuffix(host, ".kiwix.org")
}

func hostOf(raw string) string {
	if parsed, err := url.Parse(raw); err == nil {
		return parsed.Host
	}
	return ""
}
