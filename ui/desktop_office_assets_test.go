package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopOfficeAssetsAreEmbeddedAndRouted(t *testing.T) {
	t.Parallel()

	desktopHTML := readDesktopOfficeTestFile(t, "desktop.html")
	requiredHTML := []string{
		"/css/quill.snow.css",
		"/js/vendor/quill.js",
		"/js/desktop/apps/writer.js",
		"/js/desktop/apps/sheets.js",
	}
	for _, marker := range requiredHTML {
		if !strings.Contains(desktopHTML, marker) {
			t.Fatalf("desktop.html missing %q", marker)
		}
	}

	mainJS := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "main.js"))
	requiredMain := []string{
		"writer: 'documents'",
		"sheets: 'spreadsheet'",
		"window.WriterApp.render",
		"window.SheetsApp.render",
		"openApp('writer'",
		"openApp('sheets'",
		"/api/desktop/download?path=",
	}
	for _, marker := range requiredMain {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("desktop main.js missing %q", marker)
		}
	}

	fileManagerJS := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "file-manager.js"))
	if !strings.Contains(fileManagerJS, "/api/desktop/download?path=") {
		t.Fatal("file manager should use the binary-safe desktop download endpoint")
	}
}

func TestDesktopOfficeAppScriptsAvoidAlert(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		filepath.Join("js", "desktop", "apps", "writer.js"),
		filepath.Join("js", "desktop", "apps", "sheets.js"),
	} {
		content := readDesktopOfficeTestFile(t, path)
		if strings.Contains(content, "alert(") {
			t.Fatalf("%s must use desktop notifications/modals instead of alert()", path)
		}
	}
}

func TestDesktopSheetsSupportsSelectionFormulaBarAndContextMenu(t *testing.T) {
	t.Parallel()

	sheetsJS := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "apps", "sheets.js"))
	requiredJS := []string{
		"data-formula-bar",
		"data-range-name",
		"showSheetContextMenu",
		"'copy-range'",
		"'paste-range'",
		"'clear-range'",
		"'insert-row-above'",
		"'insert-col-left'",
		"applyFormulaBar",
		"office-cell-selected",
	}
	for _, marker := range requiredJS {
		if !strings.Contains(sheetsJS, marker) {
			t.Fatalf("sheets app missing spreadsheet UX marker %q", marker)
		}
	}

	desktopCSS := readDesktopOfficeTestFile(t, filepath.Join("css", "desktop.css"))
	requiredCSS := []string{
		".office-formula-bar",
		".office-cell-selected",
		".office-sheet-context-menu",
	}
	for _, marker := range requiredCSS {
		if !strings.Contains(desktopCSS, marker) {
			t.Fatalf("desktop.css missing spreadsheet UX style %q", marker)
		}
	}
}

func TestDesktopOfficeAppsRespectReadonlyMode(t *testing.T) {
	t.Parallel()

	mainJS := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "main.js"))
	for _, marker := range []string{"readonly: !!((state.bootstrap || {}).readonly)"} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("desktop main.js missing readonly propagation marker %q", marker)
		}
	}

	for _, app := range []string{"writer.js", "sheets.js"} {
		source := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "apps", app))
		for _, marker := range []string{
			"const readonly = !!ctx.readonly;",
			"applyReadonlyState",
			"if (readonly) return;",
			"disabled = readonly",
		} {
			if !strings.Contains(source, marker) {
				t.Fatalf("%s missing readonly marker %q", app, marker)
			}
		}
	}

	fileManagerJS := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "file-manager.js"))
	for _, marker := range []string{
		"function isReadonly()",
		"data-readonly=\"true\"",
		"if (isReadonly()) return;",
		"readonlyGuardItems",
	} {
		if !strings.Contains(fileManagerJS, marker) {
			t.Fatalf("file manager missing readonly marker %q", marker)
		}
	}
}

func TestDesktopSheetsDisplaysFormulaResultsWithoutLosingSourceFormula(t *testing.T) {
	t.Parallel()

	sheetsJS := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "apps", "sheets.js"))
	for _, marker := range []string{
		"function evaluateFormulaForSheet",
		"data-formula=",
		"data-display-value=",
		"cellFromInputElement(input)",
		"showFormulaForEditing(input)",
		"showFormulaResult(input)",
	} {
		if !strings.Contains(sheetsJS, marker) {
			t.Fatalf("sheets formula display missing marker %q", marker)
		}
	}
}

func TestDesktopOfficeI18NKeys(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.app_writer",
		"desktop.app_sheets",
		"desktop.writer_save",
		"desktop.writer_saved",
		"desktop.writer_download_docx",
		"desktop.writer_export_html",
		"desktop.writer_export_md",
		"desktop.writer_placeholder",
		"desktop.writer_loading",
		"desktop.writer_title_placeholder",
		"desktop.sheets_save",
		"desktop.sheets_saved",
		"desktop.sheets_download_xlsx",
		"desktop.sheets_export_csv",
		"desktop.sheets_add_row",
		"desktop.sheets_add_column",
		"desktop.sheets_loading",
		"desktop.sheets_sheet",
	}
	entries, err := os.ReadDir(filepath.Join("lang", "desktop"))
	if err != nil {
		t.Fatalf("read desktop lang dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected desktop language files")
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var data map[string]string
		raw := readDesktopOfficeTestFile(t, filepath.Join("lang", "desktop", entry.Name()))
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			t.Fatalf("%s is invalid JSON: %v", entry.Name(), err)
		}
		for _, key := range keys {
			if strings.TrimSpace(data[key]) == "" {
				t.Fatalf("%s missing non-empty key %q", entry.Name(), key)
			}
		}
	}
}

func readDesktopOfficeTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
