package security

import (
	"bytes"
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

func TestReadLimitedBodyRejectsOversize(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), int(MaxSSRFResponseBytes)+1)
	body, err := ReadLimitedBody(bytes.NewReader(payload), MaxSSRFResponseBytes)
	if err == nil || err.Error() != "response exceeds 5 MiB" {
		t.Fatalf("expected response exceeds 5 MiB, got %v", err)
	}
	if body != nil {
		t.Fatalf("oversize body = %q, want nil slice", body)
	}
}

func TestIsRestrictedNetworkIPRejectsSpecialPurposeRanges(t *testing.T) {
	tests := []struct {
		name string
		ip   string
	}{
		{"IPv6 site-local fec0::/10", "fec0::1"},
		{"SIIT IPv4-translated ::ffff:0:0:0/96 to a LAN host", "::ffff:0:10.0.0.1"},
		{"SIIT IPv4-translated ::ffff:0:0:0/96 to a public host", "::ffff:0:8.8.8.8"},
		{"IPv6 discard-only 100::/64", "100::1"},
		{"IPv6 benchmarking 2001:2::/48", "2001:2::1"},
		{"IPv6 ORCHID 2001:10::/28", "2001:10::1"},
		{"6to4 relay anycast 192.88.99.0/24", "192.88.99.1"},
		{"IPv6 documentation 3fff::/20", "3fff::1"},
		{"IPv6 SRv6 SIDs 5f00::/16", "5f00::1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("failed to parse %q", tc.ip)
			}
			if !IsRestrictedNetworkIP(ip) {
				t.Errorf("IsRestrictedNetworkIP(%s) = false, want true", tc.ip)
			}
			if isStrictPublicIP(ip) {
				t.Errorf("isStrictPublicIP(%s) = true, want false", tc.ip)
			}
		})
	}
}

func TestIsRestrictedNetworkIPKeepsPublicAddresses(t *testing.T) {
	for _, raw := range []string{
		"8.8.8.8",
		"1.1.1.1",
		"192.88.98.1",
		"192.88.100.1",
		"::ffff:8.8.8.8",
		"2606:4700:4700::1111",
		"2001:4860:4860::8888",
		"2a00:1450:4001:81b::200e",
	} {
		ip := net.ParseIP(raw)
		if ip == nil {
			t.Fatalf("failed to parse %q", raw)
		}
		if IsRestrictedNetworkIP(ip) {
			t.Errorf("IsRestrictedNetworkIP(%s) = true, want false", raw)
		}
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
