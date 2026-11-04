package mailer

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestMailDialTargetPolicy(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "::1", "192.168.1.10", "10.0.0.25", "100.64.1.2", "fd12::25", "8.8.8.8"} {
		if !allowedMailIP(net.ParseIP(raw)) {
			t.Fatalf("configured LAN/public mail target refused: %s", raw)
		}
	}
	for _, raw := range []string{"0.0.0.0", "::", "169.254.169.254", "::ffff:169.254.169.254", "fe80::1", "224.0.0.1", "ff02::1", "255.255.255.255", "fd00:ec2::254", "100.100.100.200", "64:ff9b::a9fe:a9fe", "2002:7f00:1::"} {
		if allowedMailIP(net.ParseIP(raw)) {
			t.Fatalf("restricted mail target accepted: %s", raw)
		}
	}
	for _, addr := range []string{"169.254.169.254:80", "[fd00:ec2::254]:80", "127.0.0.1:0", "127.0.0.1:65536", ":25"} {
		if _, err := dialMailTransport(context.Background(), "tcp", addr, time.Second); !errors.Is(err, ErrMailTarget) {
			t.Fatalf("target %q: error=%v", addr, err)
		}
	}
}

func TestMailDialAllowsConfiguredLoopback(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	conn, err := dialTransport(context.Background(), TransportConfig{}, "tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}
