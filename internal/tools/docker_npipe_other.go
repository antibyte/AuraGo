//go:build !windows

package tools

import (
	"context"
	"fmt"
	"net"
	"runtime"
)

func dialDockerNamedPipe(ctx context.Context, host string) (net.Conn, error) {
	_ = ctx
	return nil, fmt.Errorf("windows named pipes are not supported on %s (host %s)", runtime.GOOS, host)
}
