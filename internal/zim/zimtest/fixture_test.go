package zimtest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRemoveStaleTempFiles(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-time.Hour)
	for name, mod := range map[string]time.Time{
		".zim-fixture-stale":     old,
		".zim-fixture-running":   time.Now(),
		RealFixtureName:          old, // the published fixture is never a temp file
		".zim-fixture-other.txt": old,
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	removeStaleTempFiles(dir)
	for name, wantKept := range map[string]bool{
		".zim-fixture-stale":     false,
		".zim-fixture-other.txt": false, // matches the temp pattern and is old
		".zim-fixture-running":   true,
		RealFixtureName:          true,
	} {
		_, err := os.Stat(filepath.Join(dir, name))
		if kept := err == nil; kept != wantKept {
			t.Errorf("%s kept = %v, want %v", name, kept, wantKept)
		}
	}
}
