//go:build !windows

package desktop

import (
	"os"
	"syscall"
)

func openFileNoFollow(path string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag|syscall.O_NOFOLLOW, perm)
}
