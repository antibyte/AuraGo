//go:build windows

package dockerutil

import (
	"context"
	"fmt"
	"net"

	winio "github.com/tailscale/go-winio"
)

func dialNamedPipe(ctx context.Context, host string) (net.Conn, error) {
	pipePath, err := NormalizeNamedPipeHost(host)
	if err != nil {
		return nil, err
	}
	conn, err := winio.DialPipeContext(ctx, pipePath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker named pipe %s: %w", pipePath, err)
	}
	return conn, nil
}
