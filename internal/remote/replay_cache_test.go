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
			// ValidateTimestamp still accepts a drift of exactly MaxTimestampDrift.
			if !cache.Seen("dev-1", "nonce-1", t0.Add(2*MaxTimestampDrift)) {
				t.Fatal("nonce must stay cached at the exact end of the timestamp window")
			}
			if cache.Seen("dev-1", "nonce-1", t0.Add(2*MaxTimestampDrift+time.Minute)) {
				t.Fatal("nonce should expire once the whole timestamp window has passed")
			}
		})
	}
}

// A cache that evicts live entries when full can be flushed by anyone who can
// get enough fresh signed frames delivered, re-opening the replay window for an
// older captured frame. The agent's fail-closed cache drops new nonces instead.
func TestFailClosedNonceReplayCacheReportsSeenWhenFullAndNeverEvictsALiveEntry(t *testing.T) {
	cache := NewFailClosedNonceReplayCache(time.Minute, 3)
	t0 := time.Now().UTC()
	for _, nonce := range []string{"nonce-1", "nonce-2", "nonce-3"} {
		if cache.Seen("dev-1", nonce, t0) {
			t.Fatalf("%s: first use while the cache has room must not be a replay", nonce)
		}
	}
	if !cache.Seen("dev-1", "nonce-4", t0) {
		t.Fatal("a new nonce must be reported as seen while the cache is full of live entries")
	}
	for _, nonce := range []string{"nonce-1", "nonce-2", "nonce-3", "nonce-4"} {
		if !cache.Seen("dev-1", nonce, t0.Add(30*time.Second)) {
			t.Fatalf("%s: must still be reported as seen; a full fail-closed cache never evicts", nonce)
		}
	}
	if cache.Seen("dev-1", "nonce-5", t0.Add(2*time.Minute)) {
		t.Fatal("once the live entries expire, a new nonce must be accepted again")
	}

	// The supervisor keeps the evicting form so one device cannot lock out others.
	evicting := newNonceReplayCache(time.Minute, 3)
	for _, nonce := range []string{"nonce-1", "nonce-2", "nonce-3"} {
		evicting.Seen("dev-1", nonce, t0)
	}
	if evicting.Seen("dev-2", "nonce-4", t0) {
		t.Fatal("the evicting cache must keep accepting new nonces when full")
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
