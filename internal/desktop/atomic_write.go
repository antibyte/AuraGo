package desktop

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"

	"aurago/internal/fileutil"
)

func secureWriteWorkspaceFileRoot(ctx context.Context, rootPath, path string, content []byte) (os.FileInfo, error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	rel, err := filepath.Rel(rootPath, path)
	if err != nil || !filepath.IsLocal(rel) {
		return nil, fmt.Errorf("invalid desktop write path")
	}
	if err := root.MkdirAll(filepath.Dir(rel), 0700); err != nil {
		return nil, err
	}
	temp := filepath.Join(filepath.Dir(rel), fmt.Sprintf(".aurago-write-%x", rand.Text()))
	file, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	defer root.Remove(temp)
	_, writeErr := file.Write(content)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	info, statErr := file.Stat()
	closeErr := file.Close()
	if writeErr != nil {
		return nil, writeErr
	}
	if statErr != nil {
		return nil, statErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := fileutil.RenameRootContext(ctx, root, temp, rel); err != nil {
		return nil, err
	}
	return info, nil
}
