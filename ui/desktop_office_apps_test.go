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
	rowClamp := jsFunctionBody(t, source, "clampCellRow")
	for _, marker := range []string{"displayRowCount() - 1", "Math.min(", "Math.max("} {
		if !strings.Contains(rowClamp, marker) {
			t.Fatalf("clampCellRow missing bounded navigation marker %q", marker)
		}
	}

	colClamp := jsFunctionBody(t, source, "clampCellCol")
	for _, marker := range []string{"displayColCount() - 1", "Math.min(", "Math.max("} {
		if !strings.Contains(colClamp, marker) {
			t.Fatalf("clampCellCol missing bounded navigation marker %q", marker)
		}
	}

	const callsite = "cellInput(clampCellRow(move[0]), clampCellCol(move[1]))"
	if !strings.Contains(source, callsite) {
		t.Fatalf("sheets keyboard navigation missing marker %q", callsite)
	}
}

func TestOfficeAppsFocusExistingFileWindow(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile(filepath.Join("js", "desktop", "main.js"))
	if err != nil {
		t.Fatalf("read main.js: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"function findExistingAppWindow",
		"context && context.path != null",
		"win.context && win.context.path === context.path",
		"appId === 'writer' || appId === 'sheets'",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("office same-file dedupe missing marker %q", marker)
		}
	}
}

func jsFunctionBody(t *testing.T, source, name string) string {
	t.Helper()

	startMarker := "function " + name
	start := strings.Index(source, startMarker)
	if start < 0 {
		t.Fatalf("missing function %s", name)
	}
	openBrace := strings.Index(source[start:], "{")
	if openBrace < 0 {
		t.Fatalf("missing opening brace for function %s", name)
	}
	bodyStart := start + openBrace + 1
	closeBrace := strings.Index(source[bodyStart:], "}")
	if closeBrace < 0 {
		t.Fatalf("missing closing brace for function %s", name)
	}
	return source[bodyStart : bodyStart+closeBrace]
}
