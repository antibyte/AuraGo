//go:build windows

package desktop

import "os"

// secureOpenAfterLstat is a test-only seam run between the pre-open Lstat and
// the open. It is nil in production.
var secureOpenAfterLstat func(path string)

// openFileNoFollow refuses symlinks without O_NOFOLLOW: it inspects the entry
// with Lstat and requires the opened handle to be that same file, so a swap
// between the check and the open is refused before any truncation.
func openFileNoFollow(path string, flag int, perm os.FileMode) (*os.File, error) {
	// The deferred truncate below would fail on an append-only handle (no write access); no caller uses O_APPEND.
	if flag&os.O_APPEND != 0 && flag&os.O_TRUNC != 0 {
		return nil, &os.PathError{Op: "open", Path: "desktop path", Err: os.ErrInvalid}
	}
	var pre os.FileInfo
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, errSymlinkPath()
		}
		// Windows Lstat loads the file ID lazily from the path the first time
		// SameFile needs it, which would read the post-swap entry. Load it now;
		// an entry whose file ID cannot be read is refused.
		if !os.SameFile(info, info) {
			return nil, errSymlinkPath()
		}
		pre = info
	}
	// pre stays nil after any Lstat error, usually a missing entry for O_CREATE.
	if secureOpenAfterLstat != nil {
		secureOpenAfterLstat(path)
	}
	// os.OpenFile truncates inside the open, so truncate only after the check.
	truncate := flag&os.O_TRUNC != 0
	file, err := os.OpenFile(path, flag&^os.O_TRUNC, perm)
	if err != nil {
		return nil, err
	}
	if pre != nil {
		opened, err := file.Stat()
		if err != nil || !os.SameFile(pre, opened) {
			_ = file.Close()
			return nil, errSymlinkPath()
		}
	} else if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		// No identity to compare: refuse a symlink planted before the open.
		_ = file.Close()
		return nil, errSymlinkPath()
	}
	if truncate {
		if err := file.Truncate(0); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	return file, nil
}

func errSymlinkPath() error {
	return &os.PathError{Op: "open", Path: "desktop path", Err: os.ErrPermission}
}
