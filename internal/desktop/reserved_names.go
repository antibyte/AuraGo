package desktop

import (
	"fmt"
	"path/filepath"
	"strings"
)

// isWindowsReservedDeviceName reports whether Windows opens a path segment as a
// device. Win32 resolves these names in every directory and ignores anything
// after the first dot or colon plus trailing spaces, so "Pictures/com1 .png"
// would write to a serial port instead of a file.
func isWindowsReservedDeviceName(segment string) bool {
	base := segment
	if i := strings.IndexAny(base, ".:"); i >= 0 {
		base = base[:i]
	}
	base = strings.ToUpper(strings.TrimRight(base, ". "))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	if !strings.HasPrefix(base, "COM") && !strings.HasPrefix(base, "LPT") {
		return false
	}
	switch base[3:] {
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³":
		return true
	}
	return false
}

// validateNoReservedDeviceNames refuses a workspace-relative path when any
// segment is a Windows device name. Workspaces may be synced to Windows, so the
// rule applies on every platform, and both separators split segments.
func validateNoReservedDeviceNames(rel string) error {
	for _, part := range strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '\\' }) {
		if isWindowsReservedDeviceName(part) {
			return fmt.Errorf("desktop path uses reserved device name %q", part)
		}
	}
	return nil
}

// validateWorkspaceDeviceNames checks the segments of candidate below root.
func validateWorkspaceDeviceNames(root, candidate string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve desktop root: %w", err)
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return fmt.Errorf("resolve desktop path: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return fmt.Errorf("resolve desktop relative path: %w", err)
	}
	return validateNoReservedDeviceNames(rel)
}
