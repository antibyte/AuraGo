package security

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteFileAtomicSyncedWritesWithPrivatePermissions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "tokens.json")
	if err := writeFileAtomicSynced(path, []byte("secret"), 0o600); err != nil {
		t.Fatalf("writeFileAtomicSynced: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "secret" {
		t.Fatalf("content = %q, want secret", string(data))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}

	matches, err := filepath.Glob(path + ".tmp-*")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}
}
