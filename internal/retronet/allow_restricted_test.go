package retronet

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// allowRestrictedUse matches a struct literal field ("AllowRestricted: true")
// and an assignment ("d.AllowRestricted = true"). The field declaration in
// dial.go ("AllowRestricted bool") and reads ("!d.AllowRestricted") do not match.
var allowRestrictedUse = regexp.MustCompile(`AllowRestricted\s*:|\.AllowRestricted\s*=[^=]`)

// moduleRoot returns the directory holding go.mod, searching upwards from the
// test's working directory (the package directory).
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

// TestAllowRestrictedOnlyInTests guards Dialer.AllowRestricted, which skips
// the public-address check: production code must never switch it on. Test
// files may; dial.go only declares and reads the field.
func TestAllowRestrictedOnlyInTests(t *testing.T) {
	root := moduleRoot(t)
	skipDirs := map[string]bool{".worktrees": true, "node_modules": true, "vendor": true, "disposable": true, ".git": true}
	scanned, sawDial := 0, false
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil // no such top-level directory in this checkout
				}
				return err
			}
			if d.IsDir() {
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanned++
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if rel == "internal/retronet/dial.go" {
				sawDial = true
			}
			if !bytes.Contains(data, []byte("AllowRestricted")) {
				return nil
			}
			if loc := allowRestrictedUse.FindIndex(data); loc != nil {
				line := 1 + bytes.Count(data[:loc[0]], []byte("\n"))
				t.Errorf("%s:%d sets AllowRestricted outside a test file", rel, line)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", top, err)
		}
	}
	if !sawDial || scanned < 50 {
		t.Fatalf("scanned %d files (dial.go seen: %v); the walk does not cover the module", scanned, sawDial)
	}
}
