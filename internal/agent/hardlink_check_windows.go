//go:build windows

package agent

import "os"

// fileHasMultipleHardlinks reports whether the file has more than one
// hard link. On Windows this is a best-effort no-op; symlinks are already
// blocked separately and the manifest SaveTool path check prevents creation
// of tools outside the tools directory.
func fileHasMultipleHardlinks(fi os.FileInfo) bool {
	return false
}
