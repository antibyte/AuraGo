//go:build windows

package desktop

import "os"

func openFileNoFollow(path string, flag int, perm os.FileMode) (*os.File, error) {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, errSymlinkPath()
	}
	file, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		_ = file.Close()
		return nil, errSymlinkPath()
	}
	return file, nil
}

func errSymlinkPath() error {
	return &os.PathError{Op: "open", Path: "desktop path", Err: os.ErrPermission}
}
