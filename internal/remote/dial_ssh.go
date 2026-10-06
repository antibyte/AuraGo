//go:build !remote_minimal

package remote

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// DialSSH opens an authenticated SSH client that the caller keeps open, for
// example to forward a Docker Engine socket. Dial and handshake use exactly
// the budget, host-key policy (known_hosts unless InsecureHostKey) and
// cancellation of ExecuteRemoteCommand: the earlier of the SSH timeout and
// the ctx deadline bounds them, and cancelling ctx closes the connection
// while the handshake runs. After a successful handshake the client no
// longer follows ctx; the caller must Close it.
//
// The dial block is a copy of ExecuteRemoteCommand's (remote.go), which is
// left untouched because of its call-graph reach.
func DialSSH(ctx context.Context, host string, port int, user string, secret []byte) (*ssh.Client, error) {
	config, err := GetSSHConfig(user, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get ssh config: %w", err)
	}

	addr := net.JoinHostPort(host, fmt.Sprint(port))
	d := net.Dialer{Timeout: config.Timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	deadline := time.Now().Add(config.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		stopClose()
		conn.Close()
		return nil, fmt.Errorf("ssh handshake failed: %w", err)
	}
	if !stopClose() {
		// ctx ended right after the handshake; AfterFunc already closed conn.
		_ = sshConn.Close()
		return nil, fmt.Errorf("ssh dial cancelled: %w", context.Cause(ctx))
	}
	_ = conn.SetDeadline(time.Time{})
	return ssh.NewClient(sshConn, chans, reqs), nil
}
