package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestExecuteOfficeDocumentWritePatchReadExport(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeDocument.Enabled = true
	write := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation": "write",
		"path":      "Documents/report.docx",
		"title":     "Report",
		"content":   "Hello Writer",
	})
	var writePayload struct {
		Status string `json:"status"`
		Data   struct {
			Path          string                 `json:"path"`
			OfficeVersion map[string]interface{} `json:"office_version"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(write.Output), &writePayload); err != nil {
		t.Fatalf("decode write payload: %v output=%s", err, write.Output)
	}
	if writePayload.Status != "ok" || writePayload.Data.Path != "Documents/report.docx" || writePayload.Data.OfficeVersion["etag"] == "" {
		t.Fatalf("write payload = %+v output=%s", writePayload, write.Output)
	}

	patch := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation":    "patch",
		"path":         "Documents/report.docx",
		"prepend_text": "Draft\n",
		"append_text":  "\nDone",
		"replacements": []interface{}{
			map[string]interface{}{"find": "Writer", "replace": "Agent"},
		},
	})
	var patchPayload struct {
		Status string `json:"status"`
		Data   struct {
			Document struct {
				Text string `json:"text"`
			} `json:"document"`
			OfficeVersion map[string]interface{} `json:"office_version"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(patch.Output), &patchPayload); err != nil {
		t.Fatalf("decode patch payload: %v output=%s", err, patch.Output)
	}
	if patchPayload.Status != "ok" || patchPayload.Data.Document.Text != "Draft\nHello Agent\nDone" || patchPayload.Data.OfficeVersion["etag"] == "" {
		t.Fatalf("patch payload = %+v output=%s", patchPayload, patch.Output)
	}

	read := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation": "read",
		"path":      "Documents/report.docx",
	})
	var readPayload struct {
		Status string `json:"status"`
		Data   struct {
			Document struct {
				Text string `json:"text"`
			} `json:"document"`
			OfficeVersion map[string]interface{} `json:"office_version"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(read.Output), &readPayload); err != nil {
		t.Fatalf("decode read payload: %v output=%s", err, read.Output)
	}
	if readPayload.Status != "ok" || readPayload.Data.Document.Text != "Draft\nHello Agent\nDone" || readPayload.Data.OfficeVersion["etag"] == "" {
		t.Fatalf("read payload = %+v output=%s", readPayload, read.Output)
	}

	exported := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation":   "export",
		"path":        "Documents/report.docx",
		"output_path": "Documents/report.md",
		"format":      "md",
	})
	var exportPayload struct {
		Status string `json:"status"`
		Data   struct {
			OutputPath string `json:"output_path"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(exported.Output), &exportPayload); err != nil {
		t.Fatalf("decode export payload: %v output=%s", err, exported.Output)
	}
	if exportPayload.Status != "ok" || exportPayload.Data.OutputPath != "Documents/report.md" {
		t.Fatalf("export payload = %+v output=%s", exportPayload, exported.Output)
	}
}

func TestExecuteOfficeWorkbookSetRangeEvaluateAndExport(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeWorkbook.Enabled = true
	setRange := ExecuteOfficeWorkbook(context.Background(), cfg, map[string]interface{}{
		"operation":  "set_range",
		"path":       "Documents/budget.xlsx",
		"sheet":      "Budget",
		"start_cell": "A1",
		"values": []interface{}{
			[]interface{}{"Item", "Amount"},
			[]interface{}{"Coffee", "12.50"},
			[]interface{}{"Tea", "7.50"},
			[]interface{}{"Total", map[string]interface{}{"formula": "SUM(B2:B3)"}},
		},
	})
	var setRangePayload struct {
		Status string `json:"status"`
		Data   struct {
			Workbook struct {
				Sheets []struct {
					Rows [][]struct {
						Value   string `json:"value"`
						Formula string `json:"formula"`
					} `json:"rows"`
				} `json:"sheets"`
			} `json:"workbook"`
			OfficeVersion map[string]interface{} `json:"office_version"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(setRange.Output), &setRangePayload); err != nil {
		t.Fatalf("decode set_range payload: %v output=%s", err, setRange.Output)
	}
	if setRangePayload.Status != "ok" || setRangePayload.Data.Workbook.Sheets[0].Rows[3][1].Formula != "SUM(B2:B3)" || setRangePayload.Data.OfficeVersion["etag"] == "" {
		t.Fatalf("set_range payload = %+v output=%s", setRangePayload, setRange.Output)
	}

	evaluate := ExecuteOfficeWorkbook(context.Background(), cfg, map[string]interface{}{
		"operation": "evaluate_formula",
		"path":      "Documents/budget.xlsx",
		"sheet":     "Budget",
		"formula":   "SUM(B2:B3)",
	})
	var evaluatePayload struct {
		Status string `json:"status"`
		Data   struct {
			Result string `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(evaluate.Output), &evaluatePayload); err != nil {
		t.Fatalf("decode evaluate payload: %v output=%s", err, evaluate.Output)
	}
	if evaluatePayload.Status != "ok" || evaluatePayload.Data.Result != "20" {
		t.Fatalf("evaluate payload = %+v output=%s", evaluatePayload, evaluate.Output)
	}

	exported := ExecuteOfficeWorkbook(context.Background(), cfg, map[string]interface{}{
		"operation":   "export",
		"path":        "Documents/budget.xlsx",
		"output_path": "Documents/budget.csv",
		"format":      "csv",
		"sheet":       "Budget",
	})
	if !strings.Contains(exported.Output, `"status":"ok"`) {
		t.Fatalf("export output=%s", exported.Output)
	}
}

func TestExecuteOfficeToolsRespectVirtualDesktopReadOnly(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeDocument.Enabled = true
	cfg.VirtualDesktop.ReadOnly = true
	exec := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation": "write",
		"path":      "Documents/blocked.docx",
		"content":   "blocked",
	})
	if !strings.Contains(exec.Output, `"status":"error"`) || !strings.Contains(exec.Output, "read-only") {
		t.Fatalf("expected read-only error, got %s", exec.Output)
	}
}

func TestExecuteVirtualDesktopOfficeConvenienceRespectsOfficeToolToggles(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)

	docExec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "patch_document",
		"path":      "Documents/blocked.md",
		"content":   "blocked",
	})
	if !strings.Contains(docExec.Output, `"status":"error"`) || !strings.Contains(docExec.Output, "office_document tool is disabled") {
		t.Fatalf("patch_document should be blocked by office_document toggle, got %s", docExec.Output)
	}

	workbookExec := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "set_cell",
		"path":      "Documents/blocked.xlsx",
		"cell":      "A1",
		"value":     "blocked",
	})
	if !strings.Contains(workbookExec.Output, `"status":"error"`) || !strings.Contains(workbookExec.Output, "office_workbook tool is disabled") {
		t.Fatalf("set_cell should be blocked by office_workbook toggle, got %s", workbookExec.Output)
	}
}

func TestExecuteOfficeDocumentReadOnlyPolicy(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeDocument.Enabled = true
	write := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation": "write",
		"path":      "Documents/readonly.docx",
		"content":   "Read me",
	})
	if !strings.Contains(write.Output, `"status":"ok"`) {
		t.Fatalf("seed write output=%s", write.Output)
	}

	cfg.Tools.OfficeDocument.ReadOnly = true
	read := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation": "read",
		"path":      "Documents/readonly.docx",
	})
	if !strings.Contains(read.Output, `"status":"ok"`) || !strings.Contains(read.Output, "Read me") {
		t.Fatalf("read should be allowed in readonly mode, got %s", read.Output)
	}

	for _, tc := range []map[string]interface{}{
		{"operation": "write", "path": "Documents/blocked.docx", "content": "blocked"},
		{"operation": "patch", "path": "Documents/readonly.docx", "append_text": "blocked"},
		{"operation": "export", "path": "Documents/readonly.docx", "output_path": "Documents/blocked.md", "format": "md"},
	} {
		exec := ExecuteOfficeDocument(context.Background(), cfg, tc)
		if !strings.Contains(exec.Output, `"status":"error"`) || !strings.Contains(exec.Output, "office_document tool is in read-only mode") {
			t.Fatalf("%v should be blocked in readonly mode, got %s", tc["operation"], exec.Output)
		}
	}
}

func TestExecuteOfficeWorkbookReadOnlyPolicy(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeWorkbook.Enabled = true
	setRange := ExecuteOfficeWorkbook(context.Background(), cfg, map[string]interface{}{
		"operation":  "set_range",
		"path":       "Documents/readonly.xlsx",
		"sheet":      "Budget",
		"start_cell": "A1",
		"values": []interface{}{
			[]interface{}{"Item", "Amount"},
			[]interface{}{"Coffee", "12.50"},
			[]interface{}{"Total", map[string]interface{}{"formula": "SUM(B2:B2)"}},
		},
	})
	if !strings.Contains(setRange.Output, `"status":"ok"`) {
		t.Fatalf("seed set_range output=%s", setRange.Output)
	}

	cfg.Tools.OfficeWorkbook.ReadOnly = true
	read := ExecuteOfficeWorkbook(context.Background(), cfg, map[string]interface{}{
		"operation": "read",
		"path":      "Documents/readonly.xlsx",
	})
	if !strings.Contains(read.Output, `"status":"ok"`) || !strings.Contains(read.Output, "Coffee") {
		t.Fatalf("read should be allowed in readonly mode, got %s", read.Output)
	}
	evaluate := ExecuteOfficeWorkbook(context.Background(), cfg, map[string]interface{}{
		"operation": "evaluate_formula",
		"path":      "Documents/readonly.xlsx",
		"sheet":     "Budget",
		"formula":   "SUM(B2:B2)",
	})
	if !strings.Contains(evaluate.Output, `"status":"ok"`) || !strings.Contains(evaluate.Output, `"result":"12.5"`) {
		t.Fatalf("evaluate should be allowed in readonly mode, got %s", evaluate.Output)
	}

	for _, tc := range []map[string]interface{}{
		{"operation": "write", "path": "Documents/blocked.xlsx", "workbook": map[string]interface{}{"sheets": []interface{}{map[string]interface{}{"name": "Sheet1"}}}},
		{"operation": "set_cell", "path": "Documents/readonly.xlsx", "cell": "A1", "value": "blocked"},
		{"operation": "set_range", "path": "Documents/readonly.xlsx", "start_cell": "A1", "values": []interface{}{[]interface{}{"blocked"}}},
		{"operation": "export", "path": "Documents/readonly.xlsx", "output_path": "Documents/blocked.csv", "format": "csv"},
	} {
		exec := ExecuteOfficeWorkbook(context.Background(), cfg, tc)
		if !strings.Contains(exec.Output, `"status":"error"`) || !strings.Contains(exec.Output, "office_workbook tool is in read-only mode") {
			t.Fatalf("%v should be blocked in readonly mode, got %s", tc["operation"], exec.Output)
		}
	}
}

func TestExecuteOfficeDocumentPatchKeepsHTMLRepresentation(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeDocument.Enabled = true

	write := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation": "write",
		"path":      "Documents/formatted.html",
		"title":     "Formatted",
		"html":      "<!doctype html><body><h1>Formatted</h1><p>Hello <strong>World</strong></p></body>",
	})
	if !strings.Contains(write.Output, `"status":"ok"`) {
		t.Fatalf("write output=%s", write.Output)
	}

	patch := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation":    "patch",
		"path":         "Documents/formatted.html",
		"replacements": []interface{}{map[string]interface{}{"find": "World", "replace": "Agent"}},
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Document struct {
				Text string `json:"text"`
				HTML string `json:"html"`
			} `json:"document"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(patch.Output), &payload); err != nil {
		t.Fatalf("decode patch payload: %v output=%s", err, patch.Output)
	}
	if payload.Status != "ok" || !strings.Contains(payload.Data.Document.Text, "Agent") || strings.Contains(payload.Data.Document.Text, "World") {
		t.Fatalf("patch text payload = %+v output=%s", payload, patch.Output)
	}
	if !strings.Contains(payload.Data.Document.HTML, "Agent") || strings.TrimSpace(payload.Data.Document.HTML) == "" {
		t.Fatalf("patch should keep non-empty patched HTML, got %q", payload.Data.Document.HTML)
	}
}

func TestExecuteOfficeDocumentPatchEscapesHTMLReplacement(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeDocument.Enabled = true

	write := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation": "write",
		"path":      "Documents/escaped.html",
		"html":      "<!doctype html><body><p>Hello World</p></body>",
	})
	if !strings.Contains(write.Output, `"status":"ok"`) {
		t.Fatalf("write output=%s", write.Output)
	}

	patch := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation":    "patch",
		"path":         "Documents/escaped.html",
		"replacements": []interface{}{map[string]interface{}{"find": "World", "replace": `<script>alert("x")</script>`}},
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Document struct {
				HTML string `json:"html"`
			} `json:"document"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(patch.Output), &payload); err != nil {
		t.Fatalf("decode patch payload: %v output=%s", err, patch.Output)
	}
	if payload.Status != "ok" {
		t.Fatalf("patch output=%s", patch.Output)
	}
	if strings.Contains(payload.Data.Document.HTML, "<script>") || !strings.Contains(payload.Data.Document.HTML, "&lt;script&gt;") {
		t.Fatalf("replacement should be escaped in HTML, got %q", payload.Data.Document.HTML)
	}
}

func TestExecuteOfficeDocumentPatchIgnoresNonStringAppendText(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeDocument.Enabled = true
	patch := ExecuteOfficeDocument(context.Background(), cfg, map[string]interface{}{
		"operation":   "patch",
		"path":        "Documents/numeric.md",
		"content":     "Base",
		"append_text": 123,
	})
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Document struct {
				Text string `json:"text"`
			} `json:"document"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(patch.Output), &payload); err != nil {
		t.Fatalf("decode patch payload: %v output=%s", err, patch.Output)
	}
	if payload.Status != "ok" || payload.Data.Document.Text != "Base" {
		t.Fatalf("numeric append_text should be ignored, got %+v output=%s", payload, patch.Output)
	}
}

func TestExecuteOfficeWorkbookSetCellDoesNotOverwriteUnreadableWorkbook(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeWorkbook.Enabled = true

	svc, err := officeToolService(context.Background(), cfg)
	if err != nil {
		t.Fatalf("officeToolService: %v", err)
	}
	defer svc.Close()
	if err := svc.WriteFileBytes(context.Background(), "Documents/corrupt.xlsx", []byte("not an xlsx"), desktop.SourceAgent); err != nil {
		t.Fatalf("seed corrupt workbook: %v", err)
	}

	exec := ExecuteOfficeWorkbook(context.Background(), cfg, map[string]interface{}{
		"operation": "set_cell",
		"path":      "Documents/corrupt.xlsx",
		"cell":      "A1",
		"value":     "new",
	})
	if !strings.Contains(exec.Output, `"status":"error"`) {
		t.Fatalf("expected corrupt workbook mutation to fail, got %s", exec.Output)
	}
}

func TestExecuteOfficeWorkbookExportDefaultsToXLSXWhenTargetHasNoExtension(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeWorkbook.Enabled = true
	write := ExecuteOfficeWorkbook(context.Background(), cfg, map[string]interface{}{
		"operation": "set_cell",
		"path":      "Documents/budget.xlsx",
		"cell":      "A1",
		"value":     "Amount",
	})
	if !strings.Contains(write.Output, `"status":"ok"`) {
		t.Fatalf("write output=%s", write.Output)
	}

	exported := ExecuteOfficeWorkbook(context.Background(), cfg, map[string]interface{}{
		"operation":   "export",
		"path":        "Documents/budget.xlsx",
		"output_path": "Documents/budget-copy",
	})
	if !strings.Contains(exported.Output, `"status":"ok"`) {
		t.Fatalf("export should default to xlsx, got %s", exported.Output)
	}
}

func TestExecuteVirtualDesktopOfficeConvenienceOperations(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
	cfg.Tools.OfficeDocument.Enabled = true
	cfg.Tools.OfficeWorkbook.Enabled = true
	patch := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation":    "patch_document",
		"path":         "Documents/notes.md",
		"content":      "Hello World",
		"replacements": []interface{}{map[string]interface{}{"find": "World", "replace": "Agent"}},
	})
	if !strings.Contains(patch.Output, `"status":"ok"`) || !strings.Contains(patch.Output, "Hello Agent") {
		t.Fatalf("patch_document output=%s", patch.Output)
	}

	setRange := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation":  "set_range",
		"path":       "Documents/range.xlsx",
		"sheet":      "Sheet1",
		"start_cell": "A1",
		"values":     []interface{}{[]interface{}{"A", "B"}, []interface{}{"1", "2"}},
	})
	if !strings.Contains(setRange.Output, `"status":"ok"`) {
		t.Fatalf("set_range output=%s", setRange.Output)
	}

	evaluate := ExecuteVirtualDesktop(context.Background(), cfg, map[string]interface{}{
		"operation": "evaluate_formula",
		"path":      "Documents/range.xlsx",
		"sheet":     "Sheet1",
		"formula":   "SUM(A2:B2)",
	})
	if !strings.Contains(evaluate.Output, `"result":"3"`) {
		t.Fatalf("evaluate_formula output=%s", evaluate.Output)
	}
}
