package server

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// readDashboardMarkupForTest returns ui/dashboard.html followed by every section partial in
// ui/dashboard/sections (sorted), with LF line endings.
func readDashboardMarkupForTest(t *testing.T) string {
	t.Helper()
	ui := filepath.Join("..", "..", "ui")
	shell, err := os.ReadFile(filepath.Join(ui, "dashboard.html"))
	if err != nil {
		t.Fatal(err)
	}
	partials, err := filepath.Glob(filepath.Join(ui, "dashboard", "sections", "*.html"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(partials)
	var b strings.Builder
	b.Write(shell)
	for _, path := range partials {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		b.WriteString("\n")
		b.Write(data)
	}
	return strings.ReplaceAll(b.String(), "\r\n", "\n")
}
