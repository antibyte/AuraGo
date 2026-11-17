//go:build !windows

package dbutil

import (
	"fmt"
	"os"
)

func migrationBackupIsPrivate(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("migration backup grants access to other users")
	}
	return nil
}
