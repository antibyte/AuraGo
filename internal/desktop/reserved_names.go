package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
// segment is a Windows device name; both separators split segments.
func validateNoReservedDeviceNames(rel string) error {
	for _, part := range strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '\\' }) {
		if isWindowsReservedDeviceName(part) {
			return fmt.Errorf("desktop path uses reserved device name %q", part)
		}
	}
	return nil
}

// desktopDevicePlatform is the GOOS the device-name rule applies for; tests
// override it to exercise both platforms.
var desktopDevicePlatform = runtime.GOOS

// validateWorkspaceDeviceNames checks the segments of candidate below root.
// On Windows such paths are devices, and existence checks are unreliable, so
// they are always refused. Elsewhere an existing entry stays readable,
// writable and deletable; only creating a new entry (the target does not exist)
// and move/copy/rename/symlink destinations (destination true) are refused.
func validateWorkspaceDeviceNames(root, candidate string, destination bool) error {
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
	if desktopDevicePlatform != "windows" && !destination {
		if _, statErr := os.Lstat(candidateAbs); statErr == nil {
			return nil
		}
	}
	return validateNoReservedDeviceNames(rel)
}
