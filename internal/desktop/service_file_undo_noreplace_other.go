//go:build !linux && !darwin && !windows

package desktop

import (
	"fmt"
	"os"
)

func renameDesktopPathNoReplace(_ *os.Root, _, _ string, _ *os.File) error {
	return fmt.Errorf("atomic no-replace rename is unsupported on this platform")
}
