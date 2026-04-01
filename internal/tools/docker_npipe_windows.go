//go:build windows

package tools

import (
	"context"
	"fmt"
	"net"

	winio "github.com/tailscale/go-winio"
)

func dialDockerNamedPipe(ctx context.Context, host string) (net.Conn, error) {
	pipePath, err := normalizeDockerNamedPipeHost(host)
	if err != nil {
		return nil, err
	}
	conn, err := winio.DialPipeContext(ctx, pipePath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker named pipe %s: %w", pipePath, err)
	}
	return conn, nil
}
