package server

import (
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// neuteredFileSystem wraps an http.FileSystem so that it serves regular files
// only (isServableMode): directories (no listings) and other non-regular files
// are not found.
type neuteredFileSystem struct {
	fs http.FileSystem
}

// Open implements the http.FileSystem interface.
// If the requested path is not a servable file, it returns os.ErrNotExist.
func (nfs neuteredFileSystem) Open(path string) (http.File, error) {
	f, err := nfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !isServableMode(s.Mode()) {
		// Close the entry and return an error pretending it doesn't exist
		f.Close()
		return nil, os.ErrNotExist
	}

	return f, nil
}

// rootBoundFileSystem serves dir through an os.Root: symlinks resolve only
// while they stay inside dir, and escapes are reported as not found. A fresh
// root is opened per request so directory recreation never leaves a stale
// handle behind; open files stay valid after the root closes.
//
// os.Root also refuses absolute link targets (and Windows junctions) even when
// they point back inside dir, and its escape error is unexported. So Open falls
// back for every error that is neither not-exist nor permission: escapes,
// fs.ErrInvalid names, ELOOP, ENAMETOOLONG, sharing violations, a file used as
// a directory, and the like. The fallback resolves dir and the requested path
// to their real locations (resolveServedPath) and, when the result stays
// inside the real dir, opens that resolved relative path through a root on the
// real dir, under the requested name. A link swapped in after the check cannot
// escape: the final open still runs through os.Root, which enforces every
// component again. Resolution errors are all reported as not found, so a link
// cannot reveal what exists outside dir; only the final root-bound open may
// answer with a permission error (403).
//
// Every other failure is reported as not found, so invalid names (backslashes,
// NUL/CON, "::$DATA") answer 404 as http.Dir did.
type rootBoundFileSystem string

func (dir rootBoundFileSystem) Open(name string) (http.File, error) {
	f, err := openInRoot(string(dir), name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrPermission) {
		f, err = openResolvedInRoot(string(dir), name)
	}
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, err
		}
		return nil, os.ErrNotExist
	}
	return f, nil
}

func openInRoot(dir, name string) (http.File, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return http.FS(root.FS()).Open(name)
}

// openResolvedInRoot is rootBoundFileSystem's fallback for links that os.Root
// refuses although their target may stay inside dir.
func openResolvedInRoot(dir, name string) (http.File, error) {
	cleaned := path.Clean("/" + name)
	local, err := filepath.Localize(strings.TrimPrefix(cleaned, "/"))
	if err != nil {
		return nil, os.ErrNotExist
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, os.ErrNotExist
	}
	realDir, err := resolveServedPath(absDir)
	if err != nil {
		return nil, os.ErrNotExist
	}
	resolved, err := resolveServedPath(filepath.Join(realDir, local))
	if err != nil {
		return nil, os.ErrNotExist
	}
	rel, err := filepath.Rel(realDir, resolved)
	if err != nil || !pathStaysWithinDir(realDir, resolved) {
		slog.Debug("file server refused a link that leaves the served directory", "dir", dir, "target", resolved)
		return nil, os.ErrNotExist
	}
	f, err := openInRoot(realDir, "/"+filepath.ToSlash(rel))
	if err != nil {
		return nil, err
	}
	return requestNamedFile{File: f, name: path.Base(cleaned)}, nil
}

// requestNamedFile reports the requested base name from Stat, so a link opened
// through the fallback is served under its own name (and Content-Type), as
// http.Dir did, not under its target's.
type requestNamedFile struct {
	http.File
	name string
}

func (f requestNamedFile) Stat() (fs.FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return requestNamedInfo{FileInfo: info, name: f.name}, nil
}

type requestNamedInfo struct {
	fs.FileInfo
	name string
}

func (i requestNamedInfo) Name() string { return i.name }

// readRootBoundFile reads the regular file at the slash-separated rel path
// inside dir with the same rules as the /files/ mounts.
func readRootBoundFile(dir, rel string) ([]byte, fs.FileInfo, error) {
	f, err := neuteredFileSystem{rootBoundFileSystem(dir)}.Open(path.Clean("/" + rel))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	content, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}
	return content, info, nil
}
