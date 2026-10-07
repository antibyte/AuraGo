package desktop

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/fileutil"
)

func (s *Service) checkPathTargetLocked(path, relative string, check FileWritePrecondition) error {
	if check == nil {
		check = func(current FileWriteState) error {
			if current.Exists {
				return &PathConflict{Path: relative, Directory: current.Entry.Type == "directory"}
			}
			return nil
		}
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return check(FileWriteState{})
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("desktop destination is a symlink")
	}
	if info.IsDir() {
		return check(FileWriteState{Exists: true, Entry: FileEntry{Path: relative, Type: "directory"}})
	}
	state, err := s.fileWriteStateLocked(path, desktopCopyMaxBytes)
	if err != nil {
		return err
	}
	state.Entry.Path = relative
	return check(state)
}

func desktopRootTransferPaths(rootPath, from, to string) (string, string, error) {
	src, e1 := filepath.Rel(rootPath, from)
	dst, e2 := filepath.Rel(rootPath, to)
	if e1 != nil || e2 != nil || !filepath.IsLocal(src) || !filepath.IsLocal(dst) || src == "." || dst == "." {
		return "", "", fmt.Errorf("invalid desktop transfer path")
	}
	if strings.HasPrefix(strings.ToLower(dst)+string(filepath.Separator), strings.ToLower(src)+string(filepath.Separator)) {
		return "", "", fmt.Errorf("destination cannot be inside the source")
	}
	return src, dst, nil
}

func moveDesktopPathRoot(ctx context.Context, rootPath, from, to string) error {
	src, dst, err := desktopRootTransferPaths(rootPath, from, to)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := root.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	return fileutil.RenameRootContext(ctx, root, src, dst)
}

func copyDesktopPathRoot(ctx context.Context, rootPath, from, to string) error {
	src, dst, err := desktopRootTransferPaths(rootPath, from, to)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	stage := filepath.Join(filepath.Dir(dst), ".aurago-copy-"+rand.Text())
	defer root.RemoveAll(stage)
	stats := &desktopCopyStats{}
	if err := copyDesktopTree(ctx, root, src, stage, 0, stats); err != nil {
		return err
	}
	return fileutil.RenameRootContext(ctx, root, stage, dst)
}

func copyDesktopTree(ctx context.Context, root *os.Root, src, dst string, depth int, stats *desktopCopyStats) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	stats.entries++
	if depth > desktopCopyMaxDepth || stats.entries > desktopCopyMaxEntries {
		return fmt.Errorf("desktop copy limit exceeded")
	}
	info, err := root.Lstat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return fmt.Errorf("desktop copy refuses links and special files")
	}
	in, err := root.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if info.IsDir() {
		if err := root.Mkdir(dst, 0700); err != nil {
			return err
		}
		for {
			entries, readErr := in.ReadDir(128)
			for _, entry := range entries {
				if err := copyDesktopTree(ctx, root, filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name()), depth+1, stats); err != nil {
					return err
				}
			}
			if readErr == io.EOF {
				return nil
			}
			if readErr != nil {
				return readErr
			}
		}
	}
	out, err := root.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(out, io.LimitReader(desktopContextReader{ctx, in}, desktopCopyMaxBytes-stats.bytes+1))
	stats.bytes += n
	if copyErr == nil && stats.bytes > desktopCopyMaxBytes {
		copyErr = fmt.Errorf("desktop copy size limit exceeded")
	}
	if copyErr == nil {
		copyErr = out.Sync()
	}
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

type desktopContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r desktopContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
