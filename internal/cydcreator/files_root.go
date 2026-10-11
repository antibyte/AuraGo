package cydcreator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"aurago/internal/fileutil"
)

var errFileTooLarge = errors.New("file exceeds the size limit")

// rootReadFile reads a regular file inside root, at most max bytes.
func rootReadFile(root *os.Root, rel string, max int64) ([]byte, error) {
	f, err := root.Open(filepath.FromSlash(rel))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", rel)
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, errFileTooLarge
	}
	return data, nil
}

// rootWriteFile publishes data at rel inside root atomically: a synced
// temporary sibling is renamed over the target (fileutil retries Windows
// sharing violations and honours the context's publication gate).
func rootWriteFile(ctx context.Context, root *os.Root, rel string, data []byte) error {
	dir := path.Dir(rel)
	if dir != "." {
		if err := root.MkdirAll(filepath.FromSlash(dir), 0o750); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	var suffix [6]byte
	_, _ = rand.Read(suffix[:])
	tmp := filepath.FromSlash(path.Join(dir, ".cyd-write-"+hex.EncodeToString(suffix[:])))
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", rel, err)
	}
	_, werr := f.Write(data)
	if werr == nil {
		werr = f.Sync()
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = fileutil.RenameRootContext(ctx, root, tmp, filepath.FromSlash(rel))
	}
	if werr != nil {
		_ = root.Remove(tmp)
		return fmt.Errorf("write %s: %w", rel, werr)
	}
	return nil
}
