package ui

import (
	"io/fs"
	"sort"
	"strings"
	"testing"
)

// readDashboardJS returns every ui/js/dashboard/**/*.js file, sorted by path, with LF line endings, as one string.
// Presence and count pins read this instead of a single file, so moving code between dashboard scripts (the P4
// section modules) keeps them meaningful; pins that must hold within one file read that file.
func readDashboardJS(t *testing.T) string {
	t.Helper()
	var files []string
	if err := fs.WalkDir(Content, "js/dashboard", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".js") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk js/dashboard: %v", err)
	}
	sort.Strings(files)
	var b strings.Builder
	for _, file := range files {
		data, err := fs.ReadFile(Content, file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		b.WriteString(normalizeAssetText(data))
		b.WriteString("\n")
	}
	return b.String()
}
