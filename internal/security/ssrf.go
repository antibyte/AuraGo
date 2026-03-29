package security

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// privateRanges holds all IP ranges that must not be reachable via user-supplied URLs.
var privateRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"0.0.0.0/8",      // This network (RFC 1122)
		"10.0.0.0/8",     // RFC 1918 private
		"100.64.0.0/10",  // CGNAT (RFC 6598); also used by Tailscale VPN IPs — blocked for api_request; the Tailscale tool uses its own dedicated HTTP client
		"127.0.0.0/8",    // Loopback
		"169.254.0.0/16", // Link-local / cloud metadata (AWS 169.254.169.254, GCP, Azure)
		"172.16.0.0/12",  // RFC 1918 private
		"192.168.0.0/16", // RFC 1918 private
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique-local (RFC 4193)
		"fe80::/10",      // IPv6 link-local
	} {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			privateRanges = append(privateRanges, ipNet)
		}
	}
}

// isPrivateIP reports whether ip falls in a private or reserved range.
func isPrivateIP(ip net.IP) bool {
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateSSRF validates rawURL to prevent Server-Side Request Forgery.
// It rejects non-HTTP(S) schemes, empty hosts, and URLs that resolve to
// private/internal IP addresses (RFC 1918, loopback, link-local, cloud metadata, etc.).
func ValidateSSRF(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("disallowed URL scheme %q: only http and https are permitted", parsed.Scheme)
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no host")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("access to internal address %s is blocked (SSRF protection)", ip)
		}
		return nil
	}

	ips, err := resolvePublicIPs(context.Background(), host)
	if err != nil {
		return fmt.Errorf("hostname resolution failed for %q: %w", host, err)
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("access to internal address %s is blocked (SSRF protection)", ip)
		}
	}
	return nil
}

func resolvePublicIPs(ctx context.Context, host string) ([]net.IP, error) {
	resolver := net.Resolver{}
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	addrs, err := resolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no A/AAAA records found")
	}

	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		if isPrivateIP(addr.IP) {
			return nil, fmt.Errorf("access to internal address %s is blocked (SSRF protection)", addr.IP)
		}
		ips = append(ips, addr.IP)
	}
	return ips, nil
}

func validatedSSRFDialTarget(ctx context.Context, addr string) (networkAddr string, serverName string, err error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", fmt.Errorf("invalid target address %q: %w", addr, err)
	}
	serverName = host
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return "", "", fmt.Errorf("access to internal address %s is blocked (SSRF protection)", ip)
		}
		return net.JoinHostPort(ip.String(), port), serverName, nil
	}

	ips, err := resolvePublicIPs(ctx, host)
	if err != nil {
		return "", "", err
	}
	return net.JoinHostPort(ips[0].String(), port), serverName, nil
}

// NewSSRFProtectedHTTPClient returns an HTTP client that validates the initial URL,
// revalidates redirects, and pins outbound dials to a public IP selected during validation.
func NewSSRFProtectedHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 15 * time.Second}

	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		targetAddr, _, err := validatedSSRFDialTarget(ctx, addr)
		if err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, targetAddr)
	}
	transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		targetAddr, serverName, err := validatedSSRFDialTarget(ctx, addr)
		if err != nil {
			return nil, err
		}
		rawConn, err := dialer.DialContext(ctx, network, targetAddr)
		if err != nil {
			return nil, err
		}
		tlsConn := tls.Client(rawConn, &tls.Config{
			ServerName: serverName,
			MinVersion: tls.VersionTLS12,
		})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			rawConn.Close()
			return nil, err
		}
		return tlsConn, nil
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return ValidateSSRF(req.URL.String())
	}
	return client
}
