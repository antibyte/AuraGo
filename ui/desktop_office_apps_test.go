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

	writerBytes, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "writer.js"))
	if err != nil {
		t.Fatalf("read writer.js: %v", err)
	}
	sheetsBytes, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "sheets.js"))
	if err != nil {
		t.Fatalf("read sheets.js: %v", err)
	}

	source := readDesktopAssetText(t, "js/desktop/main.js") + "\n" + string(writerBytes) + "\n" + string(sheetsBytes)
	for _, marker := range []string{
		"function findExistingAppWindow",
		"function normalizeDesktopPath",
		"function updateWindowContext",
		"context && context.path != null",
		"normalizeDesktopPath(context.path)",
		"win.context && normalizeDesktopPath(win.context.path) === requestedPath",
		"appId === 'writer' || appId === 'sheets'",
		"context: windowContext",
		"updateWindowContext: updateWindowContext",
		"ctx.updateWindowContext(windowId, { path: currentPath })",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("office same-file dedupe missing marker %q", marker)
		}
	}
}

func TestOfficeAppsSendOptimisticVersion(t *testing.T) {
	t.Parallel()

	for _, app := range []string{"writer.js", "sheets.js"} {
		sourceBytes, err := os.ReadFile(filepath.Join("js", "desktop", "apps", app))
		if err != nil {
			t.Fatalf("read %s: %v", app, err)
		}
		source := string(sourceBytes)
		for _, marker := range []string{
			"let officeVersion = null;",
			"officeVersion = body.office_version || null;",
			"office_version: officeVersion",
			"officeVersion = body.office_version || officeVersion;",
		} {
			if !strings.Contains(source, marker) {
				t.Fatalf("%s missing optimistic office version marker %q", app, marker)
			}
		}
	}
}

func TestSheetsFormulaStateUsesSingleSetter(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "sheets.js"))
	if err != nil {
		t.Fatalf("read sheets.js: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"function setCellFromInput(input, raw)",
		"raw.startsWith('=') ? { formula: raw.slice(1) } : { value: raw }",
		"setCellFromInput(input, formulaInput.value);",
		"setCellFromInput(input, raw);",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("sheets formula state missing marker %q", marker)
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
