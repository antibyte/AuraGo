package security

import (
	"fmt"
	"net"
	"net/url"
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
		"::ffff:0:0/96",  // IPv4-mapped IPv6 addresses
	} {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			privateRanges = append(privateRanges, ipNet)
		}
	}
}

// isPrivateIP reports whether ip falls in a private or reserved range.
func isPrivateIP(ip net.IP) bool {
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

	// Resolve the hostname to IP addresses and check each against blocked ranges.
	// If DNS resolution fails, the HTTP client will also fail — not a security concern.
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("access to internal address %s is blocked (SSRF protection)", ip)
		}
	}
	return nil
}
