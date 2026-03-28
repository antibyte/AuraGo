package security

import (
	"net"
	"testing"
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
