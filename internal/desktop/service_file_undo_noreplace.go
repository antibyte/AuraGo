package desktop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func desktopNoReplaceCollision(err error) bool {
	return errors.Is(err, fs.ErrExist)
}

func moveDesktopPathRootNoReplace(ctx context.Context, rootPath, from, to string, expected *os.File) error {
	src, dst, err := desktopRootTransferPaths(rootPath, from, to)
	if err != nil {
		return err
	}
	if expected == nil {
		return ErrFileUndoUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateNoSymlinkComponents(rootPath, filepath.Join(rootPath, filepath.Dir(src)), false); err != nil {
		return err
	}
	if err := validateNoSymlinkComponents(rootPath, filepath.Join(rootPath, filepath.Dir(dst)), true); err != nil {
		return err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := root.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return ErrFileUndoUnavailable
	}
	opened, err := root.Open(src)
	if err != nil {
		return err
	}
	openedInfo, statErr := opened.Stat()
	_ = opened.Close()
	if statErr != nil {
		return statErr
	}
	pinnedInfo, err := expected.Stat()
	if err != nil || !os.SameFile(openedInfo, pinnedInfo) {
		return ErrFileUndoUnavailable
	}
	if err := root.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := renameDesktopPathNoReplace(root, src, dst, expected); err != nil {
		return fmt.Errorf("rename without replacement: %w", err)
	}
	return nil
}

func moveDesktopSymlinkRootNoReplace(ctx context.Context, rootPath, from, to string) error {
	src, dst, err := desktopRootTransferPaths(rootPath, from, to)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateNoSymlinkComponents(rootPath, filepath.Join(rootPath, filepath.Dir(src)), false); err != nil {
		return err
	}
	if err := validateNoSymlinkComponents(rootPath, filepath.Join(rootPath, filepath.Dir(dst)), true); err != nil {
		return err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err := root.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("desktop entry is no longer a symlink")
	}
	if err := root.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := renameDesktopPathNoReplace(root, src, dst, nil); err != nil {
		return fmt.Errorf("move symlink without replacement: %w", err)
	}
	return nil
}
