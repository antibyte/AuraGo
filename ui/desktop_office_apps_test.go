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
		"appId === 'editor' || appId === 'writer' || appId === 'sheets'",
		"if (appId === 'editor' && context && context.path != null) renderEditor(existing.id, context.path, context.content || '');",
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

func TestOfficeAppsExposeNewAndSaveAsMenus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		file    string
		markers []string
	}{
		{
			name: "writer",
			file: "writer.js",
			markers: []string{
				"function newDocument()",
				"function saveAs()",
				"id: 'new-document'",
				"labelKey: 'desktop.writer_new'",
				"id: 'save-as'",
				"labelKey: 'desktop.writer_save_as'",
				"ctx.promptDialog",
			},
		},
		{
			name: "sheets",
			file: "sheets.js",
			markers: []string{
				"function newWorkbook()",
				"function saveAs()",
				"id: 'new-workbook'",
				"labelKey: 'desktop.sheets_new'",
				"id: 'save-as'",
				"labelKey: 'desktop.sheets_save_as'",
				"ctx.promptDialog",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceBytes, err := os.ReadFile(filepath.Join("js", "desktop", "apps", tt.file))
			if err != nil {
				t.Fatalf("read %s: %v", tt.file, err)
			}
			source := string(sourceBytes)
			for _, marker := range tt.markers {
				if !strings.Contains(source, marker) {
					t.Fatalf("%s missing menu marker %q", tt.file, marker)
				}
			}
		})
	}
}

func TestEditorWriterAndSheetsExposeAgentMenus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		markers []string
	}{
		{
			name: "editor",
			path: "js/desktop/apps/settings-calculator.js",
			markers: []string{
				"id: 'agent'",
				"labelKey: 'desktop.menu_agent'",
				"id: 'agent-task'",
				"labelKey: 'desktop.agent_task_for_agent'",
				"id: 'agent-send-chat'",
				"labelKey: 'desktop.agent_send_to_chat'",
				"openAgentChatForFile",
				"await saveEditor()",
			},
		},
		{
			name: "writer",
			path: "js/desktop/apps/writer.js",
			markers: []string{
				"id: 'agent'",
				"labelKey: 'desktop.menu_agent'",
				"id: 'agent-task'",
				"labelKey: 'desktop.agent_task_for_agent'",
				"id: 'agent-send-chat'",
				"labelKey: 'desktop.agent_send_to_chat'",
				"ctx.openAgentChatForFile",
				"await save()",
			},
		},
		{
			name: "sheets",
			path: "js/desktop/apps/sheets.js",
			markers: []string{
				"id: 'agent'",
				"labelKey: 'desktop.menu_agent'",
				"id: 'agent-task'",
				"labelKey: 'desktop.agent_task_for_agent'",
				"id: 'agent-send-chat'",
				"labelKey: 'desktop.agent_send_to_chat'",
				"ctx.openAgentChatForFile",
				"await save()",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := readDesktopAssetText(t, tt.path)
			for _, marker := range tt.markers {
				if !strings.Contains(source, marker) {
					t.Fatalf("%s missing Agent menu marker %q", tt.path, marker)
				}
			}
		})
	}
}

func TestDesktopEditorFillsWindowContent(t *testing.T) {
	t.Parallel()

	css := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-office.css"), "\r\n", "\n")
	editorRule := desktopOfficeCSSRuleBody(t, css, ".vd-editor")
	for _, marker := range []string{
		"grid-template-rows: auto minmax(0, 1fr);",
		"height: 100%;",
		"min-height: 0;",
		"min-width: 0;",
	} {
		if !strings.Contains(editorRule, marker) {
			t.Fatalf("editor root layout rule missing marker %q", marker)
		}
	}

	textareaRule := desktopOfficeCSSRuleBody(t, css, ".vd-editor textarea")
	for _, marker := range []string{
		"width: 100%;",
		"height: 100%;",
		"box-sizing: border-box;",
		"overflow: auto;",
	} {
		if !strings.Contains(textareaRule, marker) {
			t.Fatalf("editor textarea layout rule missing marker %q", marker)
		}
	}
}

func TestDesktopEditorLazyLoadsOfficeStyles(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	for _, marker := range []string{
		"'editor': {",
		"styles: appStyles('/css/desktop-app-office.css')",
	} {
		if !strings.Contains(loader, marker) {
			t.Fatalf("desktop editor asset registry missing marker %q", marker)
		}
	}
}

func TestDesktopAgentLaunchContextPreservesSourceApp(t *testing.T) {
	t.Parallel()

	editorSource := readDesktopAssetText(t, "js/desktop/apps/settings-calculator.js")
	for _, marker := range []string{
		"sourceApp: 'editor'",
		"chat_source_app = sourceApp",
	} {
		if !strings.Contains(editorSource, marker) {
			t.Fatalf("editor Agent launch context missing marker %q", marker)
		}
	}

	chatSource := readDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	for _, marker := range []string{
		"host.dataset.chatSourceApp",
		"payload.origin_app = sourceApp",
	} {
		if !strings.Contains(chatSource, marker) {
			t.Fatalf("desktop chat launch context missing marker %q", marker)
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

func desktopOfficeCSSRuleBody(t *testing.T, source, selector string) string {
	t.Helper()
	start := strings.Index(source, selector+" {")
	if start < 0 {
		t.Fatalf("desktop office CSS missing selector %q", selector)
	}
	open := strings.Index(source[start:], "{")
	if open < 0 {
		t.Fatalf("desktop office CSS selector %q is missing opening brace", selector)
	}
	bodyStart := start + open + 1
	close := strings.Index(source[bodyStart:], "}")
	if close < 0 {
		t.Fatalf("desktop office CSS selector %q is missing closing brace", selector)
	}
	return source[bodyStart : bodyStart+close]
}
