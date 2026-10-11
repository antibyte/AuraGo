package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"aurago/internal/fileutil"
	"aurago/internal/remote"
	"aurago/internal/uid"
)

type fileOperation struct {
	*Executor
	ctx   context.Context
	root  *os.Root
	path  string
	limit int64
}

type remoteFileLock struct {
	mu   sync.Mutex
	refs int
}

func (e *Executor) lockFile(path string) func() {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if runtime.GOOS == "windows" {
		path = strings.ToLower(filepath.Clean(path))
	}
	e.mu.Lock()
	if e.fileLocks == nil {
		e.fileLocks = make(map[string]*remoteFileLock)
	}
	lock := e.fileLocks[path]
	if lock == nil {
		lock = &remoteFileLock{}
		e.fileLocks[path] = lock
	}
	lock.refs++
	e.mu.Unlock()
	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		e.mu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(e.fileLocks, path)
		}
		e.mu.Unlock()
	}
}

func (e *fileOperation) authorize(path string, allowed []string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for _, dir := range allowed {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		base, err := filepath.Abs(dir)
		if err != nil || !pathWithinDir(abs, base) {
			continue
		}
		rel, err := filepath.Rel(base, abs)
		if err != nil {
			continue
		}
		root, err := os.OpenRoot(base)
		if err != nil {
			return fmt.Errorf("open allowed root: %w", err)
		}
		e.root, e.path = root, rel
		return nil
	}
	return fmt.Errorf("path is outside allowed_paths")
}

func (e *fileOperation) relativePath(_ string) (string, error) {
	if e.root == nil {
		return "", fmt.Errorf("file root is not authorized")
	}
	return e.path, nil
}

func (e *fileOperation) validateReadSize(path string) error {
	info, err := e.root.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("only regular files are supported")
	}
	if info.Size() > e.limit {
		return fmt.Errorf("file exceeds configured max_file_size_mb limit")
	}
	return e.ctx.Err()
}

type checkedReader struct {
	io.ReadCloser
	ctx       context.Context
	remaining int64
}

func (r *checkedReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > r.remaining+1 {
		p = p[:r.remaining+1]
	}
	n, err := r.ReadCloser.Read(p)
	r.remaining -= int64(n)
	if r.remaining < 0 {
		return 0, fmt.Errorf("file exceeds configured max_file_size_mb limit")
	}
	return n, err
}
func (e *fileOperation) openRead(path string) (io.ReadCloser, error) {
	if err := e.ctx.Err(); err != nil {
		return nil, err
	}
	f, err := e.root.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > e.limit {
		f.Close()
		return nil, fmt.Errorf("file is not regular or exceeds configured max_file_size_mb limit")
	}
	return &checkedReader{ReadCloser: f, ctx: e.ctx, remaining: e.limit}, nil
}
func (e *fileOperation) readFile(path string) ([]byte, error) {
	f, err := e.openRead(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (e *fileOperation) writeFile(path string, data []byte, mode os.FileMode) error {
	if int64(len(data)) > e.limit {
		return fmt.Errorf("file exceeds configured max_file_size_mb limit")
	}
	if err := e.ctx.Err(); err != nil {
		return err
	}
	// Pin the parent as well: renaming it during preparation cannot move publication elsewhere.
	parent, err := e.root.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer parent.Close()
	name := filepath.Base(path)
	var original *os.File
	if info, err := parent.Lstat(name); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("write target must be a regular file, not a symlink")
		}
		mode = info.Mode().Perm()
		original, err = parent.Open(name)
		if err != nil {
			return err
		}
		defer original.Close()
	} else if !os.IsNotExist(err) {
		return err
	}
	temp := ".aurago-" + uid.New() + ".tmp"
	f, err := parent.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer parent.Remove(temp)
	if _, err = f.Write(data); err == nil {
		if chmodErr := f.Chmod(mode.Perm()); chmodErr != nil {
			err = fmt.Errorf("set prepared file mode: %w", chmodErr)
		}
	}
	if err == nil && original != nil {
		if aclErr := preserveFilePermissions(original, f); aclErr != nil {
			err = fmt.Errorf("preserve file permissions: %w", aclErr)
		}
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return fileutil.RenameRootContext(e.ctx, parent, temp, name)
}

func (e *fileOperation) remove(path string) error {
	return fileutil.PublishContext(e.ctx, func() error { return e.root.Remove(path) })
}

func (e *fileOperation) walk(path string, visit filepath.WalkFunc) error {
	return fs.WalkDir(e.root.FS(), filepath.ToSlash(path), func(p string, d fs.DirEntry, err error) error {
		if ctxErr := e.ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return visit(filepath.FromSlash(p), info, nil)
	})
}

type fileSizeBuffer struct {
	bytes.Buffer
	limit int64
}

func (b *fileSizeBuffer) Write(p []byte) (int, error) {
	if int64(b.Len()+len(p)) > b.limit {
		return 0, fmt.Errorf("file exceeds configured max_file_size_mb limit")
	}
	return b.Buffer.Write(p)
}

const truncationMarker = "\n[output truncated]"

type boundedOutput struct {
	mu sync.Mutex
	bytes.Buffer
	truncated bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := remote.MaxTextOutputBytes - len(truncationMarker) - b.Len()
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}
func (b *boundedOutput) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := strings.ToValidUTF8(b.Buffer.String(), "")
	if b.truncated {
		s += truncationMarker
	}
	return s
}
