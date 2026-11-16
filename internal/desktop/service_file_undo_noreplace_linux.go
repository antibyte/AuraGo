//go:build linux

package desktop

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func renameDesktopPathNoReplace(root *os.Root, src, dst string, expected *os.File) error {
	srcParent, err := root.OpenRoot(filepath.Dir(src))
	if err != nil {
		return err
	}
	defer srcParent.Close()
	dstParent, err := root.OpenRoot(filepath.Dir(dst))
	if err != nil {
		return err
	}
	defer dstParent.Close()
	srcDir, err := srcParent.Open(".")
	if err != nil {
		return err
	}
	defer srcDir.Close()
	dstDir, err := dstParent.Open(".")
	if err != nil {
		return err
	}
	defer dstDir.Close()
	var pinnedInfo os.FileInfo
	if expected != nil {
		source, err := srcParent.Open(filepath.Base(src))
		if err != nil {
			return err
		}
		sourceInfo, err := source.Stat()
		_ = source.Close()
		if err != nil {
			return err
		}
		pinnedInfo, err = expected.Stat()
		if err != nil || !os.SameFile(sourceInfo, pinnedInfo) {
			return ErrFileUndoUnavailable
		}
	} else {
		info, err := srcParent.Lstat(filepath.Base(src))
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			return ErrFileUndoUnavailable
		}
	}
	srcConn, err := srcDir.SyscallConn()
	if err != nil {
		return err
	}
	dstConn, err := dstDir.SyscallConn()
	if err != nil {
		return err
	}
	var renameErr error
	if err := srcConn.Control(func(srcFD uintptr) {
		err := dstConn.Control(func(dstFD uintptr) {
			renameErr = unix.Renameat2(int(srcFD), filepath.Base(src), int(dstFD), filepath.Base(dst), unix.RENAME_NOREPLACE)
		})
		if err != nil && renameErr == nil {
			renameErr = err
		}
	}); err != nil && renameErr == nil {
		return err
	}
	if renameErr != nil {
		if errors.Is(renameErr, unix.EEXIST) {
			return fs.ErrExist
		}
		return renameErr
	}
	if expected == nil {
		movedInfo, err := root.Lstat(dst)
		if err != nil || movedInfo.Mode()&os.ModeSymlink == 0 {
			return ErrFileUndoUncertain
		}
	} else {
		moved, err := root.Open(dst)
		if err != nil {
			return ErrFileUndoUncertain
		}
		movedInfo, statErr := moved.Stat()
		_ = moved.Close()
		if statErr != nil || !os.SameFile(movedInfo, pinnedInfo) {
			return ErrFileUndoUncertain
		}
	}
	return nil
}
