//go:build !windows

package tools

import (
	"context"
	"net"

	"aurago/internal/dockerutil"
)

func dialDockerNamedPipe(ctx context.Context, host string) (net.Conn, error) {
	return dockerutil.DialContext(ctx, host)
}
