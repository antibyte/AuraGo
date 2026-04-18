package remote

import (
	"net"
	"testing"
	"time"
)

func TestNonceReplayCacheRejectsReuseWithinTTL(t *testing.T) {
	cache := newNonceReplayCache(5*time.Minute, 10)
	now := time.Now().UTC()

	if cache.Seen("dev-1", "nonce-1", now) {
		t.Fatal("first nonce use should not be treated as replay")
	}
	if !cache.Seen("dev-1", "nonce-1", now.Add(time.Minute)) {
		t.Fatal("second nonce use within ttl should be treated as replay")
	}
}

func TestNonceReplayCacheAllowsReuseAfterExpiry(t *testing.T) {
	cache := newNonceReplayCache(2*time.Minute, 10)
	now := time.Now().UTC()

	if cache.Seen("dev-1", "nonce-1", now) {
		t.Fatal("first nonce use should not be treated as replay")
	}
	if cache.Seen("dev-1", "nonce-1", now.Add(3*time.Minute)) {
		t.Fatal("nonce should be accepted again after expiry")
	}
}

func TestIsTrustedAutoApproveRemoteAddr(t *testing.T) {
	tests := []struct {
		name string
		addr net.Addr
		want bool
	}{
		{name: "loopback", addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8443}, want: true},
		{name: "private", addr: &net.TCPAddr{IP: net.ParseIP("192.168.1.25"), Port: 8443}, want: true},
		{name: "public", addr: &net.TCPAddr{IP: net.ParseIP("8.8.8.8"), Port: 8443}, want: false},
		{name: "nil", addr: nil, want: false},
	}
	for _, tt := range tests {
		if got := isTrustedAutoApproveRemoteAddr(tt.addr); got != tt.want {
			t.Fatalf("%s: got %v want %v", tt.name, got, tt.want)
		}
	}
}
