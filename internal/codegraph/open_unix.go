//go:build !windows

package codegraph

import (
	"os"
	"syscall"
)

// A concurrent replacement with a FIFO must not stall the single parser worker.
func openSource(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
}
