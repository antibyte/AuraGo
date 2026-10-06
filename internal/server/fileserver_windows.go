//go:build windows

package server

import (
	"io/fs"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// isServableMode reports whether neuteredFileSystem may serve a file of mode
// m: regular files, plus files Go reports as irregular because they carry a
// reparse tag that is not a link (cloud placeholders, WOF-compressed files),
// which http.Dir served. os.Root has already refused link reparse points, and
// directories, pipes, devices and sockets carry their own type bits.
func isServableMode(m fs.FileMode) bool {
	return m.IsRegular() || m.Type() == fs.ModeIrregular
}

// resolveServedPath returns the final path of p with every symlink and
// junction resolved. filepath.EvalSymlinks cannot be used here: since Go 1.23
// it reports junctions as irregular files and stops at them.
func resolveServedPath(p string) (string, error) {
	name, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return "", err
	}
	h, err := windows.CreateFile(name, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", &os.PathError{Op: "open", Path: p, Err: err}
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	for {
		// Flags 0 = FILE_NAME_NORMALIZED | VOLUME_NAME_DOS.
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 0)
		if err != nil {
			return "", &os.PathError{Op: "resolve", Path: p, Err: err}
		}
		if n < uint32(len(buf)) {
			final := windows.UTF16ToString(buf[:n])
			if rest, ok := strings.CutPrefix(final, `\\?\UNC\`); ok {
				return `\\` + rest, nil
			}
			return strings.TrimPrefix(final, `\\?\`), nil
		}
		buf = make([]uint16, n)
	}
}
