package security

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestIsPrivateIPAllowsPublicIPv4MappedIPv6(t *testing.T) {
	ip := net.ParseIP("::ffff:35.157.26.135")
	if ip == nil {
		t.Fatal("failed to parse IP")
	}
	if isPrivateIP(ip) {
		t.Fatal("expected public IPv4-mapped IPv6 address to be allowed")
	}
}

func TestIsPrivateIPBlocksLoopbackIPv4MappedIPv6(t *testing.T) {
	ip := net.ParseIP("::ffff:127.0.0.1")
	if ip == nil {
		t.Fatal("failed to parse IP")
	}
	if !isPrivateIP(ip) {
		t.Fatal("expected loopback IPv4-mapped IPv6 address to be blocked")
	}
}

func TestStrictPublicHTTPClientIgnoresLoopbackEscapeHatch(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	for _, rawURL := range []string{"https://127.0.0.1/upload", "https://[::1]/upload", "https://user:pass@8.8.8.8/upload", "https://8.8.8.8/upload#token"} {
		_, err := NewStrictPublicHTTPClientForURL(rawURL, time.Second)
		if err == nil || !strings.Contains(err.Error(), "strict") {
			t.Fatalf("strict client accepted %q with loopback escape hatch: %v", rawURL, err)
		}
	}
}
