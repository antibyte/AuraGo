//go:build windows

package tools

import "syscall"

// mdnsSocketControl sets SO_REUSEADDR so the socket can bind to port 5353.
// Windows does not support SO_REUSEPORT; SO_REUSEADDR is sufficient.
func mdnsSocketControl(_ string, _ string, c syscall.RawConn) error {
	return c.Control(func(fd uintptr) {
		_ = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	})
}
