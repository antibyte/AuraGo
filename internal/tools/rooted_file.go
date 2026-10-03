package tools

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// rootedToolPath binds a previously validated tool path to an open directory.
// Root operations reject a symlink exchanged for a path component after validation.
func rootedToolPath(path string) (*os.Root, string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	base := filepath.Dir(path)
	for current := base; ; current = filepath.Dir(current) {
		if strings.EqualFold(filepath.Base(current), "agent_workspace") {
			base = current
			break
		}
		if parent := filepath.Dir(current); parent == current {
			break
		}
	}
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, "", fmt.Errorf("path outside tool root: %s", path)
	}
	root, err := os.OpenRoot(base)
	return root, rel, err
}

func rootedToolReadFile(path string) ([]byte, error) {
	root, rel, err := rootedToolPath(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.ReadFile(rel)
}

func rootedToolOpen(path string) (*os.File, error) {
	root, rel, err := rootedToolPath(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(rel)
}

func rootedToolOpenFile(path string, flags int, mode os.FileMode) (*os.File, error) {
	root, rel, err := rootedToolPath(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.OpenFile(rel, flags, mode)
}

func rootedToolLstat(path string) (os.FileInfo, error) {
	root, rel, err := rootedToolPath(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Lstat(rel)
}

func rootedToolWalkDir(path string, visit func(string, os.DirEntry, error) error) error {
	root, rel, err := rootedToolPath(path)
	if err != nil {
		return err
	}
	defer root.Close()
	return fs.WalkDir(root.FS(), filepath.ToSlash(rel), func(name string, entry fs.DirEntry, walkErr error) error {
		return visit(filepath.Join(root.Name(), filepath.FromSlash(name)), entry, walkErr)
	})
}

func rootedToolStat(path string) (os.FileInfo, error) {
	root, rel, err := rootedToolPath(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Stat(rel)
}

func rootedToolMkdirAll(path string, mode os.FileMode) error {
	root, rel, err := rootedToolPath(path)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.MkdirAll(rel, mode)
}

func rootedToolChmod(path string, mode os.FileMode) error {
	root, rel, err := rootedToolPath(path)
	if err != nil {
		return err
	}
	defer root.Close()
	f, err := root.OpenFile(rel, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Chmod(mode)
}

func rootedToolWriteFileAtomic(path string, data []byte) error {
	return rootedToolWriteFromReaderAtomic(path, bytes.NewReader(data))
}

func rootedToolWriteFromReaderAtomic(path string, source io.Reader) error {
	return rootedToolWriteFromReaderAtomicMode(path, source, 0o644)
}

func rootedToolWriteFromReaderAtomicMode(path string, source io.Reader, mode os.FileMode) error {
	root, rel, err := rootedToolPath(path)
	if err != nil {
		return err
	}
	defer root.Close()
	return writeRootFromReaderAtomic(root, rel, source, mode, true)
}

// writeRootFromReaderAtomic replaces rel inside root with source. It writes a
// private temporary file in the destination directory, syncs it, checks the
// Close error and renames it over rel, so a failed write never truncates an
// existing destination. keepExistingMode keeps an existing destination's
// permission bits; otherwise the file gets exactly mode.
func writeRootFromReaderAtomic(root *os.Root, rel string, source io.Reader, mode os.FileMode, keepExistingMode bool) error {
	if err := root.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	if keepExistingMode {
		if info, err := root.Stat(rel); err == nil {
			mode = info.Mode().Perm()
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat destination: %w", err)
		}
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Errorf("create temporary name: %w", err)
	}
	temp := filepath.Join(filepath.Dir(rel), ".aurago_edit_"+hex.EncodeToString(random[:]))
	f, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	defer root.Remove(temp)
	if _, err := io.Copy(f, source); err != nil {
		f.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if !keepExistingMode {
		if err := f.Chmod(mode); err != nil {
			f.Close()
			return fmt.Errorf("set temporary file mode: %w", err)
		}
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := root.Rename(temp, rel); err != nil {
		return fmt.Errorf("replace destination: %w", err)
	}
	return nil
}

func stageRootedToolFile(source, destination string) error {
	input, err := rootedToolOpen(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
