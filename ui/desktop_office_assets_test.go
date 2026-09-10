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

	desktopHTML := readDesktopAssetText(t, "desktop.html")
	for _, forbidden := range []string{
		`href="/css/quill.snow.css`,
		`src="/js/vendor/quill.js`,
		`src="/js/desktop/apps/writer.js`,
		`src="/js/desktop/apps/sheets.js`,
	} {
		if strings.Contains(desktopHTML, forbidden) {
			t.Fatalf("desktop.html should lazy-load office asset %q", forbidden)
		}
	}

	moduleLoader := readDesktopAssetText(t, filepath.Join("js", "desktop", "core", "module-loader.js"))
	requiredLazyAssets := []string{
		"/js/vendor/writer/engine.css",
		"/js/desktop/apps/writer-session.js",
		"/js/desktop/apps/writer-panels.js",
		"/js/desktop/apps/writer.js",
		"/js/desktop/apps/sheets.js",
	}
	for _, marker := range requiredLazyAssets {
		if !strings.Contains(moduleLoader, marker) {
			t.Fatalf("desktop lazy asset registry missing %q", marker)
		}
	}

	mainJS := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "main.js"))
	requiredMain := []string{
		"writer: 'writer'",
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
	source := readDesktopAssetText(t, "js/desktop/apps/sheets.js")
	for _, marker := range []string{"data-formula", "data-address", "commitFormula", "SelectionChanged", "pinSelection", "UniverSheetsCorePreset"} {
		if !strings.Contains(source, marker) {
			t.Fatalf("Missing editor contract %q", marker)
		}
	}
}

func TestDesktopSheetsEnhancedFeatures(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/sheets.js") + "\n" + readDesktopAssetText(t, "js/desktop/apps/sheets-panels.js")
	for _, marker := range []string{"OfficeSession.create", "UniverSheetsDataValidationPreset", "UniverSheetsConditionalFormattingPreset", "UniverSheetsNotePreset", "range.setValues(matrix)", "setColumnFilterCriteria", "moveSheet", "prepareOutput", "generateHTML()", "source_revision", "selection_size"} {
		if !strings.Contains(source, marker) {
			t.Fatalf("Missing editor contract %q", marker)
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

	writer := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "apps", "writer.js"))
	if !strings.Contains(writer, "mode:ctx.readonly?'view':undefined") {
		t.Fatal("Writer must enforce native read-only mode")
	}
	for _, app := range []string{"sheets.js"} {
		source := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "apps", app))
		for _, marker := range []string{
			"book.setEditable(!ctx.readonly)", "if(!book||loading||ctx.readonly)return", "control.disabled=true",
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

func TestDesktopWriterUsesSheetsDarkWritingSurface(t *testing.T) {
	t.Parallel()

	css := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-office.css"), "\r\n", "\n")
	writerRule := desktopOfficeCSSRuleBody(t, css, ".office-writer")
	for _, marker := range []string{
		"--vd-editor-bg: var(--vd-theme-app-bg);",
		"--vd-editor-page-bg: #ffffff;",
		"--vd-editor-text: var(--vd-text);",
		"--vd-editor-toolbar-bg: var(--vd-theme-chrome-bg);",
		"grid-template-rows: auto auto minmax(0, 1fr);",
		"background: var(--vd-editor-bg);",
		"color: var(--vd-editor-text);",
	} {
		if !strings.Contains(writerRule, marker) {
			t.Fatalf("writer dark-surface rule missing marker %q", marker)
		}
	}

	writerEditorRule := desktopOfficeCSSRuleBody(t, css, ".office-writer-editor .ql-editor")
	for _, marker := range []string{
		"background: var(--vd-editor-page-bg, #ffffff);",
		"color: #1f2937;",
	} {
		if !strings.Contains(writerEditorRule, marker) {
			t.Fatalf("writer editor dark-surface rule missing marker %q", marker)
		}
	}

	for _, marker := range []string{
		".office-sheet-grid-wrap {",
		"background: var(--vd-theme-app-bg);",
		"background: var(--vd-theme-panel-bg);",
		".office-writer .ql-stroke {",
		"stroke: var(--vd-editor-icon);",
		".office-writer .ql-fill {",
		"fill: var(--vd-editor-icon);",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("writer dark-surface styling missing marker %q", marker)
		}
	}
}

func TestVirtualDesktopConfigExposesOfficeToolToggles(t *testing.T) {
	t.Parallel()

	source := readDesktopOfficeTestFile(t, filepath.Join("cfg", "virtual_desktop.js"))
	for _, marker := range []string{
		"tools.office_document.enabled",
		"tools.office_document.readonly",
		"tools.office_workbook.enabled",
		"tools.office_workbook.readonly",
		"config.virtual_desktop.office_tools_note",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("virtual_desktop config missing office toggle marker %q", marker)
		}
	}
}

func TestVirtualDesktopConfigExposesRemoteSessionLimits(t *testing.T) {
	t.Parallel()

	source := readDesktopOfficeTestFile(t, filepath.Join("cfg", "virtual_desktop.js"))
	for _, marker := range []string{
		"remote_max_session_minutes",
		"remote_idle_timeout_minutes",
		"config.virtual_desktop.remote_max_session_label",
		"config.virtual_desktop.remote_idle_timeout_label",
		"help.virtual_desktop.remote_max_session_minutes",
		"help.virtual_desktop.remote_idle_timeout_minutes",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("virtual_desktop config missing remote session marker %q", marker)
		}
	}
}

func TestDesktopSheetsDisplaysFormulaResultsWithoutLosingSourceFormula(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/sheets.js")
	for _, marker := range []string{"range.getFormula()", "range.getRawValue()", "cell.f=active.getRange(+row,+col).getFormula()", "delete cell.si"} {
		if !strings.Contains(source, marker) {
			t.Fatalf("Missing editor contract %q", marker)
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
		"desktop.menu_agent",
		"desktop.agent_task_for_agent",
		"desktop.agent_send_to_chat",
		"desktop.agent_task_title",
		"desktop.agent_task_placeholder",
		"desktop.agent_task_prompt",
		"config.virtual_desktop.office_tools_note",
		"config.virtual_desktop.office_document_label",
		"help.virtual_desktop.office_document",
		"config.virtual_desktop.office_document_readonly_label",
		"help.virtual_desktop.office_document_readonly",
		"config.virtual_desktop.office_workbook_label",
		"help.virtual_desktop.office_workbook",
		"config.virtual_desktop.office_workbook_readonly_label",
		"help.virtual_desktop.office_workbook_readonly",
		"desktop.sheets_format_bold",
		"desktop.sheets_format_italic",
		"desktop.sheets_format_underline",
		"desktop.sheets_format_font_color",
		"desktop.sheets_format_fill_color",
		"desktop.sheets_format_align_left",
		"desktop.sheets_format_align_center",
		"desktop.sheets_format_align_right",
		"desktop.sheets_format_number",
		"desktop.sheets_format_currency",
		"desktop.sheets_format_percent",
		"desktop.sheets_format_date",
		"desktop.sheets_format_text",
		"desktop.sheets_format_borders",
		"desktop.sheets_format_border_outer",
		"desktop.sheets_format_border_all",
		"desktop.sheets_format_border_none",
		"desktop.sheets_format_border_top",
		"desktop.sheets_format_border_bottom",
		"desktop.sheets_format_border_left",
		"desktop.sheets_format_border_right",
		"desktop.sheets_search",
		"desktop.sheets_replace",
		"desktop.sheets_replace_all",
		"desktop.sheets_match_case",
		"desktop.sheets_no_matches",
		"desktop.sheets_match_count",
		"desktop.sheets_undo",
		"desktop.sheets_redo",
		"desktop.sheets_add_sheet",
		"desktop.sheets_rename_sheet",
		"desktop.sheets_delete_sheet",
		"desktop.sheets_duplicate_sheet",
		"desktop.sheets_status_sum",
		"desktop.sheets_status_count",
		"desktop.sheets_status_avg",
		"desktop.sheets_autosave",
		"desktop.sheets_dirty_indicator",
		"desktop.menu_format",
		"desktop.sheets_rename_sheet_title",
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
	if strings.HasPrefix(filepath.ToSlash(path), "js/desktop/") {
		return readDesktopAssetText(t, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
