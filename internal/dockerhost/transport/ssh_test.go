package transport

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/remote"
	"aurago/internal/testutil"

	"golang.org/x/crypto/ssh"
)

type streamLocalOpenPayload struct {
	SocketPath string
	Reserved0  string
	Reserved1  uint32
}

// sshFixture is an SSH server (user "fixture", password "fixture") that
// forwards every direct-streamlocal channel to a backend TCP address, standing
// in for sshd plus the Docker socket. open counts the authenticated SSH
// connections that are still open, accepted all of them, sockets receives the
// requested socket paths. With holdChannels the server never answers a channel
// open, like an sshd that stalls on the socket.
type sshFixture struct {
	host         string
	port         int
	holdChannels bool
	open         atomic.Int64
	accepted     atomic.Int64
	sockets      chan string
}

func startSSHFixture(t *testing.T, backend string, holdChannels bool) *sshFixture {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if meta.User() == "fixture" && string(password) == "fixture" {
				return nil, nil
			}
			return nil, fmt.Errorf("access denied")
		},
	}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	fixture := &sshFixture{holdChannels: holdChannels, sockets: make(chan string, 16)}
	addr := listener.Addr().(*net.TCPAddr)
	fixture.host, fixture.port = addr.IP.String(), addr.Port
	go func() {
		for {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			go fixture.serve(raw, config, backend)
		}
	}()
	return fixture
}

func (f *sshFixture) serve(raw net.Conn, config *ssh.ServerConfig, backend string) {
	defer raw.Close()
	_, channels, requests, err := ssh.NewServerConn(raw, config)
	if err != nil {
		return
	}
	// open first: a caller that saw accepted >= 1 also sees this connection in open.
	f.open.Add(1)
	f.accepted.Add(1)
	defer f.open.Add(-1)
	go ssh.DiscardRequests(requests)
	var held []ssh.NewChannel
	for newChannel := range channels {
		if f.holdChannels {
			held = append(held, newChannel) // neither accepted nor rejected
			continue
		}
		if newChannel.ChannelType() != "direct-streamlocal@openssh.com" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "only stream-local forwarding")
			continue
		}
		var open streamLocalOpenPayload
		if err := ssh.Unmarshal(newChannel.ExtraData(), &open); err != nil {
			_ = newChannel.Reject(ssh.ConnectionFailed, "malformed stream-local request")
			continue
		}
		select {
		case f.sockets <- open.SocketPath:
		default:
		}
		upstream, err := net.Dial("tcp", backend)
		if err != nil {
			_ = newChannel.Reject(ssh.ConnectionFailed, err.Error())
			continue
		}
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			_ = upstream.Close()
			continue
		}
		go ssh.DiscardRequests(channelRequests)
		go func() {
			defer channel.Close()
			defer upstream.Close()
			done := make(chan struct{}, 2)
			go func() { _, _ = io.Copy(upstream, channel); done <- struct{}{} }()
			go func() { _, _ = io.Copy(channel, upstream); done <- struct{}{} }()
			<-done
		}()
	}
}

// waitForClosedSSHConnections waits until the fixture holds no open SSH
// connection: closing the tunnel conn must close its SSH client.
func (f *sshFixture) waitForClosedSSHConnections(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for f.open.Load() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := f.open.Load(); n != 0 {
		t.Fatalf("%d SSH connections still open; closing the tunnel conn must close its SSH client", n)
	}
}

func useInsecureHostKeyForTest(t *testing.T) {
	t.Helper()
	prior := remote.InsecureHostKey
	remote.InsecureHostKey = true
	t.Cleanup(func() { remote.InsecureHostKey = prior })
}

func TestDialEngineOverSSHForwardsTheSocketAndCloseReleasesTheSSHClient(t *testing.T) {
	backend := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "pong")
	}))
	defer backend.Close()
	fixture := startSSHFixture(t, strings.TrimPrefix(backend.URL, "http://"), false)
	useInsecureHostKeyForTest(t)

	for _, tc := range []struct{ name, socket, wantSocket string }{
		{"default socket", "", "/var/run/docker.sock"},
		{"rootless socket", "/run/user/1000/docker.sock", "/run/user/1000/docker.sock"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			conn, err := DialEngineOverSSH(ctx, SSHTarget{Host: fixture.host, Port: fixture.port, User: "fixture", Secret: []byte("fixture"), Socket: tc.socket}, time.Second)
			if err != nil {
				t.Fatalf("DialEngineOverSSH: %v", err)
			}
			if _, err := io.WriteString(conn, "GET / HTTP/1.0\r\n\r\n"); err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(bufio.NewReader(conn))
			if err != nil || !strings.Contains(string(body), "pong") {
				t.Fatalf("answer through the tunnel = %q, %v; want the backend's pong", body, err)
			}
			if got := <-fixture.sockets; got != tc.wantSocket {
				t.Fatalf("forwarded socket = %q, want %q", got, tc.wantSocket)
			}
			_ = conn.Close() // the backend already ended the stream; EOF is expected
			fixture.waitForClosedSSHConnections(t)
		})
	}
}

func TestDialEngineOverSSHBoundsAStalledSocketOpen(t *testing.T) {
	fixture := startSSHFixture(t, "127.0.0.1:1", true)
	useInsecureHostKeyForTest(t)

	// net/http dials with a context that never ends; Background stands in for it.
	start := time.Now()
	conn, err := DialEngineOverSSH(context.Background(), SSHTarget{Host: fixture.host, Port: fixture.port, User: "fixture", Secret: []byte("fixture")}, 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("a socket open the server never answers succeeded")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("the stalled socket open took %v, want it bounded by the open budget", elapsed)
	}
	if !strings.Contains(err.Error(), "open /var/run/docker.sock") {
		t.Fatalf("error = %v, want the socket open failure", err)
	}
	if fixture.accepted.Load() != 1 {
		t.Fatalf("SSH connections = %d, want 1", fixture.accepted.Load())
	}
	fixture.waitForClosedSSHConnections(t)
}

func TestDialEngineOverSSHClosesTheClientWhenTheForwardFailsOrTheSecretIsWrong(t *testing.T) {
	// A backend address with nothing listening: sshd rejects the stream-local
	// channel, as it does for a missing socket or AllowStreamLocalForwarding no.
	closed, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	backend := closed.Addr().String()
	_ = closed.Close()
	fixture := startSSHFixture(t, backend, false)
	useInsecureHostKeyForTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = DialEngineOverSSH(ctx, SSHTarget{Host: fixture.host, Port: fixture.port, User: "fixture", Secret: []byte("fixture")}, time.Second)
	if err == nil || !strings.Contains(err.Error(), "docker over SSH: open /var/run/docker.sock") {
		t.Fatalf("rejected socket forward = %v, want the stream-local error", err)
	}
	fixture.waitForClosedSSHConnections(t)

	_, err = DialEngineOverSSH(ctx, SSHTarget{Host: fixture.host, Port: fixture.port, User: "fixture", Secret: []byte("wrong")}, time.Second)
	if err == nil || !strings.HasPrefix(err.Error(), "docker over SSH: ") || !strings.Contains(err.Error(), "ssh") {
		t.Fatalf("wrong password = %v, want an SSH authentication error behind the docker over SSH prefix", err)
	}
}
