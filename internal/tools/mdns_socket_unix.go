//go:build !windows

package tools

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// mdnsSocketControl sets SO_REUSEADDR and SO_REUSEPORT so multiple processes
// (e.g. avahi, systemd-resolved) can co-exist on port 5353.
func mdnsSocketControl(_ string, _ string, c syscall.RawConn) error {
	return c.Control(func(fd uintptr) {
		_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
		_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	})
}
