//go:build windows

package localwiki

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// finalPathNormalizedDOS is FILE_NAME_NORMALIZED | VOLUME_NAME_DOS (both 0),
// which golang.org/x/sys/windows does not define.
const finalPathNormalizedDOS = 0

// resolveExistingDir returns the real location of an existing directory as
// Windows opens it: GetFinalPathNameByHandle resolves symbolic links and
// junctions (also in the last path element, which filepath.EvalSymlinks
// leaves alone), 8.3 short names, ignored trailing dots and spaces and mapped
// network drives (as \\server\share). When Windows cannot
// name the volume with a drive letter or a UNC path, filepath.EvalSymlinks is
// the fallback.
func resolveExistingDir(dir string) (string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]uint16, 512)
	for {
		n, err := windows.GetFinalPathNameByHandle(windows.Handle(f.Fd()), &buf[0], uint32(len(buf)), finalPathNormalizedDOS)
		if err != nil {
			return filepath.EvalSymlinks(dir)
		}
		if int(n) < len(buf) {
			return stripFinalPathPrefix(windows.UTF16ToString(buf[:n])), nil
		}
		buf = make([]uint16, n+1)
	}
}

// stripFinalPathPrefix turns \\?\C:\dir into C:\dir and \\?\UNC\server\share
// into \\server\share. Any other form keeps its prefix, which the sensitive
// path check refuses as unclassifiable.
func stripFinalPathPrefix(p string) string {
	if rest, ok := strings.CutPrefix(p, `\\?\UNC\`); ok {
		return `\\` + rest
	}
	if rest, ok := strings.CutPrefix(p, `\\?\`); ok && len(rest) >= 2 && rest[1] == ':' {
		return rest
	}
	return p
}
