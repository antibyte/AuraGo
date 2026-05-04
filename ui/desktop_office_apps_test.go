package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSheetsKeyboardNavigationIsBounded(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "sheets.js"))
	if err != nil {
		t.Fatalf("read sheets.js: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"function clampCellRow",
		"function clampCellCol",
		"cellInput(clampCellRow(move[0]), clampCellCol(move[1]))",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("sheets keyboard navigation missing marker %q", marker)
		}
	}
}
