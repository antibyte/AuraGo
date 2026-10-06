//go:build !remote_minimal

package remote

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type streamLocalOpenMsg struct {
	SocketPath string
	Reserved0  string
	Reserved1  uint32
}

// startStreamLocalEchoServer accepts password "fixture" for user "fixture"
// and echoes every direct-streamlocal channel. It reports the socket paths.
func startStreamLocalEchoServer(t *testing.T) (string, int, <-chan string) {
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
	sockets := make(chan string, 8)
	go func() {
		for {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			go func(raw net.Conn) {
				defer raw.Close()
				_, channels, requests, err := ssh.NewServerConn(raw, config)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(requests)
				for newChannel := range channels {
					if newChannel.ChannelType() != "direct-streamlocal@openssh.com" {
						_ = newChannel.Reject(ssh.UnknownChannelType, "stream-local only")
						continue
					}
					var open streamLocalOpenMsg
					if err := ssh.Unmarshal(newChannel.ExtraData(), &open); err != nil {
						_ = newChannel.Reject(ssh.ConnectionFailed, "malformed stream-local request")
						continue
					}
					select {
					case sockets <- open.SocketPath:
					default:
					}
					channel, channelRequests, err := newChannel.Accept()
					if err != nil {
						continue
					}
					go ssh.DiscardRequests(channelRequests)
					go func() {
						defer channel.Close()
						_, _ = io.Copy(channel, channel)
					}()
				}
			}(raw)
		}
	}()
	addr := listener.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port, sockets
}

func TestDialSSHOutlivesDialContextAndForwardsStreamLocal(t *testing.T) {
	host, port, sockets := startStreamLocalEchoServer(t)
	prior := InsecureHostKey
	InsecureHostKey = true
	defer func() { InsecureHostKey = prior }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	client, err := DialSSH(ctx, host, port, "fixture", []byte("fixture"))
	if err != nil {
		cancel()
		t.Fatalf("DialSSH: %v", err)
	}
	defer client.Close()
	cancel() // the client belongs to the caller once the handshake succeeded

	conn, err := client.Dial("unix", "/var/run/docker.sock")
	if err != nil {
		t.Fatalf("stream-local dial after the dial context ended: %v", err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "ping\n"); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, len("ping\n"))
	if _, err := io.ReadFull(conn, reply); err != nil || string(reply) != "ping\n" {
		t.Fatalf("echo = %q, %v; want ping", reply, err)
	}
	select {
	case got := <-sockets:
		if got != "/var/run/docker.sock" {
			t.Fatalf("socket path = %q, want /var/run/docker.sock", got)
		}
	case <-time.After(time.Second):
		t.Fatal("server saw no stream-local channel")
	}
}

func TestDialSSHHonoursCancellationDuringHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		c, err := listener.Accept()
		if err == nil {
			defer c.Close()
			_, _ = io.Copy(io.Discard, c)
		}
	}()
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	prior := InsecureHostKey
	InsecureHostKey = true
	defer func() { InsecureHostKey = prior }()

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	client, err := DialSSH(ctx, host, port, "fixture", []byte("fixture"))
	if err == nil {
		client.Close()
		t.Fatal("silent peer completed an SSH handshake")
	}
	if time.Since(start) > time.Second {
		t.Fatal("DialSSH ignored cancellation during the handshake")
	}
}

func TestDialSSHRequiresKnownHostsWithoutInsecureOptIn(t *testing.T) {
	prior := InsecureHostKey
	InsecureHostKey = false
	defer func() { InsecureHostKey = prior }()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	knownHostsCache.mu.Lock()
	knownHostsCache.callback, knownHostsCache.path, knownHostsCache.expiresAt = nil, "", time.Time{}
	knownHostsCache.mu.Unlock()

	_, err := DialSSH(context.Background(), "127.0.0.1", 1, "fixture", []byte("fixture"))
	if err == nil || !strings.Contains(err.Error(), "known_hosts") {
		t.Fatalf("DialSSH without known_hosts = %v, want the known_hosts refusal", err)
	}
}
