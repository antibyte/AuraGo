package transport

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"aurago/internal/remote"

	"golang.org/x/crypto/ssh"
)

// SSHProbeTimeout bounds the Engine version probe for an Engine reached over
// SSH. The probe's first dial includes a full SSH login (10 s budget in
// remote.DialSSH), so 20 s leaves 10 s for the socket open and /version.
const SSHProbeTimeout = 20 * time.Second

// DefaultEngineSocket is the Engine socket on the remote host when a target
// names none.
const DefaultEngineSocket = "/var/run/docker.sock"

// DefaultSocketOpenTimeout is the socket-open budget used when the caller
// passes a non-positive timeout.
const DefaultSocketOpenTimeout = 10 * time.Second

// SSHTarget names one Engine socket behind SSH.
type SSHTarget struct {
	Host   string
	Port   int // <=0 means 22
	User   string
	Secret []byte // SSH private key or password (remote.DialSSH semantics)
	Socket string // remote Engine socket path, "" means /var/run/docker.sock
}

// DialEngineOverSSH opens one SSH connection (known_hosts verification and
// finite dial/handshake budget via remote.DialSSH) and forwards a stream to the
// remote Engine socket. socketOpenTimeout bounds the direct-streamlocal open:
// net/http detaches the dial from the request, so without it an authenticated
// sshd that never answers the open would keep the SSH client and its
// goroutines alive forever. Closing the returned conn closes the SSH client.
func DialEngineOverSSH(ctx context.Context, target SSHTarget, socketOpenTimeout time.Duration) (net.Conn, error) {
	port := target.Port
	if port <= 0 {
		port = 22
	}
	socket := target.Socket
	if socket == "" {
		socket = DefaultEngineSocket
	}
	if socketOpenTimeout <= 0 {
		socketOpenTimeout = DefaultSocketOpenTimeout
	}
	client, err := remote.DialSSH(ctx, target.Host, port, target.User, target.Secret)
	if err != nil {
		return nil, fmt.Errorf("docker over SSH: %w", err)
	}
	openCtx, cancel := context.WithTimeout(ctx, socketOpenTimeout)
	defer cancel()
	conn, err := client.DialContext(openCtx, "unix", socket)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("docker over SSH: open %s: %w", socket, err)
	}
	return &sshTunnelConn{Conn: conn, client: client}, nil
}

// sshTunnelConn is a forwarded Engine stream that owns its SSH client.
type sshTunnelConn struct {
	net.Conn
	client *ssh.Client
	once   sync.Once
}

func (c *sshTunnelConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { _ = c.client.Close() })
	return err
}
