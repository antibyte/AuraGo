package security

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var publicURLLookupIP = net.DefaultResolver.LookupIPAddr

var nonPublicURLPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
	netip.MustParsePrefix("2001:db8::/32"),
}

// ValidatePublicHTTPURL accepts only HTTP(S) URLs whose host resolves entirely
// to publicly routable addresses. Unlike ValidateSSRF, this strict validator
// deliberately ignores the development loopback escape hatch because the
// remote provider must be able to fetch the URL from the public internet.
func ValidatePublicHTTPURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("only public http and https URLs are permitted")
	}
	if parsed.User != nil {
		return fmt.Errorf("URL credentials are not permitted")
	}
	host := strings.TrimSuffix(strings.TrimSpace(parsed.Hostname()), ".")
	if host == "" {
		return fmt.Errorf("URL has no host")
	}

	if ip := net.ParseIP(host); ip != nil {
		if !isStrictPublicIP(ip) {
			return fmt.Errorf("URL host is not publicly routable")
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addrs, err := publicURLLookupIP(ctx, host)
	if err != nil {
		return fmt.Errorf("hostname resolution failed: %w", err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("hostname has no A or AAAA records")
	}
	for _, addr := range addrs {
		if !isStrictPublicIP(addr.IP) {
			return fmt.Errorf("URL host resolves to a non-public address")
		}
	}
	return nil
}

func isStrictPublicIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() {
		return false
	}
	for _, prefix := range nonPublicURLPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}
