//go:build !remote_minimal

package remote

import (
	"io"
	"log/slog"
	"net"
	"testing"
)

// The supervisor's cache must keep a nonce for the whole ±MaxTimestampDrift
// window (see TestNonceReplayCacheCoversFullTimestampWindow).
func TestSupervisorNonceCacheCoversFullTimestampWindow(t *testing.T) {
	assertNonceCacheCoversTimestampWindow(t, NewRemoteHub(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil))).nonceCache)
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
