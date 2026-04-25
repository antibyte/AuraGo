package dockerutil

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"strings"
	"time"
)

// APIVersion is the Docker Engine API version used for AuraGo Docker requests.
const APIVersion = "v1.45"

const (
	defaultUnixHost    = "unix:///var/run/docker.sock"
	defaultWindowsHost = "npipe:////./pipe/docker_engine"
)

// DefaultHost returns the Docker host endpoint for the current OS.
func DefaultHost() string {
	return DefaultHostForGOOS(runtime.GOOS)
}

// DefaultHostForGOOS returns the Docker host endpoint for a specific GOOS value.
func DefaultHostForGOOS(goos string) string {
	if goos == "windows" {
		return defaultWindowsHost
	}
	return defaultUnixHost
}

// NormalizeHost returns host when set, otherwise the OS-appropriate Docker host.
func NormalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return DefaultHost()
	}
	return host
}

// DialContext opens a connection to a Docker Engine endpoint.
func DialContext(ctx context.Context, host string) (net.Conn, error) {
	host = NormalizeHost(host)
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	switch {
	case strings.HasPrefix(host, "unix://"):
		return dialer.DialContext(ctx, "unix", strings.TrimPrefix(host, "unix://"))
	case strings.HasPrefix(host, "npipe://"):
		return dialNamedPipe(ctx, host)
	case strings.HasPrefix(host, "tcp://"):
		return dialer.DialContext(ctx, "tcp", strings.TrimPrefix(host, "tcp://"))
	default:
		return dialer.DialContext(ctx, "tcp", host)
	}
}

// NormalizeNamedPipeHost converts Docker npipe:// hosts to Windows pipe paths.
func NormalizeNamedPipeHost(host string) (string, error) {
	if !strings.HasPrefix(host, "npipe://") {
		return "", fmt.Errorf("invalid Docker named pipe host %q", host)
	}
	path := strings.TrimPrefix(host, "npipe://")
	path = strings.ReplaceAll(path, "/", `\`)
	switch {
	case strings.HasPrefix(path, `\\.\pipe\`):
		return path, nil
	case strings.HasPrefix(path, `.\pipe\`):
		return `\\` + path, nil
	case strings.HasPrefix(path, `pipe\`):
		return `\\.\` + path, nil
	default:
		return "", fmt.Errorf("invalid Docker named pipe path %q", host)
	}
}

// NormalizeHostPathForBind normalizes host paths for Docker bind mount strings.
func NormalizeHostPathForBind(hostPath string) string {
	return strings.ReplaceAll(strings.TrimSpace(hostPath), `\`, "/")
}

// FormatBindMount formats a Docker bind mount string with normalized host slashes.
func FormatBindMount(hostPath, containerPath string, opts ...string) string {
	parts := []string{NormalizeHostPathForBind(hostPath), containerPath}
	parts = append(parts, opts...)
	return strings.Join(parts, ":")
}
