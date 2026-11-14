//go:build !windows

package dbutil

import (
	"os"
	"testing"
)

func assertBackupPrivate(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("backup permissions: %v %v", info, err)
	}
}
