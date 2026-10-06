package desktop

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// NewRootedHTTPFileSystem serves files beneath root while allowing only symlinks
// whose resolved targets stay inside that root.
func NewRootedHTTPFileSystem(root string) (http.FileSystem, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve rooted file system: %w", err)
	}
	opened, err := os.OpenRoot(abs)
	if err != nil {
		return nil, fmt.Errorf("open rooted file system: %w", err)
	}
	if err := opened.Close(); err != nil {
		return nil, fmt.Errorf("close rooted file system check: %w", err)
	}
	return rootedHTTPFileSystem{root: abs}, nil
}

type rootedHTTPFileSystem struct {
	root string
}

func (fsys rootedHTTPFileSystem) Open(name string) (http.File, error) {
	rel := strings.TrimPrefix(filepath.ToSlash(name), "/")
	if rel == "" {
		rel = "."
	}
	if !fs.ValidPath(rel) || strings.ContainsAny(rel, "\\:\x00") {
		return nil, fs.ErrInvalid
	}
	workspaceRoot, err := filepath.EvalSymlinks(fsys.root)
	if err != nil {
		return nil, fmt.Errorf("resolve rooted file system root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(fsys.root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, fmt.Errorf("resolve rooted file %q: %w", rel, err)
	}
	if !isWithinPath(workspaceRoot, resolved) {
		return nil, fs.ErrPermission
	}
	rootedPath, err := filepath.Rel(workspaceRoot, resolved)
	if err != nil {
		return nil, fmt.Errorf("make rooted file path %q relative: %w", rel, err)
	}
	root, err := os.OpenRoot(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("open rooted file root: %w", err)
	}
	file, err := root.Open(rootedPath)
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("open rooted file %q: %w", rel, err)
	}
	return rootedHTTPFile{File: file, root: root}, nil
}

type rootedHTTPFile struct {
	*os.File
	root *os.Root
}

func (f rootedHTTPFile) Close() error {
	return errors.Join(f.File.Close(), f.root.Close())
}
