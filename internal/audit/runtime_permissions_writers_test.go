package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Agent runs narrow tool gates through tools.WithRuntimePermissions. Only the
// process owner (startup and the server's config publication) may write the
// process-wide fallback snapshot.
func TestRuntimePermissionsAreWrittenOnlyByStartupAndServer(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	allowedFiles := map[string]bool{
		"cmd/aurago/main.go":            true, // startup fallback before the server binds its resolver
		"internal/tools/permissions.go": true, // definition
	}

	var offenders []string
	for _, root := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, root), func(path string, d os.DirEntry, err error) error {
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
			if !strings.Contains(string(data), "ConfigureRuntimePermissions(") {
				return nil
			}
			rel, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if allowedFiles[rel] || strings.HasPrefix(rel, "internal/server/") {
				return nil
			}
			offenders = append(offenders, rel)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("ConfigureRuntimePermissions may only be called from cmd/aurago and internal/server; agent runs must use tools.WithRuntimePermissions: %s", strings.Join(offenders, ", "))
	}
}
