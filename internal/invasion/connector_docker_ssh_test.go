package invasion

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/dockerutil"
	"aurago/internal/remote"
	"aurago/internal/testutil"

	"golang.org/x/crypto/ssh"
)

type streamLocalOpenPayload struct {
	SocketPath string
	Reserved0  string
	Reserved1  uint32
}

// dockerSSHFixture is an SSH server (user "fixture", password "fixture") that
// forwards direct-streamlocal channels for /var/run/docker.sock to a backend
// TCP address, standing in for sshd plus the Docker socket. open counts the
// authenticated SSH connections that are still open; accepted counts all of
// them. With holdChannels the server never answers a channel open, like an
// sshd that stalls on the socket.
type dockerSSHFixture struct {
	host         string
	port         int
	holdChannels bool
	open         atomic.Int64
	accepted     atomic.Int64
	sockets      chan string
}

func startDockerSSHFixture(t *testing.T, backend string) *dockerSSHFixture {
	t.Helper()
	return startDockerSSHFixtureWith(t, backend, false)
}

func startDockerSSHFixtureWith(t *testing.T, backend string, holdChannels bool) *dockerSSHFixture {
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
	fixture := &dockerSSHFixture{holdChannels: holdChannels, sockets: make(chan string, 256)}
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

func (f *dockerSSHFixture) serve(raw net.Conn, config *ssh.ServerConfig, backend string) {
	defer raw.Close()
	_, channels, requests, err := ssh.NewServerConn(raw, config)
	if err != nil {
		return
	}
	f.accepted.Add(1)
	f.open.Add(1)
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
		if open.SocketPath != "/var/run/docker.sock" {
			_ = newChannel.Reject(ssh.ConnectionFailed, "unexpected socket")
			continue
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
// connection: every Engine connection must have closed its SSH client.
func (f *dockerSSHFixture) waitForClosedSSHConnections(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for f.open.Load() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := f.open.Load(); n != 0 {
		t.Fatalf("%d SSH connections still open after the operations; every Engine connection must close its SSH client", n)
	}
}

func useInsecureHostKeyForTest(t *testing.T) {
	t.Helper()
	prior := remote.InsecureHostKey
	remote.InsecureHostKey = true
	t.Cleanup(func() { remote.InsecureHostKey = prior })
}

func TestGetConnector_DockerSSH(t *testing.T) {
	if _, ok := GetConnector(NestRecord{DeployMethod: "docker_ssh"}).(*DockerConnector); !ok {
		t.Fatal("docker_ssh must map to the Docker connector, not the SSH binary deploy")
	}
}

func TestDockerConnector_apiURL_DockerSSH(t *testing.T) {
	got := (&DockerConnector{}).apiURL(NestRecord{Host: "10.0.0.5", Port: 22, DeployMethod: "docker_ssh"}, "/version")
	if want := fmt.Sprintf("http://localhost/%s/version", dockerAPIVersion); got != want {
		t.Fatalf("apiURL = %q, want %q", got, want)
	}
}

func TestDockerSSHIsNeitherPlaintextNorTLS(t *testing.T) {
	// A docker_ssh nest never carries Docker TLS (the handlers reject it), but
	// even a leftover mode must not route it to the HTTPS transport or the
	// plaintext warnings.
	for _, mode := range []string{DockerTLSOff, DockerTLSServer, DockerTLSMutual} {
		nest := NestRecord{DeployMethod: "docker_ssh", DockerTLS: mode}
		if DockerRemotePlaintext(nest) {
			t.Fatalf("docker_ssh with docker_tls %q reported as plaintext docker_remote", mode)
		}
		if DockerRemoteUsesTLS(nest) {
			t.Fatalf("docker_ssh with docker_tls %q reported as a TLS docker_remote nest", mode)
		}
	}
}

func TestDockerConnectorSSHTransportNegotiatesAPIVersion(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	ts := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			_ = json.NewEncoder(w).Encode(map[string]string{"ApiVersion": "1.41", "MinAPIVersion": "1.24"})
			return
		}
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]string{"ApiVersion": "1.41"})
	}))
	defer ts.Close()
	fixture := startDockerSSHFixture(t, strings.TrimPrefix(ts.URL, "http://"))
	useInsecureHostKeyForTest(t)

	nest := NestRecord{ID: "12345678-abcd-ef12-3456-7890abcdef12", Host: fixture.host, Port: fixture.port, Username: "fixture", DeployMethod: "docker_ssh"}
	if _, ok := (&DockerConnector{}).httpClient(nest, nil).Transport.(*dockerutil.VersionTransport); !ok {
		t.Fatal("the docker_ssh client must negotiate the Engine API version like the other Docker transports")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := (&DockerConnector{}).Validate(ctx, nest, []byte("fixture")); err != nil {
		t.Fatalf("Validate over SSH: %v", err)
	}
	mu.Lock()
	got := strings.Join(paths, ",")
	mu.Unlock()
	if got != "/v1.41/version" {
		t.Fatalf("Engine saw %q, want the negotiated /v1.41/version", got)
	}
	fixture.waitForClosedSSHConnections(t)
}

func TestDockerConnectorSSHTunnelsEngineSocketAndClosesSessions(t *testing.T) {
	var archivePath atomic.Value
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/version": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]string{"Version": "24.0.0"})
		},
		"/images/create": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "{\"status\":\"Status: Image is up to date\"}\n")
		},
		"/containers/create": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"Id": "abc123"})
		},
		"/containers/": func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPut:
				archivePath.Store(r.URL.Query().Get("path"))
				w.WriteHeader(http.StatusOK)
			case http.MethodPost, http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		},
	})
	defer ts.Close()
	fixture := startDockerSSHFixture(t, strings.TrimPrefix(ts.URL, "http://"))
	useInsecureHostKeyForTest(t)

	nest := NestRecord{ID: "12345678-abcd-ef12-3456-7890abcdef12", Host: fixture.host, Port: fixture.port, Username: "fixture", DeployMethod: "docker_ssh"}
	secret := []byte("fixture")
	c := &DockerConnector{}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := c.Validate(ctx, nest, secret); err != nil {
		t.Fatalf("Validate over SSH: %v", err)
	}
	if err := c.Deploy(ctx, nest, secret, EggDeployPayload{ConfigYAML: []byte("server:\n  port: 8099\n"), EggPort: 8099}); err != nil {
		t.Fatalf("Deploy over SSH: %v", err)
	}
	if got, _ := archivePath.Load().(string); got != "/app/data" {
		t.Fatalf("config archive path = %q, want /app/data", got)
	}
	select {
	case socket := <-fixture.sockets:
		if socket != "/var/run/docker.sock" {
			t.Fatalf("forwarded socket = %q, want /var/run/docker.sock", socket)
		}
	default:
		t.Fatal("the SSH server saw no stream-local channel")
	}
	fixture.waitForClosedSSHConnections(t)
	t.Logf("SSH connections opened for Validate and Deploy: %d", fixture.accepted.Load())
}

func TestDockerConnectorSSHClosesClientWhenSocketForwardFails(t *testing.T) {
	// A backend address with nothing listening: sshd rejects the stream-local
	// channel, as it does for a missing socket or AllowStreamLocalForwarding no.
	closed, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	backend := closed.Addr().String()
	_ = closed.Close()
	fixture := startDockerSSHFixture(t, backend)
	useInsecureHostKeyForTest(t)

	nest := NestRecord{ID: "12345678-abcd-ef12-3456-7890abcdef12", Host: fixture.host, Port: fixture.port, Username: "fixture", DeployMethod: "docker_ssh"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = (&DockerConnector{}).Validate(ctx, nest, []byte("fixture"))
	if err == nil || !strings.Contains(err.Error(), "open /var/run/docker.sock") {
		t.Fatalf("Validate with a rejected socket forward = %v, want the stream-local error", err)
	}
	if fixture.accepted.Load() == 0 {
		t.Fatal("the SSH handshake never happened")
	}
	fixture.waitForClosedSSHConnections(t)
}

// shortDockerSSHSocketOpenTimeout shrinks the socket-open budget for one test.
func shortDockerSSHSocketOpenTimeout(t *testing.T, budget time.Duration) {
	t.Helper()
	prior := dockerSSHSocketOpenTimeout
	dockerSSHSocketOpenTimeout = budget
	t.Cleanup(func() { dockerSSHSocketOpenTimeout = prior })
}

func TestDialDockerEngineOverSSHBoundsAStalledSocketOpen(t *testing.T) {
	fixture := startDockerSSHFixtureWith(t, "127.0.0.1:1", true)
	useInsecureHostKeyForTest(t)
	shortDockerSSHSocketOpenTimeout(t, 200*time.Millisecond)

	nest := NestRecord{ID: "12345678-abcd-ef12-3456-7890abcdef12", Host: fixture.host, Port: fixture.port, Username: "fixture", DeployMethod: "docker_ssh"}
	// net/http dials with a context that never ends; Background stands in for it.
	start := time.Now()
	conn, err := dialDockerEngineOverSSH(context.Background(), nest, []byte("fixture"))
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

func TestDockerConnectorSSHClosesClientAfterAStalledSocketOpenThroughHTTP(t *testing.T) {
	fixture := startDockerSSHFixtureWith(t, "127.0.0.1:1", true)
	useInsecureHostKeyForTest(t)
	shortDockerSSHSocketOpenTimeout(t, 300*time.Millisecond)

	nest := NestRecord{ID: "12345678-abcd-ef12-3456-7890abcdef12", Host: fixture.host, Port: fixture.port, Username: "fixture", DeployMethod: "docker_ssh"}
	// The request gives up first; the dial it started must still end within
	// the open budget and close its SSH client.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := (&DockerConnector{}).Validate(ctx, nest, []byte("fixture")); err == nil {
		t.Fatal("Validate against a stalled socket open succeeded")
	}
	fixture.waitForClosedSSHConnections(t)
}

func TestDockerConnectorSSHRejectsWrongCredential(t *testing.T) {
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{})
	defer ts.Close()
	fixture := startDockerSSHFixture(t, strings.TrimPrefix(ts.URL, "http://"))
	useInsecureHostKeyForTest(t)

	nest := NestRecord{ID: "12345678-abcd-ef12-3456-7890abcdef12", Host: fixture.host, Port: fixture.port, Username: "fixture", DeployMethod: "docker_ssh"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := (&DockerConnector{}).Validate(ctx, nest, []byte("wrong"))
	if err == nil || !strings.Contains(err.Error(), "ssh") {
		t.Fatalf("Validate with a wrong password = %v, want an SSH authentication error", err)
	}
}
