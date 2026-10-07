//go:build windows

package desktop

import "os"

// secureOpenAfterLstat is a test-only seam run between the pre-open Lstat and
// the open. It is nil in production.
var secureOpenAfterLstat func(path string)

// openFileNoFollow refuses symlinks without O_NOFOLLOW: it inspects the entry
// with Lstat and then requires the opened handle to be that same file, so a
// swap between the check and the open is refused instead of followed.
func openFileNoFollow(path string, flag int, perm os.FileMode) (*os.File, error) {
	pre, err := os.Lstat(path)
	if err == nil {
		if pre.Mode()&os.ModeSymlink != 0 {
			return nil, errSymlinkPath()
		}
		// Windows Lstat reads the file ID lazily from the path when SameFile
		// first needs it, which would compare the post-swap entry. Pin it now.
		if !os.SameFile(pre, pre) {
			return nil, errSymlinkPath()
		}
	} else {
		pre = nil
	}
	if secureOpenAfterLstat != nil {
		secureOpenAfterLstat(path)
	}
	file, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}
	if pre != nil {
		opened, err := file.Stat()
		if err != nil || !os.SameFile(pre, opened) {
			_ = file.Close()
			return nil, errSymlinkPath()
		}
		return file, nil
	}
	// The entry did not exist before (O_CREATE): there is no identity to
	// compare, so keep the path check for a symlink planted in between.
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		_ = file.Close()
		return nil, errSymlinkPath()
	}
	return file, nil
}

func errSymlinkPath() error {
	return &os.PathError{Op: "open", Path: "desktop path", Err: os.ErrPermission}
}
