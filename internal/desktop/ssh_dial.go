package desktop

import (
	"context"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

func dialDesktopSSH(ctx context.Context, address string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	clientConn, channels, requests, err := ssh.NewClientConn(conn, address, cfg)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		clientConn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return ssh.NewClient(clientConn, channels, requests), nil
}
