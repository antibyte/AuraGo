//go:build !windows

package agent

import (
	"os"
	"syscall"
)

// fileHasMultipleHardlinks reports whether the file has more than one
// hard link. This is a best-effort check; on platforms where the link
// count is unavailable it returns false.
func fileHasMultipleHardlinks(fi os.FileInfo) bool {
	if fi == nil {
		return false
	}
	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		return stat.Nlink > 1
	}
	return false
}
