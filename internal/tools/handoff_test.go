package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteSensitiveMaintenanceFileUsesOwnerOnlyPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current_plan.md")
	if err := writeSensitiveMaintenanceFile(path, []byte("secret plan")); err != nil {
		t.Fatalf("writeSensitiveMaintenanceFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if runtime.GOOS == "windows" {
		if info.Size() != int64(len("secret plan")) {
			t.Fatalf("size = %d, want %d", info.Size(), len("secret plan"))
		}
		return
	}
	if perms := info.Mode().Perm(); perms != 0o600 {
		t.Fatalf("permissions = %04o, want 0600", perms)
	}
}
