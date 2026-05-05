package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
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

func TestExecuteVirtualDesktopOfficeConvenienceOperations(t *testing.T) {
	t.Parallel()

	cfg := testVirtualDesktopConfig(t)
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
