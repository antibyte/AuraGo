package mailer

import (
	"context"
	"errors"
	"io"
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

func TestMailDialFallsBackBeforeCallerDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ips := []net.IPAddr{{IP: net.ParseIP("fd12::1")}, {IP: net.ParseIP("127.0.0.1")}}
	conn, err := dialMailIPs(ctx, "tcp", "993", ips, func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "[fd12::1]:993" {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		if address != "127.0.0.1:993" || network != "tcp" {
			t.Errorf("unexpected unpinned target: %s %s", network, address)
		}
		client, peer := net.Pipe()
		_ = peer.Close()
		return client, nil
	})
	if err != nil {
		t.Fatalf("healthy IPv4 fallback missed the caller deadline: %v", err)
	}
	_ = conn.Close()
}

func TestMailDialClosesLateLosingConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	first, firstPeer := net.Pipe()
	second, secondPeer := net.Pipe()
	defer first.Close()
	defer firstPeer.Close()
	defer second.Close()
	defer secondPeer.Close()
	secondStarted, releaseSecond := make(chan struct{}), make(chan struct{})
	ips := []net.IPAddr{{IP: net.ParseIP("fd12::1")}, {IP: net.ParseIP("127.0.0.1")}}
	conn, err := dialMailIPs(ctx, "tcp", "993", ips, func(ctx context.Context, _, address string) (net.Conn, error) {
		if address == "[fd12::1]:993" {
			select {
			case <-secondStarted:
				return first, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		close(secondStarted)
		<-releaseSecond // Simulate a successful dial arriving after cancellation.
		return second, nil
	})
	close(releaseSecond)
	if err != nil || conn != first {
		t.Fatalf("preferred connection did not win: %v", err)
	}
	_ = secondPeer.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := secondPeer.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("losing connection was not closed: %v", err)
	}
}

func TestMailDialCancellationAndAddressFamily(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	done := make(chan error, 1)
	ips := []net.IPAddr{{IP: net.ParseIP("fd12::1")}, {IP: net.ParseIP("127.0.0.1")}}
	go func() {
		_, err := dialMailIPs(ctx, "tcp4", "993", ips, func(ctx context.Context, _, address string) (net.Conn, error) {
			if address != "127.0.0.1:993" {
				t.Errorf("tcp4 attempted another family: %s", address)
			}
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("IPv4 dial was not started")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation was lost: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled dial did not return")
	}
}
