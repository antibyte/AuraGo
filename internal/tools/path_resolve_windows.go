//go:build windows

package tools

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func resolveExistingToolPath(path string) (string, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	// A metadata-only handle follows junctions without requiring file read access.
	handle, err := windows.CreateFile(name, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", &os.PathError{Op: "open", Path: path, Err: err}
	}
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, 512)
	for {
		n, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
		if err != nil {
			return "", &os.PathError{Op: "resolve", Path: path, Err: err}
		}
		if int(n) >= len(buffer) {
			buffer = make([]uint16, n+1)
			continue
		}
		final := windows.UTF16ToString(buffer[:n])
		if rest, ok := strings.CutPrefix(final, `\\?\UNC\`); ok {
			return `\\` + rest, nil
		}
		if rest, ok := strings.CutPrefix(final, `\\?\`); ok && len(rest) >= 3 && rest[1] == ':' && rest[2] == '\\' {
			return rest, nil
		}
		return "", fmt.Errorf("cannot classify final Windows path for %q", path)
	}
}
