package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestServerFileMountsNeverUseHTTPDir keeps internal/server's static file
// mounts root-bound: http.Dir follows symlinks out of the served
// directory, so production code serves files through
// neuteredFileSystem{rootBoundFileSystem(dir)} instead.
func TestServerFileMountsNeverUseHTTPDir(t *testing.T) {
	root := filepath.Join("..", "server")
	var offenders []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "http.Dir(") {
			offenders = append(offenders, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(offenders) > 0 {
		t.Fatalf("serve files through rootBoundFileSystem, not http.Dir: %s", strings.Join(offenders, ", "))
	}
}
