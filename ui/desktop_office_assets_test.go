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
		"/css/quill.snow.css",
		"/js/vendor/quill.js",
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

	desktopCSS := readAllDesktopCSS(t)
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

func TestDesktopSheetsEnhancedFeatures(t *testing.T) {
	t.Parallel()

	sheetsJS := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "apps", "sheets.js"))
	requiredJS := []string{
		"window.SheetsFormulas",
		"window.SheetsFormat",
		"window.SheetsSearch",
		"undoStack",
		"redoStack",
		"isDirty",
		"autosaveTimer",
		"pushSnapshot",
		"office-status-bar",
		"office-format-bar",
		"openSearch",
		"addNewSheet",
		"renameSheetPrompt",
		"duplicateSheet",
		"deleteSheet",
	}
	for _, marker := range requiredJS {
		if !strings.Contains(sheetsJS, marker) {
			t.Fatalf("sheets enhanced feature missing marker %q", marker)
		}
	}

	formulasJS := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "apps", "sheets-formulas.js"))
	for _, marker := range []string{
		"window.SheetsFormulas",
		"evaluateFormulaForSheet",
		"parseCellRef",
		"cellName",
		"columnName",
	} {
		if !strings.Contains(formulasJS, marker) {
			t.Fatalf("sheets-formulas.js missing marker %q", marker)
		}
	}

	formatJS := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "apps", "sheets-format.js"))
	for _, marker := range []string{
		"window.SheetsFormat",
		"renderToolbar",
		"applyFormat",
		"renderFormatStyles",
		"formatDisplayValue",
		"renderBorderStyle",
	} {
		if !strings.Contains(formatJS, marker) {
			t.Fatalf("sheets-format.js missing marker %q", marker)
		}
	}

	// Verify sheets.js actually invokes the format renderer (prevents silent regression)
	for _, marker := range []string{
		"applyCellFormats",
		"formatModule.renderFormatStyles",
		"data-raw-value",
		"data-num-format",
	} {
		if !strings.Contains(sheetsJS, marker) {
			t.Fatalf("sheets.js missing format wiring marker %q", marker)
		}
	}

	searchJS := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "apps", "sheets-search.js"))
	for _, marker := range []string{
		"window.SheetsSearch",
		"openSearch",
		"closeSearch",
		"replaceAll",
	} {
		if !strings.Contains(searchJS, marker) {
			t.Fatalf("sheets-search.js missing marker %q", marker)
		}
	}

	moduleLoader := readDesktopAssetText(t, filepath.Join("js", "desktop", "core", "module-loader.js"))
	for _, marker := range []string{
		"/js/desktop/apps/sheets-formulas.js",
		"/js/desktop/apps/sheets-format.js",
		"/js/desktop/apps/sheets-search.js",
	} {
		if !strings.Contains(moduleLoader, marker) {
			t.Fatalf("module-loader.js missing sheets sub-module %q", marker)
		}
	}

	desktopCSS := readAllDesktopCSS(t)
	requiredCSS := []string{
		".office-format-toolbar",
		".office-search-overlay",
		".office-status-bar",
		".office-color-picker",
		".office-cell-search-match",
		".office-sheet-add-btn",
	}
	for _, marker := range requiredCSS {
		if !strings.Contains(desktopCSS, marker) {
			t.Fatalf("desktop.css missing enhanced sheets style %q", marker)
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

func TestDesktopWriterUsesSheetsDarkWritingSurface(t *testing.T) {
	t.Parallel()

	css := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-office.css"), "\r\n", "\n")
	writerRule := desktopOfficeCSSRuleBody(t, css, ".office-writer")
	for _, marker := range []string{
		"--vd-editor-bg: var(--ds-color-bg-raised, #181f2c);",
		"--vd-editor-text: var(--ds-color-fg-primary, #f6f7fb);",
		"--vd-editor-toolbar-bg:",
		"grid-template-rows: auto auto minmax(0, 1fr);",
		"background: var(--vd-editor-bg);",
		"color: var(--vd-editor-text);",
	} {
		if !strings.Contains(writerRule, marker) {
			t.Fatalf("writer dark-surface rule missing marker %q", marker)
		}
	}

	writerEditorRule := desktopOfficeCSSRuleBody(t, css, ".office-writer-editor")
	for _, marker := range []string{
		"background: var(--vd-editor-bg);",
		"color: var(--vd-editor-text);",
	} {
		if !strings.Contains(writerEditorRule, marker) {
			t.Fatalf("writer editor dark-surface rule missing marker %q", marker)
		}
	}

	for _, marker := range []string{
		".office-sheet-grid-wrap {",
		"background: var(--ds-color-bg-raised, #181f2c);",
		"background: var(--ds-color-bg-overlay, #1d2533);",
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

	formulasJS := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "apps", "sheets-formulas.js"))
	for _, marker := range []string{
		"function evaluateFormulaForSheet",
	} {
		if !strings.Contains(formulasJS, marker) {
			t.Fatalf("sheets-formulas.js missing formula marker %q", marker)
		}
	}

	sheetsJS := readDesktopOfficeTestFile(t, filepath.Join("js", "desktop", "apps", "sheets.js"))
	for _, marker := range []string{
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
