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
