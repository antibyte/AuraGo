package remote

import (
	"io"
	"log/slog"
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

func TestNonceReplayCacheExportedWrapperMatchesSupervisorCache(t *testing.T) {
	cache := NewNonceReplayCache(2*time.Minute, 10)
	now := time.Now().UTC()

	if cache.Seen("dev-1", "nonce-1", now) {
		t.Fatal("first nonce use should not be treated as replay")
	}
	if !cache.Seen("dev-1", "nonce-1", now.Add(time.Minute)) {
		t.Fatal("second nonce use within ttl should be treated as replay")
	}
	if cache.Seen("dev-1", "nonce-1", now.Add(3*time.Minute)) {
		t.Fatal("nonce should be accepted again after expiry")
	}
	if !cache.Seen("", "nonce-2", now) || !cache.Seen("dev-1", "", now) {
		t.Fatal("empty device id or nonce must always be treated as seen")
	}
}

// A frame is accepted while its timestamp is within ±MaxTimestampDrift, so a
// nonce first seen when the frame is future-dated must stay cached until the
// frame turns stale.
func TestNonceReplayCacheCoversFullTimestampWindow(t *testing.T) {
	caches := map[string]*nonceReplayCache{
		"default ttl": newNonceReplayCache(0, 10),
		"supervisor":  NewRemoteHub(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil))).nonceCache,
	}
	for name, cache := range caches {
		t.Run(name, func(t *testing.T) {
			t0 := time.Now().UTC()
			if cache.Seen("dev-1", "nonce-1", t0) {
				t.Fatal("first nonce use should not be treated as replay")
			}
			if !cache.Seen("dev-1", "nonce-1", t0.Add(MaxTimestampDrift+time.Minute)) {
				t.Fatal("nonce must stay cached past MaxTimestampDrift while the frame can still be fresh")
			}
			if cache.Seen("dev-1", "nonce-1", t0.Add(2*MaxTimestampDrift+time.Minute)) {
				t.Fatal("nonce should expire once the whole timestamp window has passed")
			}
		})
	}
}

func TestIsTrustedAutoApproveRemoteAddr(t *testing.T) {
	tests := []struct {
		name string
		addr net.Addr
		want bool
	}{
		{name: "loopback", addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8443}, want: false},
		{name: "private", addr: &net.TCPAddr{IP: net.ParseIP("192.168.1.25"), Port: 8443}, want: false},
		{name: "public", addr: &net.TCPAddr{IP: net.ParseIP("8.8.8.8"), Port: 8443}, want: false},
		{name: "nil", addr: nil, want: false},
	}
	for _, tt := range tests {
		if got := isTrustedAutoApproveRemoteAddr(tt.addr); got != tt.want {
			t.Fatalf("%s: got %v want %v", tt.name, got, tt.want)
		}
	}
}
